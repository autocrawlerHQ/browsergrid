package scheduler

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/autocrawlerHQ/browsergrid/internal/sessions"
	"github.com/autocrawlerHQ/browsergrid/internal/tasks"
	"github.com/autocrawlerHQ/browsergrid/internal/workpool"
)

func setupTestScheduler(t *testing.T) (*Service, *gorm.DB, *asynq.Inspector) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	require.NoError(t, err)

	err = db.AutoMigrate(
		&sessions.Session{},
		&sessions.SessionEvent{},
		&sessions.SessionMetrics{},
		&workpool.WorkPool{},
	)
	require.NoError(t, err)

	redisOpt := asynq.RedisClientOpt{Addr: "localhost:6379"}

	service := New(db, redisOpt)
	inspector := asynq.NewInspector(redisOpt)

	queues, _ := inspector.Queues()
	for _, q := range queues {
		inspector.DeleteAllPendingTasks(q)
		inspector.DeleteAllScheduledTasks(q)
	}

	return service, db, inspector
}

func TestHandlePoolScale(t *testing.T) {
	service, db, inspector := setupTestScheduler(t)
	defer service.Stop()

	ctx := context.Background()

	pool := &workpool.WorkPool{
		ID:                 uuid.New(),
		Name:               "test-pool",
		Provider:           workpool.ProviderDocker,
		MaxConcurrency:     10,
		MaxSessionDuration: 300,
	}
	err := db.Create(pool).Error
	require.NoError(t, err)

	payload := tasks.PoolScalePayload{
		WorkPoolID:      pool.ID,
		DesiredSessions: 3,
	}

	data, _ := payload.Marshal()
	task := asynq.NewTask(tasks.TypePoolScale, data)

	err = service.handlePoolScale(ctx, task)
	assert.NoError(t, err)

	var sessionCount int64
	err = db.Model(&sessions.Session{}).
		Where("work_pool_id = ?", pool.ID).
		Count(&sessionCount).Error
	require.NoError(t, err)
	assert.Equal(t, int64(3), sessionCount)

	time.Sleep(100 * time.Millisecond)

	pendingTasks, err := inspector.ListPendingTasks("default")
	require.NoError(t, err)

	startTaskCount := 0
	for _, task := range pendingTasks {
		if task.Type == tasks.TypeSessionStart {
			startTaskCount++
		}
	}
	assert.Equal(t, 3, startTaskCount)
}

func TestHandleCleanupExpired(t *testing.T) {
	service, db, _ := setupTestScheduler(t)
	defer service.Stop()

	ctx := context.Background()

	oldTime := time.Now().Add(-48 * time.Hour)

	oldSessions := []sessions.Session{
		{
			ID:        uuid.New(),
			Status:    sessions.StatusCompleted,
			UpdatedAt: oldTime,
		},
		{
			ID:        uuid.New(),
			Status:    sessions.StatusFailed,
			UpdatedAt: oldTime,
		},
		{
			ID:        uuid.New(),
			Status:    sessions.StatusExpired,
			UpdatedAt: oldTime,
		},
	}

	for _, sess := range oldSessions {
		err := db.Create(&sess).Error
		require.NoError(t, err)
	}

	recentSession := sessions.Session{
		ID:        uuid.New(),
		Status:    sessions.StatusCompleted,
		UpdatedAt: time.Now(),
	}
	err := db.Create(&recentSession).Error
	require.NoError(t, err)

	activeSession := sessions.Session{
		ID:        uuid.New(),
		Status:    sessions.StatusRunning,
		UpdatedAt: oldTime,
	}
	err = db.Create(&activeSession).Error
	require.NoError(t, err)

	payload := tasks.CleanupExpiredPayload{
		MaxAge: 24,
	}

	data, _ := payload.Marshal()
	task := asynq.NewTask(tasks.TypeCleanupExpired, data)

	err = service.handleCleanupExpired(ctx, task)
	assert.NoError(t, err)

	var remainingCount int64
	err = db.Model(&sessions.Session{}).Count(&remainingCount).Error
	require.NoError(t, err)
	assert.Equal(t, int64(2), remainingCount)

	var remaining sessions.Session
	err = db.First(&remaining, "id = ?", recentSession.ID).Error
	assert.NoError(t, err)

	err = db.First(&remaining, "id = ?", activeSession.ID).Error
	assert.NoError(t, err)
}

func TestCreateSessionFromPool(t *testing.T) {
	service, _, _ := setupTestScheduler(t)
	defer service.Stop()

	tests := []struct {
		name string
		pool *workpool.WorkPool
	}{
		{
			name: "pool with defaults",
			pool: &workpool.WorkPool{
				ID:           uuid.New(),
				Name:         "test-pool",
				Provider:     workpool.ProviderDocker,
				DefaultEnv:   nil,
				DefaultImage: nil,
			},
		},
		{
			name: "pool with custom image",
			pool: &workpool.WorkPool{
				ID:           uuid.New(),
				Name:         "test-pool",
				Provider:     workpool.ProviderDocker,
				DefaultImage: ptr("custom:latest"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session := service.createSessionFromPool(tt.pool)

			assert.NotEqual(t, uuid.Nil, session.ID)
			assert.Equal(t, sessions.StatusPending, session.Status)
			assert.Equal(t, string(tt.pool.Provider), session.Provider)
			assert.Equal(t, &tt.pool.ID, session.WorkPoolID)
			assert.Equal(t, sessions.BrowserChrome, session.Browser)
			assert.True(t, session.Headless)

			if tt.pool.DefaultImage != nil {
				var env map[string]string
				err := session.Environment.Scan([]byte(session.Environment))
				require.NoError(t, err)
				assert.Equal(t, *tt.pool.DefaultImage, env["BROWSER_IMAGE"])
			}
		})
	}
}

func ptr(s string) *string {
	return &s
}
