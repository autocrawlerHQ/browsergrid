package deployments

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/autocrawlerHQ/browsergrid/internal/sessions"
)

// SessionsStore interface for session operations
type SessionsStore interface {
	CreateSession(ctx context.Context, session *sessions.Session) error
	GetSession(ctx context.Context, id uuid.UUID) (*sessions.Session, error)
}

// DeploymentRunner handles the execution of deployment packages
type DeploymentRunner struct {
	store     *Store
	sessStore SessionsStore
	workDir   string
}

// NewDeploymentRunner creates a new deployment runner
func NewDeploymentRunner(db *gorm.DB, workDir string) *DeploymentRunner {
	if workDir == "" {
		workDir = "/tmp/deployments"
	}

	return &DeploymentRunner{
		store:     NewStore(db),
		sessStore: sessions.NewStore(db),
		workDir:   workDir,
	}
}

// ExecuteDeployment executes a deployment run
func (r *DeploymentRunner) ExecuteDeployment(ctx context.Context, runID uuid.UUID) error {
	// Get the deployment run
	run, err := r.store.GetDeploymentRun(ctx, runID)
	if err != nil {
		return fmt.Errorf("failed to get deployment run: %w", err)
	}

	// Get the deployment
	deployment, err := r.store.GetDeployment(ctx, run.DeploymentID)
	if err != nil {
		return fmt.Errorf("failed to get deployment: %w", err)
	}

	// Update run status to running
	if err := r.store.UpdateDeploymentRunStatus(ctx, runID, RunStatusRunning); err != nil {
		return fmt.Errorf("failed to update run status: %w", err)
	}

	// Create browser session if needed
	sessionID, err := r.createBrowserSession(ctx, deployment, run)
	if err != nil {
		r.failDeploymentRun(ctx, runID, fmt.Sprintf("failed to create browser session: %v", err))
		return fmt.Errorf("failed to create browser session: %w", err)
	}

	// Update run with session ID
	if sessionID != nil {
		if err := r.store.UpdateDeploymentRun(ctx, runID, map[string]interface{}{
			"session_id": *sessionID,
		}); err != nil {
			return fmt.Errorf("failed to update run with session ID: %w", err)
		}
	}

	// Execute the deployment
	result, err := r.executeDeploymentPackage(ctx, deployment, run, sessionID)
	if err != nil {
		r.failDeploymentRun(ctx, runID, fmt.Sprintf("execution failed: %v", err))
		return fmt.Errorf("failed to execute deployment: %w", err)
	}

	// Complete the run
	if err := r.store.CompleteDeploymentRun(ctx, runID, RunStatusCompleted, result, nil); err != nil {
		return fmt.Errorf("failed to complete deployment run: %w", err)
	}

	return nil
}

// createBrowserSession creates a browser session for the deployment if needed
func (r *DeploymentRunner) createBrowserSession(ctx context.Context, deployment *Deployment, run *DeploymentRun) (*uuid.UUID, error) {
	// Parse deployment config
	var config DeploymentConfig
	if err := json.Unmarshal(deployment.Config, &config); err != nil {
		return nil, fmt.Errorf("failed to parse deployment config: %w", err)
	}

	// Check if deployment needs a browser session
	if len(config.BrowserRequests) == 0 {
		return nil, nil // No browser session needed
	}

	// For now, create a single browser session from the first request
	// In a full implementation, this could handle multiple browser sessions
	browserRequest := config.BrowserRequests[0]

	// Create session request
	sessionReq := &sessions.Session{
		Browser:         sessions.Browser(browserRequest.Browser),
		Version:         sessions.BrowserVersion(browserRequest.Version),
		Headless:        browserRequest.Headless,
		OperatingSystem: sessions.OperatingSystem(browserRequest.OperatingSystem),
		Provider:        "docker",
		Status:          sessions.StatusPending,
	}

	// Set default values
	if sessionReq.Browser == "" {
		sessionReq.Browser = sessions.BrowserChrome
	}
	if sessionReq.Version == "" {
		sessionReq.Version = sessions.VerLatest
	}
	if sessionReq.OperatingSystem == "" {
		sessionReq.OperatingSystem = sessions.OSLinux
	}

	// Set screen configuration
	if browserRequest.Screen != nil {
		sessionReq.Screen = sessions.ScreenConfig{
			Width:  int(browserRequest.Screen["width"].(float64)),
			Height: int(browserRequest.Screen["height"].(float64)),
			DPI:    96,
			Scale:  1.0,
		}
	} else {
		sessionReq.Screen = sessions.ScreenConfig{
			Width:  1920,
			Height: 1080,
			DPI:    96,
			Scale:  1.0,
		}
	}

	// Set resource limits
	cpu := 2.0
	memory := "2GB"
	timeout := 30
	sessionReq.ResourceLimits = sessions.ResourceLimits{
		CPU:            &cpu,
		Memory:         &memory,
		TimeoutMinutes: &timeout,
	}

	// Set environment variables
	if browserRequest.Environment != nil {
		envJSON, err := json.Marshal(browserRequest.Environment)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal environment: %w", err)
		}
		sessionReq.Environment = envJSON
	} else {
		sessionReq.Environment = []byte("{}")
	}

	// Set expiration
	expires := time.Now().Add(1 * time.Hour)
	sessionReq.ExpiresAt = &expires

	// Set profile ID if provided
	if browserRequest.ProfileID != nil {
		sessionReq.ProfileID = browserRequest.ProfileID
	}

	// Create the session
	if err := r.sessStore.CreateSession(ctx, sessionReq); err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	return &sessionReq.ID, nil
}

// executeDeploymentPackage executes the deployment package
func (r *DeploymentRunner) executeDeploymentPackage(ctx context.Context, deployment *Deployment, run *DeploymentRun, sessionID *uuid.UUID) (map[string]interface{}, error) {
	// Download deployment package
	packagePath, err := r.downloadPackage(ctx, deployment.PackageURL, deployment.PackageHash)
	if err != nil {
		return nil, fmt.Errorf("failed to download package: %w", err)
	}
	defer os.RemoveAll(packagePath)

	// Extract package
	extractDir, err := r.extractPackage(packagePath)
	if err != nil {
		return nil, fmt.Errorf("failed to extract package: %w", err)
	}
	defer os.RemoveAll(extractDir)

	// Wait for browser session to be ready (if needed)
	if sessionID != nil {
		session, err := r.waitForSessionReady(ctx, *sessionID)
		if err != nil {
			return nil, fmt.Errorf("failed to wait for session: %w", err)
		}

		// Set browser connection environment variables
		if err := r.setBrowserEnvironment(extractDir, session); err != nil {
			return nil, fmt.Errorf("failed to set browser environment: %w", err)
		}
	}

	// Execute the deployment based on runtime
	switch deployment.Runtime {
	case RuntimeNode:
		return r.executeNodeDeployment(ctx, extractDir, deployment, run)
	case RuntimePython:
		return r.executePythonDeployment(ctx, extractDir, deployment, run)
	default:
		return nil, fmt.Errorf("unsupported runtime: %s", deployment.Runtime)
	}
}

// downloadPackage downloads the deployment package
func (r *DeploymentRunner) downloadPackage(ctx context.Context, packageURL, expectedHash string) (string, error) {
	// Create temporary directory for the package
	tmpDir := filepath.Join(r.workDir, "packages")
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create temp directory: %w", err)
	}

	// Download the package
	req, err := http.NewRequestWithContext(ctx, "GET", packageURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to download package: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to download package: status %d", resp.StatusCode)
	}

	// Save to temporary file
	packagePath := filepath.Join(tmpDir, fmt.Sprintf("package-%s-%s.zip", expectedHash, uuid.New().String()))
	file, err := os.Create(packagePath)
	if err != nil {
		return "", fmt.Errorf("failed to create package file: %w", err)
	}
	defer file.Close()

	if _, err := io.Copy(file, resp.Body); err != nil {
		return "", fmt.Errorf("failed to save package: %w", err)
	}

	// TODO: Verify package hash
	// In a full implementation, you would calculate the hash and compare with expectedHash

	return packagePath, nil
}

// extractPackage extracts the deployment package
func (r *DeploymentRunner) extractPackage(packagePath string) (string, error) {
	// Create extraction directory
	extractDir := filepath.Join(r.workDir, "extracted", fmt.Sprintf("deploy-%d", time.Now().UnixNano()))
	if err := os.MkdirAll(extractDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create extraction directory: %w", err)
	}

	// TODO: Extract ZIP file
	// In a full implementation, you would extract the ZIP file to extractDir
	// For now, we'll just create a placeholder structure

	return extractDir, nil
}

// waitForSessionReady waits for the browser session to be ready
func (r *DeploymentRunner) waitForSessionReady(ctx context.Context, sessionID uuid.UUID) (*sessions.Session, error) {
	timeout := time.NewTimer(5 * time.Minute)
	defer timeout.Stop()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	// Check immediately before starting the ticker loop
	session, err := r.sessStore.GetSession(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	if session.Status == sessions.StatusRunning {
		return session, nil
	}

	if sessions.IsTerminalStatus(session.Status) {
		return nil, fmt.Errorf("session failed with status: %s", session.Status)
	}

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timeout.C:
			return nil, fmt.Errorf("timeout waiting for session to be ready")
		case <-ticker.C:
			session, err := r.sessStore.GetSession(ctx, sessionID)
			if err != nil {
				return nil, fmt.Errorf("failed to get session: %w", err)
			}

			if session.Status == sessions.StatusRunning {
				return session, nil
			}

			if sessions.IsTerminalStatus(session.Status) {
				return nil, fmt.Errorf("session failed with status: %s", session.Status)
			}
		}
	}
}

// setBrowserEnvironment sets browser connection environment variables
func (r *DeploymentRunner) setBrowserEnvironment(extractDir string, session *sessions.Session) error {
	envFile := filepath.Join(extractDir, ".env")
	file, err := os.Create(envFile)
	if err != nil {
		return fmt.Errorf("failed to create .env file: %w", err)
	}
	defer file.Close()

	// Write browser connection details
	if session.WSEndpoint != nil {
		file.WriteString(fmt.Sprintf("BROWSER_WS_ENDPOINT=%s\n", *session.WSEndpoint))
	}
	if session.LiveURL != nil {
		file.WriteString(fmt.Sprintf("BROWSER_LIVE_URL=%s\n", *session.LiveURL))
	}
	file.WriteString(fmt.Sprintf("BROWSER_SESSION_ID=%s\n", session.ID))

	return nil
}

// executeNodeDeployment executes a Node.js deployment
func (r *DeploymentRunner) executeNodeDeployment(ctx context.Context, extractDir string, deployment *Deployment, run *DeploymentRun) (map[string]interface{}, error) {
	// TODO: Implement Node.js deployment execution
	// This would involve:
	// 1. Installing dependencies (npm install)
	// 2. Running the deployment script
	// 3. Capturing output and results

	result := map[string]interface{}{
		"runtime":    "node",
		"status":     "completed",
		"message":    "Node.js deployment executed successfully",
		"output":     "Mock Node.js execution output",
		"started_at": time.Now(),
	}

	return result, nil
}

// executePythonDeployment executes a Python deployment
func (r *DeploymentRunner) executePythonDeployment(ctx context.Context, extractDir string, deployment *Deployment, run *DeploymentRun) (map[string]interface{}, error) {
	// TODO: Implement Python deployment execution
	// This would involve:
	// 1. Installing dependencies (pip install -r requirements.txt)
	// 2. Running the deployment script
	// 3. Capturing output and results

	result := map[string]interface{}{
		"runtime":    "python",
		"status":     "completed",
		"message":    "Python deployment executed successfully",
		"output":     "Mock Python execution output",
		"started_at": time.Now(),
	}

	return result, nil
}

// failDeploymentRun marks a deployment run as failed
func (r *DeploymentRunner) failDeploymentRun(ctx context.Context, runID uuid.UUID, errorMsg string) {
	// Try to mark the run as failed, but don't fail if the database is not available
	if err := r.store.CompleteDeploymentRun(ctx, runID, RunStatusFailed, nil, &errorMsg); err != nil {
		// Only log the error if it's not a "relation does not exist" error (which can happen during tests)
		if !strings.Contains(err.Error(), "does not exist") {
			fmt.Printf("Failed to mark deployment run as failed: %v\n", err)
		}
	}
}

// CleanupExpiredRuns cleans up old deployment runs
func (r *DeploymentRunner) CleanupExpiredRuns(ctx context.Context, maxAge time.Duration) error {
	return r.store.CleanupOldRuns(ctx, maxAge)
}

// GetRunnerStats returns statistics about the deployment runner
func (r *DeploymentRunner) GetRunnerStats(ctx context.Context) (map[string]interface{}, error) {
	runningRuns, err := r.store.GetRunningDeploymentRuns(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get running runs: %w", err)
	}

	activeDeployments, err := r.store.GetActiveDeployments(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get active deployments: %w", err)
	}

	stats := map[string]interface{}{
		"running_runs":       len(runningRuns),
		"active_deployments": len(activeDeployments),
		"work_dir":           r.workDir,
	}

	return stats, nil
}

// DeploymentManifest represents the deployment manifest file
type DeploymentManifest struct {
	Name        string            `json:"name"`
	Version     string            `json:"version"`
	Runtime     string            `json:"runtime"`
	EntryPoint  string            `json:"entry_point"`
	Environment map[string]string `json:"environment,omitempty"`
	Config      DeploymentConfig  `json:"config,omitempty"`
}

// LoadManifest loads the deployment manifest from a directory
func LoadManifest(dir string) (*DeploymentManifest, error) {
	manifestPath := filepath.Join(dir, "browsergrid.json")
	if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
		// Try YAML format
		manifestPath = filepath.Join(dir, "browsergrid.yaml")
		if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
			return nil, fmt.Errorf("no manifest file found (browsergrid.json or browsergrid.yaml)")
		}
	}

	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest: %w", err)
	}

	var manifest DeploymentManifest
	if strings.HasSuffix(manifestPath, ".json") {
		if err := json.Unmarshal(data, &manifest); err != nil {
			return nil, fmt.Errorf("failed to parse JSON manifest: %w", err)
		}
	} else {
		// TODO: Add YAML support
		return nil, fmt.Errorf("YAML manifests not yet supported")
	}

	return &manifest, nil
}
