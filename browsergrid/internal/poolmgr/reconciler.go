package poolmgr

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	"github.com/autocrawlerHQ/browsergrid/internal/sessions"
	"github.com/autocrawlerHQ/browsergrid/internal/workpool"
)

type Reconciler struct {
	db        *gorm.DB
	wpStore   *workpool.Store
	sessStore *sessions.Store

	tickInterval   time.Duration
	workerTTL      time.Duration
	cleanupEnabled bool
}

func NewReconciler(db *gorm.DB) *Reconciler {
	return &Reconciler{
		db:        db,
		wpStore:   workpool.NewStore(db),
		sessStore: sessions.NewStore(db),

		tickInterval:   1 * time.Minute,
		workerTTL:      5 * time.Minute,
		cleanupEnabled: true,
	}
}

func (r *Reconciler) Start(ctx context.Context) error {
	log.Println("Starting pool reconciler...")

	ticker := time.NewTicker(r.tickInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("Pool reconciler stopping...")
			return ctx.Err()

		case <-ticker.C:
			if err := r.reconcile(ctx); err != nil {
				log.Printf("Reconciliation error: %v", err)
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
		if err := r.reconcilePool(ctx, &pool); err != nil {
			log.Printf("Error reconciling pool %s: %v", pool.Name, err)
		}
	}

	if r.cleanupEnabled {
		if err := r.cleanupOrphanedSessions(ctx); err != nil {
			log.Printf("Error cleaning up orphaned sessions: %v", err)
		}

		if err := r.cleanupExpiredSessions(ctx); err != nil {
			log.Printf("Error cleaning up expired sessions: %v", err)
		}

		if err := r.cleanupStaleWorkers(ctx); err != nil {
			log.Printf("Error cleaning up stale workers: %v", err)
		}
	}

	return nil
}

func (r *Reconciler) reconcilePool(ctx context.Context, pool *workpool.WorkPool) error {
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

	_, workerActive, err := r.wpStore.GetWorkerCapacity(ctx, pool.ID)
	if err != nil {
		return err
	}

	workerSlots, _, err := r.wpStore.GetWorkerCapacity(ctx, pool.ID)
	if err != nil {
		return err
	}

	availableWorkerSlots := workerSlots - workerActive

	log.Printf("Pool %s: active=%d, pending=%d, worker_slots=%d, worker_active=%d, min_size=%d",
		pool.Name, activeCount, pendingCount, workerSlots, workerActive, pool.MinSize)

	sessionsToCreate := pool.SessionsToCreate(activeCount, pendingCount, availableWorkerSlots)

	if sessionsToCreate > 0 {
		log.Printf("Scaling up pool %s by %d sessions (policy-driven)", pool.Name, sessionsToCreate)

		for i := 0; i < sessionsToCreate; i++ {
			sess := r.createSessionFromPool(pool)
			if err := r.sessStore.CreateSession(ctx, sess); err != nil {
				log.Printf("Error creating session for pool %s: %v", pool.Name, err)
				break
			}
		}
	}

	if err := r.cleanupIdleSessionsForPool(ctx, pool); err != nil {
		log.Printf("Error cleaning up idle sessions for pool %s: %v", pool.Name, err)
	}

	return nil
}

func (r *Reconciler) createSessionFromPool(pool *workpool.WorkPool) *sessions.Session {
	env := pool.DefaultEnv
	if env == nil {
		envData, _ := json.Marshal(map[string]string{})
		env = datatypes.JSON(envData)
	}

	sess := &sessions.Session{
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
		Environment: env,
		Status:      sessions.StatusPending,
		Provider:    string(pool.Provider),
		WorkPoolID:  &pool.ID,
		IsPooled:    false,
	}

	if pool.DefaultImage != nil {
		var envMap map[string]string
		if err := json.Unmarshal(sess.Environment, &envMap); err != nil {
			envMap = make(map[string]string)
		}
		envMap["BROWSER_IMAGE"] = *pool.DefaultImage

		envData, _ := json.Marshal(envMap)
		sess.Environment = datatypes.JSON(envData)
	}

	return sess
}

func (r *Reconciler) countSessionsByStatus(ctx context.Context, poolID uuid.UUID, statuses []sessions.SessionStatus) (int, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&sessions.Session{}).
		Where("work_pool_id = ? AND status IN ?", poolID, statuses).
		Count(&count).Error

	return int(count), err
}

func (r *Reconciler) cleanupIdleSessionsForPool(ctx context.Context, pool *workpool.WorkPool) error {
	if pool.MaxIdleTime <= 0 {
		return nil
	}

	idleTimeout := time.Duration(pool.MaxIdleTime) * time.Second
	cutoff := time.Now().Add(-idleTimeout)

	result := r.db.WithContext(ctx).
		Model(&sessions.Session{}).
		Where("work_pool_id = ? AND status = ? AND updated_at < ?",
			pool.ID, sessions.StatusIdle, cutoff).
		Update("status", sessions.StatusExpired)

	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected > 0 {
		log.Printf("Pool %s: marked %d idle sessions as expired (idle_timeout=%ds)",
			pool.Name, result.RowsAffected, pool.MaxIdleTime)
	}

	return nil
}

func (r *Reconciler) cleanupExpiredSessions(ctx context.Context) error {
	cleanupAge := 24 * time.Hour
	cleanupCutoff := time.Now().Add(-cleanupAge)

	terminalStatuses := []sessions.SessionStatus{
		sessions.StatusCompleted,
		sessions.StatusFailed,
		sessions.StatusExpired,
		sessions.StatusCrashed,
		sessions.StatusTimedOut,
		sessions.StatusTerminated,
	}

	result := r.db.WithContext(ctx).
		Where("status IN ? AND updated_at < ?", terminalStatuses, cleanupCutoff).
		Delete(&sessions.Session{})

	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected > 0 {
		log.Printf("Cleaned up %d old terminated sessions", result.RowsAffected)
	}

	return nil
}

// cleanupOrphanedSessions finds sessions that are still in non-terminal states
// but whose worker is missing or has not sent a heartbeat within workerTTL.
// These sessions are marked as crashed to prevent them from lingering forever.
func (r *Reconciler) cleanupOrphanedSessions(ctx context.Context) error {
	cutoff := time.Now().Add(-r.workerTTL)

	// Sessions in these states should have an active worker
	liveStates := []sessions.SessionStatus{
		sessions.StatusStarting,
		sessions.StatusRunning,
		sessions.StatusIdle,
	}

	// Find sessions whose worker is missing or has stale heartbeat
	result := r.db.WithContext(ctx).
		Exec(`
			UPDATE sessions 
			SET status = ?, updated_at = ? 
			WHERE status IN (?) 
			AND (
				worker_id IS NULL 
				OR worker_id NOT IN (
					SELECT id FROM workers WHERE last_beat > ?
				)
			)
		`, sessions.StatusCrashed, time.Now(), liveStates, cutoff)

	if result.Error != nil {
		return result.Error
	}

	orphanedCount := result.RowsAffected

	// Also cleanup sessions that have been stuck in "Starting" state for too long
	// This handles cases where a worker claimed a session but crashed before updating it
	startingTimeout := 10 * time.Minute // Allow 10 minutes for a session to start
	startingCutoff := time.Now().Add(-startingTimeout)

	result = r.db.WithContext(ctx).
		Model(&sessions.Session{}).
		Where("status = ? AND updated_at < ?", sessions.StatusStarting, startingCutoff).
		Updates(map[string]interface{}{
			"status":     sessions.StatusCrashed,
			"updated_at": time.Now(),
		})

	if result.Error != nil {
		return result.Error
	}

	stuckStartingCount := result.RowsAffected
	totalCleaned := orphanedCount + stuckStartingCount

	if totalCleaned > 0 {
		log.Printf("Marked %d orphaned sessions as crashed (%d worker missing/stale, %d stuck starting)",
			totalCleaned, orphanedCount, stuckStartingCount)
	}

	return nil
}

// cleanupStaleWorkers removes workers that haven't sent a heartbeat within the TTL
// This prevents the workers table from growing indefinitely and removes zombie workers
func (r *Reconciler) cleanupStaleWorkers(ctx context.Context) error {
	// Use a longer TTL for cleanup than for considering workers online
	// This gives workers time to recover from temporary network issues
	cleanupTTL := r.workerTTL * 3 // 3x the normal TTL (15 minutes by default)
	cutoff := time.Now().Add(-cleanupTTL)

	result := r.db.WithContext(ctx).
		Where("last_beat < ?", cutoff).
		Delete(&workpool.Worker{})

	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected > 0 {
		log.Printf("Cleaned up %d stale workers (last_beat < %v)",
			result.RowsAffected, cutoff.Format(time.RFC3339))
	}

	return nil
}

func (r *Reconciler) GetPoolStats(ctx context.Context, poolID uuid.UUID) (*PoolStats, error) {
	pool, err := r.wpStore.GetWorkPool(ctx, poolID)
	if err != nil {
		return nil, err
	}

	statusCounts := make(map[sessions.SessionStatus]int)
	statuses := []sessions.SessionStatus{
		sessions.StatusPending, sessions.StatusStarting, sessions.StatusAvailable,
		sessions.StatusClaimed, sessions.StatusRunning, sessions.StatusIdle,
		sessions.StatusCompleted, sessions.StatusFailed, sessions.StatusExpired,
		sessions.StatusCrashed, sessions.StatusTimedOut, sessions.StatusTerminated,
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

	workers, err := r.wpStore.ListWorkers(ctx, &poolID, nil)
	if err != nil {
		return nil, err
	}

	onlineWorkers := 0
	totalWorkerSlots := 0
	activeWorkerSlots := 0

	for _, worker := range workers {
		if worker.IsOnline(r.workerTTL) {
			onlineWorkers++
			totalWorkerSlots += worker.MaxSlots
			activeWorkerSlots += worker.Active
		}
	}

	utilizationPercent := 0.0
	if pool.MaxConcurrency > 0 {
		activeSessions := statusCounts[sessions.StatusStarting] +
			statusCounts[sessions.StatusRunning] +
			statusCounts[sessions.StatusIdle]
		utilizationPercent = float64(activeSessions) / float64(pool.MaxConcurrency) * 100
	}

	return &PoolStats{
		Pool:               *pool,
		SessionsByStatus:   statusCounts,
		TotalWorkers:       len(workers),
		OnlineWorkers:      onlineWorkers,
		TotalWorkerSlots:   totalWorkerSlots,
		ActiveWorkerSlots:  activeWorkerSlots,
		UtilizationPercent: utilizationPercent,
		ScalingInfo: ScalingInfo{
			MinSize:            pool.MinSize,
			MaxConcurrency:     pool.MaxConcurrency,
			AutoScale:          pool.AutoScale,
			MaxIdleTime:        pool.MaxIdleTime,
			MaxSessionDuration: pool.MaxSessionDuration,
		},
	}, nil
}

type PoolStats struct {
	Pool               workpool.WorkPool              `json:"pool"`
	SessionsByStatus   map[sessions.SessionStatus]int `json:"sessions_by_status"`
	TotalWorkers       int                            `json:"total_workers"`
	OnlineWorkers      int                            `json:"online_workers"`
	TotalWorkerSlots   int                            `json:"total_worker_slots"`
	ActiveWorkerSlots  int                            `json:"active_worker_slots"`
	UtilizationPercent float64                        `json:"utilization_percent"`
	ScalingInfo        ScalingInfo                    `json:"scaling_info"`
}

type ScalingInfo struct {
	MinSize            int  `json:"min_size"`
	MaxConcurrency     int  `json:"max_concurrency"`
	AutoScale          bool `json:"auto_scale"`
	MaxIdleTime        int  `json:"max_idle_time"`
	MaxSessionDuration int  `json:"max_session_duration"`
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
