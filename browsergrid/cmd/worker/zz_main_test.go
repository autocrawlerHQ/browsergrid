package main

import (
	"context"
	"flag"
	"log"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"gorm.io/datatypes"
	postgresDriver "gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/autocrawlerHQ/browsergrid/internal/sessions"
	"github.com/autocrawlerHQ/browsergrid/internal/workpool"
)

var (
	testContainer *postgres.PostgresContainer
	testConnStr   string
)

func TestMain(m *testing.M) {
	ctx := context.Background()

	var err error
	testContainer, err = postgres.Run(ctx,
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

	testConnStr, err = testContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		log.Fatalf("Failed to get connection string: %v", err)
	}

	code := m.Run()

	if err := testContainer.Terminate(ctx); err != nil {
		log.Printf("Failed to terminate PostgreSQL container: %v", err)
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
		&workpool.WorkPool{}, &workpool.Worker{},
	)
	if err != nil {
		t.Logf("Warning: Failed to drop tables (may not exist): %v", err)
	}

	err = db.AutoMigrate(
		&sessions.Session{}, &sessions.SessionEvent{}, &sessions.SessionMetrics{}, &sessions.Pool{},
		&workpool.WorkPool{}, &workpool.Worker{},
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
	ProviderType     workpool.ProviderType
}

func NewMockProvider(providerType workpool.ProviderType) *MockProvider {
	return &MockProvider{
		ProviderType: providerType,
	}
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
	return &sessions.SessionMetrics{
		SessionID:      sess.ID,
		CPUPercent:     &[]float64{25.5}[0],
		MemoryMB:       &[]float64{512.0}[0],
		NetworkRXBytes: &[]int64{1024}[0],
		NetworkTXBytes: &[]int64{2048}[0],
	}, nil
}

func (m *MockProvider) GetType() workpool.ProviderType {
	if m.ProviderType == "" {
		return workpool.ProviderDocker
	}
	return m.ProviderType
}

func TestLoadConfig(t *testing.T) {
	originalArgs := os.Args
	defer func() { os.Args = originalArgs }()

	tests := []struct {
		name        string
		args        []string
		env         map[string]string
		expectFatal bool
		validate    func(t *testing.T, cfg WorkerConfig)
	}{
		{
			name: "valid config with all flags",
			args: []string{
				"worker",
				"--pool", "550e8400-e29b-41d4-a716-446655440000",
				"--name", "test-worker",
				"--provider", "docker",
				"--concurrency", "3",
				"--db", "postgres://test:test@localhost/test",
				"--poll-interval", "5s",
			},
			validate: func(t *testing.T, cfg WorkerConfig) {
				assert.Equal(t, "550e8400-e29b-41d4-a716-446655440000", cfg.WorkPoolID)
				assert.Equal(t, "test-worker", cfg.Name)
				assert.Equal(t, "docker", cfg.Provider)
				assert.Equal(t, 3, cfg.Concurrency)
				assert.Equal(t, "postgres://test:test@localhost/test", cfg.DatabaseURL)
				assert.Equal(t, 5*time.Second, cfg.PollInterval)
			},
		},
		{
			name: "config with environment variables",
			args: []string{
				"worker",
				"--pool", "550e8400-e29b-41d4-a716-446655440000",
			},
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
			args: []string{
				"worker",
				"--pool", "550e8400-e29b-41d4-a716-446655440000",
			},
			validate: func(t *testing.T, cfg WorkerConfig) {
				assert.Equal(t, 1, cfg.Concurrency)
				assert.Equal(t, "docker", cfg.Provider)
				assert.Equal(t, 10*time.Second, cfg.PollInterval)
				assert.Equal(t, "postgres://user:password@localhost/browsergrid?sslmode=disable", cfg.DatabaseURL)
			},
		},
		{
			name: "missing required pool flag",
			args: []string{"worker"},
			validate: func(t *testing.T, cfg WorkerConfig) {
			},
			expectFatal: true,
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

			if tt.expectFatal {
				cfg := WorkerConfig{
					PollInterval: 10 * time.Second,
					Concurrency:  1,
				}

				flag.StringVar(&cfg.WorkPoolID, "pool", "", "Work pool ID (required)")
				flag.Parse()

				if cfg.WorkPoolID == "" {
					return
				}
			} else {
				cfg := loadConfig()
				tt.validate(t, cfg)
			}
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

func TestWorkerRuntime_AtomicOperations(t *testing.T) {
	worker := &WorkerRuntime{
		Worker: &workpool.Worker{
			ID:       uuid.New(),
			MaxSlots: 5,
			Active:   0,
		},
		activeCount: 0,
	}

	atomic.AddInt32(&worker.activeCount, 1)
	assert.Equal(t, int32(1), atomic.LoadInt32(&worker.activeCount))

	atomic.AddInt32(&worker.activeCount, 2)
	assert.Equal(t, int32(3), atomic.LoadInt32(&worker.activeCount))

	atomic.AddInt32(&worker.activeCount, -1)
	assert.Equal(t, int32(2), atomic.LoadInt32(&worker.activeCount))
}

func TestHandleSession(t *testing.T) {
	db := setupTestDB(t)
	ws := workpool.NewStore(db)
	ss := sessions.NewStore(db)

	pool := &workpool.WorkPool{
		Name:           "test-pool",
		Provider:       workpool.ProviderDocker,
		MaxConcurrency: 10,
	}
	ctx := context.Background()
	err := ws.CreateWorkPool(ctx, pool)
	require.NoError(t, err)

	worker := &WorkerRuntime{
		Worker: &workpool.Worker{
			ID:       uuid.New(),
			PoolID:   pool.ID,
			Name:     "test-worker",
			MaxSlots: 5,
			Active:   0,
		},
		activeCount: 0,
	}
	err = ws.RegisterWorker(ctx, worker.Worker)
	require.NoError(t, err)

	tests := []struct {
		name          string
		session       sessions.Session
		provider      *MockProvider
		expectStarted bool
		expectStopped bool
		timeout       time.Duration
	}{
		{
			name: "session with provider start error",
			session: sessions.Session{
				ID:              uuid.New(),
				Browser:         sessions.BrowserChrome,
				Version:         sessions.VerLatest,
				OperatingSystem: sessions.OSLinux,
				Screen: sessions.ScreenConfig{
					Width: 1920, Height: 1080, DPI: 96, Scale: 1.0,
				},
				Status:      sessions.StatusStarting,
				WorkPoolID:  &pool.ID,
				WorkerID:    &worker.ID,
				Environment: datatypes.JSON(`{}`),
			},
			provider: &MockProvider{
				StartError:   assert.AnError,
				ProviderType: workpool.ProviderDocker,
			},
			expectStarted: true,
			expectStopped: false,
			timeout:       2 * time.Second,
		},
		{
			name: "session with health check failure",
			session: sessions.Session{
				ID:              uuid.New(),
				Browser:         sessions.BrowserChrome,
				Version:         sessions.VerLatest,
				OperatingSystem: sessions.OSLinux,
				Screen: sessions.ScreenConfig{
					Width: 1920, Height: 1080, DPI: 96, Scale: 1.0,
				},
				Status:      sessions.StatusStarting,
				WorkPoolID:  &pool.ID,
				WorkerID:    &worker.ID,
				Environment: datatypes.JSON(`{}`),
			},
			provider: &MockProvider{
				ProviderType:     workpool.ProviderDocker,
				HealthCheckError: assert.AnError,
			},
			expectStarted: true,
			expectStopped: true,
			timeout:       35 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ss.CreateSession(ctx, &tt.session)
			require.NoError(t, err)

			testCtx, cancel := context.WithTimeout(ctx, tt.timeout)
			defer cancel()

			initialCount := atomic.LoadInt32(&worker.activeCount)

			done := make(chan struct{})
			go func() {
				defer close(done)
				handleSession(testCtx, tt.provider, ss, ws, worker, tt.session)
			}()

			select {
			case <-done:
			case <-testCtx.Done():
				t.Fatal("Session handling timed out")
			}

			assert.Equal(t, tt.expectStarted, tt.provider.StartCalled)
			assert.Equal(t, tt.expectStopped, tt.provider.StopCalled)

			finalCount := atomic.LoadInt32(&worker.activeCount)
			assert.Equal(t, initialCount, finalCount, "Active count should return to initial value")

			updatedSession, err := ss.GetSession(ctx, tt.session.ID)
			require.NoError(t, err)

			if tt.provider.StartError != nil {
				assert.Equal(t, sessions.StatusFailed, updatedSession.Status)
			} else {
				assert.NotEqual(t, sessions.StatusStarting, updatedSession.Status)
			}
		})
	}
}

func TestRunWorker_Basic(t *testing.T) {
	db := setupTestDB(t)
	ws := workpool.NewStore(db)
	ss := sessions.NewStore(db)

	pool := &workpool.WorkPool{
		Name:           "test-pool",
		Provider:       workpool.ProviderDocker,
		MaxConcurrency: 10,
	}
	ctx := context.Background()
	err := ws.CreateWorkPool(ctx, pool)
	require.NoError(t, err)

	worker := &WorkerRuntime{
		Worker: &workpool.Worker{
			ID:       uuid.New(),
			PoolID:   pool.ID,
			Name:     "test-worker",
			MaxSlots: 2,
			Active:   0,
		},
		activeCount: 0,
	}
	err = ws.RegisterWorker(ctx, worker.Worker)
	require.NoError(t, err)

	mockProvider := NewMockProvider(workpool.ProviderDocker)

	testSessions := []*sessions.Session{
		{
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
	}

	for _, sess := range testSessions {
		err := ss.CreateSession(ctx, sess)
		require.NoError(t, err)
	}

	testCtx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()

	err = runWorker(testCtx, worker, ws, ss, mockProvider, 100*time.Millisecond)

	assert.Equal(t, context.DeadlineExceeded, err)
}

func TestRunWorker_PausedWorker(t *testing.T) {
	db := setupTestDB(t)
	ws := workpool.NewStore(db)
	ss := sessions.NewStore(db)

	pool := &workpool.WorkPool{
		Name:           "test-pool",
		Provider:       workpool.ProviderDocker,
		MaxConcurrency: 10,
	}
	ctx := context.Background()
	err := ws.CreateWorkPool(ctx, pool)
	require.NoError(t, err)

	worker := &WorkerRuntime{
		Worker: &workpool.Worker{
			ID:       uuid.New(),
			PoolID:   pool.ID,
			Name:     "test-worker",
			MaxSlots: 2,
			Active:   0,
			Paused:   true,
		},
		activeCount: 0,
	}
	err = ws.RegisterWorker(ctx, worker.Worker)
	require.NoError(t, err)

	mockProvider := NewMockProvider(workpool.ProviderDocker)

	testSession := &sessions.Session{
		Browser:         sessions.BrowserChrome,
		Version:         sessions.VerLatest,
		OperatingSystem: sessions.OSLinux,
		Screen: sessions.ScreenConfig{
			Width: 1920, Height: 1080, DPI: 96, Scale: 1.0,
		},
		Status:      sessions.StatusPending,
		WorkPoolID:  &pool.ID,
		Environment: datatypes.JSON(`{}`),
	}
	err = ss.CreateSession(ctx, testSession)
	require.NoError(t, err)

	testCtx, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
	defer cancel()

	err = runWorker(testCtx, worker, ws, ss, mockProvider, 50*time.Millisecond)
	assert.Equal(t, context.DeadlineExceeded, err)

	assert.False(t, mockProvider.StartCalled)
}

func TestRunWorker_CapacityLimit(t *testing.T) {
	db := setupTestDB(t)
	ws := workpool.NewStore(db)
	ss := sessions.NewStore(db)

	pool := &workpool.WorkPool{
		Name:           "test-pool",
		Provider:       workpool.ProviderDocker,
		MaxConcurrency: 10,
	}
	ctx := context.Background()
	err := ws.CreateWorkPool(ctx, pool)
	require.NoError(t, err)

	worker := &WorkerRuntime{
		Worker: &workpool.Worker{
			ID:       uuid.New(),
			PoolID:   pool.ID,
			Name:     "test-worker",
			MaxSlots: 1,
			Active:   1,
		},
		activeCount: 1,
	}
	err = ws.RegisterWorker(ctx, worker.Worker)
	require.NoError(t, err)

	mockProvider := NewMockProvider(workpool.ProviderDocker)

	testSession := &sessions.Session{
		Browser:         sessions.BrowserChrome,
		Version:         sessions.VerLatest,
		OperatingSystem: sessions.OSLinux,
		Screen: sessions.ScreenConfig{
			Width: 1920, Height: 1080, DPI: 96, Scale: 1.0,
		},
		Status:      sessions.StatusPending,
		WorkPoolID:  &pool.ID,
		Environment: datatypes.JSON(`{}`),
	}
	err = ss.CreateSession(ctx, testSession)
	require.NoError(t, err)

	testCtx, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
	defer cancel()

	err = runWorker(testCtx, worker, ws, ss, mockProvider, 50*time.Millisecond)
	assert.Equal(t, context.DeadlineExceeded, err)

	assert.False(t, mockProvider.StartCalled)
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

func TestEnsureDefaultPool(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	require.NoError(t, err)

	store := workpool.NewStore(db)

	err = db.Exec(`
		CREATE TABLE work_pools (
			id TEXT PRIMARY KEY,
			name TEXT UNIQUE NOT NULL,
			description TEXT,
			provider TEXT NOT NULL,
			min_size INTEGER NOT NULL DEFAULT 0,
			max_concurrency INTEGER NOT NULL DEFAULT 10,
			max_idle_time INTEGER NOT NULL DEFAULT 1800,
			auto_scale BOOLEAN NOT NULL DEFAULT true,
			paused BOOLEAN NOT NULL DEFAULT false,
			default_priority INTEGER NOT NULL DEFAULT 0,
			queue_strategy TEXT NOT NULL DEFAULT 'fifo',
			default_env TEXT DEFAULT '{}',
			default_image TEXT,
			created_at DATETIME,
			updated_at DATETIME
		)
	`).Error
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("creates new default pool when none exists", func(t *testing.T) {
		pool, err := ensureDefaultPool(ctx, store, "docker")
		require.NoError(t, err)
		assert.Equal(t, "default-docker", pool.Name)
		assert.Equal(t, workpool.ProviderDocker, pool.Provider)
		assert.Equal(t, 10, pool.MaxConcurrency)
		assert.True(t, pool.AutoScale)
	})

	t.Run("returns existing pool when it already exists", func(t *testing.T) {
		pool1, err := ensureDefaultPool(ctx, store, "docker")
		require.NoError(t, err)

		pool2, err := ensureDefaultPool(ctx, store, "docker")
		require.NoError(t, err)

		assert.Equal(t, pool1.ID, pool2.ID)
		assert.Equal(t, pool1.Name, pool2.Name)
	})

	t.Run("creates different pools for different providers", func(t *testing.T) {
		dockerPool, err := ensureDefaultPool(ctx, store, "docker")
		require.NoError(t, err)

		aciPool, err := ensureDefaultPool(ctx, store, "azure_aci")
		require.NoError(t, err)

		assert.NotEqual(t, dockerPool.ID, aciPool.ID)
		assert.Equal(t, "default-docker", dockerPool.Name)
		assert.Equal(t, "default-azure_aci", aciPool.Name)
	})
}
