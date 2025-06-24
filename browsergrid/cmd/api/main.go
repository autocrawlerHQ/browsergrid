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

	"github.com/autocrawlerHQ/browsergrid/internal/config"
	"github.com/autocrawlerHQ/browsergrid/internal/db"
	"github.com/autocrawlerHQ/browsergrid/internal/poolmgr"
	"github.com/autocrawlerHQ/browsergrid/internal/router"

	_ "github.com/autocrawlerHQ/browsergrid/docs"
)

// @title           BrowserGrid API
// @version         1.0
// @description     BrowserGrid is a distributed browser automation platform that provides scalable browser sessions and worker pool management.
// @termsOfService  http://swagger.io/terms/

// @contact.name   API Support
// @contact.url    http://www.swagger.io/support
// @contact.email  support@swagger.io

// @license.name  Apache 2.0
// @license.url   http://www.apache.org/licenses/LICENSE-2.0.html

// @host      localhost:8080
// @BasePath  /api/v1

// @securityDefinitions.basic  BasicAuth

// @externalDocs.description  OpenAPI
// @externalDocs.url          https://swagger.io/resources/open-api/

func main() {
	cfg := config.Load()
	log.Printf("Starting BrowserGrid v2 API server on port %d", cfg.Port)
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
	log.Println("Database connection established")

	log.Println("Migrations are handled by Atlas during container startup")

	reconciler := poolmgr.NewReconciler(database.DB)
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

	r := router.New(database, reconciler)

	srv := &http.Server{
		Addr:    ":" + strconv.Itoa(cfg.Port),
		Handler: r,
	}

	go func() {
		log.Printf("API listening on :%d", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()

	<-ctx.Done()

	log.Printf("Graceful shutdown...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatal("Server forced to shutdown:", err)
	}
	log.Println("Server exited")
}
