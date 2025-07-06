package poolmgr

import (
	"context"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"gorm.io/gorm"

	"github.com/autocrawlerHQ/browsergrid/internal/sessions"
	"github.com/autocrawlerHQ/browsergrid/internal/tasks"
	"github.com/autocrawlerHQ/browsergrid/internal/workpool"
)

// Reconciler monitors work pools and enqueues scaling tasks as needed
type Reconciler struct {
	db         *gorm.DB
	wpStore    *workpool.Store
	sessStore  *sessions.Store
	taskClient *asynq.Client
	redisOpt   asynq.RedisClientOpt

	tickInterval time.Duration
}

func NewReconciler(db *gorm.DB, taskClient *asynq.Client) *Reconciler {
	return &Reconciler{
		db:         db,
		wpStore:    workpool.NewStore(db),
		sessStore:  sessions.NewStore(db),
		taskClient: taskClient,

		tickInterval: 30 * time.Second, // Check every 30 seconds
	}
}

func (r *Reconciler) Start(ctx context.Context) error {
	log.Println("[RECONCILER] Starting pool reconciler...")

	// Schedule periodic cleanup tasks
	if err := r.scheduleCleanupTasks(); err != nil {
		log.Printf("[RECONCILER] Failed to schedule cleanup tasks: %v", err)
	}

	ticker := time.NewTicker(r.tickInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("[RECONCILER] Pool reconciler stopping...")
			return ctx.Err()

		case <-ticker.C:
			if err := r.reconcile(ctx); err != nil {
				log.Printf("[RECONCILER] Reconciliation error: %v", err)
			}
		}
	}
}

func (r *Reconciler) reconcile(ctx context.Context) error {
	pools, err := r.wpStore.ListWorkPools(ctx, nil)
	if err != nil {
		return err
	}

	for _, pool := range pools {
		if pool.Paused || !pool.AutoScale {
			continue
		}

		if err := r.reconcilePool(ctx, &pool); err != nil {
			log.Printf("[RECONCILER] Error reconciling pool %s: %v", pool.Name, err)
		}
	}

	return nil
}

func (r *Reconciler) reconcilePool(ctx context.Context, pool *workpool.WorkPool) error {
	// Count sessions by status
	activeCount, err := r.countSessionsByStatus(ctx, pool.ID, []sessions.SessionStatus{
		sessions.StatusStarting, sessions.StatusRunning, sessions.StatusIdle,
	})
	if err != nil {
		return err
	}

	pendingCount, err := r.countSessionsByStatus(ctx, pool.ID, []sessions.SessionStatus{
		sessions.StatusPending,
	})
	if err != nil {
		return err
	}

	totalSessions := activeCount + pendingCount

	log.Printf("[RECONCILER] Pool %s: active=%d, pending=%d, min_size=%d, max=%d",
		pool.Name, activeCount, pendingCount, pool.MinSize, pool.MaxConcurrency)

	// Calculate how many sessions we need to create
	var sessionsToCreate int
	if totalSessions < pool.MinSize {
		sessionsToCreate = pool.MinSize - totalSessions
	}

	// Don't exceed max concurrency
	if totalSessions+sessionsToCreate > pool.MaxConcurrency {
		sessionsToCreate = pool.MaxConcurrency - totalSessions
	}

	if sessionsToCreate > 0 {
		log.Printf("[RECONCILER] Pool %s needs %d more sessions", pool.Name, sessionsToCreate)

		// Enqueue a scaling task
		payload := tasks.PoolScalePayload{
			WorkPoolID:      pool.ID,
			DesiredSessions: sessionsToCreate,
		}

		task, err := tasks.NewPoolScaleTask(payload)
		if err != nil {
			return err
		}

		info, err := r.taskClient.Enqueue(task,
			asynq.Queue("low"),
			asynq.MaxRetry(3),
			asynq.Timeout(5*time.Minute),
		)
		if err != nil {
			return err
		}

		log.Printf("[RECONCILER] Enqueued scaling task %s for pool %s", info.ID, pool.Name)
	}

	// Check for idle sessions that should be terminated
	if pool.MaxIdleTime > 0 {
		idleTimeout := time.Duration(pool.MaxIdleTime) * time.Second
		cutoff := time.Now().Add(-idleTimeout)

		status := sessions.StatusIdle
		sessionsList, err := r.sessStore.ListSessions(ctx, &status, nil, &cutoff, 0, 1000) // adjust limit as needed
		if err != nil {
			return err
		}
		// Filter by WorkPoolID in Go, since ListSessions does not support WorkPoolID directly
		var idleSessions []sessions.Session
		for _, sess := range sessionsList {
			if sess.WorkPoolID != nil && *sess.WorkPoolID == pool.ID {
				idleSessions = append(idleSessions, sess)
			}
		}

		for _, sess := range idleSessions {
			log.Printf("[RECONCILER] Session %s has been idle for too long, terminating", sess.ID)

			// Enqueue stop task
			stopPayload := tasks.SessionStopPayload{
				SessionID: sess.ID,
				Reason:    "idle_timeout",
			}

			stopTask, _ := tasks.NewSessionStopTask(stopPayload)
			r.taskClient.Enqueue(stopTask, asynq.Queue("default"))
		}
	}

	return nil
}

func (r *Reconciler) scheduleCleanupTasks() error {
	// Schedule hourly cleanup of expired sessions
	scheduler := asynq.NewScheduler(
		asynq.RedisClientOpt{Addr: r.redisOpt.Addr},
		nil,
	)

	// Cleanup expired sessions every hour
	cleanupPayload := tasks.CleanupExpiredPayload{
		MaxAge: 24, // Remove sessions older than 24 hours
	}
	cleanupTask, _ := tasks.NewCleanupExpiredTask(cleanupPayload)

	_, err := scheduler.Register("0 * * * *", cleanupTask, asynq.Queue("low"))
	if err != nil {
		return err
	}

	if err := scheduler.Start(); err != nil {
		return err
	}

	log.Println("[RECONCILER] Scheduled periodic cleanup tasks")
	return nil
}

func (r *Reconciler) countSessionsByStatus(ctx context.Context, poolID uuid.UUID, statuses []sessions.SessionStatus) (int, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&sessions.Session{}).
		Where("work_pool_id = ? AND status IN ?", poolID, statuses).
		Count(&count).Error

	return int(count), err
}

// GetPoolStats returns statistics for monitoring
func (r *Reconciler) GetPoolStats(ctx context.Context, poolID uuid.UUID) (*PoolStats, error) {
	pool, err := r.wpStore.GetWorkPool(ctx, poolID)
	if err != nil {
		return nil, err
	}

	// Get queue stats from Redis
	inspector := asynq.NewInspector(r.redisOpt)

	queueName := getQueueNameForProvider(pool.Provider)
	queueInfo, err := inspector.GetQueueInfo(queueName)
	if err != nil {
		log.Printf("[RECONCILER] Failed to get queue info: %v", err)
	}

	// Count sessions by status
	statusCounts := make(map[sessions.SessionStatus]int)
	statuses := []sessions.SessionStatus{
		sessions.StatusPending, sessions.StatusStarting, sessions.StatusRunning,
		sessions.StatusIdle, sessions.StatusCompleted, sessions.StatusFailed,
		sessions.StatusExpired, sessions.StatusCrashed, sessions.StatusTimedOut,
		sessions.StatusTerminated,
	}

	for _, status := range statuses {
		var count int64
		err := r.db.WithContext(ctx).Model(&sessions.Session{}).
			Where("work_pool_id = ? AND status = ?", poolID, status).
			Count(&count).Error
		if err != nil {
			return nil, err
		}
		statusCounts[status] = int(count)
	}

	activeSessions := statusCounts[sessions.StatusStarting] +
		statusCounts[sessions.StatusRunning] +
		statusCounts[sessions.StatusIdle]

	utilizationPercent := 0.0
	if pool.MaxConcurrency > 0 {
		utilizationPercent = float64(activeSessions) / float64(pool.MaxConcurrency) * 100
	}

	stats := &PoolStats{
		Pool:               *pool,
		SessionsByStatus:   statusCounts,
		UtilizationPercent: utilizationPercent,
		QueueStats: QueueStats{
			Pending:   queueInfo.Pending,
			Active:    queueInfo.Active,
			Scheduled: queueInfo.Scheduled,
			Retry:     queueInfo.Retry,
			Archived:  queueInfo.Archived,
			Completed: queueInfo.Completed,
		},
		ScalingInfo: ScalingInfo{
			MinSize:            pool.MinSize,
			MaxConcurrency:     pool.MaxConcurrency,
			AutoScale:          pool.AutoScale,
			MaxIdleTime:        pool.MaxIdleTime,
			MaxSessionDuration: pool.MaxSessionDuration,
		},
	}

	return stats, nil
}

// Types

type PoolStats struct {
	Pool               workpool.WorkPool              `json:"pool"`
	SessionsByStatus   map[sessions.SessionStatus]int `json:"sessions_by_status"`
	UtilizationPercent float64                        `json:"utilization_percent"`
	QueueStats         QueueStats                     `json:"queue_stats"`
	ScalingInfo        ScalingInfo                    `json:"scaling_info"`
}

type QueueStats struct {
	Pending   int `json:"pending"`
	Active    int `json:"active"`
	Scheduled int `json:"scheduled"`
	Retry     int `json:"retry"`
	Archived  int `json:"archived"`
	Completed int `json:"completed"`
}

type ScalingInfo struct {
	MinSize            int  `json:"min_size"`
	MaxConcurrency     int  `json:"max_concurrency"`
	AutoScale          bool `json:"auto_scale"`
	MaxIdleTime        int  `json:"max_idle_time"`
	MaxSessionDuration int  `json:"max_session_duration"`
}

// Helper functions

func getQueueNameForProvider(provider workpool.ProviderType) string {
	switch provider {
	case workpool.ProviderDocker:
		return "default"
	case workpool.ProviderACI:
		return "azure"
	case workpool.ProviderLocal:
		return "local"
	default:
		return "default"
	}
}

func ptr[T any](v T) *T {
	return &v
}
