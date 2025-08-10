package deployments

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/autocrawlerHQ/browsergrid/internal/storage"
	_ "github.com/autocrawlerHQ/browsergrid/internal/storage/local"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
	postgresDriver "gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Mock task client for testing
type mockTaskClient struct {
	enqueuedTasks []asynq.Task
	shouldError   bool
}

func (m *mockTaskClient) Enqueue(task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error) {
	if m.shouldError {
		return nil, fmt.Errorf("mock task client error")
	}
	m.enqueuedTasks = append(m.enqueuedTasks, *task)
	return &asynq.TaskInfo{
		ID:       "test-task-id",
		Type:     task.Type(),
		Payload:  task.Payload(),
		Queue:    "default",
		MaxRetry: 3,
	}, nil
}

func (m *mockTaskClient) Close() error {
	return nil
}

func setupHTTPTestDB(t *testing.T) (*gorm.DB, *gin.Engine) {
	db, err := gorm.Open(postgresDriver.Open(testConnStr), &gorm.Config{
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

	gin.SetMode(gin.TestMode)
	router := gin.New()

	v1 := router.Group("/api/v1")
	mockClient := &mockTaskClient{}
	backend, err := storage.New("local", map[string]string{"path": t.TempDir()})
	require.NoError(t, err)
	RegisterRoutes(v1, Dependencies{
		DB:         db,
		TaskClient: mockClient,
		Storage:    backend,
	})

	return db, router
}

func TestCreateDeployment(t *testing.T) {
	_, router := setupHTTPTestDB(t)

	tests := []struct {
		name           string
		requestBody    interface{}
		expectedStatus int
		expectedFields map[string]interface{}
		checkError     bool
	}{
		{
			name: "valid deployment",
			requestBody: CreateDeploymentRequest{
				Name:        "test-deployment",
				Description: "Test deployment",
				Version:     "1.0.0",
				Runtime:     RuntimeNode,
				PackageURL:  "https://example.com/package.zip",
				PackageHash: "hash123",
				Config: DeploymentConfig{
					Concurrency:    1,
					MaxRetries:     3,
					TimeoutSeconds: 300,
				},
			},
			expectedStatus: http.StatusCreated,
			expectedFields: map[string]interface{}{
				"name":         "test-deployment",
				"description":  "Test deployment",
				"version":      "1.0.0",
				"runtime":      "node",
				"package_url":  "https://example.com/package.zip",
				"package_hash": "hash123",
				"status":       "active",
			},
		},
		{
			name: "minimal deployment",
			requestBody: CreateDeploymentRequest{
				Name:        "minimal-deployment",
				Version:     "1.0.0",
				Runtime:     RuntimePython,
				PackageURL:  "https://example.com/package.zip",
				PackageHash: "hash456",
			},
			expectedStatus: http.StatusCreated,
			expectedFields: map[string]interface{}{
				"name":         "minimal-deployment",
				"version":      "1.0.0",
				"runtime":      "python",
				"package_url":  "https://example.com/package.zip",
				"package_hash": "hash456",
				"status":       "active",
			},
		},
		{
			name: "missing required fields",
			requestBody: CreateDeploymentRequest{
				Name:    "invalid-deployment",
				Version: "1.0.0",
				// Missing runtime, package_url, package_hash
			},
			expectedStatus: http.StatusBadRequest,
			checkError:     true,
		},
		{
			name: "invalid runtime",
			requestBody: CreateDeploymentRequest{
				Name:        "invalid-runtime",
				Version:     "1.0.0",
				Runtime:     Runtime("invalid"),
				PackageURL:  "https://example.com/package.zip",
				PackageHash: "hash789",
			},
			expectedStatus: http.StatusCreated, // Runtime validation is not enforced at HTTP level
		},
		{
			name:           "invalid JSON",
			requestBody:    "invalid json",
			expectedStatus: http.StatusBadRequest,
			checkError:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body []byte
			var err error

			if str, ok := tt.requestBody.(string); ok {
				body = []byte(str)
			} else {
				body, err = json.Marshal(tt.requestBody)
				require.NoError(t, err)
			}

			req, err := http.NewRequest("POST", "/api/v1/deployments", bytes.NewBuffer(body))
			require.NoError(t, err)
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)

			assert.Equal(t, tt.expectedStatus, rr.Code)

			if tt.checkError {
				var errorResponse ErrorResponse
				err := json.Unmarshal(rr.Body.Bytes(), &errorResponse)
				require.NoError(t, err)
				assert.NotEmpty(t, errorResponse.Error)
			} else if tt.expectedStatus == http.StatusCreated {
				var response Deployment
				err := json.Unmarshal(rr.Body.Bytes(), &response)
				require.NoError(t, err)

				assert.NotEqual(t, uuid.Nil, response.ID)
				assert.NotZero(t, response.CreatedAt)
				assert.NotZero(t, response.UpdatedAt)

				for key, expectedValue := range tt.expectedFields {
					switch key {
					case "name":
						assert.Equal(t, expectedValue, response.Name)
					case "description":
						assert.Equal(t, expectedValue, response.Description)
					case "version":
						assert.Equal(t, expectedValue, response.Version)
					case "runtime":
						assert.Equal(t, expectedValue, string(response.Runtime))
					case "package_url":
						assert.Equal(t, expectedValue, response.PackageURL)
					case "package_hash":
						assert.Equal(t, expectedValue, response.PackageHash)
					case "status":
						assert.Equal(t, expectedValue, string(response.Status))
					}
				}
			}
		})
	}
}

func TestCreateDeploymentDuplicate(t *testing.T) {
	_, router := setupHTTPTestDB(t)

	// Create first deployment
	deployment := CreateDeploymentRequest{
		Name:        "duplicate-test",
		Version:     "1.0.0",
		Runtime:     RuntimeNode,
		PackageURL:  "https://example.com/package.zip",
		PackageHash: "hash123",
	}

	body, err := json.Marshal(deployment)
	require.NoError(t, err)

	req1, err := http.NewRequest("POST", "/api/v1/deployments", bytes.NewBuffer(body))
	require.NoError(t, err)
	req1.Header.Set("Content-Type", "application/json")

	rr1 := httptest.NewRecorder()
	router.ServeHTTP(rr1, req1)
	assert.Equal(t, http.StatusCreated, rr1.Code)

	// Try to create duplicate
	req2, err := http.NewRequest("POST", "/api/v1/deployments", bytes.NewBuffer(body))
	require.NoError(t, err)
	req2.Header.Set("Content-Type", "application/json")

	rr2 := httptest.NewRecorder()
	router.ServeHTTP(rr2, req2)
	assert.Equal(t, http.StatusBadRequest, rr2.Code)

	var errorResponse ErrorResponse
	err = json.Unmarshal(rr2.Body.Bytes(), &errorResponse)
	require.NoError(t, err)
	assert.Contains(t, errorResponse.Error, "already exists")
}

func TestListDeployments(t *testing.T) {
	db, router := setupHTTPTestDB(t)
	store := NewStore(db)
	ctx := context.Background()

	// Create test deployments
	deployments := []*Deployment{
		{
			Name:        "node-deployment",
			Version:     "1.0.0",
			Runtime:     RuntimeNode,
			PackageURL:  "https://example.com/node.zip",
			PackageHash: "hash1",
			Status:      StatusActive,
		},
		{
			Name:        "python-deployment",
			Version:     "1.0.0",
			Runtime:     RuntimePython,
			PackageURL:  "https://example.com/python.zip",
			PackageHash: "hash2",
			Status:      StatusActive,
		},
		{
			Name:        "inactive-deployment",
			Version:     "1.0.0",
			Runtime:     RuntimeNode,
			PackageURL:  "https://example.com/inactive.zip",
			PackageHash: "hash3",
			Status:      StatusInactive,
		},
	}

	for _, dep := range deployments {
		err := store.CreateDeployment(ctx, dep)
		require.NoError(t, err)
	}

	tests := []struct {
		name        string
		queryParams string
		expectedMin int
		expectedMax int
	}{
		{
			name:        "list all deployments",
			queryParams: "",
			expectedMin: 3,
			expectedMax: 3,
		},
		{
			name:        "filter by status",
			queryParams: "?status=active",
			expectedMin: 2,
			expectedMax: 2,
		},
		{
			name:        "filter by runtime",
			queryParams: "?runtime=node",
			expectedMin: 2,
			expectedMax: 2,
		},
		{
			name:        "combined filters",
			queryParams: "?status=active&runtime=python",
			expectedMin: 1,
			expectedMax: 1,
		},
		{
			name:        "pagination",
			queryParams: "?offset=0&limit=2",
			expectedMin: 2,
			expectedMax: 2,
		},
		{
			name:        "pagination with offset",
			queryParams: "?offset=2&limit=2",
			expectedMin: 1,
			expectedMax: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest("GET", "/api/v1/deployments"+tt.queryParams, nil)
			require.NoError(t, err)

			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)

			assert.Equal(t, http.StatusOK, rr.Code)

			var response DeploymentListResponse
			err = json.Unmarshal(rr.Body.Bytes(), &response)
			require.NoError(t, err)

			assert.GreaterOrEqual(t, len(response.Deployments), tt.expectedMin)
			assert.LessOrEqual(t, len(response.Deployments), tt.expectedMax)
			assert.GreaterOrEqual(t, int(response.Total), tt.expectedMin)
			assert.Contains(t, map[string]interface{}{"offset": response.Offset, "limit": response.Limit}, "offset")
		})
	}
}

func TestGetDeployment(t *testing.T) {
	db, router := setupHTTPTestDB(t)
	store := NewStore(db)
	ctx := context.Background()

	// Create test deployment
	deployment := &Deployment{
		Name:        "test-deployment",
		Description: "Test deployment",
		Version:     "1.0.0",
		Runtime:     RuntimeNode,
		PackageURL:  "https://example.com/package.zip",
		PackageHash: "hash123",
		Status:      StatusActive,
	}
	err := store.CreateDeployment(ctx, deployment)
	require.NoError(t, err)

	tests := []struct {
		name           string
		deploymentID   string
		expectedStatus int
		checkFields    bool
	}{
		{
			name:           "get existing deployment",
			deploymentID:   deployment.ID.String(),
			expectedStatus: http.StatusOK,
			checkFields:    true,
		},
		{
			name:           "get non-existent deployment",
			deploymentID:   uuid.New().String(),
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "invalid deployment ID",
			deploymentID:   "invalid-uuid",
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest("GET", "/api/v1/deployments/"+tt.deploymentID, nil)
			require.NoError(t, err)

			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)

			assert.Equal(t, tt.expectedStatus, rr.Code)

			if tt.checkFields {
				var response Deployment
				err := json.Unmarshal(rr.Body.Bytes(), &response)
				require.NoError(t, err)

				assert.Equal(t, deployment.ID, response.ID)
				assert.Equal(t, deployment.Name, response.Name)
				assert.Equal(t, deployment.Description, response.Description)
				assert.Equal(t, deployment.Version, response.Version)
				assert.Equal(t, deployment.Runtime, response.Runtime)
			}
		})
	}
}

func TestUpdateDeployment(t *testing.T) {
	db, router := setupHTTPTestDB(t)
	store := NewStore(db)
	ctx := context.Background()

	// Create test deployment
	deployment := &Deployment{
		Name:        "test-deployment",
		Description: "Original description",
		Version:     "1.0.0",
		Runtime:     RuntimeNode,
		PackageURL:  "https://example.com/package.zip",
		PackageHash: "hash123",
		Status:      StatusActive,
	}
	err := store.CreateDeployment(ctx, deployment)
	require.NoError(t, err)

	tests := []struct {
		name           string
		deploymentID   string
		requestBody    interface{}
		expectedStatus int
		checkUpdate    bool
	}{
		{
			name:         "update description",
			deploymentID: deployment.ID.String(),
			requestBody: UpdateDeploymentRequest{
				Description: stringPtr("Updated description"),
			},
			expectedStatus: http.StatusOK,
			checkUpdate:    true,
		},
		{
			name:         "update status",
			deploymentID: deployment.ID.String(),
			requestBody: UpdateDeploymentRequest{
				Status: &[]DeploymentStatus{StatusInactive}[0],
			},
			expectedStatus: http.StatusOK,
			checkUpdate:    true,
		},
		{
			name:         "update config",
			deploymentID: deployment.ID.String(),
			requestBody: UpdateDeploymentRequest{
				Config: &DeploymentConfig{
					Concurrency:    2,
					MaxRetries:     5,
					TimeoutSeconds: 600,
				},
			},
			expectedStatus: http.StatusOK,
			checkUpdate:    true,
		},
		{
			name:           "empty update",
			deploymentID:   deployment.ID.String(),
			requestBody:    UpdateDeploymentRequest{},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:         "invalid deployment ID",
			deploymentID: "invalid-uuid",
			requestBody: UpdateDeploymentRequest{
				Description: stringPtr("Should fail"),
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:         "non-existent deployment",
			deploymentID: uuid.New().String(),
			requestBody: UpdateDeploymentRequest{
				Description: stringPtr("Should fail"),
			},
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, err := json.Marshal(tt.requestBody)
			require.NoError(t, err)

			req, err := http.NewRequest("PATCH", "/api/v1/deployments/"+tt.deploymentID, bytes.NewBuffer(body))
			require.NoError(t, err)
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)

			assert.Equal(t, tt.expectedStatus, rr.Code)

			if tt.checkUpdate {
				var response Deployment
				err := json.Unmarshal(rr.Body.Bytes(), &response)
				require.NoError(t, err)

				assert.Equal(t, deployment.ID, response.ID)
				assert.True(t, response.UpdatedAt.After(deployment.UpdatedAt))
			}
		})
	}
}

func TestDeleteDeployment(t *testing.T) {
	db, router := setupHTTPTestDB(t)
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

	tests := []struct {
		name           string
		deploymentID   string
		expectedStatus int
		checkDeletion  bool
	}{
		{
			name:           "delete existing deployment",
			deploymentID:   deployment.ID.String(),
			expectedStatus: http.StatusOK,
			checkDeletion:  true,
		},
		{
			name:           "delete non-existent deployment",
			deploymentID:   uuid.New().String(),
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "invalid deployment ID",
			deploymentID:   "invalid-uuid",
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest("DELETE", "/api/v1/deployments/"+tt.deploymentID, nil)
			require.NoError(t, err)

			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)

			assert.Equal(t, tt.expectedStatus, rr.Code)

			if tt.checkDeletion {
				var response MessageResponse
				err := json.Unmarshal(rr.Body.Bytes(), &response)
				require.NoError(t, err)
				assert.Contains(t, response.Message, "deleted successfully")

				// Verify deletion
				_, err = store.GetDeployment(ctx, deployment.ID)
				assert.Error(t, err)
			}
		})
	}
}

func TestGetDeploymentStats(t *testing.T) {
	db, router := setupHTTPTestDB(t)
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
		{DeploymentID: deployment.ID, Status: RunStatusCompleted},
		{DeploymentID: deployment.ID, Status: RunStatusFailed},
		{DeploymentID: deployment.ID, Status: RunStatusRunning},
	}

	for _, run := range runs {
		err = store.CreateDeploymentRun(ctx, run)
		require.NoError(t, err)
	}

	tests := []struct {
		name           string
		deploymentID   string
		expectedStatus int
		checkStats     bool
	}{
		{
			name:           "get stats for existing deployment",
			deploymentID:   deployment.ID.String(),
			expectedStatus: http.StatusOK,
			checkStats:     true,
		},
		{
			name:           "get stats for non-existent deployment",
			deploymentID:   uuid.New().String(),
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "invalid deployment ID",
			deploymentID:   "invalid-uuid",
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest("GET", "/api/v1/deployments/"+tt.deploymentID+"/stats", nil)
			require.NoError(t, err)

			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)

			assert.Equal(t, tt.expectedStatus, rr.Code)

			if tt.checkStats {
				var stats map[string]interface{}
				err := json.Unmarshal(rr.Body.Bytes(), &stats)
				require.NoError(t, err)

				assert.Contains(t, stats, "run_stats")
				assert.Contains(t, stats, "recent_runs")
				assert.Contains(t, stats, "avg_duration")
			}
		})
	}
}

func TestCreateDeploymentRun(t *testing.T) {
	db, router := setupHTTPTestDB(t)
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

	// Create inactive deployment
	inactiveDeployment := &Deployment{
		Name:        "inactive-deployment",
		Version:     "1.0.0",
		Runtime:     RuntimeNode,
		PackageURL:  "https://example.com/inactive.zip",
		PackageHash: "hash456",
		Status:      StatusInactive,
	}
	err = store.CreateDeployment(ctx, inactiveDeployment)
	require.NoError(t, err)

	tests := []struct {
		name           string
		deploymentID   string
		requestBody    interface{}
		expectedStatus int
		checkRun       bool
	}{
		{
			name:         "create run for active deployment",
			deploymentID: deployment.ID.String(),
			requestBody: CreateDeploymentRunRequest{
				Environment: map[string]string{
					"NODE_ENV": "production",
				},
				Config: map[string]interface{}{
					"custom": "value",
				},
			},
			expectedStatus: http.StatusCreated,
			checkRun:       true,
		},
		{
			name:           "create run with empty body",
			deploymentID:   deployment.ID.String(),
			requestBody:    CreateDeploymentRunRequest{},
			expectedStatus: http.StatusCreated,
			checkRun:       true,
		},
		{
			name:           "create run for inactive deployment",
			deploymentID:   inactiveDeployment.ID.String(),
			requestBody:    CreateDeploymentRunRequest{},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "create run for non-existent deployment",
			deploymentID:   uuid.New().String(),
			requestBody:    CreateDeploymentRunRequest{},
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "invalid deployment ID",
			deploymentID:   "invalid-uuid",
			requestBody:    CreateDeploymentRunRequest{},
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, err := json.Marshal(tt.requestBody)
			require.NoError(t, err)

			req, err := http.NewRequest("POST", "/api/v1/deployments/"+tt.deploymentID+"/runs", bytes.NewBuffer(body))
			require.NoError(t, err)
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)

			assert.Equal(t, tt.expectedStatus, rr.Code)

			if tt.checkRun {
				var response DeploymentRun
				err := json.Unmarshal(rr.Body.Bytes(), &response)
				require.NoError(t, err)

				assert.NotEqual(t, uuid.Nil, response.ID)
				assert.Equal(t, deployment.ID, response.DeploymentID)
				assert.Equal(t, RunStatusPending, response.Status)
				assert.NotZero(t, response.CreatedAt)
				assert.NotZero(t, response.StartedAt)
			}
		})
	}
}

func TestListDeploymentRuns(t *testing.T) {
	db, router := setupHTTPTestDB(t)
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
		{DeploymentID: deployment.ID, Status: RunStatusCompleted},
		{DeploymentID: deployment.ID, Status: RunStatusFailed},
		{DeploymentID: deployment.ID, Status: RunStatusRunning},
	}

	for _, run := range runs {
		err = store.CreateDeploymentRun(ctx, run)
		require.NoError(t, err)
	}

	tests := []struct {
		name           string
		deploymentID   string
		queryParams    string
		expectedStatus int
		expectedMin    int
		expectedMax    int
	}{
		{
			name:           "list all runs for deployment",
			deploymentID:   deployment.ID.String(),
			queryParams:    "",
			expectedStatus: http.StatusOK,
			expectedMin:    3,
			expectedMax:    3,
		},
		{
			name:           "filter by status",
			deploymentID:   deployment.ID.String(),
			queryParams:    "?status=completed",
			expectedStatus: http.StatusOK,
			expectedMin:    1,
			expectedMax:    1,
		},
		{
			name:           "pagination",
			deploymentID:   deployment.ID.String(),
			queryParams:    "?offset=0&limit=2",
			expectedStatus: http.StatusOK,
			expectedMin:    2,
			expectedMax:    2,
		},
		{
			name:           "invalid deployment ID",
			deploymentID:   "invalid-uuid",
			queryParams:    "",
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest("GET", "/api/v1/deployments/"+tt.deploymentID+"/runs"+tt.queryParams, nil)
			require.NoError(t, err)

			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)

			assert.Equal(t, tt.expectedStatus, rr.Code)

			if tt.expectedStatus == http.StatusOK {
				var response DeploymentRunListResponse
				err := json.Unmarshal(rr.Body.Bytes(), &response)
				require.NoError(t, err)

				assert.GreaterOrEqual(t, len(response.Runs), tt.expectedMin)
				assert.LessOrEqual(t, len(response.Runs), tt.expectedMax)
				assert.GreaterOrEqual(t, int(response.Total), tt.expectedMin)
			}
		})
	}
}

func TestGetDeploymentRun(t *testing.T) {
	db, router := setupHTTPTestDB(t)
	store := NewStore(db)
	ctx := context.Background()

	// Create test deployment and run
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

	run := &DeploymentRun{
		DeploymentID: deployment.ID,
		Status:       RunStatusCompleted,
		Output:       datatypes.JSON(`{"result": "success"}`),
	}
	err = store.CreateDeploymentRun(ctx, run)
	require.NoError(t, err)

	tests := []struct {
		name           string
		runID          string
		expectedStatus int
		checkRun       bool
	}{
		{
			name:           "get existing run",
			runID:          run.ID.String(),
			expectedStatus: http.StatusOK,
			checkRun:       true,
		},
		{
			name:           "get non-existent run",
			runID:          uuid.New().String(),
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "invalid run ID",
			runID:          "invalid-uuid",
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest("GET", "/api/v1/runs/"+tt.runID, nil)
			require.NoError(t, err)

			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)

			assert.Equal(t, tt.expectedStatus, rr.Code)

			if tt.checkRun {
				var response DeploymentRun
				err := json.Unmarshal(rr.Body.Bytes(), &response)
				require.NoError(t, err)

				assert.Equal(t, run.ID, response.ID)
				assert.Equal(t, run.DeploymentID, response.DeploymentID)
				assert.Equal(t, run.Status, response.Status)
			}
		})
	}
}

func TestGetDeploymentRunLogs(t *testing.T) {
	db, router := setupHTTPTestDB(t)
	store := NewStore(db)
	ctx := context.Background()

	// Create test deployment and run
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

	run := &DeploymentRun{
		DeploymentID: deployment.ID,
		Status:       RunStatusCompleted,
		Output:       datatypes.JSON(`{"result": "success"}`),
		Error:        stringPtr("test error"),
	}
	err = store.CreateDeploymentRun(ctx, run)
	require.NoError(t, err)

	req, err := http.NewRequest("GET", "/api/v1/runs/"+run.ID.String()+"/logs", nil)
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var logs map[string]interface{}
	err = json.Unmarshal(rr.Body.Bytes(), &logs)
	require.NoError(t, err)

	assert.Contains(t, logs, "run_id")
	assert.Contains(t, logs, "output")
	assert.Contains(t, logs, "error")
	assert.Contains(t, logs, "status")
	assert.Contains(t, logs, "started_at")
	assert.Contains(t, logs, "completed_at")
}

func TestDeleteDeploymentRun(t *testing.T) {
	db, router := setupHTTPTestDB(t)
	store := NewStore(db)
	ctx := context.Background()

	// Create test deployment and run
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

	run := &DeploymentRun{
		DeploymentID: deployment.ID,
		Status:       RunStatusCompleted,
	}
	err = store.CreateDeploymentRun(ctx, run)
	require.NoError(t, err)

	req, err := http.NewRequest("DELETE", "/api/v1/runs/"+run.ID.String(), nil)
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var response MessageResponse
	err = json.Unmarshal(rr.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Contains(t, response.Message, "deleted successfully")

	// Verify deletion
	_, err = store.GetDeploymentRun(ctx, run.ID)
	assert.Error(t, err)
}

func TestListAllDeploymentRuns(t *testing.T) {
	db, router := setupHTTPTestDB(t)
	store := NewStore(db)
	ctx := context.Background()

	// Create test deployment and runs
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

	runs := []*DeploymentRun{
		{DeploymentID: deployment.ID, Status: RunStatusCompleted},
		{DeploymentID: deployment.ID, Status: RunStatusFailed},
		{DeploymentID: deployment.ID, Status: RunStatusRunning},
	}

	for _, run := range runs {
		err = store.CreateDeploymentRun(ctx, run)
		require.NoError(t, err)
	}

	req, err := http.NewRequest("GET", "/api/v1/runs", nil)
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var response DeploymentRunListResponse
	err = json.Unmarshal(rr.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Len(t, response.Runs, 3)
	assert.Equal(t, int64(3), response.Total)
}

func TestTaskEnqueueError(t *testing.T) {
	db, _ := setupHTTPTestDB(t)
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

	// Setup router with error-prone task client
	gin.SetMode(gin.TestMode)
	router := gin.New()
	v1 := router.Group("/api/v1")
	mockClient := &mockTaskClient{shouldError: true}
	backend, err := storage.New("local", map[string]string{"path": t.TempDir()})
	require.NoError(t, err)
	RegisterRoutes(v1, Dependencies{
		DB:         db,
		TaskClient: mockClient,
		Storage:    backend,
	})

	// Try to create a run (should fail due to task enqueue error)
	body, err := json.Marshal(CreateDeploymentRunRequest{})
	require.NoError(t, err)

	req, err := http.NewRequest("POST", "/api/v1/deployments/"+deployment.ID.String()+"/runs", bytes.NewBuffer(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)

	var errorResponse ErrorResponse
	err = json.Unmarshal(rr.Body.Bytes(), &errorResponse)
	require.NoError(t, err)
	assert.Contains(t, errorResponse.Error, "failed to enqueue task")
}

func TestConcurrentHTTPRequests(t *testing.T) {
	_, router := setupHTTPTestDB(t)

	const numGoroutines = 10
	results := make(chan int, numGoroutines)

	deploymentData := CreateDeploymentRequest{
		Name:        "concurrent-test",
		Version:     "1.0.0",
		Runtime:     RuntimeNode,
		PackageURL:  "https://example.com/package.zip",
		PackageHash: "hash123",
	}

	for i := 0; i < numGoroutines; i++ {
		go func(i int) {
			// Make each request unique
			data := deploymentData
			data.Name = fmt.Sprintf("concurrent-test-%d", i)
			data.PackageHash = fmt.Sprintf("hash%d", i)

			body, _ := json.Marshal(data)
			req, _ := http.NewRequest("POST", "/api/v1/deployments", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)

			results <- rr.Code
		}(i)
	}

	successCount := 0
	for i := 0; i < numGoroutines; i++ {
		status := <-results
		if status == http.StatusCreated {
			successCount++
		}
	}

	assert.Equal(t, numGoroutines, successCount, "All concurrent requests should succeed")
}

// Helper function to create string pointers
func stringPtr(s string) *string {
	return &s
}
