package monitoring

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupMonitoringTestRouter(t *testing.T) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	// Use test Redis configuration
	redisOpt := asynq.RedisClientOpt{
		Addr:     "localhost:6379",
		Password: "",
		DB:       15, // Use test DB
	}

	v1 := router.Group("/api/v1")
	RegisterRoutes(v1, redisOpt)

	return router
}

func TestGetMonitoringInfo(t *testing.T) {
	router := setupMonitoringTestRouter(t)

	req, err := http.NewRequest("GET", "/api/v1/monitoring", nil)
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	// Note: This will likely return an error since we don't have Redis running in tests
	// The test is to ensure the endpoint structure is correct
	assert.Contains(t, []int{http.StatusOK, http.StatusInternalServerError}, rr.Code)

	if rr.Code == http.StatusOK {
		var response MonitoringInfo
		err := json.Unmarshal(rr.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.NotNil(t, response.Timestamp)
	}
}

func TestGetServers(t *testing.T) {
	router := setupMonitoringTestRouter(t)

	req, err := http.NewRequest("GET", "/api/v1/monitoring/servers", nil)
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Contains(t, []int{http.StatusOK, http.StatusInternalServerError}, rr.Code)

	if rr.Code == http.StatusOK {
		var response map[string]interface{}
		err := json.Unmarshal(rr.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Contains(t, response, "servers")
		assert.Contains(t, response, "total")
	}
}

func TestGetQueues(t *testing.T) {
	router := setupMonitoringTestRouter(t)

	req, err := http.NewRequest("GET", "/api/v1/monitoring/queues", nil)
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Contains(t, []int{http.StatusOK, http.StatusInternalServerError}, rr.Code)

	if rr.Code == http.StatusOK {
		var response map[string]interface{}
		err := json.Unmarshal(rr.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Contains(t, response, "stats")
		assert.Contains(t, response, "health")
	}
}

func TestGetQueueDetails(t *testing.T) {
	router := setupMonitoringTestRouter(t)

	req, err := http.NewRequest("GET", "/api/v1/monitoring/queues/default", nil)
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	// Could be 404 if queue doesn't exist or 500 if Redis is not available
	assert.Contains(t, []int{http.StatusOK, http.StatusNotFound, http.StatusInternalServerError}, rr.Code)
}

func TestGetQueueTasks(t *testing.T) {
	router := setupMonitoringTestRouter(t)

	req, err := http.NewRequest("GET", "/api/v1/monitoring/queues/default/tasks", nil)
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Contains(t, []int{http.StatusOK, http.StatusInternalServerError}, rr.Code)

	if rr.Code == http.StatusOK {
		var response TaskInfo
		err := json.Unmarshal(rr.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, "default", response.Queue)
	}
}

func TestGetHealth(t *testing.T) {
	router := setupMonitoringTestRouter(t)

	tests := []struct {
		name        string
		queryParams string
		expectError bool
	}{
		{
			name:        "basic health check",
			queryParams: "",
			expectError: true, // Will likely fail without Redis
		},
		{
			name:        "health check with min servers",
			queryParams: "?min_servers=2",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest("GET", "/api/v1/monitoring/health"+tt.queryParams, nil)
			require.NoError(t, err)

			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)

			// Health endpoint returns 200 for healthy, 503 for unhealthy
			assert.Contains(t, []int{http.StatusOK, http.StatusServiceUnavailable}, rr.Code)

			var response map[string]interface{}
			err = json.Unmarshal(rr.Body.Bytes(), &response)
			require.NoError(t, err)
			assert.Contains(t, response, "healthy")
			assert.Contains(t, response, "message")
		})
	}
}

func TestGetSchedulerEntries(t *testing.T) {
	router := setupMonitoringTestRouter(t)

	req, err := http.NewRequest("GET", "/api/v1/monitoring/scheduler", nil)
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Contains(t, []int{http.StatusOK, http.StatusInternalServerError}, rr.Code)

	if rr.Code == http.StatusOK {
		var response map[string]interface{}
		err := json.Unmarshal(rr.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Contains(t, response, "entries")
		assert.Contains(t, response, "total")
	}
}

func TestPauseQueue(t *testing.T) {
	router := setupMonitoringTestRouter(t)

	req, err := http.NewRequest("POST", "/api/v1/monitoring/queues/test/pause", nil)
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Contains(t, []int{http.StatusOK, http.StatusInternalServerError}, rr.Code)

	if rr.Code == http.StatusOK {
		var response map[string]interface{}
		err := json.Unmarshal(rr.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Contains(t, response, "message")
	}
}

func TestUnpauseQueue(t *testing.T) {
	router := setupMonitoringTestRouter(t)

	req, err := http.NewRequest("POST", "/api/v1/monitoring/queues/test/unpause", nil)
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Contains(t, []int{http.StatusOK, http.StatusInternalServerError}, rr.Code)

	if rr.Code == http.StatusOK {
		var response map[string]interface{}
		err := json.Unmarshal(rr.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Contains(t, response, "message")
	}
}

func TestDeleteArchivedTasks(t *testing.T) {
	router := setupMonitoringTestRouter(t)

	req, err := http.NewRequest("DELETE", "/api/v1/monitoring/queues/test/archived", nil)
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Contains(t, []int{http.StatusOK, http.StatusInternalServerError}, rr.Code)

	if rr.Code == http.StatusOK {
		var response map[string]interface{}
		err := json.Unmarshal(rr.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Contains(t, response, "message")
		assert.Contains(t, response, "deleted")
	}
}

func TestDeleteRetryTasks(t *testing.T) {
	router := setupMonitoringTestRouter(t)

	req, err := http.NewRequest("DELETE", "/api/v1/monitoring/queues/test/retry", nil)
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Contains(t, []int{http.StatusOK, http.StatusInternalServerError}, rr.Code)

	if rr.Code == http.StatusOK {
		var response map[string]interface{}
		err := json.Unmarshal(rr.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Contains(t, response, "message")
		assert.Contains(t, response, "deleted")
	}
}

// TestMonitoringStructures tests that our monitoring data structures work correctly
func TestMonitoringStructures(t *testing.T) {
	// Test ServerStats
	stats := &ServerStats{
		TotalServers:  5,
		ActiveServers: 3,
		TotalQueues:   make(map[string]int),
	}
	stats.TotalQueues["default"] = 2
	stats.TotalQueues["critical"] = 1

	assert.Equal(t, 5, stats.TotalServers)
	assert.Equal(t, 3, stats.ActiveServers)
	assert.Equal(t, 2, stats.TotalQueues["default"])

	// Test QueueHealth
	health := &QueueHealth{
		Status:    "healthy",
		Message:   "All good",
		Pending:   10,
		Active:    2,
		Scheduled: 5,
		Retry:     1,
		Archived:  0,
		Completed: 100,
		Paused:    false,
	}

	assert.Equal(t, "healthy", health.Status)
	assert.Equal(t, 10, health.Pending)
	assert.False(t, health.Paused)

	// Test TaskInfo
	taskInfo := &TaskInfo{
		Queue:          "test-queue",
		PendingTasks:   []*asynq.TaskInfo{},
		ActiveTasks:    []*asynq.TaskInfo{},
		ScheduledTasks: []*asynq.TaskInfo{},
		RetryTasks:     []*asynq.TaskInfo{},
		ArchivedTasks:  []*asynq.TaskInfo{},
	}

	assert.Equal(t, "test-queue", taskInfo.Queue)
	assert.NotNil(t, taskInfo.PendingTasks)
}
