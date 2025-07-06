package sessions

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
)

type mockPoolService struct{}

func (m *mockPoolService) GetOrCreateDefault(ctx context.Context, provider string) (uuid.UUID, error) {
	return uuid.New(), nil
}

func setupHTTPTestDB(t *testing.T) (*gorm.DB, *gin.Engine) {
	db, err := gorm.Open(postgresDriver.Open(testConnStr), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)

	err = db.Migrator().DropTable(&Session{}, &SessionEvent{}, &SessionMetrics{}, &Pool{})
	if err != nil {
		t.Logf("Warning: Failed to drop tables (may not exist): %v", err)
	}

	err = db.AutoMigrate(&Session{}, &SessionEvent{}, &SessionMetrics{}, &Pool{})
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	router := gin.New()

	v1 := router.Group("/api/v1")
	mockPool := &mockPoolService{}
	RegisterRoutes(v1, Dependencies{
		DB:         db,
		PoolSvc:    mockPool,
		TaskClient: nil,
	})

	return db, router
}

func TestCreateSession(t *testing.T) {
	_, router := setupHTTPTestDB(t)

	tests := []struct {
		name           string
		requestBody    interface{}
		expectedStatus int
		expectedFields map[string]interface{}
	}{
		{
			name: "create valid session",
			requestBody: Session{
				Browser:         BrowserChrome,
				Version:         VerLatest,
				OperatingSystem: OSLinux,
				Screen: ScreenConfig{
					Width:  1920,
					Height: 1080,
					DPI:    96,
					Scale:  1.0,
				},
			},
			expectedStatus: http.StatusCreated,
			expectedFields: map[string]interface{}{
				"browser":          "chrome",
				"version":          "latest",
				"operating_system": "linux",
				"status":           "pending",
			},
		},
		{
			name: "create session with custom screen config",
			requestBody: Session{
				Browser:         BrowserFirefox,
				Version:         VerStable,
				OperatingSystem: OSWindows,
				Screen: ScreenConfig{
					Width:  1280,
					Height: 720,
					DPI:    120,
					Scale:  1.25,
				},
			},
			expectedStatus: http.StatusCreated,
			expectedFields: map[string]interface{}{
				"browser":          "firefox",
				"version":          "stable",
				"operating_system": "windows",
				"status":           "pending",
			},
		},
		{
			name:           "create session with invalid JSON",
			requestBody:    "invalid json",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "create session with empty request body",
			requestBody:    map[string]interface{}{},
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

			req, err := http.NewRequest("POST", "/api/v1/sessions", bytes.NewBuffer(body))
			require.NoError(t, err)
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)

			assert.Equal(t, tt.expectedStatus, rr.Code)

			if tt.expectedStatus == http.StatusCreated {
				var response Session
				err := json.Unmarshal(rr.Body.Bytes(), &response)
				require.NoError(t, err)

				assert.NotEqual(t, uuid.Nil, response.ID)

				for key, expectedValue := range tt.expectedFields {
					switch key {
					case "browser":
						assert.Equal(t, expectedValue, string(response.Browser))
					case "version":
						assert.Equal(t, expectedValue, string(response.Version))
					case "operating_system":
						assert.Equal(t, expectedValue, string(response.OperatingSystem))
					case "status":
						assert.Equal(t, expectedValue, string(response.Status))
					}
				}
			}
		})
	}
}

func TestListSessions(t *testing.T) {
	db, router := setupHTTPTestDB(t)
	store := NewStore(db)

	ctx := context.Background()

	now := time.Now()
	testSessions := []*Session{
		{
			Browser: BrowserChrome, Version: VerLatest, OperatingSystem: OSLinux,
			Screen: ScreenConfig{Width: 1920, Height: 1080, DPI: 96, Scale: 1.0},
			Status: StatusPending,
		},
		{
			Browser: BrowserFirefox, Version: VerStable, OperatingSystem: OSWindows,
			Screen: ScreenConfig{Width: 1280, Height: 720, DPI: 96, Scale: 1.0},
			Status: StatusRunning,
		},
		{
			Browser: BrowserChrome, Version: VerLatest, OperatingSystem: OSMacOS,
			Screen: ScreenConfig{Width: 1440, Height: 900, DPI: 96, Scale: 1.0},
			Status: StatusCompleted,
		},
	}

	for _, session := range testSessions {
		err := store.CreateSession(ctx, session)
		require.NoError(t, err)
	}

	tests := []struct {
		name           string
		queryParams    string
		expectedStatus int
		minSessions    int
		maxSessions    int
		statusFilter   *SessionStatus
	}{
		{
			name:           "list all sessions",
			queryParams:    "",
			expectedStatus: http.StatusOK,
			minSessions:    3,
			maxSessions:    3,
		},
		{
			name:           "filter by status",
			queryParams:    "?status=pending",
			expectedStatus: http.StatusOK,
			minSessions:    1,
			maxSessions:    1,
			statusFilter:   &[]SessionStatus{StatusPending}[0],
		},
		{
			name:           "pagination with limit",
			queryParams:    "?limit=2",
			expectedStatus: http.StatusOK,
			minSessions:    2,
			maxSessions:    2,
		},
		{
			name:           "pagination with offset",
			queryParams:    "?offset=1&limit=2",
			expectedStatus: http.StatusOK,
			minSessions:    2,
			maxSessions:    2,
		},
		{
			name:           "time range filter",
			queryParams:    fmt.Sprintf("?start_time=%s", now.Add(-1*time.Hour).Format(time.RFC3339)),
			expectedStatus: http.StatusOK,
			minSessions:    3,
			maxSessions:    3,
		},
		{
			name:           "empty result with future time filter",
			queryParams:    fmt.Sprintf("?start_time=%s", now.Add(1*time.Hour).Format(time.RFC3339)),
			expectedStatus: http.StatusOK,
			minSessions:    0,
			maxSessions:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest("GET", "/api/v1/sessions"+tt.queryParams, nil)
			require.NoError(t, err)

			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)

			assert.Equal(t, tt.expectedStatus, rr.Code)

			if tt.expectedStatus == http.StatusOK {
				var response map[string]interface{}
				err := json.Unmarshal(rr.Body.Bytes(), &response)
				require.NoError(t, err)

				sessions, ok := response["sessions"].([]interface{})
				require.True(t, ok)

				assert.GreaterOrEqual(t, len(sessions), tt.minSessions)
				assert.LessOrEqual(t, len(sessions), tt.maxSessions)

				if tt.statusFilter != nil {
					for _, sessionInterface := range sessions {
						sessionMap, ok := sessionInterface.(map[string]interface{})
						require.True(t, ok)
						status, ok := sessionMap["status"].(string)
						require.True(t, ok)
						assert.Equal(t, string(*tt.statusFilter), status)
					}
				}

				assert.Contains(t, response, "total")
				assert.Contains(t, response, "offset")
				assert.Contains(t, response, "limit")
			}
		})
	}
}

func TestGetSession(t *testing.T) {
	db, router := setupHTTPTestDB(t)
	store := NewStore(db)

	ctx := context.Background()

	testSession := &Session{
		Browser: BrowserChrome, Version: VerLatest, OperatingSystem: OSLinux,
		Screen: ScreenConfig{Width: 1920, Height: 1080, DPI: 96, Scale: 1.0},
		Status: StatusPending,
	}
	err := store.CreateSession(ctx, testSession)
	require.NoError(t, err)

	tests := []struct {
		name           string
		sessionID      string
		expectedStatus int
	}{
		{
			name:           "get existing session",
			sessionID:      testSession.ID.String(),
			expectedStatus: http.StatusOK,
		},
		{
			name:           "get non-existent session",
			sessionID:      uuid.New().String(),
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "get session with invalid UUID",
			sessionID:      "invalid-uuid",
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest("GET", "/api/v1/sessions/"+tt.sessionID, nil)
			require.NoError(t, err)

			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)

			assert.Equal(t, tt.expectedStatus, rr.Code)

			if tt.expectedStatus == http.StatusOK {
				var response Session
				err := json.Unmarshal(rr.Body.Bytes(), &response)
				require.NoError(t, err)
				assert.Equal(t, testSession.ID, response.ID)
				assert.Equal(t, testSession.Browser, response.Browser)
			}
		})
	}
}

func TestCreateEvent(t *testing.T) {
	db, router := setupHTTPTestDB(t)
	store := NewStore(db)

	ctx := context.Background()

	testSession := &Session{
		Browser: BrowserChrome, Version: VerLatest, OperatingSystem: OSLinux,
		Screen: ScreenConfig{Width: 1920, Height: 1080, DPI: 96, Scale: 1.0},
		Status: StatusPending,
	}
	err := store.CreateSession(ctx, testSession)
	require.NoError(t, err)

	tests := []struct {
		name           string
		requestBody    interface{}
		expectedStatus int
		checkDB        bool
	}{
		{
			name: "create valid event",
			requestBody: SessionEvent{
				SessionID: testSession.ID,
				Event:     EvtSessionStarting,
				Data:      datatypes.JSON(`{"test": "data"}`),
			},
			expectedStatus: http.StatusCreated,
			checkDB:        true,
		},
		{
			name: "create event with session state change",
			requestBody: SessionEvent{
				SessionID: testSession.ID,
				Event:     EvtBrowserStarted,
				Data:      datatypes.JSON(`{"container_id": "abc123"}`),
			},
			expectedStatus: http.StatusCreated,
			checkDB:        true,
		},
		{
			name:           "create event with invalid JSON",
			requestBody:    "invalid json",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "create event with missing session ID",
			requestBody: map[string]interface{}{
				"event": "session.starting",
				"data":  map[string]string{"test": "data"},
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

			req, err := http.NewRequest("POST", "/api/v1/events", bytes.NewBuffer(body))
			require.NoError(t, err)
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)

			assert.Equal(t, tt.expectedStatus, rr.Code)

			if tt.expectedStatus == http.StatusCreated {
				var response SessionEvent
				err := json.Unmarshal(rr.Body.Bytes(), &response)
				require.NoError(t, err)

				assert.NotEqual(t, uuid.Nil, response.ID)
				assert.Equal(t, testSession.ID, response.SessionID)

				if tt.checkDB {
					var dbEvent SessionEvent
					err := db.First(&dbEvent, "id = ?", response.ID).Error
					require.NoError(t, err)
					assert.Equal(t, response.SessionID, dbEvent.SessionID)

					if tt.requestBody.(SessionEvent).Event == EvtBrowserStarted {
						updatedSession, err := store.GetSession(ctx, testSession.ID)
						require.NoError(t, err)
						assert.Equal(t, StatusStarting, updatedSession.Status)
					}
				}
			}
		})
	}
}

func TestListEvents(t *testing.T) {
	db, router := setupHTTPTestDB(t)
	store := NewStore(db)

	ctx := context.Background()

	session1 := &Session{
		Browser: BrowserChrome, Version: VerLatest, OperatingSystem: OSLinux,
		Screen: ScreenConfig{Width: 1920, Height: 1080, DPI: 96, Scale: 1.0},
		Status: StatusPending,
	}
	session2 := &Session{
		Browser: BrowserFirefox, Version: VerStable, OperatingSystem: OSWindows,
		Screen: ScreenConfig{Width: 1280, Height: 720, DPI: 96, Scale: 1.0},
		Status: StatusPending,
	}

	err := store.CreateSession(ctx, session1)
	require.NoError(t, err)
	err = store.CreateSession(ctx, session2)
	require.NoError(t, err)

	events := []*SessionEvent{
		{
			SessionID: session1.ID,
			Event:     EvtSessionStarting,
			Data:      datatypes.JSON(`{"test": "data1"}`),
		},
		{
			SessionID: session1.ID,
			Event:     EvtBrowserStarted,
			Data:      datatypes.JSON(`{"test": "data2"}`),
		},
		{
			SessionID: session2.ID,
			Event:     EvtSessionStarting,
			Data:      datatypes.JSON(`{"test": "data3"}`),
		},
	}

	for _, event := range events {
		err := store.CreateEvent(ctx, event)
		require.NoError(t, err)
	}

	tests := []struct {
		name           string
		queryParams    string
		expectedStatus int
		minEvents      int
		maxEvents      int
	}{
		{
			name:           "list all events",
			queryParams:    "",
			expectedStatus: http.StatusOK,
			minEvents:      3,
			maxEvents:      3,
		},
		{
			name:           "filter by session ID",
			queryParams:    "?session_id=" + session1.ID.String(),
			expectedStatus: http.StatusOK,
			minEvents:      2,
			maxEvents:      2,
		},
		{
			name:           "filter by event type",
			queryParams:    "?event_type=session_starting",
			expectedStatus: http.StatusOK,
			minEvents:      2,
			maxEvents:      2,
		},
		{
			name:           "pagination with limit",
			queryParams:    "?limit=2",
			expectedStatus: http.StatusOK,
			minEvents:      2,
			maxEvents:      2,
		},
		{
			name:           "combine filters",
			queryParams:    "?session_id=" + session1.ID.String() + "&event_type=browser_started",
			expectedStatus: http.StatusOK,
			minEvents:      1,
			maxEvents:      1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest("GET", "/api/v1/events"+tt.queryParams, nil)
			require.NoError(t, err)

			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)

			assert.Equal(t, tt.expectedStatus, rr.Code)

			if tt.expectedStatus == http.StatusOK {
				var response map[string]interface{}
				err := json.Unmarshal(rr.Body.Bytes(), &response)
				require.NoError(t, err)

				events, ok := response["events"].([]interface{})
				require.True(t, ok)

				assert.GreaterOrEqual(t, len(events), tt.minEvents)
				assert.LessOrEqual(t, len(events), tt.maxEvents)

				assert.Contains(t, response, "total")
				assert.Contains(t, response, "offset")
				assert.Contains(t, response, "limit")
			}
		})
	}
}

func TestCreateMetrics(t *testing.T) {
	db, router := setupHTTPTestDB(t)
	store := NewStore(db)

	ctx := context.Background()

	testSession := &Session{
		Browser: BrowserChrome, Version: VerLatest, OperatingSystem: OSLinux,
		Screen: ScreenConfig{Width: 1920, Height: 1080, DPI: 96, Scale: 1.0},
		Status: StatusRunning,
	}
	err := store.CreateSession(ctx, testSession)
	require.NoError(t, err)

	tests := []struct {
		name           string
		requestBody    interface{}
		expectedStatus int
		checkDB        bool
	}{
		{
			name: "create valid metrics",
			requestBody: SessionMetrics{
				SessionID:  testSession.ID,
				CPUPercent: &[]float64{45.5}[0],
				MemoryMB:   &[]float64{512.0}[0],
			},
			expectedStatus: http.StatusCreated,
			checkDB:        true,
		},
		{
			name: "create metrics with all fields",
			requestBody: SessionMetrics{
				SessionID:      testSession.ID,
				CPUPercent:     &[]float64{75.2}[0],
				MemoryMB:       &[]float64{1024.0}[0],
				NetworkRXBytes: &[]int64{105472000}[0],
				NetworkTXBytes: &[]int64{52693750}[0],
			},
			expectedStatus: http.StatusCreated,
			checkDB:        true,
		},
		{
			name:           "create metrics with invalid JSON",
			requestBody:    "invalid json",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "create metrics with missing session ID",
			requestBody: map[string]interface{}{
				"cpu_percent": 45.5,
				"memory_mb":   512.0,
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

			req, err := http.NewRequest("POST", "/api/v1/metrics", bytes.NewBuffer(body))
			require.NoError(t, err)
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)

			assert.Equal(t, tt.expectedStatus, rr.Code)

			if tt.expectedStatus == http.StatusCreated {
				var response SessionMetrics
				err := json.Unmarshal(rr.Body.Bytes(), &response)
				require.NoError(t, err)

				assert.NotEqual(t, uuid.Nil, response.ID)
				assert.Equal(t, testSession.ID, response.SessionID)

				if tt.checkDB {
					var dbMetrics SessionMetrics
					err := db.First(&dbMetrics, "id = ?", response.ID).Error
					require.NoError(t, err)
					assert.Equal(t, response.SessionID, dbMetrics.SessionID)

					if originalMetrics, ok := tt.requestBody.(SessionMetrics); ok {
						if originalMetrics.CPUPercent != nil {
							require.NotNil(t, dbMetrics.CPUPercent)
							assert.Equal(t, *originalMetrics.CPUPercent, *dbMetrics.CPUPercent)
						}
						if originalMetrics.MemoryMB != nil {
							require.NotNil(t, dbMetrics.MemoryMB)
							assert.Equal(t, *originalMetrics.MemoryMB, *dbMetrics.MemoryMB)
						}
					}
				}
			}
		})
	}
}

func TestSessionStatusTransitions(t *testing.T) {
	db, router := setupHTTPTestDB(t)
	store := NewStore(db)

	ctx := context.Background()

	testSession := &Session{
		Browser: BrowserChrome, Version: VerLatest, OperatingSystem: OSLinux,
		Screen: ScreenConfig{Width: 1920, Height: 1080, DPI: 96, Scale: 1.0},
		Status: StatusPending,
	}
	err := store.CreateSession(ctx, testSession)
	require.NoError(t, err)

	statusTransitionTests := []struct {
		event          SessionEventType
		expectedStatus SessionStatus
	}{
		{EvtSessionStarting, StatusStarting},
		{EvtBrowserStarted, StatusStarting},
		{EvtSessionCompleted, StatusCompleted},
	}

	for _, tt := range statusTransitionTests {
		t.Run(fmt.Sprintf("transition via %s", tt.event), func(t *testing.T) {
			eventBody := SessionEvent{
				SessionID: testSession.ID,
				Event:     tt.event,
				Data:      datatypes.JSON(`{}`),
			}

			body, err := json.Marshal(eventBody)
			require.NoError(t, err)

			req, err := http.NewRequest("POST", "/api/v1/events", bytes.NewBuffer(body))
			require.NoError(t, err)
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)

			assert.Equal(t, http.StatusCreated, rr.Code)

			updatedSession, err := store.GetSession(ctx, testSession.ID)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedStatus, updatedSession.Status)
		})
	}
}

func TestConcurrentRequests(t *testing.T) {
	_, router := setupHTTPTestDB(t)

	const numGoroutines = 10
	results := make(chan int, numGoroutines)

	sessionData := Session{
		Browser:         BrowserChrome,
		Version:         VerLatest,
		OperatingSystem: OSLinux,
		Screen:          ScreenConfig{Width: 1920, Height: 1080, DPI: 96, Scale: 1.0},
	}

	for i := 0; i < numGoroutines; i++ {
		go func() {
			body, _ := json.Marshal(sessionData)
			req, _ := http.NewRequest("POST", "/api/v1/sessions", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)

			results <- rr.Code
		}()
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
