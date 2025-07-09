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

		sessionDeps := sessions.Dependencies{
			DB:         database.DB,
			PoolSvc:    poolAdapter,
			TaskClient: taskClient,
		}
		sessions.RegisterRoutes(v1, sessionDeps)

		workpool.RegisterRoutes(v1, database.DB)

		poolmgr.RegisterRoutes(v1, reconciler)

	}

	return r
}
