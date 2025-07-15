package deployments

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/autocrawlerHQ/browsergrid/internal/sessions"
	"github.com/autocrawlerHQ/browsergrid/internal/tasks"
)

// IntegrationTestSuite provides a complete test environment
type IntegrationTestSuite struct {
	DB        *gorm.DB
	Router    *gin.Engine
	Runner    *DeploymentRunner
	Store     *Store
	TaskQueue *mockTaskQueue
	TempDir   string
}

// Mock task queue that can execute tasks synchronously for testing
type mockTaskQueue struct {
	tasks   []asynq.Task
	handler func(*asynq.Task) error
	mu      sync.RWMutex
}

func (m *mockTaskQueue) Enqueue(task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.tasks = append(m.tasks, *task)

	// Execute immediately if handler is set
	if m.handler != nil {
		go m.handler(task)
	}

	return &asynq.TaskInfo{
		ID:       uuid.New().String(),
		Type:     task.Type(),
		Payload:  task.Payload(),
		Queue:    "default",
		MaxRetry: 3,
	}, nil
}

func (m *mockTaskQueue) Close() error {
	return nil
}

func (m *mockTaskQueue) GetTasks() []asynq.Task {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]asynq.Task{}, m.tasks...)
}

func (m *mockTaskQueue) SetHandler(handler func(*asynq.Task) error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.handler = handler
}

func setupIntegrationTest(t *testing.T) *IntegrationTestSuite {
	// Setup database
	db, err := gorm.Open(postgres.Open(testConnStr), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("Failed to connect to test database: %v", err)
	}

	// Clean up tables
	err = db.Migrator().DropTable(&Deployment{}, &DeploymentRun{}, &sessions.Session{})
	if err != nil {
		t.Logf("Warning: Failed to drop tables: %v", err)
	}

	// Auto-migrate
	err = db.AutoMigrate(&Deployment{}, &DeploymentRun{}, &sessions.Session{})
	require.NoError(t, err)

	// Create temp directory
	tempDir := t.TempDir()

	// Setup components
	store := NewStore(db)
	runner := NewDeploymentRunner(db, tempDir)
	taskQueue := &mockTaskQueue{}

	// Setup HTTP router
	gin.SetMode(gin.TestMode)
	router := gin.New()
	v1 := router.Group("/api/v1")
	RegisterRoutes(v1, Dependencies{
		DB:         db,
		TaskClient: taskQueue,
	})

	// Setup task handler
	taskQueue.SetHandler(func(task *asynq.Task) error {
		switch task.Type() {
		case tasks.TypeDeploymentRun:
			var payload tasks.DeploymentRunPayload
			if err := json.Unmarshal(task.Payload(), &payload); err != nil {
				return err
			}
			return runner.ExecuteDeployment(context.Background(), payload.RunID)
		default:
			return nil
		}
	})

	return &IntegrationTestSuite{
		DB:        db,
		Router:    router,
		Runner:    runner,
		Store:     store,
		TaskQueue: taskQueue,
		TempDir:   tempDir,
	}
}

func TestIntegration_CompleteDeploymentFlow(t *testing.T) {
	suite := setupIntegrationTest(t)
	ctx := context.Background()

	// Step 1: Create deployment via API
	deploymentReq := CreateDeploymentRequest{
		Name:        "integration-test",
		Description: "Integration test deployment",
		Version:     "1.0.0",
		Runtime:     RuntimeNode,
		PackageURL:  "https://example.com/package.zip",
		PackageHash: "hash123",
		Config: DeploymentConfig{
			Concurrency:    1,
			MaxRetries:     3,
			TimeoutSeconds: 300,
		},
	}

	body, err := json.Marshal(deploymentReq)
	require.NoError(t, err)

	req, err := http.NewRequest("POST", "/api/v1/deployments", bytes.NewBuffer(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	suite.Router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code)

	var deployment Deployment
	err = json.Unmarshal(rr.Body.Bytes(), &deployment)
	require.NoError(t, err)

	// Step 2: Verify deployment in database
	dbDeployment, err := suite.Store.GetDeployment(ctx, deployment.ID)
	require.NoError(t, err)
	assert.Equal(t, deployment.Name, dbDeployment.Name)
	assert.Equal(t, deployment.Version, dbDeployment.Version)
	assert.Equal(t, StatusActive, dbDeployment.Status)

	// Step 3: List deployments
	req, err = http.NewRequest("GET", "/api/v1/deployments", nil)
	require.NoError(t, err)

	rr = httptest.NewRecorder()
	suite.Router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var listResponse DeploymentListResponse
	err = json.Unmarshal(rr.Body.Bytes(), &listResponse)
	require.NoError(t, err)
	assert.Len(t, listResponse.Deployments, 1)
	assert.Equal(t, int64(1), listResponse.Total)

	// Step 4: Get deployment details
	req, err = http.NewRequest("GET", "/api/v1/deployments/"+deployment.ID.String(), nil)
	require.NoError(t, err)

	rr = httptest.NewRecorder()
	suite.Router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var detailResponse Deployment
	err = json.Unmarshal(rr.Body.Bytes(), &detailResponse)
	require.NoError(t, err)
	assert.Equal(t, deployment.ID, detailResponse.ID)

	// Step 5: Update deployment
	updateReq := UpdateDeploymentRequest{
		Description: stringPtr("Updated description"),
		Status:      &[]DeploymentStatus{StatusInactive}[0],
	}

	body, err = json.Marshal(updateReq)
	require.NoError(t, err)

	req, err = http.NewRequest("PATCH", "/api/v1/deployments/"+deployment.ID.String(), bytes.NewBuffer(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	rr = httptest.NewRecorder()
	suite.Router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	// Verify update
	updated, err := suite.Store.GetDeployment(ctx, deployment.ID)
	require.NoError(t, err)
	assert.Equal(t, "Updated description", updated.Description)
	assert.Equal(t, StatusInactive, updated.Status)

	// Step 6: Reactivate deployment for run test
	err = suite.Store.UpdateDeploymentStatus(ctx, deployment.ID, StatusActive)
	require.NoError(t, err)

	// Step 7: Create deployment run
	runReq := CreateDeploymentRunRequest{
		Environment: map[string]string{
			"NODE_ENV": "test",
		},
		Config: map[string]interface{}{
			"custom": "value",
		},
	}

	body, err = json.Marshal(runReq)
	require.NoError(t, err)

	req, err = http.NewRequest("POST", "/api/v1/deployments/"+deployment.ID.String()+"/runs", bytes.NewBuffer(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	rr = httptest.NewRecorder()
	suite.Router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code)

	var run DeploymentRun
	err = json.Unmarshal(rr.Body.Bytes(), &run)
	require.NoError(t, err)
	assert.Equal(t, deployment.ID, run.DeploymentID)
	assert.Equal(t, RunStatusPending, run.Status)

	// Step 8: Verify task was enqueued
	tasks := suite.TaskQueue.GetTasks()
	assert.Len(t, tasks, 1)
	assert.Equal(t, tasks[0].Type(), "deployment:run")

	// Step 9: List runs for deployment
	req, err = http.NewRequest("GET", "/api/v1/deployments/"+deployment.ID.String()+"/runs", nil)
	require.NoError(t, err)

	rr = httptest.NewRecorder()
	suite.Router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var runsResponse DeploymentRunListResponse
	err = json.Unmarshal(rr.Body.Bytes(), &runsResponse)
	require.NoError(t, err)
	assert.Len(t, runsResponse.Runs, 1)
	assert.Equal(t, run.ID, runsResponse.Runs[0].ID)

	// Step 10: Get run details
	req, err = http.NewRequest("GET", "/api/v1/runs/"+run.ID.String(), nil)
	require.NoError(t, err)

	rr = httptest.NewRecorder()
	suite.Router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var runDetails DeploymentRun
	err = json.Unmarshal(rr.Body.Bytes(), &runDetails)
	require.NoError(t, err)
	assert.Equal(t, run.ID, runDetails.ID)

	// Step 11: Get deployment statistics
	req, err = http.NewRequest("GET", "/api/v1/deployments/"+deployment.ID.String()+"/stats", nil)
	require.NoError(t, err)

	rr = httptest.NewRecorder()
	suite.Router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var stats map[string]interface{}
	err = json.Unmarshal(rr.Body.Bytes(), &stats)
	require.NoError(t, err)
	assert.Contains(t, stats, "run_stats")
	assert.Contains(t, stats, "recent_runs")

	// Step 12: Delete deployment run
	req, err = http.NewRequest("DELETE", "/api/v1/runs/"+run.ID.String(), nil)
	require.NoError(t, err)

	rr = httptest.NewRecorder()
	suite.Router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	// Verify deletion
	_, err = suite.Store.GetDeploymentRun(ctx, run.ID)
	assert.Error(t, err)

	// Step 13: Delete deployment
	req, err = http.NewRequest("DELETE", "/api/v1/deployments/"+deployment.ID.String(), nil)
	require.NoError(t, err)

	rr = httptest.NewRecorder()
	suite.Router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	// Verify deletion
	_, err = suite.Store.GetDeployment(ctx, deployment.ID)
	assert.Error(t, err)
}

func TestIntegration_DeploymentWithBrowserSession(t *testing.T) {
	suite := setupIntegrationTest(t)

	// Mock HTTP server for package download
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/zip")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("mock package content"))
	}))
	defer server.Close()

	// Create deployment with browser configuration
	deploymentReq := CreateDeploymentRequest{
		Name:        "browser-test",
		Version:     "1.0.0",
		Runtime:     RuntimeNode,
		PackageURL:  server.URL + "/package.zip",
		PackageHash: "hash123",
		Config: DeploymentConfig{
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
	}

	body, err := json.Marshal(deploymentReq)
	require.NoError(t, err)

	req, err := http.NewRequest("POST", "/api/v1/deployments", bytes.NewBuffer(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	suite.Router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code)

	var deployment Deployment
	err = json.Unmarshal(rr.Body.Bytes(), &deployment)
	require.NoError(t, err)

	// Mock sessions store for browser session creation
	mockSessStore := newMockSessionsStore()
	suite.Runner.sessStore = mockSessStore

	// Create and execute deployment run
	runReq := CreateDeploymentRunRequest{}
	body, err = json.Marshal(runReq)
	require.NoError(t, err)

	req, err = http.NewRequest("POST", "/api/v1/deployments/"+deployment.ID.String()+"/runs", bytes.NewBuffer(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	rr = httptest.NewRecorder()
	suite.Router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code)

	var run DeploymentRun
	err = json.Unmarshal(rr.Body.Bytes(), &run)
	require.NoError(t, err)

	// Wait for task execution (simulate async behavior)
	time.Sleep(100 * time.Millisecond)

	// Make sessions available
	for _, session := range mockSessStore.sessions {
		mockSessStore.SetSessionStatus(session.ID, sessions.StatusRunning)
	}

	// Verify browser session was created
	assert.Len(t, mockSessStore.sessions, 1)
	for _, session := range mockSessStore.sessions {
		assert.Equal(t, sessions.StatusRunning, session.Status)
		assert.Equal(t, sessions.BrowserChrome, session.Browser)
		assert.Equal(t, sessions.VerLatest, session.Version)
		assert.Equal(t, sessions.OSLinux, session.OperatingSystem)
		assert.True(t, session.Headless)
	}
}

func TestIntegration_MultipleDeployments(t *testing.T) {
	suite := setupIntegrationTest(t)

	// Create multiple deployments
	deployments := []CreateDeploymentRequest{
		{
			Name:        "deployment-1",
			Version:     "1.0.0",
			Runtime:     RuntimeNode,
			PackageURL:  "https://example.com/package1.zip",
			PackageHash: "hash1",
		},
		{
			Name:        "deployment-2",
			Version:     "1.0.0",
			Runtime:     RuntimePython,
			PackageURL:  "https://example.com/package2.zip",
			PackageHash: "hash2",
		},
		{
			Name:        "deployment-3",
			Version:     "2.0.0",
			Runtime:     RuntimeNode,
			PackageURL:  "https://example.com/package3.zip",
			PackageHash: "hash3",
		},
	}

	createdDeployments := make([]Deployment, len(deployments))

	// Create deployments
	for i, deploymentReq := range deployments {
		body, err := json.Marshal(deploymentReq)
		require.NoError(t, err)

		req, err := http.NewRequest("POST", "/api/v1/deployments", bytes.NewBuffer(body))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")

		rr := httptest.NewRecorder()
		suite.Router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusCreated, rr.Code)

		err = json.Unmarshal(rr.Body.Bytes(), &createdDeployments[i])
		require.NoError(t, err)
	}

	// Test filtering by runtime
	req, err := http.NewRequest("GET", "/api/v1/deployments?runtime=node", nil)
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	suite.Router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var nodeDeployments DeploymentListResponse
	err = json.Unmarshal(rr.Body.Bytes(), &nodeDeployments)
	require.NoError(t, err)
	assert.Len(t, nodeDeployments.Deployments, 2)

	// Test filtering by status
	req, err = http.NewRequest("GET", "/api/v1/deployments?status=active", nil)
	require.NoError(t, err)

	rr = httptest.NewRecorder()
	suite.Router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var activeDeployments DeploymentListResponse
	err = json.Unmarshal(rr.Body.Bytes(), &activeDeployments)
	require.NoError(t, err)
	assert.Len(t, activeDeployments.Deployments, 3)

	// Test pagination
	req, err = http.NewRequest("GET", "/api/v1/deployments?limit=2&offset=0", nil)
	require.NoError(t, err)

	rr = httptest.NewRecorder()
	suite.Router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var paginatedDeployments DeploymentListResponse
	err = json.Unmarshal(rr.Body.Bytes(), &paginatedDeployments)
	require.NoError(t, err)
	assert.Len(t, paginatedDeployments.Deployments, 2)
	assert.Equal(t, int64(3), paginatedDeployments.Total)

	// Create runs for each deployment
	for _, deployment := range createdDeployments {
		runReq := CreateDeploymentRunRequest{}
		body, err := json.Marshal(runReq)
		require.NoError(t, err)

		req, err := http.NewRequest("POST", "/api/v1/deployments/"+deployment.ID.String()+"/runs", bytes.NewBuffer(body))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")

		rr := httptest.NewRecorder()
		suite.Router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusCreated, rr.Code)
	}

	// Test listing all runs
	req, err = http.NewRequest("GET", "/api/v1/runs", nil)
	require.NoError(t, err)

	rr = httptest.NewRecorder()
	suite.Router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var allRuns DeploymentRunListResponse
	err = json.Unmarshal(rr.Body.Bytes(), &allRuns)
	require.NoError(t, err)
	assert.Len(t, allRuns.Runs, 3)

	// Test filtering runs by status
	req, err = http.NewRequest("GET", "/api/v1/runs?status=pending", nil)
	require.NoError(t, err)

	rr = httptest.NewRecorder()
	suite.Router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var pendingRuns DeploymentRunListResponse
	err = json.Unmarshal(rr.Body.Bytes(), &pendingRuns)
	require.NoError(t, err)
	assert.Len(t, pendingRuns.Runs, 3)
}

func TestIntegration_DeploymentManifestWorkflow(t *testing.T) {
	suite := setupIntegrationTest(t)

	// Create a deployment manifest
	manifest := DeploymentManifest{
		Name:       "manifest-test",
		Version:    "1.0.0",
		Runtime:    "node",
		EntryPoint: "index.js",
		Environment: map[string]string{
			"NODE_ENV": "production",
		},
		Config: DeploymentConfig{
			Concurrency:    2,
			MaxRetries:     5,
			TimeoutSeconds: 600,
			BrowserRequests: []BrowserRequest{
				{
					Browser:         "chrome",
					Version:         "latest",
					Headless:        true,
					OperatingSystem: "linux",
				},
			},
		},
	}

	// Create manifest file
	manifestDir := filepath.Join(suite.TempDir, "test-manifest")
	err := os.MkdirAll(manifestDir, 0755)
	require.NoError(t, err)

	manifestFile := filepath.Join(manifestDir, "browsergrid.json")
	manifestContent, err := json.MarshalIndent(manifest, "", "  ")
	require.NoError(t, err)

	err = os.WriteFile(manifestFile, manifestContent, 0644)
	require.NoError(t, err)

	// Create example script
	scriptContent := `
const { chromium } = require('playwright');

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
	err = os.WriteFile(filepath.Join(manifestDir, "index.js"), []byte(scriptContent), 0644)
	require.NoError(t, err)

	// Test loading manifest
	loadedManifest, err := LoadManifest(manifestDir)
	require.NoError(t, err)
	assert.Equal(t, manifest.Name, loadedManifest.Name)
	assert.Equal(t, manifest.Version, loadedManifest.Version)
	assert.Equal(t, manifest.Runtime, loadedManifest.Runtime)
	assert.Equal(t, manifest.EntryPoint, loadedManifest.EntryPoint)
	assert.Equal(t, manifest.Environment, loadedManifest.Environment)
	assert.Equal(t, manifest.Config.Concurrency, loadedManifest.Config.Concurrency)
	assert.Equal(t, manifest.Config.MaxRetries, loadedManifest.Config.MaxRetries)
	assert.Equal(t, manifest.Config.TimeoutSeconds, loadedManifest.Config.TimeoutSeconds)
	assert.Len(t, loadedManifest.Config.BrowserRequests, 1)

	// Create deployment from manifest
	deploymentReq := CreateDeploymentRequest{
		Name:        loadedManifest.Name,
		Version:     loadedManifest.Version,
		Runtime:     Runtime(loadedManifest.Runtime),
		PackageURL:  "https://example.com/package.zip",
		PackageHash: "hash123",
		Config:      loadedManifest.Config,
	}

	body, err := json.Marshal(deploymentReq)
	require.NoError(t, err)

	req, err := http.NewRequest("POST", "/api/v1/deployments", bytes.NewBuffer(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	suite.Router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code)

	var deployment Deployment
	err = json.Unmarshal(rr.Body.Bytes(), &deployment)
	require.NoError(t, err)

	// Verify deployment config
	var deploymentConfig DeploymentConfig
	err = json.Unmarshal(deployment.Config, &deploymentConfig)
	require.NoError(t, err)
	assert.Equal(t, manifest.Config.Concurrency, deploymentConfig.Concurrency)
	assert.Equal(t, manifest.Config.MaxRetries, deploymentConfig.MaxRetries)
	assert.Equal(t, manifest.Config.TimeoutSeconds, deploymentConfig.TimeoutSeconds)
	assert.Len(t, deploymentConfig.BrowserRequests, 1)
	assert.Equal(t, "chrome", deploymentConfig.BrowserRequests[0].Browser)
}

func TestIntegration_ErrorHandling(t *testing.T) {
	suite := setupIntegrationTest(t)

	// Test creating deployment with invalid data
	invalidDeployment := CreateDeploymentRequest{
		Name:    "invalid-deployment",
		Version: "1.0.0",
		// Missing required fields
	}

	body, err := json.Marshal(invalidDeployment)
	require.NoError(t, err)

	req, err := http.NewRequest("POST", "/api/v1/deployments", bytes.NewBuffer(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	suite.Router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)

	var errorResponse ErrorResponse
	err = json.Unmarshal(rr.Body.Bytes(), &errorResponse)
	require.NoError(t, err)
	assert.NotEmpty(t, errorResponse.Error)

	// Test accessing non-existent deployment
	req, err = http.NewRequest("GET", "/api/v1/deployments/"+uuid.New().String(), nil)
	require.NoError(t, err)

	rr = httptest.NewRecorder()
	suite.Router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)

	// Test creating run for non-existent deployment
	runReq := CreateDeploymentRunRequest{}
	body, err = json.Marshal(runReq)
	require.NoError(t, err)

	req, err = http.NewRequest("POST", "/api/v1/deployments/"+uuid.New().String()+"/runs", bytes.NewBuffer(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	rr = httptest.NewRecorder()
	suite.Router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)

	// Test invalid UUID format
	req, err = http.NewRequest("GET", "/api/v1/deployments/invalid-uuid", nil)
	require.NoError(t, err)

	rr = httptest.NewRecorder()
	suite.Router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestIntegration_ConcurrentOperations(t *testing.T) {
	suite := setupIntegrationTest(t)

	const numDeployments = 5
	const numRuns = 3

	// Create deployments concurrently
	deploymentChan := make(chan Deployment, numDeployments)
	var wg sync.WaitGroup

	for i := 0; i < numDeployments; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()

			deploymentReq := CreateDeploymentRequest{
				Name:        fmt.Sprintf("concurrent-deployment-%d", i),
				Version:     "1.0.0",
				Runtime:     RuntimeNode,
				PackageURL:  fmt.Sprintf("https://example.com/package%d.zip", i),
				PackageHash: fmt.Sprintf("hash%d", i),
			}

			body, err := json.Marshal(deploymentReq)
			require.NoError(t, err)

			req, err := http.NewRequest("POST", "/api/v1/deployments", bytes.NewBuffer(body))
			require.NoError(t, err)
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			suite.Router.ServeHTTP(rr, req)

			assert.Equal(t, http.StatusCreated, rr.Code)

			var deployment Deployment
			err = json.Unmarshal(rr.Body.Bytes(), &deployment)
			require.NoError(t, err)

			deploymentChan <- deployment
		}(i)
	}

	wg.Wait()
	close(deploymentChan)

	// Collect created deployments
	var deployments []Deployment
	for deployment := range deploymentChan {
		deployments = append(deployments, deployment)
	}

	assert.Len(t, deployments, numDeployments)

	// Create runs concurrently for each deployment
	runChan := make(chan DeploymentRun, numDeployments*numRuns)

	for _, deployment := range deployments {
		for j := 0; j < numRuns; j++ {
			wg.Add(1)
			go func(deploymentID uuid.UUID, runIndex int) {
				defer wg.Done()

				runReq := CreateDeploymentRunRequest{
					Environment: map[string]string{
						"RUN_INDEX": fmt.Sprintf("%d", runIndex),
					},
				}

				body, err := json.Marshal(runReq)
				require.NoError(t, err)

				req, err := http.NewRequest("POST", "/api/v1/deployments/"+deploymentID.String()+"/runs", bytes.NewBuffer(body))
				require.NoError(t, err)
				req.Header.Set("Content-Type", "application/json")

				rr := httptest.NewRecorder()
				suite.Router.ServeHTTP(rr, req)

				assert.Equal(t, http.StatusCreated, rr.Code)

				var run DeploymentRun
				err = json.Unmarshal(rr.Body.Bytes(), &run)
				require.NoError(t, err)

				runChan <- run
			}(deployment.ID, j)
		}
	}

	wg.Wait()
	close(runChan)

	// Collect created runs
	var runs []DeploymentRun
	for run := range runChan {
		runs = append(runs, run)
	}

	assert.Len(t, runs, numDeployments*numRuns)

	// Verify all tasks were enqueued
	tasks := suite.TaskQueue.GetTasks()
	assert.Len(t, tasks, numDeployments*numRuns)

	// Verify all runs are in pending state
	for _, run := range runs {
		assert.Equal(t, RunStatusPending, run.Status)
	}
}

func TestIntegration_DatabaseConsistency(t *testing.T) {
	suite := setupIntegrationTest(t)
	ctx := context.Background()

	// Create deployment
	deployment := &Deployment{
		Name:        "consistency-test",
		Version:     "1.0.0",
		Runtime:     RuntimeNode,
		PackageURL:  "https://example.com/package.zip",
		PackageHash: "hash123",
		Status:      StatusActive,
	}
	err := suite.Store.CreateDeployment(ctx, deployment)
	require.NoError(t, err)

	// Create multiple runs
	runs := make([]*DeploymentRun, 3)
	for i := 0; i < 3; i++ {
		run := &DeploymentRun{
			DeploymentID: deployment.ID,
			Status:       RunStatusPending,
		}
		err = suite.Store.CreateDeploymentRun(ctx, run)
		require.NoError(t, err)
		runs[i] = run
	}

	// Complete some runs
	err = suite.Store.CompleteDeploymentRun(ctx, runs[0].ID, RunStatusCompleted, map[string]interface{}{"result": "success"}, nil)
	require.NoError(t, err)

	err = suite.Store.CompleteDeploymentRun(ctx, runs[1].ID, RunStatusFailed, nil, stringPtr("test error"))
	require.NoError(t, err)

	// Verify computed fields
	updatedDeployment, err := suite.Store.GetDeployment(ctx, deployment.ID)
	require.NoError(t, err)
	assert.Equal(t, 3, updatedDeployment.TotalRuns)
	assert.Equal(t, 1, updatedDeployment.SuccessfulRuns)
	assert.Equal(t, 1, updatedDeployment.FailedRuns)
	assert.NotNil(t, updatedDeployment.LastRunAt)

	// Test cascade deletion
	err = suite.Store.DeleteDeployment(ctx, deployment.ID)
	require.NoError(t, err)

	// Verify all runs were deleted
	for _, run := range runs {
		_, err = suite.Store.GetDeploymentRun(ctx, run.ID)
		assert.Error(t, err)
	}
}

func TestIntegration_PerformanceMetrics(t *testing.T) {
	suite := setupIntegrationTest(t)
	ctx := context.Background()

	// Create deployment
	deployment := &Deployment{
		Name:        "performance-test",
		Version:     "1.0.0",
		Runtime:     RuntimeNode,
		PackageURL:  "https://example.com/package.zip",
		PackageHash: "hash123",
		Status:      StatusActive,
	}
	err := suite.Store.CreateDeployment(ctx, deployment)
	require.NoError(t, err)

	// Create runs with different durations
	baseTime := time.Now()
	runs := []*DeploymentRun{
		{
			DeploymentID: deployment.ID,
			Status:       RunStatusCompleted,
			StartedAt:    baseTime,
			CompletedAt:  &[]time.Time{baseTime.Add(30 * time.Second)}[0],
		},
		{
			DeploymentID: deployment.ID,
			Status:       RunStatusCompleted,
			StartedAt:    baseTime.Add(1 * time.Minute),
			CompletedAt:  &[]time.Time{baseTime.Add(90 * time.Second)}[0],
		},
		{
			DeploymentID: deployment.ID,
			Status:       RunStatusCompleted,
			StartedAt:    baseTime.Add(2 * time.Minute),
			CompletedAt:  &[]time.Time{baseTime.Add(3 * time.Minute)}[0],
		},
	}

	for _, run := range runs {
		err = suite.Store.CreateDeploymentRun(ctx, run)
		require.NoError(t, err)
	}

	// Get deployment statistics
	stats, err := suite.Store.GetDeploymentStats(ctx, deployment.ID)
	require.NoError(t, err)

	// Verify statistics
	assert.Contains(t, stats, "run_stats")
	assert.Contains(t, stats, "recent_runs")
	assert.Contains(t, stats, "avg_duration")

	// Check average duration
	avgDuration, ok := stats["avg_duration"].(*float64)
	assert.True(t, ok)
	assert.NotNil(t, avgDuration)
	assert.Greater(t, *avgDuration, 0.0)

	// Check recent runs
	recentRuns, ok := stats["recent_runs"].([]DeploymentRun)
	assert.True(t, ok)
	assert.Len(t, recentRuns, 3)

	// Verify run duration calculation
	for _, run := range recentRuns {
		assert.NotNil(t, run.Duration)
		assert.Greater(t, *run.Duration, int64(0))
	}
}

func BenchmarkIntegration_DeploymentCreation(b *testing.B) {
	suite := setupIntegrationTest(&testing.T{})

	deploymentReq := CreateDeploymentRequest{
		Name:        "benchmark-deployment",
		Version:     "1.0.0",
		Runtime:     RuntimeNode,
		PackageURL:  "https://example.com/package.zip",
		PackageHash: "hash123",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Make each request unique
		req := deploymentReq
		req.Name = fmt.Sprintf("benchmark-deployment-%d", i)
		req.PackageHash = fmt.Sprintf("hash%d", i)

		body, _ := json.Marshal(req)
		httpReq, _ := http.NewRequest("POST", "/api/v1/deployments", bytes.NewBuffer(body))
		httpReq.Header.Set("Content-Type", "application/json")

		rr := httptest.NewRecorder()
		suite.Router.ServeHTTP(rr, httpReq)

		if rr.Code != http.StatusCreated {
			b.Errorf("Expected status %d, got %d", http.StatusCreated, rr.Code)
		}
	}
}

func BenchmarkIntegration_DeploymentRunCreation(b *testing.B) {
	suite := setupIntegrationTest(&testing.T{})
	ctx := context.Background()

	// Create deployment
	deployment := &Deployment{
		Name:        "benchmark-deployment",
		Version:     "1.0.0",
		Runtime:     RuntimeNode,
		PackageURL:  "https://example.com/package.zip",
		PackageHash: "hash123",
		Status:      StatusActive,
	}
	suite.Store.CreateDeployment(ctx, deployment)

	runReq := CreateDeploymentRunRequest{}
	body, _ := json.Marshal(runReq)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		httpReq, _ := http.NewRequest("POST", "/api/v1/deployments/"+deployment.ID.String()+"/runs", bytes.NewBuffer(body))
		httpReq.Header.Set("Content-Type", "application/json")

		rr := httptest.NewRecorder()
		suite.Router.ServeHTTP(rr, httpReq)

		if rr.Code != http.StatusCreated {
			b.Errorf("Expected status %d, got %d", http.StatusCreated, rr.Code)
		}
	}
}
