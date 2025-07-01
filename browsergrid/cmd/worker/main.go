package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/autocrawlerHQ/browsergrid/internal/provider"
	_ "github.com/autocrawlerHQ/browsergrid/internal/provider/docker"
	"github.com/autocrawlerHQ/browsergrid/internal/sessions"
	"github.com/autocrawlerHQ/browsergrid/internal/workpool"
)

type WorkerConfig struct {
	WorkPoolID   string
	Name         string
	Provider     string
	Concurrency  int
	DatabaseURL  string
	PollInterval time.Duration
	Hostname     string
}

type WorkerRuntime struct {
	*workpool.Worker
	activeCount int32
}

func main() {
	// Print startup banner
	log.Println("========================================")
	log.Println("       BrowserGrid Worker v1.0         ")
	log.Println("========================================")

	cfg := loadConfig()

	db, err := connectDB(cfg.DatabaseURL)
	if err != nil {
		log.Fatal("[STARTUP] ✗ Failed to connect to database:", err)
	}

	ws := workpool.NewStore(db)
	ss := sessions.NewStore(db)

	log.Printf("[STARTUP] Initializing worker...")
	log.Printf("[STARTUP] └── Name: %s", cfg.Name)
	log.Printf("[STARTUP] └── Provider: %s", cfg.Provider)
	log.Printf("[STARTUP] └── Concurrency: %d", cfg.Concurrency)
	log.Printf("[STARTUP] └── Poll Interval: %v", cfg.PollInterval)

	if cfg.WorkPoolID == "" {
		log.Printf("[STARTUP] No work pool specified, creating default pool...")
		pool, err := ensureDefaultPool(context.Background(), ws, cfg.Provider)
		if err != nil {
			log.Fatalf("[STARTUP] ✗ Could not obtain default work pool: %v", err)
		}
		cfg.WorkPoolID = pool.ID.String()
		log.Printf("[STARTUP] ✓ Using default work pool: %s (%s)", pool.Name, pool.ID)
	}

	poolID, err := uuid.Parse(cfg.WorkPoolID)
	if err != nil {
		log.Fatalf("[STARTUP] ✗ Invalid work pool ID: %v", err)
	}

	pool, err := ws.GetWorkPool(context.Background(), poolID)
	if err != nil {
		log.Fatalf("[STARTUP] ✗ Work pool not found: %v", err)
	}

	log.Printf("[STARTUP] ✓ Connected to work pool: %s", pool.Name)
	log.Printf("[STARTUP] └── Provider: %s", pool.Provider)
	log.Printf("[STARTUP] └── Max Concurrency: %d", pool.MaxConcurrency)
	log.Printf("[STARTUP] └── Auto Scale: %v", pool.AutoScale)

	prov, ok := provider.FromString(cfg.Provider)
	if !ok {
		log.Fatalf("Unknown provider type: %s", cfg.Provider)
	}

	worker := &WorkerRuntime{
		Worker: &workpool.Worker{
			ID:       uuid.New(),
			PoolID:   poolID,
			Name:     cfg.Name,
			Hostname: cfg.Hostname,
			Provider: workpool.ProviderType(cfg.Provider),
			MaxSlots: cfg.Concurrency,
			Active:   0,
			Paused:   false,
		},
		activeCount: 0,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := ws.RegisterWorker(ctx, worker.Worker); err != nil {
		log.Fatal("Failed to register worker:", err)
	}

	log.Printf("[WORKER] %s registered successfully", worker.Name)
	log.Printf("[WORKER] └── ID: %s", worker.ID)
	log.Printf("[WORKER] └── Pool: %s (%s)", pool.Name, pool.ID)
	log.Printf("[WORKER] └── Provider: %s", worker.Provider)
	log.Printf("[WORKER] └── Max Concurrency: %d", worker.MaxSlots)
	log.Printf("[WORKER] └── Hostname: %s", worker.Hostname)
	log.Printf("[WORKER] Ready to process tasks...")
	log.Printf("[WORKER] =======================================")

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := runWorker(ctx, worker, ws, ss, prov, cfg.PollInterval); err != nil {
			log.Printf("Worker error: %v", err)
		}
	}()

	<-sigChan
	log.Printf("[SHUTDOWN] Received shutdown signal, initiating graceful shutdown...")

	// Pause worker to prevent new tasks
	ws.PauseWorker(context.Background(), worker.ID, true)
	log.Printf("[SHUTDOWN] Worker paused - no new tasks will be accepted")

	cancel() // Cancel worker context

	activeCount := int(atomic.LoadInt32(&worker.activeCount))
	if activeCount > 0 {
		log.Printf("[SHUTDOWN] Waiting for %d active task(s) to complete...", activeCount)
	}

	shutdownTimeout := 30 * time.Second
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer shutdownCancel()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-shutdownCtx.Done():
			remaining := int(atomic.LoadInt32(&worker.activeCount))
			log.Printf("[SHUTDOWN] ⏰ Shutdown timeout reached with %d task(s) still running", remaining)
			log.Printf("[SHUTDOWN] Forcing exit and marking remaining sessions as failed...")
			cleanupRemainingSessionsOnShutdown(context.Background(), ss, worker.ID)
			return
		case <-ticker.C:
			current := int(atomic.LoadInt32(&worker.activeCount))
			if current != activeCount {
				log.Printf("[SHUTDOWN] %d task(s) still running...", current)
				activeCount = current
			}
		default:
			if atomic.LoadInt32(&worker.activeCount) == 0 {
				log.Printf("[SHUTDOWN] ✓ All tasks completed successfully")
				log.Printf("[SHUTDOWN] Worker shutdown complete")
				return
			}
			time.Sleep(100 * time.Millisecond)
		}
	}
}

func runWorker(
	ctx context.Context,
	worker *WorkerRuntime,
	ws *workpool.Store,
	ss *sessions.Store,
	prov provider.Provisioner,
	pollInterval time.Duration,
) error {
	log.Printf("[WORKER] Starting main loop (poll_interval=%v)", pollInterval)

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	heartbeatTicker := time.NewTicker(30 * time.Second)
	defer heartbeatTicker.Stop()

	// Send initial heartbeat and status
	activeCount := int(atomic.LoadInt32(&worker.activeCount))
	if err := ws.Heartbeat(ctx, worker.ID, activeCount); err != nil {
		log.Printf("[HEARTBEAT] Failed to send initial heartbeat: %v", err)
	} else {
		log.Printf("[HEARTBEAT] Initial heartbeat sent (active=%d)", activeCount)
	}

	for {
		select {
		case <-ctx.Done():
			log.Printf("[WORKER] Shutting down worker loop: %v", ctx.Err())
			return ctx.Err()

		case <-ticker.C:
			// Check for remote configuration updates
			currentWorker, err := ws.GetWorker(ctx, worker.ID)
			if err != nil {
				log.Printf("[WORKER] Error fetching worker config: %v", err)
				continue
			}

			// Handle configuration changes
			if currentWorker.MaxSlots != worker.MaxSlots {
				log.Printf("[CONFIG] Concurrency updated: %d → %d",
					worker.MaxSlots, currentWorker.MaxSlots)
				worker.MaxSlots = currentWorker.MaxSlots
			}

			pauseChanged := worker.Paused != currentWorker.Paused
			worker.Paused = currentWorker.Paused

			if pauseChanged {
				if worker.Paused {
					log.Printf("[STATUS] Worker paused - will not accept new tasks")
				} else {
					log.Printf("[STATUS] Worker resumed - ready to accept tasks")
				}
			}

			if currentWorker.Paused {
				continue // Skip task polling when paused
			}

			// Calculate available capacity
			currentActive := int(atomic.LoadInt32(&worker.activeCount))
			availableSlots := worker.MaxSlots - currentActive

			if availableSlots <= 0 {
				// Only log this occasionally to avoid spam
				if ticker.C == ticker.C { // Simple way to log less frequently
					log.Printf("[CAPACITY] At full capacity (%d/%d active)", currentActive, worker.MaxSlots)
				}
				continue
			}

			// Poll for new tasks
			log.Printf("[POLL] Checking for tasks (capacity: %d/%d available)", availableSlots, worker.MaxSlots)
			sessions, err := ws.DequeueSessions(ctx, worker.PoolID, worker.ID, availableSlots)
			if err != nil {
				log.Printf("[POLL] Error dequeuing sessions: %v", err)
				continue
			}

			if len(sessions) > 0 {
				log.Printf("[POLL] Received %d new task(s)", len(sessions))
				for i, sess := range sessions {
					log.Printf("[POLL] └── Task %d/%d: %s (browser=%s, status=%s)",
						i+1, len(sessions), sess.ID, sess.Browser, sess.Status)
					go handleSession(ctx, prov, ss, ws, worker, sess)
				}
			}

		case <-heartbeatTicker.C:
			activeCount := int(atomic.LoadInt32(&worker.activeCount))
			if err := ws.Heartbeat(ctx, worker.ID, activeCount); err != nil {
				log.Printf("[HEARTBEAT] Failed to send heartbeat: %v", err)
			} else {
				log.Printf("[HEARTBEAT] ♥ active=%d/%d", activeCount, worker.MaxSlots)
			}
		}
	}
}

func handleSession(
	ctx context.Context,
	prov provider.Provisioner,
	ss *sessions.Store,
	ws *workpool.Store,
	worker *WorkerRuntime,
	sess sessions.Session,
) {
	atomic.AddInt32(&worker.activeCount, 1)
	defer atomic.AddInt32(&worker.activeCount, -1)

	taskStart := time.Now()
	log.Printf("[TASK] Starting %s", sess.ID)
	log.Printf("[TASK] └── Browser: %s %s", sess.Browser, sess.Version)
	log.Printf("[TASK] └── OS: %s", sess.OperatingSystem)
	log.Printf("[TASK] └── Screen: %dx%d", sess.Screen.Width, sess.Screen.Height)
	log.Printf("[TASK] └── Headless: %v", sess.Headless)

	// Get work pool configuration for session duration settings
	pool, err := ws.GetWorkPool(ctx, worker.PoolID)
	if err != nil {
		log.Printf("[TASK] ✗ Failed to get work pool for %s: %v", sess.ID, err)
		ss.UpdateSessionStatus(ctx, sess.ID, sessions.StatusFailed)
		return
	}

	log.Printf("[TASK] Provisioning browser container...")
	wsURL, liveURL, err := prov.Start(ctx, &sess)
	if err != nil {
		elapsed := time.Since(taskStart)
		log.Printf("[TASK] ✗ Failed to start %s after %v: %v", sess.ID, elapsed, err)
		ss.UpdateSessionStatus(ctx, sess.ID, sessions.StatusFailed)
		return
	}

	if err := ss.UpdateSessionEndpoints(ctx, sess.ID, wsURL, liveURL, sessions.StatusRunning); err != nil {
		log.Printf("[TASK] Warning: Failed to update session endpoints for %s: %v", sess.ID, err)
	}

	provisionTime := time.Since(taskStart)
	log.Printf("[TASK] ✓ %s ready after %v", sess.ID, provisionTime)
	log.Printf("[TASK] └── WebSocket: %s", wsURL)
	log.Printf("[TASK] └── Live URL: %s", liveURL)

	healthTicker := time.NewTicker(30 * time.Second)
	defer healthTicker.Stop()

	// Use work pool's MaxSessionDuration, default to 30 minutes if not set
	sessionDuration := 30 * time.Minute
	if pool.MaxSessionDuration > 0 {
		sessionDuration = time.Duration(pool.MaxSessionDuration) * time.Second
	}

	// Set timeout slightly longer than session duration to allow for graceful completion
	sessionTimeout := sessionDuration + (1 * time.Minute)
	timeoutCtx, timeoutCancel := context.WithTimeout(ctx, sessionTimeout)
	defer timeoutCancel()

	log.Printf("[TASK] %s will run for max %v (pool: %s)", sess.ID, sessionDuration, pool.Name)

	healthCheckCount := 0
	for {
		select {
		case <-timeoutCtx.Done():
			elapsed := time.Since(taskStart)
			log.Printf("[TASK] ⏰ %s timed out after %v", sess.ID, elapsed)
			ss.UpdateSessionStatus(ctx, sess.ID, sessions.StatusTimedOut)
			goto cleanup

		case <-healthTicker.C:
			healthCheckCount++
			if err := prov.HealthCheck(ctx, &sess); err != nil {
				elapsed := time.Since(taskStart)
				log.Printf("[TASK] ✗ %s health check failed after %v (check #%d): %v",
					sess.ID, elapsed, healthCheckCount, err)
				ss.UpdateSessionStatus(ctx, sess.ID, sessions.StatusFailed)
				goto cleanup
			}

			// Collect metrics
			if metrics, err := prov.GetMetrics(ctx, &sess); err == nil {
				ss.CreateMetrics(ctx, metrics)
				log.Printf("[METRICS] %s: CPU=%.1f%%, Memory=%.0fMB",
					sess.ID,
					safeDeref(metrics.CPUPercent, 0.0),
					safeDeref(metrics.MemoryMB, 0.0))
			}

			// Check if session has reached its configured duration
			if time.Since(sess.CreatedAt) > sessionDuration {
				elapsed := time.Since(taskStart)
				log.Printf("[TASK] ✓ %s completed after %v (natural expiration)", sess.ID, elapsed)
				ss.UpdateSessionStatus(ctx, sess.ID, sessions.StatusCompleted)
				goto cleanup
			}

			// Log periodic status
			elapsed := time.Since(taskStart)
			log.Printf("[TASK] %s running for %v (health check #%d passed)",
				sess.ID, elapsed, healthCheckCount)
		}
	}

cleanup:
	log.Printf("[TASK] Cleaning up %s...", sess.ID)
	if err := prov.Stop(ctx, &sess); err != nil {
		log.Printf("[TASK] ✗ Error stopping %s: %v", sess.ID, err)
	}

	totalTime := time.Since(taskStart)
	log.Printf("[TASK] ✓ %s finished (total_time=%v)", sess.ID, totalTime)
}

func ensureDefaultPool(ctx context.Context, store *workpool.Store, provider string) (*workpool.WorkPool, error) {
	name := fmt.Sprintf("default-%s", provider)

	pool, err := store.GetWorkPoolByName(ctx, name)
	if err == nil {
		log.Printf("[POOL] Found existing default pool: %s", name)
		return pool, nil
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	log.Printf("[POOL] Creating new default pool: %s", name)
	pool = &workpool.WorkPool{
		Name:               name,
		Provider:           workpool.ProviderType(provider),
		MaxConcurrency:     10,
		MaxSessionDuration: 900, // 15 minutes default
		AutoScale:          true,
	}
	if err := store.CreateWorkPool(ctx, pool); err != nil {
		return nil, err
	}
	log.Printf("[POOL] ✓ Created default work pool: %s (%s)", pool.Name, pool.ID)
	return pool, nil
}

func loadConfig() WorkerConfig {
	cfg := WorkerConfig{
		PollInterval: 10 * time.Second,
		Concurrency:  1,
	}

	flag.StringVar(&cfg.WorkPoolID, "pool", "", "Work pool ID (optional - will create default if not provided)")
	flag.StringVar(&cfg.Name, "name", "", "Worker name")
	flag.StringVar(&cfg.Provider, "provider", "docker", "Provider type (docker, local, aci)")
	flag.IntVar(&cfg.Concurrency, "concurrency", 1, "Maximum concurrent sessions")
	flag.StringVar(&cfg.DatabaseURL, "db", "", "Database URL")
	flag.DurationVar(&cfg.PollInterval, "poll-interval", 10*time.Second, "Poll interval for work")

	flag.Parse()

	if cfg.DatabaseURL == "" {
		cfg.DatabaseURL = os.Getenv("DATABASE_URL")
		if cfg.DatabaseURL == "" {
			cfg.DatabaseURL = "postgres://user:password@localhost/browsergrid?sslmode=disable"
		}
	}

	if cfg.Name == "" {
		hostname, _ := os.Hostname()
		cfg.Name = fmt.Sprintf("worker-%s", hostname)
	}

	cfg.Hostname, _ = os.Hostname()

	return cfg
}

func connectDB(databaseURL string) (*gorm.DB, error) {
	return gorm.Open(postgres.Open(databaseURL), &gorm.Config{})
}

// safeDeref safely dereferences a pointer, returning the default value if nil
func safeDeref[T any](ptr *T, defaultVal T) T {
	if ptr == nil {
		return defaultVal
	}
	return *ptr
}

// cleanupRemainingSessionsOnShutdown marks any sessions still assigned to this worker
// as failed when the worker is shutting down unexpectedly or timing out
func cleanupRemainingSessionsOnShutdown(ctx context.Context, ss *sessions.Store, workerID uuid.UUID) {
	// Use a short timeout context for cleanup
	cleanupCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	log.Printf("[CLEANUP] Marking remaining sessions as failed for worker %s", workerID)

	// Mark non-terminal sessions assigned to this worker as failed
	rowsAffected, err := ss.MarkWorkerSessionsFailed(cleanupCtx, workerID)
	if err != nil {
		log.Printf("[CLEANUP] ✗ Error cleaning up sessions on shutdown: %v", err)
		return
	}

	if rowsAffected > 0 {
		log.Printf("[CLEANUP] ✓ Marked %d session(s) as failed during worker shutdown", rowsAffected)
	} else {
		log.Printf("[CLEANUP] ✓ No sessions required cleanup")
	}
}
