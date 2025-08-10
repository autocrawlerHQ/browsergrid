package poolmgr

import (
	"context"
	"encoding/json"
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
		&workpool.WorkPool{},
		&sessions.Session{}, &sessions.SessionEvent{},
		&sessions.SessionMetrics{}, &sessions.Pool{})
	if err != nil {
		t.Logf("Warning: Failed to drop tables (may not exist): %v", err)
	}

	err = db.AutoMigrate(
		&workpool.WorkPool{},
		&sessions.Session{}, &sessions.SessionEvent{},
		&sessions.SessionMetrics{}, &sessions.Pool{})
	require.NoError(t, err)

	return db
}

func setupTestRedisClient(t *testing.T) *asynq.Client {
	redisOpt := asynq.RedisClientOpt{Addr: testRedisAddr}
	client := asynq.NewClient(redisOpt)

	inspector := asynq.NewInspector(redisOpt)
	queues, _ := inspector.Queues()
	for _, q := range queues {
		inspector.DeleteAllPendingTasks(q)
		inspector.DeleteAllScheduledTasks(q)
		inspector.DeleteAllRetryTasks(q)
		inspector.DeleteAllArchivedTasks(q)
	}

	return client
}

func setupTestReconciler(t *testing.T) *Reconciler {
	db := setupTestDB(t)
	client := setupTestRedisClient(t)
	redisOpt := asynq.RedisClientOpt{Addr: testRedisAddr}
	return NewReconciler(db, client, redisOpt)
}

func TestNewReconciler(t *testing.T) {
	reconciler := setupTestReconciler(t)
	defer reconciler.taskClient.Close()

	assert.NotNil(t, reconciler)
	assert.NotNil(t, reconciler.db)
	assert.NotNil(t, reconciler.wpStore)
	assert.NotNil(t, reconciler.sessStore)
	assert.NotNil(t, reconciler.taskClient)
	assert.Equal(t, 30*time.Second, reconciler.tickInterval)
}

func TestReconciler_CountSessionsByStatus(t *testing.T) {
	reconciler := setupTestReconciler(t)
	defer reconciler.taskClient.Close()
	ctx := context.Background()

	pool := &workpool.WorkPool{
		Name:           "test-pool",
		Provider:       workpool.ProviderDocker,
		MaxConcurrency: 10,
	}
	err := reconciler.wpStore.CreateWorkPool(ctx, pool)
	require.NoError(t, err)

	testSessions := []*sessions.Session{
		createTestSession(pool.ID, sessions.StatusPending),
		createTestSession(pool.ID, sessions.StatusPending),
		createTestSession(pool.ID, sessions.StatusRunning),
		createTestSession(pool.ID, sessions.StatusCompleted),
	}

	for _, sess := range testSessions {
		err := reconciler.sessStore.CreateSession(ctx, sess)
		require.NoError(t, err)
	}

	tests := []struct {
		name     string
		statuses []sessions.SessionStatus
		expected int
	}{
		{
			name:     "count pending sessions",
			statuses: []sessions.SessionStatus{sessions.StatusPending},
			expected: 2,
		},
		{
			name:     "count active sessions",
			statuses: []sessions.SessionStatus{sessions.StatusStarting, sessions.StatusRunning, sessions.StatusIdle},
			expected: 1,
		},
		{
			name:     "count all sessions",
			statuses: []sessions.SessionStatus{sessions.StatusPending, sessions.StatusRunning, sessions.StatusCompleted},
			expected: 4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			count, err := reconciler.countSessionsByStatus(ctx, pool.ID, tt.statuses)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, count)
		})
	}
}

func TestReconciler_ReconcilePool(t *testing.T) {
	reconciler := setupTestReconciler(t)
	defer reconciler.taskClient.Close()
	ctx := context.Background()

	inspector := asynq.NewInspector(asynq.RedisClientOpt{Addr: testRedisAddr})

	tests := []struct {
		name             string
		pool             *workpool.WorkPool
		existingSessions []sessions.SessionStatus
		expectScaleTask  bool
		expectedSessions int
	}{
		{
			name: "scale up to min size",
			pool: &workpool.WorkPool{
				Name:           "test-pool",
				Provider:       workpool.ProviderDocker,
				MaxConcurrency: 10,
				MinSize:        3,
				AutoScale:      true,
			},
			existingSessions: []sessions.SessionStatus{
				sessions.StatusRunning,
			},
			expectScaleTask:  true,
			expectedSessions: 2,
		},
		{
			name: "no scaling needed",
			pool: &workpool.WorkPool{
				Name:           "test-pool-no-scale",
				Provider:       workpool.ProviderDocker,
				MaxConcurrency: 10,
				MinSize:        2,
				AutoScale:      true,
			},
			existingSessions: []sessions.SessionStatus{
				sessions.StatusRunning,
				sessions.StatusIdle,
			},
			expectScaleTask:  false,
			expectedSessions: 0,
		},
		{
			name: "paused pool - no scaling",
			pool: &workpool.WorkPool{
				Name:           "test-pool-paused",
				Provider:       workpool.ProviderDocker,
				MaxConcurrency: 10,
				MinSize:        5,
				AutoScale:      true,
				Paused:         true,
			},
			existingSessions: []sessions.SessionStatus{},
			expectScaleTask:  false,
			expectedSessions: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inspector.DeleteAllPendingTasks("low")

			err := reconciler.wpStore.CreateWorkPool(ctx, tt.pool)
			require.NoError(t, err)

			for _, status := range tt.existingSessions {
				sess := createTestSession(tt.pool.ID, status)
				err = reconciler.sessStore.CreateSession(ctx, sess)
				require.NoError(t, err)
			}

			err = reconciler.reconcilePool(ctx, tt.pool)
			assert.NoError(t, err)

			time.Sleep(100 * time.Millisecond)

			pendingTasks, err := inspector.ListPendingTasks("low")
			require.NoError(t, err)

			foundScaleTask := false
			var scalePayload tasks.PoolScalePayload

			for _, task := range pendingTasks {
				if task.Type == tasks.TypePoolScale {
					foundScaleTask = true
					err = json.Unmarshal(task.Payload, &scalePayload)
					require.NoError(t, err)
					break
				}
			}

			assert.Equal(t, tt.expectScaleTask, foundScaleTask, "Scale task expectation mismatch")

			if tt.expectScaleTask && foundScaleTask {
				assert.Equal(t, tt.pool.ID, scalePayload.WorkPoolID)
				assert.Equal(t, tt.expectedSessions, scalePayload.DesiredSessions)
			}

			err = reconciler.db.Where("work_pool_id = ?", tt.pool.ID).Delete(&sessions.Session{}).Error
			require.NoError(t, err)
			err = reconciler.db.Delete(tt.pool).Error
			require.NoError(t, err)
		})
	}
}

func TestReconciler_HandleIdleSessions(t *testing.T) {
	reconciler := setupTestReconciler(t)
	defer reconciler.taskClient.Close()
	ctx := context.Background()

	inspector := asynq.NewInspector(asynq.RedisClientOpt{Addr: testRedisAddr})

	pool := &workpool.WorkPool{
		Name:           "test-pool",
		Provider:       workpool.ProviderDocker,
		MaxConcurrency: 10,
		MaxIdleTime:    60,
		AutoScale:      true,
	}
	err := reconciler.wpStore.CreateWorkPool(ctx, pool)
	require.NoError(t, err)

	oldSession := createTestSession(pool.ID, sessions.StatusIdle)
	oldSession.UpdatedAt = time.Now().Add(-2 * time.Minute)
	err = reconciler.sessStore.CreateSession(ctx, oldSession)
	require.NoError(t, err)

	recentSession := createTestSession(pool.ID, sessions.StatusIdle)
	err = reconciler.sessStore.CreateSession(ctx, recentSession)
	require.NoError(t, err)

	inspector.DeleteAllPendingTasks("default")

	err = reconciler.reconcilePool(ctx, pool)
	assert.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	pendingTasks, err := inspector.ListPendingTasks("default")
	require.NoError(t, err)

	foundStopTask := false
	for _, task := range pendingTasks {
		if task.Type == tasks.TypeSessionStop {
			var payload tasks.SessionStopPayload
			err = json.Unmarshal(task.Payload, &payload)
			require.NoError(t, err)

			if payload.SessionID == oldSession.ID && payload.Reason == "idle_timeout" {
				foundStopTask = true
				break
			}
		}
	}

	assert.True(t, foundStopTask, "Expected stop task for idle session")
}

func TestReconciler_GetPoolStats(t *testing.T) {
	db := setupTestDB(t)
	client := setupTestRedisClient(t)
	defer client.Close()

	reconciler := NewReconciler(db, client, asynq.RedisClientOpt{Addr: testRedisAddr})
	ctx := context.Background()

	pool := &workpool.WorkPool{
		Name:               "test-pool",
		Provider:           workpool.ProviderDocker,
		MaxConcurrency:     10,
		MinSize:            2,
		AutoScale:          true,
		MaxIdleTime:        1800,
		MaxSessionDuration: 3600,
	}
	err := reconciler.wpStore.CreateWorkPool(ctx, pool)
	require.NoError(t, err)

	sessionStatuses := []sessions.SessionStatus{
		sessions.StatusPending,
		sessions.StatusPending,
		sessions.StatusRunning,
		sessions.StatusIdle,
		sessions.StatusCompleted,
	}

	for _, status := range sessionStatuses {
		sess := createTestSession(pool.ID, status)
		err = reconciler.sessStore.CreateSession(ctx, sess)
		require.NoError(t, err)
	}

	stats, err := reconciler.GetPoolStats(ctx, pool.ID)
	assert.NoError(t, err)
	assert.NotNil(t, stats)

	assert.Equal(t, pool.ID, stats.Pool.ID)
	assert.Equal(t, pool.Name, stats.Pool.Name)

	assert.Equal(t, 2, stats.SessionsByStatus[sessions.StatusPending])
	assert.Equal(t, 1, stats.SessionsByStatus[sessions.StatusRunning])
	assert.Equal(t, 1, stats.SessionsByStatus[sessions.StatusIdle])
	assert.Equal(t, 1, stats.SessionsByStatus[sessions.StatusCompleted])

	expectedUtilization := float64(2) / float64(10) * 100
	assert.Equal(t, expectedUtilization, stats.UtilizationPercent)

	assert.Equal(t, pool.MinSize, stats.ScalingInfo.MinSize)
	assert.Equal(t, pool.MaxConcurrency, stats.ScalingInfo.MaxConcurrency)
	assert.Equal(t, pool.AutoScale, stats.ScalingInfo.AutoScale)
	assert.Equal(t, pool.MaxIdleTime, stats.ScalingInfo.MaxIdleTime)
	assert.Equal(t, pool.MaxSessionDuration, stats.ScalingInfo.MaxSessionDuration)

	assert.NotNil(t, stats.QueueStats)
}

func TestReconciler_GetPoolStats_NonExistentPool(t *testing.T) {
	db := setupTestDB(t)
	client := setupTestRedisClient(t)
	defer client.Close()

	reconciler := NewReconciler(db, client, asynq.RedisClientOpt{Addr: testRedisAddr})
	ctx := context.Background()

	nonExistentID := uuid.New()
	stats, err := reconciler.GetPoolStats(ctx, nonExistentID)
	assert.Error(t, err)
	assert.Nil(t, stats)
}

func TestReconciler_Start_StopOnContext(t *testing.T) {
	db := setupTestDB(t)
	client := setupTestRedisClient(t)
	defer client.Close()

	reconciler := NewReconciler(db, client, asynq.RedisClientOpt{Addr: testRedisAddr})
	reconciler.tickInterval = 100 * time.Millisecond

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	err := reconciler.Start(ctx)
	assert.Equal(t, context.DeadlineExceeded, err)
}

func TestReconciler_ScheduleCleanupTasks(t *testing.T) {
	db := setupTestDB(t)
	client := setupTestRedisClient(t)
	defer client.Close()

	reconciler := NewReconciler(db, client, asynq.RedisClientOpt{Addr: testRedisAddr})

	err := reconciler.scheduleCleanupTasks()
	assert.NoError(t, err)
}

func TestGetQueueNameForProvider(t *testing.T) {
	tests := []struct {
		provider workpool.ProviderType
		expected string
	}{
		{workpool.ProviderDocker, "default"},
		{workpool.ProviderK8s, "k8s"},
		{"unknown", "default"},
	}

	for _, tt := range tests {
		t.Run(string(tt.provider), func(t *testing.T) {
			result := getQueueNameForProvider(tt.provider)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func createTestSession(poolID uuid.UUID, status sessions.SessionStatus) *sessions.Session {
	envData, _ := json.Marshal(map[string]string{})
	return &sessions.Session{
		ID:              uuid.New(),
		Browser:         sessions.BrowserChrome,
		Version:         sessions.VerLatest,
		Headless:        true,
		OperatingSystem: sessions.OSLinux,
		Screen: sessions.ScreenConfig{
			Width:  1920,
			Height: 1080,
			DPI:    96,
			Scale:  1.0,
		},
		Environment: datatypes.JSON(envData),
		Status:      status,
		Provider:    "docker",
		WorkPoolID:  &poolID,
		IsPooled:    false,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
}
