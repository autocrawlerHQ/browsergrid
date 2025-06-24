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
	cfg := loadConfig()

	db, err := connectDB(cfg.DatabaseURL)
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}

	ws := workpool.NewStore(db)
	ss := sessions.NewStore(db)

	if cfg.WorkPoolID == "" {
		pool, err := ensureDefaultPool(context.Background(), ws, cfg.Provider)
		if err != nil {
			log.Fatalf("could not obtain default work-pool: %v", err)
		}
		cfg.WorkPoolID = pool.ID.String()
		log.Printf("Using default work-pool %s (%s)", pool.Name, pool.ID)
	}

	poolID, err := uuid.Parse(cfg.WorkPoolID)
	if err != nil {
		log.Fatal("Invalid work pool ID:", err)
	}

	pool, err := ws.GetWorkPool(context.Background(), poolID)
	if err != nil {
		log.Fatal("Work pool not found:", err)
	}

	log.Printf("Connecting to work pool: %s (%s)", pool.Name, pool.Provider)

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

	log.Printf("Worker registered: %s (ID: %s)", worker.Name, worker.ID)
	log.Printf("Max concurrency: %d", worker.MaxSlots)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := runWorker(ctx, worker, ws, ss, prov, cfg.PollInterval); err != nil {
			log.Printf("Worker error: %v", err)
		}
	}()

	<-sigChan
	log.Println("Shutting down worker...")

	ws.PauseWorker(context.Background(), worker.ID, true)

	cancel()

	shutdownTimeout := 30 * time.Second
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer shutdownCancel()

	for {
		select {
		case <-shutdownCtx.Done():
			log.Println("Shutdown timeout reached, forcing exit")
			return
		default:
			if atomic.LoadInt32(&worker.activeCount) == 0 {
				log.Println("All sessions completed, exiting")
				return
			}
			time.Sleep(1 * time.Second)
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
	log.Printf("Starting worker loop with %v poll interval", pollInterval)

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	heartbeatTicker := time.NewTicker(30 * time.Second)
	defer heartbeatTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case <-ticker.C:
			currentWorker, err := ws.GetWorker(ctx, worker.ID)
			if err != nil {
				log.Printf("Error getting worker status: %v", err)
				continue
			}

			if currentWorker.MaxSlots != worker.MaxSlots {
				log.Printf("Remote update detected: max_slots %d → %d",
					worker.MaxSlots, currentWorker.MaxSlots)
				worker.MaxSlots = currentWorker.MaxSlots
			}
			worker.Paused = currentWorker.Paused

			if currentWorker.Paused {
				log.Println("Worker is paused, skipping work polling")
				continue
			}

			availableSlots := worker.MaxSlots - int(atomic.LoadInt32(&worker.activeCount))
			if availableSlots <= 0 {
				continue
			}

			sessions, err := ws.DequeueSessions(ctx, worker.PoolID, worker.ID, availableSlots)
			if err != nil {
				log.Printf("Error dequeuing sessions: %v", err)
				continue
			}

			for _, sess := range sessions {
				go handleSession(ctx, prov, ss, ws, worker, sess)
			}

		case <-heartbeatTicker.C:
			activeCount := int(atomic.LoadInt32(&worker.activeCount))
			if err := ws.Heartbeat(ctx, worker.ID, activeCount); err != nil {
				log.Printf("Error sending heartbeat: %v", err)
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

	log.Printf("Starting session %s", sess.ID)

	// Get work pool configuration for session duration settings
	pool, err := ws.GetWorkPool(ctx, worker.PoolID)
	if err != nil {
		log.Printf("Failed to get work pool for session %s: %v", sess.ID, err)
		ss.UpdateSessionStatus(ctx, sess.ID, sessions.StatusFailed)
		return
	}

	wsURL, liveURL, err := prov.Start(ctx, &sess)
	if err != nil {
		log.Printf("Failed to start session %s: %v", sess.ID, err)
		ss.UpdateSessionStatus(ctx, sess.ID, sessions.StatusFailed)
		return
	}

	if err := ss.UpdateSessionEndpoints(ctx, sess.ID, wsURL, liveURL, sessions.StatusRunning); err != nil {
		log.Printf("Failed to update session %s: %v", sess.ID, err)
	}

	log.Printf("Session %s is running: ws=%s, live=%s", sess.ID, wsURL, liveURL)

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

	log.Printf("Session %s will run for max %v (pool: %s)", sess.ID, sessionDuration, pool.Name)

	for {
		select {
		case <-timeoutCtx.Done():
			log.Printf("Session %s timed out", sess.ID)
			ss.UpdateSessionStatus(ctx, sess.ID, sessions.StatusTimedOut)
			goto cleanup

		case <-healthTicker.C:
			if err := prov.HealthCheck(ctx, &sess); err != nil {
				log.Printf("Session %s health check failed: %v", sess.ID, err)
				ss.UpdateSessionStatus(ctx, sess.ID, sessions.StatusFailed)
				goto cleanup
			}

			if metrics, err := prov.GetMetrics(ctx, &sess); err == nil {
				ss.CreateMetrics(ctx, metrics)
			}

			// Check if session has reached its configured duration
			if time.Since(sess.CreatedAt) > sessionDuration {
				log.Printf("Session %s completed after %v", sess.ID, time.Since(sess.CreatedAt))
				ss.UpdateSessionStatus(ctx, sess.ID, sessions.StatusCompleted)
				goto cleanup
			}
		}
	}

cleanup:
	if err := prov.Stop(ctx, &sess); err != nil {
		log.Printf("Error stopping session %s: %v", sess.ID, err)
	}

	log.Printf("Session %s finished", sess.ID)
}

func ensureDefaultPool(ctx context.Context, store *workpool.Store, provider string) (*workpool.WorkPool, error) {
	name := fmt.Sprintf("default-%s", provider)

	pool, err := store.GetWorkPoolByName(ctx, name)
	if err == nil {
		return pool, nil
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

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
	log.Printf("Created default work-pool: %s (%s)", pool.Name, pool.ID)
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
