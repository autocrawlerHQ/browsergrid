package router

import (
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/hibiken/asynq"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	"github.com/autocrawlerHQ/browsergrid/internal/db"
	"github.com/autocrawlerHQ/browsergrid/internal/poolmgr"
	"github.com/autocrawlerHQ/browsergrid/internal/sessions"
	"github.com/autocrawlerHQ/browsergrid/internal/workpool"
)

func New(database *db.DB, reconciler *poolmgr.Reconciler, taskClient *asynq.Client, redisOpt asynq.RedisClientOpt) *gin.Engine {
	r := gin.Default()

	// CORS middleware
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Length", "Content-Type", "Authorization", "X-API-Key"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
	}))

	// Health check
	r.GET("/health", func(c *gin.Context) {
		// Check database
		sqlDB, _ := database.DB.DB()
		if err := sqlDB.Ping(); err != nil {
			c.JSON(503, gin.H{"status": "unhealthy", "database": "down", "error": err.Error()})
			return
		}

		// Check Redis
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

	// Swagger documentation
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// API v1 routes
	v1 := r.Group("/api/v1")
	{
		// Work pool adapter for sessions
		poolAdapter := workpool.NewAdapter(database.DB)

		// Sessions routes with dependencies
		sessionDeps := sessions.Dependencies{
			DB:         database.DB,
			PoolSvc:    poolAdapter,
			TaskClient: taskClient,
		}
		sessions.RegisterRoutes(v1, sessionDeps)

		// Work pool routes
		workpool.RegisterRoutes(v1, database.DB)

		// Pool management routes
		poolmgr.RegisterRoutes(v1, reconciler)

	}

	return r
}
