package workpool

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func RegisterRoutes(rg *gin.RouterGroup, db *gorm.DB) {
	store := NewStore(db)

	rg.POST("/workpools", createWorkPool(store))
	rg.GET("/workpools", listWorkPools(store))
	rg.GET("/workpools/:id", getWorkPool(store))
	rg.PATCH("/workpools/:id", updateWorkPool(store))
	rg.DELETE("/workpools/:id", deleteWorkPool(store))
	rg.POST("/workpools/:id/scale", scaleWorkPool(store))
	rg.POST("/workpools/:id/drain", drainWorkPool(store))
}

// CreateWorkPool creates a new work pool
// @Summary Create a new work pool
// @Description Create a new work pool to manage browser workers
// @Tags workpools
// @Accept json
// @Produce json
// @Param workpool body WorkPool true "Work pool configuration"
// @Success 201 {object} WorkPool
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/workpools [post]
func createWorkPool(store *Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req WorkPool
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if req.Name == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
			return
		}
		if req.Provider == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "provider is required"})
			return
		}
		if req.MaxConcurrency <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "max_concurrency must be positive"})
			return
		}

		if err := store.CreateWorkPool(c.Request.Context(), &req); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusCreated, req)
	}
}

// ListWorkPools lists all work pools
// @Summary List work pools
// @Description Get a list of all work pools with optional filtering
// @Tags workpools
// @Accept json
// @Produce json
// @Param paused query boolean false "Filter by paused status"
// @Success 200 {object} WorkPoolListResponse "List of work pools"
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/workpools [get]
func listWorkPools(store *Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		var paused *bool
		if v := c.Query("paused"); v != "" {
			if p, err := strconv.ParseBool(v); err == nil {
				paused = &p
			}
		}

		pools, err := store.ListWorkPools(c.Request.Context(), paused)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"pools": pools,
			"total": len(pools),
		})
	}
}

// GetWorkPool retrieves a specific work pool by ID
// @Summary Get a work pool
// @Description Get details of a specific work pool by ID
// @Tags workpools
// @Accept json
// @Produce json
// @Param id path string true "Work Pool ID (UUID)"
// @Success 200 {object} WorkPool
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/workpools/{id} [get]
func getWorkPool(store *Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid pool ID"})
			return
		}

		pool, err := store.GetWorkPool(c.Request.Context(), id)
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "pool not found"})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			}
			return
		}

		c.JSON(http.StatusOK, pool)
	}
}

// UpdateWorkPool updates a work pool
// @Summary Update a work pool
// @Description Update configuration of an existing work pool
// @Tags workpools
// @Accept json
// @Produce json
// @Param id path string true "Work Pool ID (UUID)"
// @Param updates body map[string]interface{} true "Fields to update"
// @Success 200 {object} MessageResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/workpools/{id} [patch]
func updateWorkPool(store *Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid pool ID"})
			return
		}

		var updates map[string]interface{}
		if err := c.ShouldBindJSON(&updates); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if err := store.UpdateWorkPool(c.Request.Context(), id, updates); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "pool updated"})
	}
}

// DeleteWorkPool deletes a work pool
// @Summary Delete a work pool
// @Description Delete an existing work pool and all its workers
// @Tags workpools
// @Accept json
// @Produce json
// @Param id path string true "Work Pool ID (UUID)"
// @Success 200 {object} MessageResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/workpools/{id} [delete]
func deleteWorkPool(store *Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid pool ID"})
			return
		}

		if err := store.DeleteWorkPool(c.Request.Context(), id); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "pool deleted"})
	}
}

// ScaleWorkPool scales a work pool
// @Summary Scale a work pool
// @Description Update scaling parameters for a work pool
// @Tags workpools
// @Accept json
// @Produce json
// @Param id path string true "Work Pool ID (UUID)"
// @Param scaling body ScalingRequest true "Scaling parameters"
// @Success 200 {object} ScalingResponse "Scaling operation result"
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/workpools/{id}/scale [post]
func scaleWorkPool(store *Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid pool ID"})
			return
		}

		var req struct {
			MinSize            *int  `json:"min_size"`
			MaxConcurrency     *int  `json:"max_concurrency"`
			MaxIdleTime        *int  `json:"max_idle_time"`
			MaxSessionDuration *int  `json:"max_session_duration"`
			AutoScale          *bool `json:"auto_scale"`
			Paused             *bool `json:"paused"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		updates := make(map[string]interface{})
		if req.MinSize != nil {
			if *req.MinSize < 0 {
				c.JSON(http.StatusBadRequest, gin.H{"error": "min_size cannot be negative"})
				return
			}
			updates["min_size"] = *req.MinSize
		}
		if req.MaxConcurrency != nil {
			if *req.MaxConcurrency <= 0 {
				c.JSON(http.StatusBadRequest, gin.H{"error": "max_concurrency must be positive"})
				return
			}
			updates["max_concurrency"] = *req.MaxConcurrency
		}
		if req.MaxIdleTime != nil {
			if *req.MaxIdleTime < 0 {
				c.JSON(http.StatusBadRequest, gin.H{"error": "max_idle_time cannot be negative"})
				return
			}
			updates["max_idle_time"] = *req.MaxIdleTime
		}
		if req.MaxSessionDuration != nil {
			if *req.MaxSessionDuration < 0 {
				c.JSON(http.StatusBadRequest, gin.H{"error": "max_session_duration cannot be negative"})
				return
			}
			updates["max_session_duration"] = *req.MaxSessionDuration
		}
		if req.AutoScale != nil {
			updates["auto_scale"] = *req.AutoScale
		}
		if req.Paused != nil {
			updates["paused"] = *req.Paused
		}

		if len(updates) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "no scaling parameters provided"})
			return
		}

		if err := store.UpdateWorkPool(c.Request.Context(), id, updates); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message": "pool scaled",
			"updates": updates,
		})
	}
}

// DrainWorkPool drains a work pool
// @Summary Drain a work pool
// @Description Gracefully drain all workers from a work pool
// @Tags workpools
// @Accept json
// @Produce json
// @Param id path string true "Work Pool ID (UUID)"
// @Success 200 {object} MessageResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/workpools/{id}/drain [post]
func drainWorkPool(store *Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid pool ID"})
			return
		}

		if err := store.DrainWorkPool(c.Request.Context(), id); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "pool drained"})
	}
}
