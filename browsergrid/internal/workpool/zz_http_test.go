package workpool

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
	postgresDriver "gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/autocrawlerHQ/browsergrid/internal/sessions"
)

func setupHTTPTestDB(t *testing.T) (*gorm.DB, *gin.Engine) {
	db, err := gorm.Open(postgresDriver.Open(testConnStr), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)

	err = db.Migrator().DropTable(&WorkPool{}, &sessions.Session{}, &sessions.SessionEvent{}, &sessions.SessionMetrics{}, &sessions.Pool{})
	if err != nil {
		t.Logf("Warning: Failed to drop tables (may not exist): %v", err)
	}

	err = db.AutoMigrate(&WorkPool{}, &sessions.Session{}, &sessions.SessionEvent{}, &sessions.SessionMetrics{}, &sessions.Pool{})
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	router := gin.New()

	v1 := router.Group("/api/v1")
	RegisterRoutes(v1, db)

	return db, router
}

func TestCreateWorkPool(t *testing.T) {
	_, router := setupHTTPTestDB(t)

	tests := []struct {
		name           string
		requestBody    interface{}
		expectedStatus int
		expectedFields map[string]interface{}
	}{
		{
			name: "create valid work pool",
			requestBody: WorkPool{
				Name:            "test-pool",
				Description:     "Test pool description",
				Provider:        ProviderDocker,
				MinSize:         2,
				MaxConcurrency:  10,
				MaxIdleTime:     1800,
				AutoScale:       true,
				DefaultPriority: 5,
				QueueStrategy:   "fifo",
				DefaultEnv:      datatypes.JSON(`{"ENV": "test"}`),
			},
			expectedStatus: http.StatusCreated,
			expectedFields: map[string]interface{}{
				"name":             "test-pool",
				"provider":         "docker",
				"min_size":         2,
				"max_concurrency":  10,
				"auto_scale":       true,
				"default_priority": 5,
				"queue_strategy":   "fifo",
			},
		},
		{
			name: "create minimal work pool",
			requestBody: WorkPool{
				Name:           "minimal-pool",
				Provider:       ProviderLocal,
				MaxConcurrency: 5,
			},
			expectedStatus: http.StatusCreated,
			expectedFields: map[string]interface{}{
				"name":     "minimal-pool",
				"provider": "local",
			},
		},
		{
			name:           "create work pool with invalid JSON",
			requestBody:    "invalid json",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "create work pool with empty name",
			requestBody: map[string]interface{}{
				"provider":        "docker",
				"max_concurrency": 10,
			},
			expectedStatus: http.StatusBadRequest,
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

			req, err := http.NewRequest("POST", "/api/v1/workpools", bytes.NewBuffer(body))
			require.NoError(t, err)
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)

			assert.Equal(t, tt.expectedStatus, rr.Code)

			if tt.expectedStatus == http.StatusCreated {
				var response WorkPool
				err := json.Unmarshal(rr.Body.Bytes(), &response)
				require.NoError(t, err)

				assert.NotEqual(t, uuid.Nil, response.ID)

				for key, expectedValue := range tt.expectedFields {
					switch key {
					case "name":
						assert.Equal(t, expectedValue, response.Name)
					case "provider":
						assert.Equal(t, expectedValue, string(response.Provider))
					case "min_size":
						assert.Equal(t, expectedValue, response.MinSize)
					case "max_concurrency":
						assert.Equal(t, expectedValue, response.MaxConcurrency)
					case "auto_scale":
						assert.Equal(t, expectedValue, response.AutoScale)
					case "default_priority":
						assert.Equal(t, expectedValue, response.DefaultPriority)
					case "queue_strategy":
						assert.Equal(t, expectedValue, response.QueueStrategy)
					}
				}
			}
		})
	}
}

func TestListWorkPools(t *testing.T) {
	db, router := setupHTTPTestDB(t)
	store := NewStore(db)

	ctx := context.Background()

	testPools := []*WorkPool{
		{
			Name:           "active-pool",
			Provider:       ProviderDocker,
			MaxConcurrency: 10,
			Paused:         false,
		},
		{
			Name:           "paused-pool",
			Provider:       ProviderLocal,
			MaxConcurrency: 5,
			Paused:         true,
		},
	}

	for _, pool := range testPools {
		err := store.CreateWorkPool(ctx, pool)
		require.NoError(t, err)
	}

	tests := []struct {
		name           string
		queryParams    string
		expectedStatus int
		minPools       int
		maxPools       int
		pausedFilter   *bool
	}{
		{
			name:           "list all work pools",
			queryParams:    "",
			expectedStatus: http.StatusOK,
			minPools:       2,
			maxPools:       2,
		},
		{
			name:           "filter by active pools",
			queryParams:    "?paused=false",
			expectedStatus: http.StatusOK,
			minPools:       1,
			maxPools:       1,
			pausedFilter:   &[]bool{false}[0],
		},
		{
			name:           "filter by paused pools",
			queryParams:    "?paused=true",
			expectedStatus: http.StatusOK,
			minPools:       1,
			maxPools:       1,
			pausedFilter:   &[]bool{true}[0],
		},
		{
			name:           "invalid paused parameter",
			queryParams:    "?paused=invalid",
			expectedStatus: http.StatusOK,
			minPools:       2,
			maxPools:       2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest("GET", "/api/v1/workpools"+tt.queryParams, nil)
			require.NoError(t, err)

			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)

			assert.Equal(t, tt.expectedStatus, rr.Code)

			if tt.expectedStatus == http.StatusOK {
				var response map[string]interface{}
				err := json.Unmarshal(rr.Body.Bytes(), &response)
				require.NoError(t, err)

				pools, ok := response["pools"].([]interface{})
				require.True(t, ok)

				assert.GreaterOrEqual(t, len(pools), tt.minPools)
				assert.LessOrEqual(t, len(pools), tt.maxPools)

				if tt.pausedFilter != nil {
					for _, poolInterface := range pools {
						poolMap, ok := poolInterface.(map[string]interface{})
						require.True(t, ok)
						paused, ok := poolMap["paused"].(bool)
						require.True(t, ok)
						assert.Equal(t, *tt.pausedFilter, paused)
					}
				}

				assert.Contains(t, response, "total")
			}
		})
	}
}

func TestGetWorkPool(t *testing.T) {
	db, router := setupHTTPTestDB(t)
	store := NewStore(db)

	ctx := context.Background()

	testPool := &WorkPool{
		Name:           "test-pool",
		Provider:       ProviderDocker,
		MaxConcurrency: 10,
	}
	err := store.CreateWorkPool(ctx, testPool)
	require.NoError(t, err)

	tests := []struct {
		name           string
		poolID         string
		expectedStatus int
	}{
		{
			name:           "get existing work pool",
			poolID:         testPool.ID.String(),
			expectedStatus: http.StatusOK,
		},
		{
			name:           "get non-existent work pool",
			poolID:         uuid.New().String(),
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "get work pool with invalid UUID",
			poolID:         "invalid-uuid",
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest("GET", "/api/v1/workpools/"+tt.poolID, nil)
			require.NoError(t, err)

			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)

			assert.Equal(t, tt.expectedStatus, rr.Code)

			if tt.expectedStatus == http.StatusOK {
				var response WorkPool
				err := json.Unmarshal(rr.Body.Bytes(), &response)
				require.NoError(t, err)
				assert.Equal(t, testPool.ID, response.ID)
				assert.Equal(t, testPool.Name, response.Name)
			}
		})
	}
}

func TestUpdateWorkPool(t *testing.T) {
	db, router := setupHTTPTestDB(t)
	store := NewStore(db)

	ctx := context.Background()

	testPool := &WorkPool{
		Name:           "test-pool",
		Provider:       ProviderDocker,
		MaxConcurrency: 10,
		Paused:         false,
	}
	err := store.CreateWorkPool(ctx, testPool)
	require.NoError(t, err)

	tests := []struct {
		name           string
		poolID         string
		requestBody    interface{}
		expectedStatus int
	}{
		{
			name:   "update work pool successfully",
			poolID: testPool.ID.String(),
			requestBody: map[string]interface{}{
				"max_concurrency": 20,
				"paused":          true,
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:           "update with invalid JSON",
			poolID:         testPool.ID.String(),
			requestBody:    "invalid json",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "update non-existent work pool",
			poolID:         uuid.New().String(),
			requestBody:    map[string]interface{}{"paused": true},
			expectedStatus: http.StatusOK,
		},
		{
			name:           "update with invalid UUID",
			poolID:         "invalid-uuid",
			requestBody:    map[string]interface{}{"paused": true},
			expectedStatus: http.StatusBadRequest,
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

			req, err := http.NewRequest("PATCH", "/api/v1/workpools/"+tt.poolID, bytes.NewBuffer(body))
			require.NoError(t, err)
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)

			assert.Equal(t, tt.expectedStatus, rr.Code)

			if tt.expectedStatus == http.StatusOK {
				updatedPool, err := store.GetWorkPool(ctx, testPool.ID)
				require.NoError(t, err)

				if originalBody, ok := tt.requestBody.(map[string]interface{}); ok {
					if maxConcurrency, exists := originalBody["max_concurrency"]; exists {
						var expectedMaxConcurrency int
						switch v := maxConcurrency.(type) {
						case int:
							expectedMaxConcurrency = v
						case float64:
							expectedMaxConcurrency = int(v)
						}
						assert.Equal(t, expectedMaxConcurrency, updatedPool.MaxConcurrency)
					}
					if paused, exists := originalBody["paused"]; exists {
						assert.Equal(t, paused.(bool), updatedPool.Paused)
					}
				}
			}
		})
	}
}

func TestScaleWorkPool(t *testing.T) {
	db, router := setupHTTPTestDB(t)
	store := NewStore(db)

	ctx := context.Background()

	testPool := &WorkPool{
		Name:           "test-pool",
		Provider:       ProviderDocker,
		MinSize:        2,
		MaxConcurrency: 10,
		MaxIdleTime:    1800,
		AutoScale:      false,
		Paused:         false,
	}
	err := store.CreateWorkPool(ctx, testPool)
	require.NoError(t, err)

	tests := []struct {
		name           string
		poolID         string
		requestBody    interface{}
		expectedStatus int
	}{
		{
			name:   "scale work pool successfully",
			poolID: testPool.ID.String(),
			requestBody: map[string]interface{}{
				"min_size":        5,
				"max_concurrency": 20,
				"max_idle_time":   3600,
				"auto_scale":      true,
				"paused":          false,
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:   "scale with partial updates",
			poolID: testPool.ID.String(),
			requestBody: map[string]interface{}{
				"min_size": 3,
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:   "scale with invalid min_size",
			poolID: testPool.ID.String(),
			requestBody: map[string]interface{}{
				"min_size": -1,
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:   "scale with invalid max_concurrency",
			poolID: testPool.ID.String(),
			requestBody: map[string]interface{}{
				"max_concurrency": 0,
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:   "scale with invalid max_idle_time",
			poolID: testPool.ID.String(),
			requestBody: map[string]interface{}{
				"max_idle_time": -1,
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "scale with no parameters",
			poolID:         testPool.ID.String(),
			requestBody:    map[string]interface{}{},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "scale with invalid JSON",
			poolID:         testPool.ID.String(),
			requestBody:    "invalid json",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "scale non-existent work pool",
			poolID:         uuid.New().String(),
			requestBody:    map[string]interface{}{"min_size": 5},
			expectedStatus: http.StatusOK,
		},
		{
			name:           "scale with invalid UUID",
			poolID:         "invalid-uuid",
			requestBody:    map[string]interface{}{"min_size": 5},
			expectedStatus: http.StatusBadRequest,
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

			req, err := http.NewRequest("POST", "/api/v1/workpools/"+tt.poolID+"/scale", bytes.NewBuffer(body))
			require.NoError(t, err)
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)

			assert.Equal(t, tt.expectedStatus, rr.Code)

			if tt.expectedStatus == http.StatusOK {
				var response map[string]interface{}
				err := json.Unmarshal(rr.Body.Bytes(), &response)
				require.NoError(t, err)
				assert.Contains(t, response, "message")
				assert.Contains(t, response, "updates")
			}
		})
	}
}

func TestDrainWorkPool(t *testing.T) {
	db, router := setupHTTPTestDB(t)
	store := NewStore(db)

	ctx := context.Background()

	testPool := &WorkPool{
		Name:           "test-pool",
		Provider:       ProviderDocker,
		MinSize:        5,
		MaxConcurrency: 10,
		AutoScale:      true,
		Paused:         false,
	}
	err := store.CreateWorkPool(ctx, testPool)
	require.NoError(t, err)

	tests := []struct {
		name           string
		poolID         string
		expectedStatus int
	}{
		{
			name:           "drain work pool successfully",
			poolID:         testPool.ID.String(),
			expectedStatus: http.StatusOK,
		},
		{
			name:           "drain non-existent work pool",
			poolID:         uuid.New().String(),
			expectedStatus: http.StatusOK,
		},
		{
			name:           "drain with invalid UUID",
			poolID:         "invalid-uuid",
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest("POST", "/api/v1/workpools/"+tt.poolID+"/drain", nil)
			require.NoError(t, err)

			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)

			assert.Equal(t, tt.expectedStatus, rr.Code)

			if tt.expectedStatus == http.StatusOK {
				drainedPool, err := store.GetWorkPool(ctx, testPool.ID)
				require.NoError(t, err)
				assert.True(t, drainedPool.Paused)
				assert.False(t, drainedPool.AutoScale)
				assert.Equal(t, 0, drainedPool.MinSize)
			}
		})
	}
}

func TestDeleteWorkPool(t *testing.T) {
	db, router := setupHTTPTestDB(t)
	store := NewStore(db)

	ctx := context.Background()

	testPool := &WorkPool{
		Name:           "test-pool",
		Provider:       ProviderDocker,
		MaxConcurrency: 10,
	}
	err := store.CreateWorkPool(ctx, testPool)
	require.NoError(t, err)

	tests := []struct {
		name           string
		poolID         string
		expectedStatus int
	}{
		{
			name:           "delete work pool successfully",
			poolID:         testPool.ID.String(),
			expectedStatus: http.StatusOK,
		},
		{
			name:           "delete non-existent work pool",
			poolID:         uuid.New().String(),
			expectedStatus: http.StatusOK,
		},
		{
			name:           "delete with invalid UUID",
			poolID:         "invalid-uuid",
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest("DELETE", "/api/v1/workpools/"+tt.poolID, nil)
			require.NoError(t, err)

			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)

			assert.Equal(t, tt.expectedStatus, rr.Code)

			if tt.expectedStatus == http.StatusOK && tt.poolID == testPool.ID.String() {
				_, err := store.GetWorkPool(ctx, testPool.ID)
				assert.Error(t, err)
			}
		})
	}
}

func TestConcurrentRequests(t *testing.T) {
	_, router := setupHTTPTestDB(t)

	const numGoroutines = 10
	results := make(chan int, numGoroutines)

	poolData := WorkPool{
		Name:           fmt.Sprintf("concurrent-pool-%d", time.Now().UnixNano()),
		Provider:       ProviderDocker,
		MaxConcurrency: 10,
	}

	for i := 0; i < numGoroutines; i++ {
		go func(index int) {
			uniquePoolData := poolData
			uniquePoolData.Name = fmt.Sprintf("%s-%d", poolData.Name, index)

			body, _ := json.Marshal(uniquePoolData)
			req, _ := http.NewRequest("POST", "/api/v1/workpools", bytes.NewBuffer(body))
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
