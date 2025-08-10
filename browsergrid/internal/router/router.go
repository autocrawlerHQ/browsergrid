package router

import (
	"context"
	"fmt"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	"github.com/autocrawlerHQ/browsergrid/internal/db"
	"github.com/autocrawlerHQ/browsergrid/internal/deployments"
	"github.com/autocrawlerHQ/browsergrid/internal/poolmgr"
	"github.com/autocrawlerHQ/browsergrid/internal/profiles"
	"github.com/autocrawlerHQ/browsergrid/internal/sessions"
	"github.com/autocrawlerHQ/browsergrid/internal/workpool"

	"github.com/autocrawlerHQ/browsergrid/internal/storage"
)

func New(database *db.DB, reconciler *poolmgr.Reconciler, taskClient *asynq.Client, redisOpt asynq.RedisClientOpt, storeBackend storage.Backend) *gin.Engine {
	r := gin.Default()

	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Length", "Content-Type", "Authorization", "X-API-Key"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
	}))

	r.GET("/health", func(c *gin.Context) {
		sqlDB, _ := database.DB.DB()
		if err := sqlDB.Ping(); err != nil {
			c.JSON(503, gin.H{"status": "unhealthy", "database": "down", "error": err.Error()})
			return
		}

		inspector := asynq.NewInspector(redisOpt)
		if _, err := inspector.Queues(); err != nil {
			c.JSON(503, gin.H{"status": "unhealthy", "redis": "down", "error": err.Error()})
			return
		}

		c.JSON(200, gin.H{
			"status":   "healthy",
			"database": "up",
			"redis":    "up",
		})
	})

	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	v1 := r.Group("/api/v1")
	{
		poolAdapter := workpool.NewAdapter(database.DB)

		// Initialize profiles
		profileStore := profiles.NewStore(database.DB)

		// Create profile service adapter
		profileService := &profileServiceAdapter{
			store: profileStore,
		}

		sessionDeps := sessions.Dependencies{
			DB:         database.DB,
			PoolSvc:    poolAdapter,
			TaskClient: taskClient,
			ProfileSvc: profileService,
		}
		sessions.RegisterRoutes(v1, sessionDeps)

		profileDeps := profiles.Dependencies{
			DB:      database.DB,
			Store:   profileStore,
			Storage: storeBackend,
		}
		profiles.RegisterRoutes(v1, profileDeps)

		workpool.RegisterRoutes(v1, database.DB)

		deploymentDeps := deployments.Dependencies{
			DB:         database.DB,
			TaskClient: taskClient,
			Storage:    storeBackend,
		}
		deployments.RegisterRoutes(v1, deploymentDeps)

		poolmgr.RegisterRoutes(v1, reconciler)
	}

	return r
}

// profileServiceAdapter adapts the profiles store to the sessions ProfileService interface
type profileServiceAdapter struct {
	store *profiles.Store
}

func (p *profileServiceAdapter) ValidateProfile(ctx context.Context, profileID uuid.UUID, browser sessions.Browser) error {
	profile, err := p.store.GetProfile(ctx, profileID)
	if err != nil {
		return err
	}

	if profile.Browser != browser {
		return fmt.Errorf("profile browser type %s does not match session browser type %s", profile.Browser, browser)
	}

	return nil
}
