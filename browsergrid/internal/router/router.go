package router

import (
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"github.com/autocrawlerHQ/browsergrid/internal/db"
	"github.com/autocrawlerHQ/browsergrid/internal/poolmgr"
	"github.com/autocrawlerHQ/browsergrid/internal/sessions"
	"github.com/autocrawlerHQ/browsergrid/internal/workpool"

	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

func New(db *db.DB, reconciler *poolmgr.Reconciler) *gin.Engine {
	r := gin.New()

	r.Use(gin.Logger(), gin.Recovery())

	config := cors.Config{
		AllowOrigins:     []string{"http://localhost:3000", "http://localhost:5173", "http://localhost:8080"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Length", "Content-Type", "Authorization", "X-Requested-With"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}
	r.Use(cors.New(config))

	// Apply auth middleware after CORS
	//r.Use(middleware.Auth())

	// HealthCheck checks if the API is running
	// @Summary Health check
	// @Description Check if the API service is running and healthy
	// @Tags health
	// @Accept json
	// @Produce json
	// @Success 200 {object} map[string]string
	// @Router /health [get]
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// Swagger documentation
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	v1 := r.Group("/api/v1")

	poolService := workpool.NewPoolService(db.DB)
	sessions.RegisterRoutes(v1, db.DB, poolService)

	workpool.RegisterRoutes(v1, db.DB)

	poolmgr.RegisterRoutes(v1, reconciler)

	return r
}
