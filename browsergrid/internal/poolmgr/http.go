package poolmgr

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func RegisterRoutes(rg *gin.RouterGroup, reconciler *Reconciler) {
	rg.GET("/workpools/:id/stats", getPoolStats(reconciler))
}

// GetPoolStats retrieves statistics for a work pool
// @Summary Get work pool statistics
// @Description Get detailed statistics and metrics for a specific work pool
// @Tags pool-management
// @Accept json
// @Produce json
// @Param id path string true "Work Pool ID (UUID)"
// @Success 200 {object} map[string]interface{} "Work pool statistics"
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/workpools/{id}/stats [get]
func getPoolStats(reconciler *Reconciler) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid pool ID"})
			return
		}

		stats, err := reconciler.GetPoolStats(c.Request.Context(), id)
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "pool not found"})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			}
			return
		}

		c.JSON(http.StatusOK, stats)
	}
}
