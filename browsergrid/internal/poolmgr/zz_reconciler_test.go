package poolmgr

import (
	"context"
	"encoding/json"
	"log"
	"os"
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
		&workpool.WorkPool{}, &workpool.Worker{},
		&sessions.Session{}, &sessions.SessionEvent{},
		&sessions.SessionMetrics{}, &sessions.Pool{})
	if err != nil {
		t.Logf("Warning: Failed to drop tables (may not exist): %v", err)
	}

	err = db.AutoMigrate(
		&workpool.WorkPool{}, &workpool.Worker{},
		&sessions.Session{}, &sessions.SessionEvent{},
		&sessions.SessionMetrics{}, &sessions.Pool{})
	require.NoError(t, err)

	return db
}

func TestNewReconciler(t *testing.T) {
	db := setupTestDB(t)

	reconciler := NewReconciler(db)

	assert.NotNil(t, reconciler)
	assert.Equal(t, db, reconciler.db)
	assert.NotNil(t, reconciler.wpStore)
	assert.NotNil(t, reconciler.sessStore)
	assert.Equal(t, 1*time.Minute, reconciler.tickInterval)
	assert.Equal(t, 5*time.Minute, reconciler.workerTTL)
	assert.True(t, reconciler.cleanupEnabled)
}

func TestReconciler_CreateSessionFromPool(t *testing.T) {
	db := setupTestDB(t)
	reconciler := NewReconciler(db)

	tests := []struct {
		name string
		pool *workpool.WorkPool
	}{
		{
			name: "basic pool",
			pool: &workpool.WorkPool{
				ID:             uuid.New(),
				Name:           "test-pool",
				Provider:       workpool.ProviderDocker,
				MaxConcurrency: 10,
				DefaultEnv:     datatypes.JSON(`{"ENV": "test"}`),
			},
		},
		{
			name: "pool with default image",
			pool: &workpool.WorkPool{
				ID:             uuid.New(),
				Name:           "test-pool",
				Provider:       workpool.ProviderDocker,
				MaxConcurrency: 10,
				DefaultEnv:     datatypes.JSON(`{"ENV": "test"}`),
				DefaultImage:   stringPtr("playwright:latest"),
			},
		},
		{
			name: "pool with nil default env",
			pool: &workpool.WorkPool{
				ID:             uuid.New(),
				Name:           "test-pool",
				Provider:       workpool.ProviderDocker,
				MaxConcurrency: 10,
				DefaultEnv:     nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sess := reconciler.createSessionFromPool(tt.pool)

			assert.NotEqual(t, uuid.Nil, sess.ID)
			assert.Equal(t, sessions.BrowserChrome, sess.Browser)
			assert.Equal(t, sessions.VerLatest, sess.Version)
			assert.True(t, sess.Headless)
			assert.Equal(t, sessions.OSLinux, sess.OperatingSystem)
			assert.Equal(t, 1920, sess.Screen.Width)
			assert.Equal(t, 1080, sess.Screen.Height)
			assert.Equal(t, sessions.StatusPending, sess.Status)
			assert.Equal(t, string(tt.pool.Provider), sess.Provider)
			assert.Equal(t, &tt.pool.ID, sess.WorkPoolID)
			assert.False(t, sess.IsPooled)

			if tt.pool.DefaultEnv != nil {
				var env map[string]interface{}
				err := json.Unmarshal(sess.Environment, &env)
				assert.NoError(t, err)
				assert.Equal(t, "test", env["ENV"])
			}

			if tt.pool.DefaultImage != nil {
				var env map[string]interface{}
				err := json.Unmarshal(sess.Environment, &env)
				assert.NoError(t, err)
				assert.Equal(t, *tt.pool.DefaultImage, env["BROWSER_IMAGE"])
			}
		})
	}
}

func TestReconciler_CountSessionsByStatus(t *testing.T) {
	db := setupTestDB(t)
	reconciler := NewReconciler(db)
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

func TestReconciler_CleanupIdleSessionsForPool(t *testing.T) {
	db := setupTestDB(t)
	reconciler := NewReconciler(db)
	ctx := context.Background()

	pool := &workpool.WorkPool{
		Name:           "test-pool",
		Provider:       workpool.ProviderDocker,
		MaxConcurrency: 10,
		MaxIdleTime:    60,
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

	runningSession := createTestSession(pool.ID, sessions.StatusRunning)
	runningSession.UpdatedAt = time.Now().Add(-2 * time.Minute)
	err = reconciler.sessStore.CreateSession(ctx, runningSession)
	require.NoError(t, err)

	err = reconciler.cleanupIdleSessionsForPool(ctx, pool)
	assert.NoError(t, err)

	var expiredCount int64
	err = db.Model(&sessions.Session{}).
		Where("work_pool_id = ? AND status = ?", pool.ID, sessions.StatusExpired).
		Count(&expiredCount).Error
	assert.NoError(t, err)
	assert.Equal(t, int64(1), expiredCount)

	var idleCount int64
	err = db.Model(&sessions.Session{}).
		Where("work_pool_id = ? AND status = ?", pool.ID, sessions.StatusIdle).
		Count(&idleCount).Error
	assert.NoError(t, err)
	assert.Equal(t, int64(1), idleCount)

	var runningCount int64
	err = db.Model(&sessions.Session{}).
		Where("work_pool_id = ? AND status = ?", pool.ID, sessions.StatusRunning).
		Count(&runningCount).Error
	assert.NoError(t, err)
	assert.Equal(t, int64(1), runningCount)
}

func TestReconciler_CleanupIdleSessionsForPool_NoTimeout(t *testing.T) {
	db := setupTestDB(t)
	reconciler := NewReconciler(db)
	ctx := context.Background()

	pool := &workpool.WorkPool{
		Name:           "test-pool",
		Provider:       workpool.ProviderDocker,
		MaxConcurrency: 10,
		MaxIdleTime:    0,
	}
	err := reconciler.wpStore.CreateWorkPool(ctx, pool)
	require.NoError(t, err)

	err = reconciler.wpStore.UpdateWorkPool(ctx, pool.ID, map[string]interface{}{
		"max_idle_time": 0,
	})
	require.NoError(t, err)

	pool, err = reconciler.wpStore.GetWorkPool(ctx, pool.ID)
	require.NoError(t, err)
	require.Equal(t, 0, pool.MaxIdleTime)

	oldSession := createTestSession(pool.ID, sessions.StatusIdle)
	oldSession.UpdatedAt = time.Now().Add(-2 * time.Hour)
	err = reconciler.sessStore.CreateSession(ctx, oldSession)
	require.NoError(t, err)

	err = reconciler.cleanupIdleSessionsForPool(ctx, pool)
	assert.NoError(t, err)

	var expiredCount int64
	err = db.Model(&sessions.Session{}).
		Where("work_pool_id = ? AND status = ?", pool.ID, sessions.StatusExpired).
		Count(&expiredCount).Error
	assert.NoError(t, err)
	assert.Equal(t, int64(0), expiredCount)
}

func TestReconciler_CleanupExpiredSessions(t *testing.T) {
	db := setupTestDB(t)
	reconciler := NewReconciler(db)
	ctx := context.Background()

	pool := &workpool.WorkPool{
		Name:           "test-pool",
		Provider:       workpool.ProviderDocker,
		MaxConcurrency: 10,
	}
	err := reconciler.wpStore.CreateWorkPool(ctx, pool)
	require.NoError(t, err)

	oldTerminatedStatuses := []sessions.SessionStatus{
		sessions.StatusCompleted,
		sessions.StatusFailed,
		sessions.StatusExpired,
		sessions.StatusCrashed,
		sessions.StatusTimedOut,
		sessions.StatusTerminated,
	}

	for i, status := range oldTerminatedStatuses {
		sess := createTestSession(pool.ID, status)
		sess.UpdatedAt = time.Now().Add(-25 * time.Hour)
		err = reconciler.sessStore.CreateSession(ctx, sess)
		require.NoError(t, err, "Failed to create session %d", i)
	}

	recentSession := createTestSession(pool.ID, sessions.StatusCompleted)
	err = reconciler.sessStore.CreateSession(ctx, recentSession)
	require.NoError(t, err)

	activeSession := createTestSession(pool.ID, sessions.StatusRunning)
	activeSession.UpdatedAt = time.Now().Add(-25 * time.Hour)
	err = reconciler.sessStore.CreateSession(ctx, activeSession)
	require.NoError(t, err)

	var totalBefore int64
	err = db.Model(&sessions.Session{}).
		Where("work_pool_id = ?", pool.ID).
		Count(&totalBefore).Error
	assert.NoError(t, err)
	assert.Equal(t, int64(8), totalBefore)

	err = reconciler.cleanupExpiredSessions(ctx)
	assert.NoError(t, err)

	var totalAfter int64
	err = db.Model(&sessions.Session{}).
		Where("work_pool_id = ?", pool.ID).
		Count(&totalAfter).Error
	assert.NoError(t, err)
	assert.Equal(t, int64(2), totalAfter)
}

func TestReconciler_GetPoolStats(t *testing.T) {
	db := setupTestDB(t)
	reconciler := NewReconciler(db)
	ctx := context.Background()

	pool := &workpool.WorkPool{
		Name:           "test-pool",
		Provider:       workpool.ProviderDocker,
		MaxConcurrency: 10,
		MinSize:        2,
		AutoScale:      true,
		MaxIdleTime:    1800,
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

	now := time.Now()
	worker1 := &workpool.Worker{
		PoolID:   pool.ID,
		Name:     "worker-1",
		Provider: workpool.ProviderDocker,
		MaxSlots: 5,
		Active:   2,
		LastBeat: now,
	}
	err = reconciler.wpStore.RegisterWorker(ctx, worker1)
	require.NoError(t, err)

	worker2 := &workpool.Worker{
		PoolID:   pool.ID,
		Name:     "worker-2",
		Provider: workpool.ProviderDocker,
		MaxSlots: 3,
		Active:   1,
		LastBeat: now.Add(-10 * time.Minute),
	}
	err = reconciler.wpStore.RegisterWorker(ctx, worker2)
	require.NoError(t, err)

	offlineTime := now.Add(-10 * time.Minute)
	err = db.Model(&workpool.Worker{}).
		Where("id = ?", worker2.ID).
		Update("last_beat", offlineTime).Error
	require.NoError(t, err)

	worker2Retrieved, err := reconciler.wpStore.GetWorker(ctx, worker2.ID)
	require.NoError(t, err)
	assert.False(t, worker2Retrieved.IsOnline(5*time.Minute), "Worker2 should be offline")

	stats, err := reconciler.GetPoolStats(ctx, pool.ID)
	assert.NoError(t, err)
	assert.NotNil(t, stats)

	assert.Equal(t, pool.ID, stats.Pool.ID)
	assert.Equal(t, pool.Name, stats.Pool.Name)

	assert.Equal(t, 2, stats.SessionsByStatus[sessions.StatusPending])
	assert.Equal(t, 1, stats.SessionsByStatus[sessions.StatusRunning])
	assert.Equal(t, 1, stats.SessionsByStatus[sessions.StatusIdle])
	assert.Equal(t, 1, stats.SessionsByStatus[sessions.StatusCompleted])

	assert.Equal(t, 2, stats.TotalWorkers)
	assert.Equal(t, 1, stats.OnlineWorkers)
	assert.Equal(t, 5, stats.TotalWorkerSlots)
	assert.Equal(t, 2, stats.ActiveWorkerSlots)

	expectedUtilization := float64(2) / float64(10) * 100
	assert.Equal(t, expectedUtilization, stats.UtilizationPercent)

	assert.Equal(t, pool.MinSize, stats.ScalingInfo.MinSize)
	assert.Equal(t, pool.MaxConcurrency, stats.ScalingInfo.MaxConcurrency)
	assert.Equal(t, pool.AutoScale, stats.ScalingInfo.AutoScale)
	assert.Equal(t, pool.MaxIdleTime, stats.ScalingInfo.MaxIdleTime)
}

func TestReconciler_GetPoolStats_NonExistentPool(t *testing.T) {
	db := setupTestDB(t)
	reconciler := NewReconciler(db)
	ctx := context.Background()

	nonExistentID := uuid.New()
	stats, err := reconciler.GetPoolStats(ctx, nonExistentID)
	assert.Error(t, err)
	assert.Nil(t, stats)
}

func TestReconciler_ReconcilePool(t *testing.T) {
	db := setupTestDB(t)
	reconciler := NewReconciler(db)
	ctx := context.Background()

	tests := []struct {
		name                    string
		pool                    *workpool.WorkPool
		existingSessions        []sessions.SessionStatus
		workers                 []*workpool.Worker
		expectedSessionsCreated int
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
			workers: []*workpool.Worker{
				{
					Name:     "worker-1",
					Provider: workpool.ProviderDocker,
					MaxSlots: 5,
					Active:   1,
					LastBeat: time.Now(),
				},
			},
			expectedSessionsCreated: 2,
		},
		{
			name: "no scaling needed",
			pool: &workpool.WorkPool{
				Name:           "test-pool",
				Provider:       workpool.ProviderDocker,
				MaxConcurrency: 10,
				MinSize:        2,
				AutoScale:      true,
			},
			existingSessions: []sessions.SessionStatus{
				sessions.StatusRunning,
				sessions.StatusIdle,
			},
			workers: []*workpool.Worker{
				{
					Name:     "worker-1",
					Provider: workpool.ProviderDocker,
					MaxSlots: 5,
					Active:   2,
					LastBeat: time.Now(),
				},
			},
			expectedSessionsCreated: 0,
		},
		{
			name: "paused pool - no scaling",
			pool: &workpool.WorkPool{
				Name:           "test-pool",
				Provider:       workpool.ProviderDocker,
				MaxConcurrency: 10,
				MinSize:        5,
				AutoScale:      true,
				Paused:         true,
			},
			existingSessions: []sessions.SessionStatus{},
			workers: []*workpool.Worker{
				{
					Name:     "worker-1",
					Provider: workpool.ProviderDocker,
					MaxSlots: 5,
					Active:   0,
					LastBeat: time.Now(),
				},
			},
			expectedSessionsCreated: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := reconciler.wpStore.CreateWorkPool(ctx, tt.pool)
			require.NoError(t, err)

			for _, status := range tt.existingSessions {
				sess := createTestSession(tt.pool.ID, status)
				err = reconciler.sessStore.CreateSession(ctx, sess)
				require.NoError(t, err)
			}

			for _, worker := range tt.workers {
				worker.PoolID = tt.pool.ID
				err = reconciler.wpStore.RegisterWorker(ctx, worker)
				require.NoError(t, err)
			}

			var sessionsBefore int64
			err = db.Model(&sessions.Session{}).
				Where("work_pool_id = ?", tt.pool.ID).
				Count(&sessionsBefore).Error
			require.NoError(t, err)

			err = reconciler.reconcilePool(ctx, tt.pool)
			assert.NoError(t, err)

			var sessionsAfter int64
			err = db.Model(&sessions.Session{}).
				Where("work_pool_id = ?", tt.pool.ID).
				Count(&sessionsAfter).Error
			require.NoError(t, err)

			sessionsCreated := int(sessionsAfter - sessionsBefore)
			assert.Equal(t, tt.expectedSessionsCreated, sessionsCreated)

			err = db.Where("work_pool_id = ?", tt.pool.ID).Delete(&sessions.Session{}).Error
			require.NoError(t, err)
			err = db.Where("pool_id = ?", tt.pool.ID).Delete(&workpool.Worker{}).Error
			require.NoError(t, err)
			err = db.Delete(tt.pool).Error
			require.NoError(t, err)
		})
	}
}

func TestReconciler_Start_StopOnContext(t *testing.T) {
	db := setupTestDB(t)
	reconciler := NewReconciler(db)
	reconciler.tickInterval = 100 * time.Millisecond

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	err := reconciler.Start(ctx)
	assert.Equal(t, context.DeadlineExceeded, err)
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
	}
}

func stringPtr(s string) *string {
	return &s
}
