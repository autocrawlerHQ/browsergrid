package main

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/hibiken/asynq"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/autocrawlerHQ/browsergrid/internal/config"
	"github.com/autocrawlerHQ/browsergrid/internal/deployments"
	"github.com/autocrawlerHQ/browsergrid/internal/provider"
	"github.com/autocrawlerHQ/browsergrid/internal/provider/docker"
	"github.com/autocrawlerHQ/browsergrid/internal/sessions"
	"github.com/autocrawlerHQ/browsergrid/internal/storage"
	_ "github.com/autocrawlerHQ/browsergrid/internal/storage/local"
	_ "github.com/autocrawlerHQ/browsergrid/internal/storage/s3"
	"github.com/autocrawlerHQ/browsergrid/internal/tasks"
)

type WorkerConfig struct {
	Name        string
	Provider    string
	DatabaseURL string
	RedisAddr   string
	Concurrency int
	Queues      map[string]int
	Storage     config.StorageConfig
}

func main() {
	log.Println("========================================")
	log.Println("       BrowserGrid Worker v2.0         ")
	log.Println("========================================")

	cfg := loadConfig()

	db, err := connectDB(cfg.DatabaseURL)
	if err != nil {
		log.Fatal("[STARTUP] ✗ Failed to connect to database:", err)
	}

	sessStore := sessions.NewStore(db)

	// Initialize storage backend
	storageBackend, err := storage.New(cfg.Storage.Backend, map[string]string{
		"path":   cfg.Storage.LocalPath,
		"bucket": cfg.Storage.S3Bucket,
		"region": cfg.Storage.S3Region,
		"prefix": cfg.Storage.S3Prefix,
	})
	if err != nil {
		log.Fatal("[STARTUP] ✗ Failed to initialize storage:", err)
	}
	log.Printf("[STARTUP] ✓ Storage backend initialized: %s", cfg.Storage.Backend)

	// Initialize deployment runner
	deploymentRunner := deployments.NewDeploymentRunner(db, "/tmp/deployments")

	// Initialize provider with storage backend
	var prov provider.Provisioner
	switch cfg.Provider {
	case "docker":
		prov = docker.NewDockerProvisioner(storageBackend)
		log.Printf("[STARTUP] ✓ Docker provider initialized with storage backend")
	default:
		log.Fatalf("[STARTUP] ✗ Unknown provider type: %s", cfg.Provider)
	}

	redisOpt := asynq.RedisClientOpt{Addr: cfg.RedisAddr}

	srv := asynq.NewServer(
		redisOpt,
		asynq.Config{
			Concurrency:     cfg.Concurrency,
			Queues:          cfg.Queues,
			RetryDelayFunc:  asynq.DefaultRetryDelayFunc,
			ShutdownTimeout: 30 * time.Second,
		},
	)

	mux := asynq.NewServeMux()

	mux.HandleFunc(tasks.TypeSessionStart, handleSessionStart(sessStore, prov))
	mux.HandleFunc(tasks.TypeSessionStop, handleSessionStop(sessStore, prov))
	mux.HandleFunc(tasks.TypeSessionHealthCheck, handleSessionHealthCheck(sessStore, prov))
	mux.HandleFunc(tasks.TypeSessionTimeout, handleSessionTimeout(sessStore, prov, cfg.RedisAddr))
	mux.HandleFunc(tasks.TypeDeploymentRun, handleDeploymentRun(deploymentRunner))
	mux.HandleFunc(tasks.TypeDeploymentSchedule, handleDeploymentSchedule(deploymentRunner))

	log.Printf("[WORKER] Starting worker...")
	log.Printf("[WORKER] └── Name: %s", cfg.Name)
	log.Printf("[WORKER] └── Provider: %s", cfg.Provider)
	log.Printf("[WORKER] └── Concurrency: %d", cfg.Concurrency)
	log.Printf("[WORKER] └── Redis: %s", cfg.RedisAddr)
	log.Printf("[WORKER] └── Queues: %v", cfg.Queues)
	log.Printf("[WORKER] └── Storage: %s", cfg.Storage.Backend)
	log.Printf("[WORKER] Ready to process tasks...")
	log.Printf("[WORKER] =======================================")

	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Printf("[SHUTDOWN] Received shutdown signal...")
		cancel()
	}()

	if err := srv.Run(mux); err != nil {
		log.Fatal("[WORKER] ✗ Failed to run server:", err)
	}
}

func handleSessionStart(store *sessions.Store, prov provider.Provisioner) asynq.HandlerFunc {
	return func(ctx context.Context, t *asynq.Task) error {
		var payload tasks.SessionStartPayload
		if err := payload.Unmarshal(t.Payload()); err != nil {
			return fmt.Errorf("failed to unmarshal payload: %w", err)
		}

		log.Printf("[TASK] Starting session %s", payload.SessionID)

		sess, err := store.GetSession(ctx, payload.SessionID)
		if err != nil {
			return fmt.Errorf("failed to get session: %w", err)
		}

		// Log if profile is being used
		if sess.ProfileID != nil {
			log.Printf("[TASK] Session %s using profile %s", sess.ID, *sess.ProfileID)
		}

		if err := store.UpdateSessionStatus(ctx, sess.ID, sessions.StatusStarting); err != nil {
			log.Printf("[TASK] Warning: Failed to update session status: %v", err)
		}

		wsURL, liveURL, err := prov.Start(ctx, sess)
		if err != nil {
			log.Printf("[TASK] ✗ Failed to start session %s: %v", sess.ID, err)
			store.UpdateSessionStatus(ctx, sess.ID, sessions.StatusFailed)
			return fmt.Errorf("failed to start provider: %w", err)
		}

		if err := store.UpdateSessionEndpoints(ctx, sess.ID, wsURL, liveURL, sessions.StatusRunning); err != nil {
			return fmt.Errorf("failed to update session endpoints: %w", err)
		}

		log.Printf("[TASK] ✓ Session %s started successfully", sess.ID)
		log.Printf("[TASK] └── WebSocket: %s", wsURL)
		log.Printf("[TASK] └── Live URL: %s", liveURL)
		if sess.ProfileID != nil {
			log.Printf("[TASK] └── Profile: %s", *sess.ProfileID)
		}

		client := asynq.NewClient(asynq.RedisClientOpt{Addr: payload.RedisAddr})
		defer client.Close()

		healthCheckPayload := tasks.SessionHealthCheckPayload{
			SessionID: sess.ID,
			RedisAddr: payload.RedisAddr,
		}

		task, _ := tasks.NewSessionHealthCheckTask(healthCheckPayload)
		_, err = client.Enqueue(task,
			asynq.ProcessIn(30*time.Second),
			asynq.Queue(payload.QueueName),
		)
		if err != nil {
			log.Printf("[TASK] Warning: Failed to schedule health check: %v", err)
		}

		if payload.MaxSessionDuration > 0 {
			timeoutPayload := tasks.SessionTimeoutPayload{
				SessionID: sess.ID,
			}

			timeoutTask, _ := tasks.NewSessionTimeoutTask(timeoutPayload)
			_, err = client.Enqueue(timeoutTask,
				asynq.ProcessIn(time.Duration(payload.MaxSessionDuration)*time.Second),
				asynq.Queue(payload.QueueName),
				asynq.Unique(time.Duration(payload.MaxSessionDuration)*time.Second),
			)
			if err != nil {
				log.Printf("[TASK] Warning: Failed to schedule timeout: %v", err)
			}
		}

		return nil
	}
}

func handleSessionStop(store *sessions.Store, prov provider.Provisioner) asynq.HandlerFunc {
	return func(ctx context.Context, t *asynq.Task) error {
		var payload tasks.SessionStopPayload
		if err := payload.Unmarshal(t.Payload()); err != nil {
			return fmt.Errorf("failed to unmarshal payload: %w", err)
		}

		log.Printf("[TASK] Stopping session %s", payload.SessionID)

		sess, err := store.GetSession(ctx, payload.SessionID)
		if err != nil {
			return fmt.Errorf("failed to get session: %w", err)
		}

		// Log if profile will be saved
		if sess.ProfileID != nil {
			log.Printf("[TASK] Session %s will save profile %s", sess.ID, *sess.ProfileID)
		}

		if err := prov.Stop(ctx, sess); err != nil {
			log.Printf("[TASK] ✗ Error stopping provider for session %s: %v", sess.ID, err)
		}

		status := sessions.StatusCompleted
		if payload.Reason == "timeout" {
			status = sessions.StatusTimedOut
		} else if payload.Reason == "failed" {
			status = sessions.StatusFailed
		}

		if err := store.UpdateSessionStatus(ctx, sess.ID, status); err != nil {
			return fmt.Errorf("failed to update session status: %w", err)
		}

		log.Printf("[TASK] ✓ Session %s stopped (reason: %s)", sess.ID, payload.Reason)
		if sess.ProfileID != nil {
			log.Printf("[TASK] └── Profile %s saved", *sess.ProfileID)
		}
		return nil
	}
}

func handleSessionHealthCheck(store *sessions.Store, prov provider.Provisioner) asynq.HandlerFunc {
	return func(ctx context.Context, t *asynq.Task) error {
		var payload tasks.SessionHealthCheckPayload
		if err := payload.Unmarshal(t.Payload()); err != nil {
			return fmt.Errorf("failed to unmarshal payload: %w", err)
		}

		sess, err := store.GetSession(ctx, payload.SessionID)
		if err != nil {
			return fmt.Errorf("failed to get session: %w", err)
		}

		if sessions.IsTerminalStatus(sess.Status) {
			log.Printf("[HEALTH] Session %s is in terminal state %s, skipping health check", sess.ID, sess.Status)
			return nil
		}

		if err := prov.HealthCheck(ctx, sess); err != nil {
			log.Printf("[HEALTH] ✗ Session %s health check failed: %v", sess.ID, err)

			store.UpdateSessionStatus(ctx, sess.ID, sessions.StatusCrashed)

			client := asynq.NewClient(asynq.RedisClientOpt{Addr: payload.RedisAddr})
			defer client.Close()

			stopPayload := tasks.SessionStopPayload{
				SessionID: sess.ID,
				Reason:    "health_check_failed",
			}

			stopTask, _ := tasks.NewSessionStopTask(stopPayload)
			client.Enqueue(stopTask, asynq.Queue("critical"))

			return nil
		}

		if metrics, err := prov.GetMetrics(ctx, sess); err == nil {
			store.CreateMetrics(ctx, metrics)
			log.Printf("[METRICS] %s: CPU=%.1f%%, Memory=%.0fMB",
				sess.ID,
				safeDeref(metrics.CPUPercent, 0.0),
				safeDeref(metrics.MemoryMB, 0.0))
		}

		client := asynq.NewClient(asynq.RedisClientOpt{Addr: payload.RedisAddr})
		defer client.Close()

		nextHealthCheck, _ := tasks.NewSessionHealthCheckTask(payload)
		_, err = client.Enqueue(nextHealthCheck,
			asynq.ProcessIn(30*time.Second),
			asynq.Queue("default"),
		)

		return err
	}
}

func handleSessionTimeout(store *sessions.Store, prov provider.Provisioner, redisAddr string) asynq.HandlerFunc {
	return func(ctx context.Context, t *asynq.Task) error {
		var payload tasks.SessionTimeoutPayload
		if err := payload.Unmarshal(t.Payload()); err != nil {
			return fmt.Errorf("failed to unmarshal payload: %w", err)
		}

		log.Printf("[TIMEOUT] Session %s has reached its maximum duration", payload.SessionID)

		client := asynq.NewClient(asynq.RedisClientOpt{Addr: redisAddr})
		defer client.Close()

		stopPayload := tasks.SessionStopPayload{
			SessionID: payload.SessionID,
			Reason:    "timeout",
		}

		stopTask, _ := tasks.NewSessionStopTask(stopPayload)
		_, err := client.Enqueue(stopTask, asynq.Queue("default"))

		return err
	}
}

func healthCheck(store *sessions.Store) func() error {
	return func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		db := store.GetDB()
		sqlDB, err := db.DB()
		if err != nil {
			return err
		}

		return sqlDB.PingContext(ctx)
	}
}

func loadConfig() WorkerConfig {
	cfg := WorkerConfig{
		Concurrency: 10,
		Queues: map[string]int{
			"critical": 10,
			"default":  5,
			"low":      1,
		},
		Storage: config.StorageConfig{
			Backend:   "local",
			LocalPath: "/tmp/browsergrid/storage",
		},
	}

	flag.StringVar(&cfg.Name, "name", "", "Worker name")
	flag.StringVar(&cfg.Provider, "provider", "docker", "Provider type (docker, local, aci)")
	flag.IntVar(&cfg.Concurrency, "concurrency", 10, "Maximum concurrent tasks")
	flag.StringVar(&cfg.DatabaseURL, "db", "", "Database URL")
	flag.StringVar(&cfg.RedisAddr, "redis", "redis:6379", "Redis address")

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

	// Load storage configuration from environment
	if v := os.Getenv("STORAGE_BACKEND"); v != "" {
		cfg.Storage.Backend = v
	}
	if v := os.Getenv("STORAGE_PATH"); v != "" {
		cfg.Storage.LocalPath = v
	}
	if v := os.Getenv("STORAGE_S3_BUCKET"); v != "" {
		cfg.Storage.S3Bucket = v
	}
	if v := os.Getenv("STORAGE_S3_REGION"); v != "" {
		cfg.Storage.S3Region = v
	}
	if v := os.Getenv("STORAGE_S3_PREFIX"); v != "" {
		cfg.Storage.S3Prefix = v
	}

	return cfg
}

// handleDeploymentRun handles deployment run tasks
func handleDeploymentRun(runner *deployments.DeploymentRunner) asynq.HandlerFunc {
	return func(ctx context.Context, t *asynq.Task) error {
		var payload tasks.DeploymentRunPayload
		if err := payload.Unmarshal(t.Payload()); err != nil {
			return fmt.Errorf("failed to unmarshal payload: %w", err)
		}

		log.Printf("[DEPLOYMENT] Starting deployment run %s for deployment %s", payload.RunID, payload.DeploymentID)

		if err := runner.ExecuteDeployment(ctx, payload.RunID); err != nil {
			log.Printf("[DEPLOYMENT] ✗ Failed to execute deployment run %s: %v", payload.RunID, err)
			return fmt.Errorf("failed to execute deployment: %w", err)
		}

		log.Printf("[DEPLOYMENT] ✓ Deployment run %s completed successfully", payload.RunID)
		return nil
	}
}

// handleDeploymentSchedule handles deployment schedule tasks
func handleDeploymentSchedule(runner *deployments.DeploymentRunner) asynq.HandlerFunc {
	return func(ctx context.Context, t *asynq.Task) error {
		var payload tasks.DeploymentSchedulePayload
		if err := payload.Unmarshal(t.Payload()); err != nil {
			return fmt.Errorf("failed to unmarshal payload: %w", err)
		}

		log.Printf("[DEPLOYMENT] Processing scheduled deployment %s", payload.DeploymentID)

		// TODO: Implement deployment scheduling logic
		// This would involve:
		// 1. Parsing the cron schedule
		// 2. Creating new deployment runs based on the schedule
		// 3. Enqueuing deployment run tasks

		log.Printf("[DEPLOYMENT] ✓ Deployment schedule %s processed successfully", payload.DeploymentID)
		return nil
	}
}

// Utility functions for tar extraction - used by profile persistence
func extractTarGz(src, dest string) error {
	file, err := os.Open(src)
	if err != nil {
		return err
	}
	defer file.Close()

	gzr, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		target := filepath.Join(dest, header.Name)

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			file, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode))
			if err != nil {
				return err
			}
			if _, err := io.Copy(file, tr); err != nil {
				file.Close()
				return err
			}
			file.Close()
		}
	}

	return nil
}

func connectDB(databaseURL string) (*gorm.DB, error) {
	return gorm.Open(postgres.Open(databaseURL), &gorm.Config{})
}

func safeDeref[T any](ptr *T, defaultVal T) T {
	if ptr == nil {
		return defaultVal
	}
	return *ptr
}
