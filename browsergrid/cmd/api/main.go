package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/hibiken/asynq"

	"github.com/autocrawlerHQ/browsergrid/internal/config"
	"github.com/autocrawlerHQ/browsergrid/internal/db"
	"github.com/autocrawlerHQ/browsergrid/internal/poolmgr"
	"github.com/autocrawlerHQ/browsergrid/internal/router"
	"github.com/autocrawlerHQ/browsergrid/internal/scheduler"

	_ "github.com/autocrawlerHQ/browsergrid/docs"
)

// @title           BrowserGrid API
// @version         2.0
// @description     BrowserGrid is a distributed browser automation platform using task queues for scalable browser session management.
// @termsOfService  http://swagger.io/terms/

// @contact.name   API Support
// @contact.url    http://www.swagger.io/support
// @contact.email  support@swagger.io

// @license.name  Apache 2.0
// @license.url   http://www.apache.org/licenses/LICENSE-2.0.html

// @host      localhost:8765
// @BasePath  /

// @securityDefinitions.apikey ApiKeyAuth
// @in header
// @name X-API-Key
// @description API key for authentication. Can also be provided via query parameter 'api_key' or Authorization header.

// @externalDocs.description  OpenAPI
// @externalDocs.url          https://swagger.io/resources/open-api/

func main() {
	cfg := config.Load()
	log.Printf("========================================")
	log.Printf("      BrowserGrid v2 API Server        ")
	log.Printf("========================================")
	log.Printf("Port: %d", cfg.Port)
	log.Printf("Redis: %s", cfg.RedisAddr)
	log.Printf("========================================")

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	database, err := db.New(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	defer func() {
		if err := database.Close(); err != nil {
			log.Printf("Error closing database: %v", err)
		}
	}()

	sqlDB, err := database.DB.DB()
	if err != nil {
		log.Fatalf("getting underlying sql.DB: %v", err)
	}
	if err := sqlDB.Ping(); err != nil {
		log.Fatalf("db ping failed: %v", err)
	}
	log.Println("✓ Database connection established")

	redisOpt := asynq.RedisClientOpt{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	}

	taskClient := asynq.NewClient(redisOpt)
	defer taskClient.Close()

	inspector := asynq.NewInspector(redisOpt)
	if _, err := inspector.Queues(); err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	log.Println("✓ Redis connection established")

	schedulerSvc := scheduler.New(database.DB, redisOpt)
	go func() {
		log.Println("Starting scheduler service...")
		if err := schedulerSvc.Start(); err != nil && !errors.Is(err, context.Canceled) {
			log.Printf("Scheduler service error: %v", err)
		}
	}()
	defer schedulerSvc.Stop()

	reconciler := poolmgr.NewReconciler(database.DB, taskClient)
	reconcilerCtx, reconcilerCancel := context.WithCancel(ctx)
	defer reconcilerCancel()

	go func() {
		log.Println("Starting pool reconciler...")
		if err := reconciler.Start(reconcilerCtx); err != nil && !errors.Is(err, context.Canceled) {
			log.Printf("Pool reconciler stopped with error: %v", err)
		} else {
			log.Println("Pool reconciler stopped")
		}
	}()

	r := router.New(database, reconciler, taskClient, redisOpt)

	srv := &http.Server{
		Addr:         ":" + strconv.Itoa(cfg.Port),
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Printf("✓ API listening on :%d", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()

	<-ctx.Done()

	log.Printf("Graceful shutdown initiated...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatal("Server forced to shutdown:", err)
	}

	log.Println("✓ Server exited cleanly")
}
