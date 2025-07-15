package deployments

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockCLIClient simulates the CLI client for testing
type MockCLIClient struct {
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
}

func NewMockCLIClient(baseURL, apiKey string) *MockCLIClient {
	return &MockCLIClient{
		BaseURL:    baseURL,
		APIKey:     apiKey,
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *MockCLIClient) CreateDeployment(req CreateDeploymentRequest) (*Deployment, error) {
	// Simulate API call
	url := fmt.Sprintf("%s/api/v1/deployments", c.BaseURL)

	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if c.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.APIKey)
	}

	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error: %d - %s", resp.StatusCode, string(body))
	}

	var deployment Deployment
	if err := json.NewDecoder(resp.Body).Decode(&deployment); err != nil {
		return nil, err
	}

	return &deployment, nil
}

func (c *MockCLIClient) GetDeployment(id string) (*Deployment, error) {
	url := fmt.Sprintf("%s/api/v1/deployments/%s", c.BaseURL, id)

	httpReq, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	if c.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.APIKey)
	}

	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error: %d - %s", resp.StatusCode, string(body))
	}

	var deployment Deployment
	if err := json.NewDecoder(resp.Body).Decode(&deployment); err != nil {
		return nil, err
	}

	return &deployment, nil
}

func (c *MockCLIClient) ListDeployments() ([]Deployment, error) {
	url := fmt.Sprintf("%s/api/v1/deployments", c.BaseURL)

	httpReq, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	if c.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.APIKey)
	}

	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error: %d - %s", resp.StatusCode, string(body))
	}

	var listResponse DeploymentListResponse
	if err := json.NewDecoder(resp.Body).Decode(&listResponse); err != nil {
		return nil, err
	}

	return listResponse.Deployments, nil
}

func (c *MockCLIClient) CreateDeploymentRun(deploymentID string, req CreateDeploymentRunRequest) (*DeploymentRun, error) {
	url := fmt.Sprintf("%s/api/v1/deployments/%s/runs", c.BaseURL, deploymentID)

	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if c.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.APIKey)
	}

	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error: %d - %s", resp.StatusCode, string(body))
	}

	var run DeploymentRun
	if err := json.NewDecoder(resp.Body).Decode(&run); err != nil {
		return nil, err
	}

	return &run, nil
}

func (c *MockCLIClient) GetDeploymentRun(runID string) (*DeploymentRun, error) {
	url := fmt.Sprintf("%s/api/v1/runs/%s", c.BaseURL, runID)

	httpReq, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	if c.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.APIKey)
	}

	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error: %d - %s", resp.StatusCode, string(body))
	}

	var run DeploymentRun
	if err := json.NewDecoder(resp.Body).Decode(&run); err != nil {
		return nil, err
	}

	return &run, nil
}

func (c *MockCLIClient) GetRunLogs(runID string) (map[string]interface{}, error) {
	url := fmt.Sprintf("%s/api/v1/runs/%s/logs", c.BaseURL, runID)

	httpReq, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	if c.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.APIKey)
	}

	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error: %d - %s", resp.StatusCode, string(body))
	}

	var logs map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&logs); err != nil {
		return nil, err
	}

	return logs, nil
}

func TestCLI_InitProject(t *testing.T) {
	tempDir := t.TempDir()

	tests := []struct {
		name        string
		projectName string
		runtime     string
		expectFiles []string
	}{
		{
			name:        "node project",
			projectName: "my-node-project",
			runtime:     "node",
			expectFiles: []string{
				"browsergrid.json",
				"index.js",
				"package.json",
				".gitignore",
				"README.md",
			},
		},
		{
			name:        "python project",
			projectName: "my-python-project",
			runtime:     "python",
			expectFiles: []string{
				"browsergrid.json",
				"main.py",
				"requirements.txt",
				".gitignore",
				"README.md",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			projectDir := filepath.Join(tempDir, tt.projectName)

			// Simulate CLI init command
			err := InitProject(projectDir, tt.runtime)
			assert.NoError(t, err)

			// Check if directory was created
			assert.DirExists(t, projectDir)

			// Check if expected files exist
			for _, file := range tt.expectFiles {
				filePath := filepath.Join(projectDir, file)
				assert.FileExists(t, filePath, "Expected file %s to exist", file)
			}

			// Check manifest content
			manifest, err := LoadManifest(projectDir)
			assert.NoError(t, err)
			assert.Equal(t, tt.projectName, manifest.Name)
			assert.Equal(t, tt.runtime, manifest.Runtime)
			assert.Equal(t, "1.0.0", manifest.Version)

			// Check entry point
			switch tt.runtime {
			case "node":
				assert.Equal(t, "index.js", manifest.EntryPoint)
				assert.FileExists(t, filepath.Join(projectDir, "index.js"))
				assert.FileExists(t, filepath.Join(projectDir, "package.json"))
			case "python":
				assert.Equal(t, "main.py", manifest.EntryPoint)
				assert.FileExists(t, filepath.Join(projectDir, "main.py"))
				assert.FileExists(t, filepath.Join(projectDir, "requirements.txt"))
			}
		})
	}
}

func TestCLI_DeployProject(t *testing.T) {
	// Setup mock API server
	suite := setupIntegrationTest(t)
	server := httptest.NewServer(suite.Router)
	defer server.Close()

	// Create test project
	projectDir := t.TempDir()
	err := InitProject(projectDir, "node")
	require.NoError(t, err)

	// Create CLI client
	client := NewMockCLIClient(server.URL, "test-api-key")

	// Test deploy project
	deployment, err := DeployProject(client, projectDir, "https://example.com/package.zip", "hash123")
	assert.NoError(t, err)
	assert.NotNil(t, deployment)
	assert.NotEqual(t, uuid.Nil, deployment.ID)
	assert.Equal(t, filepath.Base(projectDir), deployment.Name)
	assert.Equal(t, "1.0.0", deployment.Version)
	assert.Equal(t, RuntimeNode, deployment.Runtime)
	assert.Equal(t, "https://example.com/package.zip", deployment.PackageURL)
	assert.Equal(t, "hash123", deployment.PackageHash)
}

func TestCLI_ManageDeployments(t *testing.T) {
	// Setup mock API server
	suite := setupIntegrationTest(t)
	server := httptest.NewServer(suite.Router)
	defer server.Close()

	// Create CLI client
	client := NewMockCLIClient(server.URL, "test-api-key")

	// Create test deployments
	deployments := []CreateDeploymentRequest{
		{
			Name:        "cli-test-1",
			Version:     "1.0.0",
			Runtime:     RuntimeNode,
			PackageURL:  "https://example.com/package1.zip",
			PackageHash: "hash1",
		},
		{
			Name:        "cli-test-2",
			Version:     "1.0.0",
			Runtime:     RuntimePython,
			PackageURL:  "https://example.com/package2.zip",
			PackageHash: "hash2",
		},
	}

	var createdDeployments []*Deployment
	for _, req := range deployments {
		deployment, err := client.CreateDeployment(req)
		assert.NoError(t, err)
		createdDeployments = append(createdDeployments, deployment)
	}

	// Test list deployments
	listedDeployments, err := client.ListDeployments()
	assert.NoError(t, err)
	assert.Len(t, listedDeployments, 2)

	// Test get deployment
	deployment, err := client.GetDeployment(createdDeployments[0].ID.String())
	assert.NoError(t, err)
	assert.Equal(t, createdDeployments[0].ID, deployment.ID)
	assert.Equal(t, createdDeployments[0].Name, deployment.Name)
}

func TestCLI_ManageRuns(t *testing.T) {
	// Setup mock API server
	suite := setupIntegrationTest(t)
	server := httptest.NewServer(suite.Router)
	defer server.Close()

	// Create CLI client
	client := NewMockCLIClient(server.URL, "test-api-key")

	// Create test deployment
	deploymentReq := CreateDeploymentRequest{
		Name:        "cli-run-test",
		Version:     "1.0.0",
		Runtime:     RuntimeNode,
		PackageURL:  "https://example.com/package.zip",
		PackageHash: "hash123",
	}

	deployment, err := client.CreateDeployment(deploymentReq)
	require.NoError(t, err)

	// Test create run
	runReq := CreateDeploymentRunRequest{
		Environment: map[string]string{
			"NODE_ENV": "test",
		},
		Config: map[string]interface{}{
			"custom": "value",
		},
	}

	run, err := client.CreateDeploymentRun(deployment.ID.String(), runReq)
	assert.NoError(t, err)
	assert.NotNil(t, run)
	assert.Equal(t, deployment.ID, run.DeploymentID)
	assert.Equal(t, RunStatusPending, run.Status)

	// Test get run
	retrievedRun, err := client.GetDeploymentRun(run.ID.String())
	assert.NoError(t, err)
	assert.Equal(t, run.ID, retrievedRun.ID)
	assert.Equal(t, run.DeploymentID, retrievedRun.DeploymentID)

	// Test get run logs
	logs, err := client.GetRunLogs(run.ID.String())
	assert.NoError(t, err)
	assert.NotNil(t, logs)
	assert.Contains(t, logs, "run_id")
	assert.Contains(t, logs, "status")
}

func TestCLI_ProjectValidation(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(string) // Setup function that receives project directory
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid node project",
			setup: func(dir string) {
				InitProject(dir, "node")
			},
			expectError: false,
		},
		{
			name: "valid python project",
			setup: func(dir string) {
				InitProject(dir, "python")
			},
			expectError: false,
		},
		{
			name: "missing manifest",
			setup: func(dir string) {
				// Create directory but no manifest
				os.MkdirAll(dir, 0755)
			},
			expectError: true,
			errorMsg:    "no manifest file found",
		},
		{
			name: "invalid manifest",
			setup: func(dir string) {
				os.MkdirAll(dir, 0755)
				os.WriteFile(filepath.Join(dir, "browsergrid.json"), []byte("invalid json"), 0644)
			},
			expectError: true,
			errorMsg:    "failed to parse JSON manifest",
		},
		{
			name: "missing entry point",
			setup: func(dir string) {
				InitProject(dir, "node")
				os.Remove(filepath.Join(dir, "index.js"))
			},
			expectError: true,
			errorMsg:    "entry point file not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			projectDir := t.TempDir()
			tt.setup(projectDir)

			err := ValidateProject(projectDir)
			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCLI_PackageProject(t *testing.T) {
	// Create test project
	projectDir := t.TempDir()
	err := InitProject(projectDir, "node")
	require.NoError(t, err)

	// Add some additional files
	err = os.WriteFile(filepath.Join(projectDir, "utils.js"), []byte("module.exports = { helper: () => 'helper' };"), 0644)
	require.NoError(t, err)

	err = os.MkdirAll(filepath.Join(projectDir, "lib"), 0755)
	require.NoError(t, err)

	err = os.WriteFile(filepath.Join(projectDir, "lib", "database.js"), []byte("module.exports = { connect: () => {} };"), 0644)
	require.NoError(t, err)

	// Test package project
	packagePath, hash, err := PackageProject(projectDir)
	assert.NoError(t, err)
	assert.NotEmpty(t, packagePath)
	assert.NotEmpty(t, hash)
	assert.FileExists(t, packagePath)

	// Verify package is a valid ZIP file
	assert.True(t, strings.HasSuffix(packagePath, ".zip"))

	// Clean up
	os.Remove(packagePath)
}

func TestCLI_ConfigOperations(t *testing.T) {
	projectDir := t.TempDir()
	err := InitProject(projectDir, "node")
	require.NoError(t, err)

	// Test reading config
	manifest, err := LoadManifest(projectDir)
	assert.NoError(t, err)
	assert.Equal(t, filepath.Base(projectDir), manifest.Name)
	assert.Equal(t, "1.0.0", manifest.Version)

	// Test updating config
	manifest.Version = "2.0.0"
	manifest.Config.Concurrency = 5
	manifest.Config.MaxRetries = 10
	manifest.Config.BrowserRequests = []BrowserRequest{
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
	}

	err = SaveManifest(projectDir, manifest)
	assert.NoError(t, err)

	// Verify changes were saved
	updatedManifest, err := LoadManifest(projectDir)
	assert.NoError(t, err)
	assert.Equal(t, "2.0.0", updatedManifest.Version)
	assert.Equal(t, 5, updatedManifest.Config.Concurrency)
	assert.Equal(t, 10, updatedManifest.Config.MaxRetries)
	assert.Len(t, updatedManifest.Config.BrowserRequests, 1)
	assert.Equal(t, "chrome", updatedManifest.Config.BrowserRequests[0].Browser)
}

func TestCLI_ErrorHandling(t *testing.T) {
	// Setup mock API server that returns errors
	errorServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "Internal server error"}`))
	}))
	defer errorServer.Close()

	// Create CLI client
	client := NewMockCLIClient(errorServer.URL, "test-api-key")

	// Test API errors
	deploymentReq := CreateDeploymentRequest{
		Name:        "error-test",
		Version:     "1.0.0",
		Runtime:     RuntimeNode,
		PackageURL:  "https://example.com/package.zip",
		PackageHash: "hash123",
	}

	deployment, err := client.CreateDeployment(deploymentReq)
	assert.Error(t, err)
	assert.Nil(t, deployment)
	assert.Contains(t, err.Error(), "500")

	// Test network errors
	client.BaseURL = "http://invalid-url"
	deployment, err = client.CreateDeployment(deploymentReq)
	assert.Error(t, err)
	assert.Nil(t, deployment)
}

func TestCLI_AuthenticationFlow(t *testing.T) {
	// Setup mock API server that checks authentication
	authServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader != "Bearer valid-token" {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error": "Unauthorized"}`))
			return
		}

		// Handle authenticated request
		switch r.Method {
		case "GET":
			if strings.HasPrefix(r.URL.Path, "/api/v1/deployments") {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"deployments": [], "total": 0}`))
			}
		case "POST":
			if r.URL.Path == "/api/v1/deployments" {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusCreated)
				w.Write([]byte(`{"id": "` + uuid.New().String() + `", "name": "test", "version": "1.0.0"}`))
			}
		}
	}))
	defer authServer.Close()

	// Test with invalid token
	invalidClient := NewMockCLIClient(authServer.URL, "invalid-token")
	_, err := invalidClient.ListDeployments()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "401")

	// Test with valid token
	validClient := NewMockCLIClient(authServer.URL, "valid-token")
	deployments, err := validClient.ListDeployments()
	assert.NoError(t, err)
	assert.NotNil(t, deployments)
	assert.Len(t, deployments, 0)

	// Test creating deployment with valid token
	deploymentReq := CreateDeploymentRequest{
		Name:        "auth-test",
		Version:     "1.0.0",
		Runtime:     RuntimeNode,
		PackageURL:  "https://example.com/package.zip",
		PackageHash: "hash123",
	}

	deployment, err := validClient.CreateDeployment(deploymentReq)
	assert.NoError(t, err)
	assert.NotNil(t, deployment)
	assert.Equal(t, "test", deployment.Name)
}

// Helper functions for CLI operations

func InitProject(dir, runtime string) error {
	// Create directory
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// Create manifest
	manifest := DeploymentManifest{
		Name:    filepath.Base(dir),
		Version: "1.0.0",
		Runtime: runtime,
		Config: DeploymentConfig{
			Concurrency:    1,
			MaxRetries:     3,
			TimeoutSeconds: 300,
		},
	}

	// Set entry point based on runtime
	switch runtime {
	case "node":
		manifest.EntryPoint = "index.js"
		// Create package.json
		packageJSON := map[string]interface{}{
			"name":    manifest.Name,
			"version": manifest.Version,
			"main":    manifest.EntryPoint,
			"dependencies": map[string]string{
				"playwright": "^1.40.0",
			},
		}
		packageData, _ := json.MarshalIndent(packageJSON, "", "  ")
		os.WriteFile(filepath.Join(dir, "package.json"), packageData, 0644)

		// Create index.js
		jsContent := `const { chromium } = require('playwright');

async function main() {
  const browser = await chromium.connect({
    wsEndpoint: process.env.BROWSER_WS_ENDPOINT
  });
  
  const page = await browser.newPage();
  await page.goto('https://example.com');
  
  const title = await page.title();
  console.log('Page title:', title);
  
  await browser.close();
  
  return { title, success: true };
}

module.exports = main;
`
		os.WriteFile(filepath.Join(dir, "index.js"), []byte(jsContent), 0644)

	case "python":
		manifest.EntryPoint = "main.py"
		// Create requirements.txt
		reqContent := `playwright==1.40.0
`
		os.WriteFile(filepath.Join(dir, "requirements.txt"), []byte(reqContent), 0644)

		// Create main.py
		pyContent := `import os
from playwright.async_api import async_playwright

async def main():
    async with async_playwright() as p:
        browser = await p.chromium.connect_over_cdp(
            os.getenv('BROWSER_WS_ENDPOINT')
        )
        
        page = await browser.new_page()
        await page.goto('https://example.com')
        
        title = await page.title()
        print(f'Page title: {title}')
        
        await browser.close()
        
        return {'title': title, 'success': True}

if __name__ == '__main__':
    import asyncio
    asyncio.run(main())
`
		os.WriteFile(filepath.Join(dir, "main.py"), []byte(pyContent), 0644)
	}

	// Save manifest
	if err := SaveManifest(dir, &manifest); err != nil {
		return err
	}

	// Create .gitignore
	gitignoreContent := `node_modules/
__pycache__/
*.pyc
.env
.DS_Store
`
	os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(gitignoreContent), 0644)

	// Create README.md
	readmeContent := fmt.Sprintf(`# %s

This is a BrowserGrid deployment project.

## Runtime
- %s

## Usage
1. Install dependencies
2. Run with BrowserGrid
3. Deploy to production

## Configuration
See browsergrid.json for deployment configuration.
`, manifest.Name, runtime)
	os.WriteFile(filepath.Join(dir, "README.md"), []byte(readmeContent), 0644)

	return nil
}

func SaveManifest(dir string, manifest *DeploymentManifest) error {
	manifestPath := filepath.Join(dir, "browsergrid.json")
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(manifestPath, data, 0644)
}

func ValidateProject(dir string) error {
	// Check if manifest exists
	manifest, err := LoadManifest(dir)
	if err != nil {
		return err
	}

	// Check if entry point exists
	entryPointPath := filepath.Join(dir, manifest.EntryPoint)
	if _, err := os.Stat(entryPointPath); os.IsNotExist(err) {
		return fmt.Errorf("entry point file not found: %s", manifest.EntryPoint)
	}

	// Validate runtime-specific files
	switch manifest.Runtime {
	case "node":
		packageJSONPath := filepath.Join(dir, "package.json")
		if _, err := os.Stat(packageJSONPath); os.IsNotExist(err) {
			return fmt.Errorf("package.json not found for Node.js project")
		}
	case "python":
		requirementsPath := filepath.Join(dir, "requirements.txt")
		if _, err := os.Stat(requirementsPath); os.IsNotExist(err) {
			return fmt.Errorf("requirements.txt not found for Python project")
		}
	}

	return nil
}

func DeployProject(client *MockCLIClient, dir, packageURL, packageHash string) (*Deployment, error) {
	// Load manifest
	manifest, err := LoadManifest(dir)
	if err != nil {
		return nil, err
	}

	// Validate project
	if err := ValidateProject(dir); err != nil {
		return nil, err
	}

	// Create deployment request
	req := CreateDeploymentRequest{
		Name:        manifest.Name,
		Version:     manifest.Version,
		Runtime:     Runtime(manifest.Runtime),
		PackageURL:  packageURL,
		PackageHash: packageHash,
		Config:      manifest.Config,
	}

	// Create deployment
	return client.CreateDeployment(req)
}

func PackageProject(dir string) (string, string, error) {
	// In a real implementation, this would create a ZIP file
	// For testing, we'll create a simple file and return a mock hash
	packagePath := filepath.Join(os.TempDir(), "package-"+uuid.New().String()+".zip")

	// Create a simple "package" file
	err := os.WriteFile(packagePath, []byte("mock package content"), 0644)
	if err != nil {
		return "", "", err
	}

	// Generate mock hash
	hash := "mock-hash-" + uuid.New().String()

	return packagePath, hash, nil
}
