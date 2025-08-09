package deployments

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/autocrawlerHQ/browsergrid/internal/sessions"
)

// Mock sessions store for testing
type mockSessionsStore struct {
	sessions map[uuid.UUID]*sessions.Session
	errors   map[string]error
}

func newMockSessionsStore() *mockSessionsStore {
	return &mockSessionsStore{
		sessions: make(map[uuid.UUID]*sessions.Session),
		errors:   make(map[string]error),
	}
}

func (m *mockSessionsStore) CreateSession(ctx context.Context, session *sessions.Session) error {
	if err, exists := m.errors["CreateSession"]; exists {
		return err
	}
	if session.ID == uuid.Nil {
		session.ID = uuid.New()
	}
	m.sessions[session.ID] = session
	return nil
}

func (m *mockSessionsStore) GetSession(ctx context.Context, id uuid.UUID) (*sessions.Session, error) {
	if err, exists := m.errors["GetSession"]; exists {
		return nil, err
	}
	if session, exists := m.sessions[id]; exists {
		return session, nil
	}
	return nil, fmt.Errorf("session not found")
}

func (m *mockSessionsStore) SetError(method string, err error) {
	m.errors[method] = err
}

func (m *mockSessionsStore) SetSessionStatus(id uuid.UUID, status sessions.SessionStatus) {
	if session, exists := m.sessions[id]; exists {
		session.Status = status
		if status == sessions.StatusRunning {
			wsEndpoint := "ws://localhost:8080/ws"
			liveURL := "http://localhost:8080/live"
			session.WSEndpoint = &wsEndpoint
			session.LiveURL = &liveURL
		}
	}
}

func setupRunnerTestDB(t *testing.T) (*gorm.DB, *DeploymentRunner) {
	db, err := gorm.Open(postgres.Open(testConnStr), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("Failed to connect to test database: %v", err)
	}

	// Drop and recreate tables
	err = db.Migrator().DropTable(&Deployment{}, &DeploymentRun{})
	if err != nil {
		t.Logf("Warning: Failed to drop tables (may not exist): %v", err)
	}

	err = db.AutoMigrate(&Deployment{}, &DeploymentRun{})
	require.NoError(t, err)

	// Create temp directory for runner
	tempDir := t.TempDir()
	runner := NewDeploymentRunner(db, tempDir)

	return db, runner
}

func TestNewDeploymentRunner(t *testing.T) {
	db, err := gorm.Open(postgres.Open(testConnStr), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Skipf("Skipping test: PostgreSQL not available: %v", err)
	}

	tests := []struct {
		name     string
		workDir  string
		expected string
	}{
		{
			name:     "with work directory",
			workDir:  "/tmp/test-deployments",
			expected: "/tmp/test-deployments",
		},
		{
			name:     "empty work directory",
			workDir:  "",
			expected: "/tmp/deployments",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := NewDeploymentRunner(db, tt.workDir)
			assert.NotNil(t, runner)
			assert.Equal(t, tt.expected, runner.workDir)
			assert.NotNil(t, runner.store)
			assert.NotNil(t, runner.sessStore)
		})
	}
}

func TestDeploymentRunner_ExecuteDeployment(t *testing.T) {
	db, runner := setupRunnerTestDB(t)
	store := NewStore(db)
	ctx := context.Background()

	// Create test deployment
	deployment := &Deployment{
		Name:        "test-deployment",
		Version:     "1.0.0",
		Runtime:     RuntimeNode,
		PackageURL:  "https://example.com/package.zip",
		PackageHash: "hash123",
		Config:      datatypes.JSON(`{"concurrency": 1}`),
		Status:      StatusActive,
	}
	err := store.CreateDeployment(ctx, deployment)
	require.NoError(t, err)

	// Create test run
	run := &DeploymentRun{
		DeploymentID: deployment.ID,
		Status:       RunStatusPending,
	}
	err = store.CreateDeploymentRun(ctx, run)
	require.NoError(t, err)

	// Mock the sessions store to simulate no browser sessions needed
	runner.sessStore = newMockSessionsStore()

	// Mock HTTP server for package download
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/package.zip" {
			w.Header().Set("Content-Type", "application/zip")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("mock package content"))
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	// Update deployment to use mock server
	deployment.PackageURL = server.URL + "/package.zip"
	err = store.UpdateDeployment(ctx, deployment.ID, map[string]interface{}{
		"package_url": deployment.PackageURL,
	})
	require.NoError(t, err)

	// Execute deployment
	err = runner.ExecuteDeployment(ctx, run.ID)
	assert.NoError(t, err)

	// Verify run was updated
	updatedRun, err := store.GetDeploymentRun(ctx, run.ID)
	require.NoError(t, err)
	assert.Equal(t, RunStatusCompleted, updatedRun.Status)
	assert.NotNil(t, updatedRun.CompletedAt)
	assert.NotNil(t, updatedRun.Output)
}

func TestDeploymentRunner_ExecuteDeploymentWithBrowser(t *testing.T) {
	db, runner := setupRunnerTestDB(t)
	store := NewStore(db)
	ctx := context.Background()

	// Create test deployment with browser requests
	config := DeploymentConfig{
		BrowserRequests: []BrowserRequest{
			{
				Browser:         "chrome",
				Version:         "latest",
				Headless:        true,
				OperatingSystem: "linux",
				Screen: map[string]interface{}{
					"width":  1920,
					"height": 1080,
				},
			},
		},
	}
	configJSON, err := json.Marshal(config)
	require.NoError(t, err)

	deployment := &Deployment{
		Name:        "test-deployment-browser",
		Version:     "1.0.0",
		Runtime:     RuntimeNode,
		PackageURL:  "https://example.com/package.zip",
		PackageHash: "hash123",
		Config:      configJSON,
		Status:      StatusActive,
	}
	err = store.CreateDeployment(ctx, deployment)
	require.NoError(t, err)

	// Create test run
	run := &DeploymentRun{
		DeploymentID: deployment.ID,
		Status:       RunStatusPending,
	}
	err = store.CreateDeploymentRun(ctx, run)
	require.NoError(t, err)

	// Mock the sessions store
	mockSessStore := newMockSessionsStore()
	runner.sessStore = mockSessStore

	// Mock HTTP server for package download
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/zip")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("mock package content"))
	}))
	defer server.Close()

	// Update deployment to use mock server
	deployment.PackageURL = server.URL + "/package.zip"
	err = store.UpdateDeployment(ctx, deployment.ID, map[string]interface{}{
		"package_url": deployment.PackageURL,
	})
	require.NoError(t, err)

	// Execute deployment in a separate goroutine to simulate async execution
	go func() {
		// Wait a bit then make the session available
		time.Sleep(100 * time.Millisecond)
		for _, session := range mockSessStore.sessions {
			mockSessStore.SetSessionStatus(session.ID, sessions.StatusRunning)
		}
	}()

	// Execute deployment
	err = runner.ExecuteDeployment(ctx, run.ID)
	assert.NoError(t, err)

	// Verify run was updated
	updatedRun, err := store.GetDeploymentRun(ctx, run.ID)
	require.NoError(t, err)
	assert.Equal(t, RunStatusCompleted, updatedRun.Status)
	assert.NotNil(t, updatedRun.SessionID)
	assert.NotNil(t, updatedRun.CompletedAt)
}

func TestDeploymentRunner_ExecuteDeploymentErrors(t *testing.T) {
	db, runner := setupRunnerTestDB(t)
	store := NewStore(db)
	ctx := context.Background()

	tests := []struct {
		name        string
		setup       func() *DeploymentRun
		expectError bool
	}{
		{
			name: "non-existent run",
			setup: func() *DeploymentRun {
				return &DeploymentRun{ID: uuid.New()}
			},
			expectError: true,
		},
		{
			name: "non-existent deployment",
			setup: func() *DeploymentRun {
				// Create a valid deployment first
				deployment := &Deployment{
					Name:        "test-deployment-to-delete",
					Version:     "1.0.0",
					Runtime:     RuntimeNode,
					PackageURL:  "https://example.com/package.zip",
					PackageHash: "hash123",
					Status:      StatusActive,
				}
				err := store.CreateDeployment(ctx, deployment)
				require.NoError(t, err)

				// Create a run with this deployment
				run := &DeploymentRun{
					DeploymentID: deployment.ID,
					Status:       RunStatusPending,
				}
				err = store.CreateDeploymentRun(ctx, run)
				require.NoError(t, err)

				// Now delete the deployment to simulate the condition
				err = store.DeleteDeployment(ctx, deployment.ID)
				require.NoError(t, err)

				return run
			},
			expectError: true,
		},
		{
			name: "package download failure",
			setup: func() *DeploymentRun {
				deployment := &Deployment{
					Name:        "test-deployment-error",
					Version:     "1.0.0",
					Runtime:     RuntimeNode,
					PackageURL:  "http://localhost:99999/package.zip", // Use localhost with invalid port
					PackageHash: "hash123",
					Status:      StatusActive,
				}
				err := store.CreateDeployment(ctx, deployment)
				require.NoError(t, err)

				run := &DeploymentRun{
					DeploymentID: deployment.ID,
					Status:       RunStatusPending,
				}
				err = store.CreateDeploymentRun(ctx, run)
				require.NoError(t, err)
				return run
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			run := tt.setup()
			err := runner.ExecuteDeployment(ctx, run.ID)

			if tt.expectError {
				assert.Error(t, err)

				// Check if run was marked as failed (only if deployment still exists)
				if run.DeploymentID != uuid.Nil {
					updatedRun, getErr := store.GetDeploymentRun(ctx, run.ID)
					if getErr == nil {
						assert.Equal(t, RunStatusFailed, updatedRun.Status)
					}
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestDeploymentRunner_createBrowserSession(t *testing.T) {
	db, runner := setupRunnerTestDB(t)
	store := NewStore(db)
	ctx := context.Background()

	tests := []struct {
		name        string
		config      DeploymentConfig
		expectError bool
		expectNil   bool
	}{
		{
			name: "no browser requests",
			config: DeploymentConfig{
				Concurrency: 1,
			},
			expectError: false,
			expectNil:   true,
		},
		{
			name: "valid browser request",
			config: DeploymentConfig{
				BrowserRequests: []BrowserRequest{
					{
						Browser:         "chrome",
						Version:         "latest",
						Headless:        true,
						OperatingSystem: "linux",
						Screen: map[string]interface{}{
							"width":  1920,
							"height": 1080,
						},
					},
				},
			},
			expectError: false,
			expectNil:   false,
		},
		{
			name: "browser request with defaults",
			config: DeploymentConfig{
				BrowserRequests: []BrowserRequest{
					{
						Browser: "firefox",
					},
				},
			},
			expectError: false,
			expectNil:   false,
		},
		{
			name: "browser request with profile",
			config: DeploymentConfig{
				BrowserRequests: []BrowserRequest{
					{
						Browser:   "chrome",
						ProfileID: func() *uuid.UUID { id := uuid.New(); return &id }(),
					},
				},
			},
			expectError: false,
			expectNil:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test deployment
			configJSON, err := json.Marshal(tt.config)
			require.NoError(t, err)

			deployment := &Deployment{
				Name:        "test-deployment",
				Version:     "1.0.0",
				Runtime:     RuntimeNode,
				PackageURL:  "https://example.com/package.zip",
				PackageHash: "hash123",
				Config:      configJSON,
				Status:      StatusActive,
			}
			err = store.CreateDeployment(ctx, deployment)
			require.NoError(t, err)

			run := &DeploymentRun{
				DeploymentID: deployment.ID,
				Status:       RunStatusPending,
			}
			err = store.CreateDeploymentRun(ctx, run)
			require.NoError(t, err)

			// Mock the sessions store
			mockSessStore := newMockSessionsStore()
			runner.sessStore = mockSessStore

			// Test createBrowserSession
			sessionID, err := runner.createBrowserSession(ctx, deployment, run)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tt.expectNil {
					assert.Nil(t, sessionID)
				} else {
					assert.NotNil(t, sessionID)
					assert.NotEqual(t, uuid.Nil, *sessionID)

					// Verify session was created
					session, exists := mockSessStore.sessions[*sessionID]
					assert.True(t, exists)
					assert.Equal(t, sessions.StatusPending, session.Status)
					assert.Equal(t, "docker", session.Provider)
				}
			}
		})
	}
}

func TestDeploymentRunner_downloadPackage(t *testing.T) {
	_, runner := setupRunnerTestDB(t)
	ctx := context.Background()

	// Mock HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/package.zip":
			w.Header().Set("Content-Type", "application/zip")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("mock package content"))
		case "/notfound.zip":
			w.WriteHeader(http.StatusNotFound)
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	tests := []struct {
		name        string
		packageURL  string
		expectError bool
	}{
		{
			name:        "successful download",
			packageURL:  server.URL + "/package.zip",
			expectError: false,
		},
		{
			name:        "not found",
			packageURL:  server.URL + "/notfound.zip",
			expectError: true,
		},
		{
			name:        "invalid URL",
			packageURL:  "invalid-url",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			packagePath, err := runner.downloadPackage(ctx, tt.packageURL, "hash123")

			if tt.expectError {
				assert.Error(t, err)
				assert.Empty(t, packagePath)
			} else {
				assert.NoError(t, err)
				assert.NotEmpty(t, packagePath)
				assert.FileExists(t, packagePath)

				// Verify file contents
				content, err := os.ReadFile(packagePath)
				assert.NoError(t, err)
				assert.Equal(t, []byte("mock package content"), content)

				// Cleanup
				os.Remove(packagePath)
			}
		})
	}
}

func TestDeploymentRunner_extractPackage(t *testing.T) {
	_, runner := setupRunnerTestDB(t)

	// Create a temporary package file
	tempFile := filepath.Join(t.TempDir(), "test-package.zip")
	err := os.WriteFile(tempFile, []byte("mock package content"), 0644)
	require.NoError(t, err)

	// Test extract package
	extractDir, err := runner.extractPackage(tempFile)
	assert.NoError(t, err)
	assert.NotEmpty(t, extractDir)
	assert.DirExists(t, extractDir)

	// Cleanup
	os.RemoveAll(extractDir)
}

func TestDeploymentRunner_waitForSessionReady(t *testing.T) {
	_, runner := setupRunnerTestDB(t)
	ctx := context.Background()

	// Mock the sessions store
	mockSessStore := newMockSessionsStore()
	runner.sessStore = mockSessStore

	// Create a test session
	sessionID := uuid.New()
	testSession := &sessions.Session{
		ID:     sessionID,
		Status: sessions.StatusPending,
	}
	mockSessStore.sessions[sessionID] = testSession

	tests := []struct {
		name         string
		setup        func()
		expectError  bool
		expectResult bool
	}{
		{
			name: "session becomes ready",
			setup: func() {
				// Make session ready after a very short delay
				go func() {
					time.Sleep(10 * time.Millisecond)
					mockSessStore.SetSessionStatus(sessionID, sessions.StatusRunning)
				}()
			},
			expectError:  false,
			expectResult: true,
		},
		{
			name: "session fails",
			setup: func() {
				// Make session fail after a very short delay
				go func() {
					time.Sleep(10 * time.Millisecond)
					mockSessStore.SetSessionStatus(sessionID, sessions.StatusFailed)
				}()
			},
			expectError:  true,
			expectResult: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset session status
			mockSessStore.SetSessionStatus(sessionID, sessions.StatusPending)

			// Setup test conditions
			tt.setup()

			// Create context with reasonable timeout
			ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()

			// Wait for session
			result, err := runner.waitForSessionReady(ctx, sessionID)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				if tt.expectResult {
					assert.NotNil(t, result)
					assert.Equal(t, sessionID, result.ID)
				}
			}
		})
	}
}

func TestDeploymentRunner_setBrowserEnvironment(t *testing.T) {
	_, runner := setupRunnerTestDB(t)

	extractDir := t.TempDir()

	testSession := &sessions.Session{
		ID:         uuid.New(),
		WSEndpoint: stringPtr("ws://localhost:8080/ws"),
		LiveURL:    stringPtr("http://localhost:8080/live"),
	}

	err := runner.setBrowserEnvironment(extractDir, testSession)
	assert.NoError(t, err)

	// Verify .env file was created
	envFile := filepath.Join(extractDir, ".env")
	assert.FileExists(t, envFile)

	// Verify file contents
	content, err := os.ReadFile(envFile)
	assert.NoError(t, err)

	envContent := string(content)
	assert.Contains(t, envContent, "BROWSER_WS_ENDPOINT=ws://localhost:8080/ws")
	assert.Contains(t, envContent, "BROWSER_LIVE_URL=http://localhost:8080/live")
	assert.Contains(t, envContent, "BROWSER_SESSION_ID="+testSession.ID.String())
}

func TestDeploymentRunner_executeRuntimes(t *testing.T) {
	_, runner := setupRunnerTestDB(t)
	ctx := context.Background()

	extractDir := t.TempDir()

	deployment := &Deployment{
		ID:          uuid.New(),
		Name:        "test-deployment",
		Version:     "1.0.0",
		Runtime:     RuntimeNode,
		PackageURL:  "https://example.com/package.zip",
		PackageHash: "hash123",
	}

	run := &DeploymentRun{
		ID:           uuid.New(),
		DeploymentID: deployment.ID,
		Status:       RunStatusRunning,
	}

	tests := []struct {
		name        string
		runtime     Runtime
		expectError bool
	}{
		{
			name:        "node runtime",
			runtime:     RuntimeNode,
			expectError: false,
		},
		{
			name:        "python runtime",
			runtime:     RuntimePython,
			expectError: false,
		},
		{
			name:        "unsupported runtime",
			runtime:     Runtime("java"),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deployment.Runtime = tt.runtime

			var result map[string]interface{}
			var err error

			switch tt.runtime {
			case RuntimeNode:
				result, err = runner.executeNodeDeployment(ctx, extractDir, deployment, run)
			case RuntimePython:
				result, err = runner.executePythonDeployment(ctx, extractDir, deployment, run)
			default:
				result, err = runner.executeDeploymentPackage(ctx, deployment, run, nil)
			}

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Contains(t, result, "runtime")
				assert.Contains(t, result, "status")
				assert.Contains(t, result, "message")
				assert.Equal(t, string(tt.runtime), result["runtime"])
				assert.Equal(t, "completed", result["status"])
			}
		})
	}
}

func TestDeploymentRunner_GetRunnerStats(t *testing.T) {
	db, runner := setupRunnerTestDB(t)
	store := NewStore(db)
	ctx := context.Background()

	// Create test deployment
	deployment := &Deployment{
		Name:        "test-deployment",
		Version:     "1.0.0",
		Runtime:     RuntimeNode,
		PackageURL:  "https://example.com/package.zip",
		PackageHash: "hash123",
		Status:      StatusActive,
	}
	err := store.CreateDeployment(ctx, deployment)
	require.NoError(t, err)

	// Create test runs
	runs := []*DeploymentRun{
		{DeploymentID: deployment.ID, Status: RunStatusRunning},
		{DeploymentID: deployment.ID, Status: RunStatusRunning},
		{DeploymentID: deployment.ID, Status: RunStatusCompleted},
	}

	for _, run := range runs {
		err = store.CreateDeploymentRun(ctx, run)
		require.NoError(t, err)
	}

	// Get runner stats
	stats, err := runner.GetRunnerStats(ctx)
	assert.NoError(t, err)
	assert.NotNil(t, stats)

	assert.Contains(t, stats, "running_runs")
	assert.Contains(t, stats, "active_deployments")
	assert.Contains(t, stats, "work_dir")

	assert.Equal(t, 2, stats["running_runs"])
	assert.Equal(t, 1, stats["active_deployments"])
	assert.Equal(t, runner.workDir, stats["work_dir"])
}

func TestDeploymentRunner_CleanupExpiredRuns(t *testing.T) {
	db, runner := setupRunnerTestDB(t)
	store := NewStore(db)
	ctx := context.Background()

	// Create test deployment
	deployment := &Deployment{
		Name:        "test-deployment",
		Version:     "1.0.0",
		Runtime:     RuntimeNode,
		PackageURL:  "https://example.com/package.zip",
		PackageHash: "hash123",
		Status:      StatusActive,
	}
	err := store.CreateDeployment(ctx, deployment)
	require.NoError(t, err)

	// Create old run
	oldRun := &DeploymentRun{
		DeploymentID: deployment.ID,
		Status:       RunStatusCompleted,
	}
	err = store.CreateDeploymentRun(ctx, oldRun)
	require.NoError(t, err)

	// Manually set old created_at time
	err = db.Model(oldRun).Update("created_at", time.Now().Add(-25*time.Hour)).Error
	require.NoError(t, err)

	// Create recent run
	recentRun := &DeploymentRun{
		DeploymentID: deployment.ID,
		Status:       RunStatusCompleted,
	}
	err = store.CreateDeploymentRun(ctx, recentRun)
	require.NoError(t, err)

	// Test cleanup
	err = runner.CleanupExpiredRuns(ctx, 24*time.Hour)
	assert.NoError(t, err)

	// Verify cleanup
	_, err = store.GetDeploymentRun(ctx, oldRun.ID)
	assert.Error(t, err)

	_, err = store.GetDeploymentRun(ctx, recentRun.ID)
	assert.NoError(t, err)
}

func TestLoadManifest(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(string) // Setup function that receives the temp directory
		expectError bool
		expected    *DeploymentManifest
	}{
		{
			name: "valid JSON manifest",
			setup: func(dir string) {
				manifest := DeploymentManifest{
					Name:       "test-deployment",
					Version:    "1.0.0",
					Runtime:    "node",
					EntryPoint: "index.js",
					Environment: map[string]string{
						"NODE_ENV": "production",
					},
					Config: DeploymentConfig{
						Concurrency: 1,
						MaxRetries:  3,
					},
				}
				content, _ := json.MarshalIndent(manifest, "", "  ")
				os.WriteFile(filepath.Join(dir, "browsergrid.json"), content, 0644)
			},
			expectError: false,
			expected: &DeploymentManifest{
				Name:       "test-deployment",
				Version:    "1.0.0",
				Runtime:    "node",
				EntryPoint: "index.js",
				Environment: map[string]string{
					"NODE_ENV": "production",
				},
				Config: DeploymentConfig{
					Concurrency: 1,
					MaxRetries:  3,
				},
			},
		},
		{
			name: "no manifest file",
			setup: func(dir string) {
				// Don't create any manifest file
			},
			expectError: true,
		},
		{
			name: "invalid JSON",
			setup: func(dir string) {
				os.WriteFile(filepath.Join(dir, "browsergrid.json"), []byte("invalid json"), 0644)
			},
			expectError: true,
		},
		{
			name: "YAML manifest (unsupported)",
			setup: func(dir string) {
				content := `
name: test-deployment
version: 1.0.0
runtime: node
entry_point: index.js
`
				os.WriteFile(filepath.Join(dir, "browsergrid.yaml"), []byte(content), 0644)
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			tt.setup(tempDir)

			manifest, err := LoadManifest(tempDir)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, manifest)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, manifest)
				assert.Equal(t, tt.expected.Name, manifest.Name)
				assert.Equal(t, tt.expected.Version, manifest.Version)
				assert.Equal(t, tt.expected.Runtime, manifest.Runtime)
				assert.Equal(t, tt.expected.EntryPoint, manifest.EntryPoint)
				assert.Equal(t, tt.expected.Environment, manifest.Environment)
				assert.Equal(t, tt.expected.Config.Concurrency, manifest.Config.Concurrency)
				assert.Equal(t, tt.expected.Config.MaxRetries, manifest.Config.MaxRetries)
			}
		})
	}
}

func TestDeploymentRunner_ConcurrentExecution(t *testing.T) {
	db, runner := setupRunnerTestDB(t)
	store := NewStore(db)
	ctx := context.Background()

	// Create test deployment
	deployment := &Deployment{
		Name:        "concurrent-test",
		Version:     "1.0.0",
		Runtime:     RuntimeNode,
		PackageURL:  "https://example.com/package.zip",
		PackageHash: "hash123",
		Status:      StatusActive,
	}
	err := store.CreateDeployment(ctx, deployment)
	require.NoError(t, err)

	// Mock HTTP server for package download
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/zip")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("mock package content"))
	}))
	defer server.Close()

	// Update deployment to use mock server
	deployment.PackageURL = server.URL + "/package.zip"
	err = store.UpdateDeployment(ctx, deployment.ID, map[string]interface{}{
		"package_url": deployment.PackageURL,
	})
	require.NoError(t, err)

	// Mock sessions store
	runner.sessStore = newMockSessionsStore()

	// Create multiple runs and execute them concurrently
	const numRuns = 5
	runs := make([]*DeploymentRun, numRuns)
	results := make(chan error, numRuns)

	for i := 0; i < numRuns; i++ {
		run := &DeploymentRun{
			DeploymentID: deployment.ID,
			Status:       RunStatusPending,
		}
		err = store.CreateDeploymentRun(ctx, run)
		require.NoError(t, err)
		runs[i] = run
	}

	// Execute all runs concurrently
	for i := 0; i < numRuns; i++ {
		go func(runID uuid.UUID) {
			results <- runner.ExecuteDeployment(ctx, runID)
		}(runs[i].ID)
	}

	// Check results
	successCount := 0
	for i := 0; i < numRuns; i++ {
		err := <-results
		if err == nil {
			successCount++
		}
	}

	assert.Equal(t, numRuns, successCount, "All concurrent executions should succeed")

	// Verify all runs completed
	for _, run := range runs {
		updatedRun, err := store.GetDeploymentRun(ctx, run.ID)
		assert.NoError(t, err)
		assert.Equal(t, RunStatusCompleted, updatedRun.Status)
	}
}
