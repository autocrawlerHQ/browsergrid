package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/modules/redis"
	"github.com/testcontainers/testcontainers-go/wait"
	"gorm.io/datatypes"
	postgresDriver "gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/autocrawlerHQ/browsergrid/internal/sessions"
	"github.com/autocrawlerHQ/browsergrid/internal/tasks"
	"github.com/autocrawlerHQ/browsergrid/internal/workpool"
)

var (
	testPostgresContainer *postgres.PostgresContainer
	testRedisContainer    *redis.RedisContainer
	testConnStr           string
	testRedisAddr         string
)

func TestMain(m *testing.M) {
	ctx := context.Background()

	var err error
	testPostgresContainer, err = postgres.Run(ctx,
		"postgres:15-alpine",
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("testuser"),
		postgres.WithPassword("testpass"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second),
		),
	)
	if err != nil {
		log.Fatalf("Failed to start PostgreSQL container: %v", err)
	}

	testConnStr, err = testPostgresContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		log.Fatalf("Failed to get connection string: %v", err)
	}

	testRedisContainer, err = redis.Run(ctx,
		"redis:7-alpine",
		redis.WithSnapshotting(10, 1),
		redis.WithLogLevel(redis.LogLevelVerbose),
	)
	if err != nil {
		log.Fatalf("Failed to start Redis container: %v", err)
	}

	redisHost, err := testRedisContainer.Host(ctx)
	if err != nil {
		log.Fatalf("Failed to get Redis host: %v", err)
	}

	redisPort, err := testRedisContainer.MappedPort(ctx, "6379")
	if err != nil {
		log.Fatalf("Failed to get Redis port: %v", err)
	}

	testRedisAddr = redisHost + ":" + redisPort.Port()

	code := m.Run()

	if err := testPostgresContainer.Terminate(ctx); err != nil {
		log.Printf("Failed to terminate PostgreSQL container: %v", err)
	}
	if err := testRedisContainer.Terminate(ctx); err != nil {
		log.Printf("Failed to terminate Redis container: %v", err)
	}

	os.Exit(code)
}

func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(postgresDriver.Open(testConnStr), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)

	err = db.Migrator().DropTable(
		&sessions.Session{}, &sessions.SessionEvent{}, &sessions.SessionMetrics{}, &sessions.Pool{},
		&workpool.WorkPool{},
	)
	if err != nil {
		t.Logf("Warning: Failed to drop tables (may not exist): %v", err)
	}

	err = db.AutoMigrate(
		&sessions.Session{}, &sessions.SessionEvent{}, &sessions.SessionMetrics{}, &sessions.Pool{},
		&workpool.WorkPool{},
	)
	require.NoError(t, err)

	return db
}

type MockProvider struct {
	StartCalled      bool
	StopCalled       bool
	HealthCheckError error
	StartError       error
	StopError        error
	Metrics          *sessions.SessionMetrics
}

func NewMockProvider() *MockProvider {
	return &MockProvider{}
}

func (m *MockProvider) Start(ctx context.Context, sess *sessions.Session) (wsURL, liveURL string, err error) {
	m.StartCalled = true
	if m.StartError != nil {
		return "", "", m.StartError
	}
	return "ws://localhost:8080/ws", "http://localhost:8080/live", nil
}

func (m *MockProvider) Stop(ctx context.Context, sess *sessions.Session) error {
	m.StopCalled = true
	return m.StopError
}

func (m *MockProvider) HealthCheck(ctx context.Context, sess *sessions.Session) error {
	return m.HealthCheckError
}

func (m *MockProvider) GetMetrics(ctx context.Context, sess *sessions.Session) (*sessions.SessionMetrics, error) {
	if m.Metrics != nil {
		return m.Metrics, nil
	}
	cpu := 25.5
	memory := 512.0
	rx := int64(1024)
	tx := int64(2048)
	return &sessions.SessionMetrics{
		SessionID:      sess.ID,
		CPUPercent:     &cpu,
		MemoryMB:       &memory,
		NetworkRXBytes: &rx,
		NetworkTXBytes: &tx,
	}, nil
}

func (m *MockProvider) GetType() workpool.ProviderType {
	return workpool.ProviderType("mock")
}

func TestLoadConfig(t *testing.T) {
	originalArgs := os.Args
	defer func() { os.Args = originalArgs }()

	tests := []struct {
		name     string
		args     []string
		env      map[string]string
		validate func(t *testing.T, cfg WorkerConfig)
	}{
		{
			name: "valid config with all flags",
			args: []string{
				"worker",
				"--name", "test-worker",
				"--provider", "docker",
				"--concurrency", "3",
				"--db", "postgres://test:test@localhost/test",
				"--redis", "localhost:6380",
			},
			validate: func(t *testing.T, cfg WorkerConfig) {
				assert.Equal(t, "test-worker", cfg.Name)
				assert.Equal(t, "docker", cfg.Provider)
				assert.Equal(t, 3, cfg.Concurrency)
				assert.Equal(t, "postgres://test:test@localhost/test", cfg.DatabaseURL)
				assert.Equal(t, "localhost:6380", cfg.RedisAddr)
			},
		},
		{
			name: "config with environment variables",
			args: []string{"worker"},
			env: map[string]string{
				"DATABASE_URL": "postgres://env:env@localhost/env",
			},
			validate: func(t *testing.T, cfg WorkerConfig) {
				assert.Equal(t, "postgres://env:env@localhost/env", cfg.DatabaseURL)
				assert.Contains(t, cfg.Name, "worker-")
			},
		},
		{
			name: "config with defaults",
			args: []string{"worker"},
			validate: func(t *testing.T, cfg WorkerConfig) {
				assert.Equal(t, 10, cfg.Concurrency)
				assert.Equal(t, "docker", cfg.Provider)
				assert.Equal(t, "localhost:6379", cfg.RedisAddr)
				assert.Equal(t, "postgres://user:password@localhost/browsergrid?sslmode=disable", cfg.DatabaseURL)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.env != nil {
				for k, v := range tt.env {
					os.Setenv(k, v)
					defer os.Unsetenv(k)
				}
			}

			flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
			os.Args = tt.args

			cfg := loadConfig()
			tt.validate(t, cfg)
		})
	}
}

func TestConnectDB(t *testing.T) {
	tests := []struct {
		name    string
		dbURL   string
		wantErr bool
	}{
		{
			name:    "valid connection string",
			dbURL:   testConnStr,
			wantErr: false,
		},
		{
			name:    "invalid connection string",
			dbURL:   "invalid://connection/string",
			wantErr: true,
		},
		{
			name:    "empty connection string",
			dbURL:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, err := connectDB(tt.dbURL)
			if (err != nil) != tt.wantErr {
				t.Errorf("connectDB() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				assert.NotNil(t, db)
				sqlDB, err := db.DB()
				assert.NoError(t, err)
				assert.NoError(t, sqlDB.Ping())
			}
		})
	}
}

func TestHandleSessionStart(t *testing.T) {
	db := setupTestDB(t)
	sessStore := sessions.NewStore(db)
	mockProvider := NewMockProvider()

	pool := &workpool.WorkPool{
		Name:               "test-pool",
		Provider:           workpool.ProviderDocker,
		MaxConcurrency:     10,
		MaxSessionDuration: 300,
	}
	ctx := context.Background()
	err := db.Create(pool).Error
	require.NoError(t, err)

	tests := []struct {
		name          string
		session       *sessions.Session
		providerError error
		expectError   bool
		checkStatus   sessions.SessionStatus
	}{
		{
			name: "successful session start",
			session: &sessions.Session{
				ID:              uuid.New(),
				Browser:         sessions.BrowserChrome,
				Version:         sessions.VerLatest,
				OperatingSystem: sessions.OSLinux,
				Screen: sessions.ScreenConfig{
					Width: 1920, Height: 1080, DPI: 96, Scale: 1.0,
				},
				Status:      sessions.StatusPending,
				WorkPoolID:  &pool.ID,
				Environment: datatypes.JSON(`{}`),
			},
			providerError: nil,
			expectError:   false,
			checkStatus:   sessions.StatusRunning,
		},
		{
			name: "provider start failure",
			session: &sessions.Session{
				ID:              uuid.New(),
				Browser:         sessions.BrowserChrome,
				Version:         sessions.VerLatest,
				OperatingSystem: sessions.OSLinux,
				Screen: sessions.ScreenConfig{
					Width: 1920, Height: 1080, DPI: 96, Scale: 1.0,
				},
				Status:      sessions.StatusPending,
				WorkPoolID:  &pool.ID,
				Environment: datatypes.JSON(`{}`),
			},
			providerError: errors.New("provider failed"),
			expectError:   true,
			checkStatus:   sessions.StatusFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := sessStore.CreateSession(ctx, tt.session)
			require.NoError(t, err)

			mockProvider.StartCalled = false
			mockProvider.StartError = tt.providerError

			handler := handleSessionStart(sessStore, mockProvider)

			payload := tasks.SessionStartPayload{
				SessionID:          tt.session.ID,
				WorkPoolID:         pool.ID,
				MaxSessionDuration: pool.MaxSessionDuration,
				RedisAddr:          testRedisAddr,
				QueueName:          "default",
			}
			data, _ := payload.Marshal()
			task := asynq.NewTask(tasks.TypeSessionStart, data)

			err = handler(ctx, task)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			assert.True(t, mockProvider.StartCalled)

			updatedSession, err := sessStore.GetSession(ctx, tt.session.ID)
			require.NoError(t, err)
			assert.Equal(t, tt.checkStatus, updatedSession.Status)

			if !tt.expectError {
				assert.NotNil(t, updatedSession.WSEndpoint)
				assert.NotNil(t, updatedSession.LiveURL)
			}
		})
	}
}

func TestHandleSessionStop(t *testing.T) {
	db := setupTestDB(t)
	sessStore := sessions.NewStore(db)
	mockProvider := NewMockProvider()

	ctx := context.Background()

	tests := []struct {
		name        string
		session     *sessions.Session
		reason      string
		expectError bool
		finalStatus sessions.SessionStatus
	}{
		{
			name: "stop running session",
			session: &sessions.Session{
				ID:              uuid.New(),
				Browser:         sessions.BrowserChrome,
				Version:         sessions.VerLatest,
				OperatingSystem: sessions.OSLinux,
				Screen: sessions.ScreenConfig{
					Width: 1920, Height: 1080, DPI: 96, Scale: 1.0,
				},
				Status:      sessions.StatusRunning,
				Environment: datatypes.JSON(`{}`),
			},
			reason:      "user_requested",
			expectError: false,
			finalStatus: sessions.StatusCompleted,
		},
		{
			name: "stop session due to timeout",
			session: &sessions.Session{
				ID:              uuid.New(),
				Browser:         sessions.BrowserChrome,
				Version:         sessions.VerLatest,
				OperatingSystem: sessions.OSLinux,
				Screen: sessions.ScreenConfig{
					Width: 1920, Height: 1080, DPI: 96, Scale: 1.0,
				},
				Status:      sessions.StatusRunning,
				Environment: datatypes.JSON(`{}`),
			},
			reason:      "timeout",
			expectError: false,
			finalStatus: sessions.StatusTimedOut,
		},
		{
			name: "stop failed session",
			session: &sessions.Session{
				ID:              uuid.New(),
				Browser:         sessions.BrowserChrome,
				Version:         sessions.VerLatest,
				OperatingSystem: sessions.OSLinux,
				Screen: sessions.ScreenConfig{
					Width: 1920, Height: 1080, DPI: 96, Scale: 1.0,
				},
				Status:      sessions.StatusRunning,
				Environment: datatypes.JSON(`{}`),
			},
			reason:      "failed",
			expectError: false,
			finalStatus: sessions.StatusFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := sessStore.CreateSession(ctx, tt.session)
			require.NoError(t, err)

			mockProvider.StopCalled = false

            handler := handleSessionStop(sessStore, mockProvider)

			payload := tasks.SessionStopPayload{
				SessionID: tt.session.ID,
				Reason:    tt.reason,
			}
			data, _ := payload.Marshal()
			task := asynq.NewTask(tasks.TypeSessionStop, data)

			err = handler(ctx, task)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			assert.True(t, mockProvider.StopCalled)

			updatedSession, err := sessStore.GetSession(ctx, tt.session.ID)
			require.NoError(t, err)
			assert.Equal(t, tt.finalStatus, updatedSession.Status)
		})
	}
}

func TestHandleSessionHealthCheck(t *testing.T) {
	db := setupTestDB(t)
	sessStore := sessions.NewStore(db)
	mockProvider := NewMockProvider()

	ctx := context.Background()

	tests := []struct {
		name             string
		session          *sessions.Session
		healthCheckError error
		expectStopTask   bool
		expectedStatus   sessions.SessionStatus
	}{
		{
			name: "healthy session",
			session: &sessions.Session{
				ID:              uuid.New(),
				Browser:         sessions.BrowserChrome,
				Version:         sessions.VerLatest,
				OperatingSystem: sessions.OSLinux,
				Screen: sessions.ScreenConfig{
					Width: 1920, Height: 1080, DPI: 96, Scale: 1.0,
				},
				Status:      sessions.StatusRunning,
				Environment: datatypes.JSON(`{}`),
			},
			healthCheckError: nil,
			expectStopTask:   false,
			expectedStatus:   sessions.StatusRunning,
		},
		{
			name: "unhealthy session",
			session: &sessions.Session{
				ID:              uuid.New(),
				Browser:         sessions.BrowserChrome,
				Version:         sessions.VerLatest,
				OperatingSystem: sessions.OSLinux,
				Screen: sessions.ScreenConfig{
					Width: 1920, Height: 1080, DPI: 96, Scale: 1.0,
				},
				Status:      sessions.StatusRunning,
				Environment: datatypes.JSON(`{}`),
			},
			healthCheckError: errors.New("container not responding"),
			expectStopTask:   true,
			expectedStatus:   sessions.StatusCrashed,
		},
		{
			name: "terminal state session",
			session: &sessions.Session{
				ID:              uuid.New(),
				Browser:         sessions.BrowserChrome,
				Version:         sessions.VerLatest,
				OperatingSystem: sessions.OSLinux,
				Screen: sessions.ScreenConfig{
					Width: 1920, Height: 1080, DPI: 96, Scale: 1.0,
				},
				Status:      sessions.StatusCompleted,
				Environment: datatypes.JSON(`{}`),
			},
			healthCheckError: nil,
			expectStopTask:   false,
			expectedStatus:   sessions.StatusCompleted,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := sessStore.CreateSession(ctx, tt.session)
			require.NoError(t, err)

			mockProvider.HealthCheckError = tt.healthCheckError

			redisOpt := asynq.RedisClientOpt{Addr: testRedisAddr}
			inspector := asynq.NewInspector(redisOpt)

			inspector.DeleteAllPendingTasks("critical")

			handler := handleSessionHealthCheck(sessStore, mockProvider)

			payload := tasks.SessionHealthCheckPayload{
				SessionID: tt.session.ID,
				RedisAddr: testRedisAddr,
			}
			data, _ := payload.Marshal()
			task := asynq.NewTask(tasks.TypeSessionHealthCheck, data)

			err = handler(ctx, task)
			assert.NoError(t, err)

			updatedSession, err := sessStore.GetSession(ctx, tt.session.ID)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedStatus, updatedSession.Status)

			if tt.expectStopTask {
				time.Sleep(100 * time.Millisecond)

				pendingTasks, err := inspector.ListPendingTasks("critical")
				require.NoError(t, err)

				foundStopTask := false
				for _, task := range pendingTasks {
					if task.Type == tasks.TypeSessionStop {
						foundStopTask = true
						break
					}
				}
				assert.True(t, foundStopTask, "Expected stop task to be enqueued")
			}
		})
	}
}

func TestHealthCheckFunction(t *testing.T) {
	db := setupTestDB(t)
	sessStore := sessions.NewStore(db)

	healthFunc := healthCheck(sessStore)

	err := healthFunc()
	assert.NoError(t, err)

	sqlDB, _ := db.DB()
	sqlDB.Close()

	err = healthFunc()
	assert.Error(t, err)
}

func TestQueueConfiguration(t *testing.T) {
	cfg := WorkerConfig{
		Queues: map[string]int{
			"critical": 10,
			"default":  5,
			"low":      1,
		},
	}

	assert.Equal(t, 10, cfg.Queues["critical"])
	assert.Equal(t, 5, cfg.Queues["default"])
	assert.Equal(t, 1, cfg.Queues["low"])
}

func createTestSession(poolID uuid.UUID, status sessions.SessionStatus) *sessions.Session {
	return &sessions.Session{
		ID:              uuid.New(),
		Browser:         sessions.BrowserChrome,
		Version:         sessions.VerLatest,
		OperatingSystem: sessions.OSLinux,
		Screen: sessions.ScreenConfig{
			Width: 1920, Height: 1080, DPI: 96, Scale: 1.0,
		},
		Status:      status,
		WorkPoolID:  &poolID,
		Environment: datatypes.JSON(`{}`),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
}
