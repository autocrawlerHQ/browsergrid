package deployments

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/autocrawlerHQ/browsergrid/internal/storage"
	"github.com/autocrawlerHQ/browsergrid/internal/tasks"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"gorm.io/gorm"
	"io"
	"net/http"
	"strconv"
)

// CreateDeploymentRequest represents the request to create a new deployment
type CreateDeploymentRequest struct {
	Name        string           `json:"name" binding:"required"`
	Description string           `json:"description"`
	Version     string           `json:"version" binding:"required"`
	Runtime     Runtime          `json:"runtime" binding:"required"`
	PackageURL  string           `json:"package_url" binding:"required"`
	PackageHash string           `json:"package_hash" binding:"required"`
	Config      DeploymentConfig `json:"config"`
}

// UpdateDeploymentRequest represents the request to update a deployment
type UpdateDeploymentRequest struct {
	Description *string           `json:"description,omitempty"`
	Config      *DeploymentConfig `json:"config,omitempty"`
	Status      *DeploymentStatus `json:"status,omitempty"`
}

// CreateDeploymentRunRequest represents the request to create a new deployment run
type CreateDeploymentRunRequest struct {
	Environment map[string]string      `json:"environment,omitempty"`
	Config      map[string]interface{} `json:"config,omitempty"`
}

// DeploymentListResponse represents the response for listing deployments
type DeploymentListResponse struct {
	Deployments []Deployment `json:"deployments"`
	Total       int64        `json:"total"`
	Offset      int          `json:"offset"`
	Limit       int          `json:"limit"`
}

// DeploymentRunListResponse represents the response for listing deployment runs
type DeploymentRunListResponse struct {
	Runs   []DeploymentRun `json:"runs"`
	Total  int64           `json:"total"`
	Offset int             `json:"offset"`
	Limit  int             `json:"limit"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error string `json:"error"`
}

// MessageResponse represents a success message response
type MessageResponse struct {
	Message string `json:"message"`
}

// TaskClient interface for task queue operations
type TaskClient interface {
	Enqueue(task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error)
	Close() error
}

// Dependencies holds the dependencies for deployment handlers
type Dependencies struct {
	DB         *gorm.DB
	TaskClient TaskClient
	Storage    storage.Backend
}

// RegisterRoutes registers deployment management routes
func RegisterRoutes(rg *gin.RouterGroup, deps Dependencies) {
	store := NewStore(deps.DB)

	// Deployment routes
	rg.POST("/deployments/upload", uploadDeployment(deps))
	rg.POST("/deployments", createDeployment(store, deps))
	rg.GET("/deployments", listDeployments(store))
	rg.GET("/deployments/:id", getDeployment(store))
	rg.PATCH("/deployments/:id", updateDeployment(store))
	rg.DELETE("/deployments/:id", deleteDeployment(store))
	rg.GET("/deployments/:id/stats", getDeploymentStats(store))

	// Deployment run routes
	rg.POST("/deployments/:id/runs", createDeploymentRun(store, deps))
	rg.GET("/deployments/:id/runs", listDeploymentRuns(store))
	rg.GET("/runs/:id", getDeploymentRun(store))
	rg.GET("/runs/:id/logs", getDeploymentRunLogs(store))
	rg.DELETE("/runs/:id", deleteDeploymentRun(store))

	// General run routes
	rg.GET("/runs", listAllDeploymentRuns(store))
}

// createDeployment creates a new deployment
// @Summary Create a new deployment
// @Description Create a new deployment package with specified configuration
// @Tags deployments
// @Accept json
// @Produce json
// @Param deployment body CreateDeploymentRequest true "Deployment configuration"
// @Success 201 {object} Deployment "Deployment created successfully"
// @Failure 400 {object} ErrorResponse "Invalid request data"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /api/v1/deployments [post]
func createDeployment(store *Store, deps Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req CreateDeploymentRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
			return
		}

		// Check if deployment with same name and version already exists
		existing, err := store.GetDeploymentByName(c.Request.Context(), req.Name, req.Version)
		if err != nil && err != gorm.ErrRecordNotFound {
			c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
			return
		}
		if existing != nil {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "deployment with this name and version already exists"})
			return
		}

		configJSON, err := json.Marshal(req.Config)
		if err != nil {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid config format"})
			return
		}

		deployment := &Deployment{
			Name:        req.Name,
			Description: req.Description,
			Version:     req.Version,
			Runtime:     req.Runtime,
			PackageURL:  req.PackageURL,
			PackageHash: req.PackageHash,
			Config:      configJSON,
			Status:      StatusActive,
		}

		if err := store.CreateDeployment(c.Request.Context(), deployment); err != nil {
			c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
			return
		}

		c.JSON(http.StatusCreated, deployment)
	}
}

// listDeployments lists all deployments
// @Summary List deployments
// @Description Get a list of all deployments with optional filtering
// @Tags deployments
// @Accept json
// @Produce json
// @Param status query string false "Filter by deployment status"
// @Param runtime query string false "Filter by runtime"
// @Param offset query int false "Number of deployments to skip" default(0)
// @Param limit query int false "Maximum number of deployments to return" default(20)
// @Success 200 {object} DeploymentListResponse "List of deployments"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /api/v1/deployments [get]
func listDeployments(store *Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		var status *DeploymentStatus
		var runtime *Runtime

		if s := c.Query("status"); s != "" {
			statusVal := DeploymentStatus(s)
			status = &statusVal
		}

		if r := c.Query("runtime"); r != "" {
			runtimeVal := Runtime(r)
			runtime = &runtimeVal
		}

		offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
		limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

		if limit > 100 {
			limit = 100
		}

		deployments, total, err := store.ListDeployments(c.Request.Context(), status, runtime, offset, limit)
		if err != nil {
			c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
			return
		}

		c.JSON(http.StatusOK, DeploymentListResponse{
			Deployments: deployments,
			Total:       total,
			Offset:      offset,
			Limit:       limit,
		})
	}
}

// getDeployment retrieves a specific deployment
// @Summary Get a deployment
// @Description Get detailed information about a specific deployment
// @Tags deployments
// @Accept json
// @Produce json
// @Param id path string true "Deployment ID (UUID)"
// @Success 200 {object} Deployment "Deployment details"
// @Failure 400 {object} ErrorResponse "Invalid deployment ID"
// @Failure 404 {object} ErrorResponse "Deployment not found"
// @Router /api/v1/deployments/{id} [get]
func getDeployment(store *Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid deployment ID"})
			return
		}

		deployment, err := store.GetDeployment(c.Request.Context(), id)
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, ErrorResponse{Error: "deployment not found"})
			} else {
				c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
			}
			return
		}

		c.JSON(http.StatusOK, deployment)
	}
}

// updateDeployment updates deployment metadata
// @Summary Update a deployment
// @Description Update deployment metadata and configuration
// @Tags deployments
// @Accept json
// @Produce json
// @Param id path string true "Deployment ID (UUID)"
// @Param deployment body UpdateDeploymentRequest true "Deployment updates"
// @Success 200 {object} Deployment "Deployment updated successfully"
// @Failure 400 {object} ErrorResponse "Invalid request data"
// @Failure 404 {object} ErrorResponse "Deployment not found"
// @Router /api/v1/deployments/{id} [patch]
func updateDeployment(store *Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid deployment ID"})
			return
		}

		var req UpdateDeploymentRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
			return
		}

		updates := make(map[string]interface{})

		if req.Description != nil {
			updates["description"] = *req.Description
		}

		if req.Config != nil {
			configJSON, err := json.Marshal(req.Config)
			if err != nil {
				c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid config format"})
				return
			}
			updates["config"] = configJSON
		}

		if req.Status != nil {
			updates["status"] = *req.Status
		}

		if len(updates) == 0 {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "no updates provided"})
			return
		}

		if err := store.UpdateDeployment(c.Request.Context(), id, updates); err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, ErrorResponse{Error: "deployment not found"})
			} else {
				c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
			}
			return
		}

		deployment, err := store.GetDeployment(c.Request.Context(), id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
			return
		}

		c.JSON(http.StatusOK, deployment)
	}
}

// deleteDeployment deletes a deployment
// @Summary Delete a deployment
// @Description Delete a deployment and all its runs
// @Tags deployments
// @Accept json
// @Produce json
// @Param id path string true "Deployment ID (UUID)"
// @Success 200 {object} MessageResponse "Deployment deleted successfully"
// @Failure 400 {object} ErrorResponse "Invalid deployment ID"
// @Failure 404 {object} ErrorResponse "Deployment not found"
// @Router /api/v1/deployments/{id} [delete]
func deleteDeployment(store *Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid deployment ID"})
			return
		}

		if err := store.DeleteDeployment(c.Request.Context(), id); err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, ErrorResponse{Error: "deployment not found"})
			} else {
				c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
			}
			return
		}

		c.JSON(http.StatusOK, MessageResponse{Message: "deployment deleted successfully"})
	}
}

// getDeploymentStats retrieves deployment statistics
// @Summary Get deployment statistics
// @Description Get detailed statistics for a deployment
// @Tags deployments
// @Accept json
// @Produce json
// @Param id path string true "Deployment ID (UUID)"
// @Success 200 {object} map[string]interface{} "Deployment statistics"
// @Failure 400 {object} ErrorResponse "Invalid deployment ID"
// @Failure 404 {object} ErrorResponse "Deployment not found"
// @Router /api/v1/deployments/{id}/stats [get]
func getDeploymentStats(store *Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid deployment ID"})
			return
		}

		stats, err := store.GetDeploymentStats(c.Request.Context(), id)
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, ErrorResponse{Error: "deployment not found"})
			} else {
				c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
			}
			return
		}

		c.JSON(http.StatusOK, stats)
	}
}

// createDeploymentRun creates a new deployment run
// @Summary Create a new deployment run
// @Description Trigger a manual run of a deployment
// @Tags deployments
// @Accept json
// @Produce json
// @Param id path string true "Deployment ID (UUID)"
// @Param run body CreateDeploymentRunRequest true "Deployment run configuration"
// @Success 201 {object} DeploymentRun "Deployment run created successfully"
// @Failure 400 {object} ErrorResponse "Invalid request data"
// @Failure 404 {object} ErrorResponse "Deployment not found"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /api/v1/deployments/{id}/runs [post]
func createDeploymentRun(store *Store, deps Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid deployment ID"})
			return
		}

		var req CreateDeploymentRunRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
			return
		}

		// Check if deployment exists
		deployment, err := store.GetDeployment(c.Request.Context(), id)
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, ErrorResponse{Error: "deployment not found"})
			} else {
				c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
			}
			return
		}

		if deployment.Status != StatusActive {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "deployment is not active"})
			return
		}

		run := &DeploymentRun{
			DeploymentID: id,
			Status:       RunStatusPending,
		}

		if err := store.CreateDeploymentRun(c.Request.Context(), run); err != nil {
			c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
			return
		}

		// Enqueue deployment run task
		if deps.TaskClient != nil {
			payload := tasks.DeploymentRunPayload{
				DeploymentID: id,
				RunID:        run.ID,
				Environment:  req.Environment,
				Config:       req.Config,
			}

			task, err := tasks.NewDeploymentRunTask(payload)
			if err != nil {
				c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "failed to create task: " + err.Error()})
				return
			}

			if _, err := deps.TaskClient.Enqueue(task, asynq.Queue("default")); err != nil {
				c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "failed to enqueue task: " + err.Error()})
				return
			}
		}

		c.JSON(http.StatusCreated, run)
	}
}

// listDeploymentRuns lists runs for a specific deployment
// @Summary List deployment runs
// @Description Get a list of runs for a specific deployment
// @Tags deployments
// @Accept json
// @Produce json
// @Param id path string true "Deployment ID (UUID)"
// @Param status query string false "Filter by run status"
// @Param offset query int false "Number of runs to skip" default(0)
// @Param limit query int false "Maximum number of runs to return" default(20)
// @Success 200 {object} DeploymentRunListResponse "List of deployment runs"
// @Failure 400 {object} ErrorResponse "Invalid deployment ID"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /api/v1/deployments/{id}/runs [get]
func listDeploymentRuns(store *Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid deployment ID"})
			return
		}

		var status *RunStatus
		if s := c.Query("status"); s != "" {
			statusVal := RunStatus(s)
			status = &statusVal
		}

		offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
		limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

		if limit > 100 {
			limit = 100
		}

		runs, total, err := store.ListDeploymentRuns(c.Request.Context(), &id, status, offset, limit)
		if err != nil {
			c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
			return
		}

		c.JSON(http.StatusOK, DeploymentRunListResponse{
			Runs:   runs,
			Total:  total,
			Offset: offset,
			Limit:  limit,
		})
	}
}

// getDeploymentRun retrieves a specific deployment run
// @Summary Get a deployment run
// @Description Get detailed information about a specific deployment run
// @Tags deployments
// @Accept json
// @Produce json
// @Param id path string true "Run ID (UUID)"
// @Success 200 {object} DeploymentRun "Deployment run details"
// @Failure 400 {object} ErrorResponse "Invalid run ID"
// @Failure 404 {object} ErrorResponse "Run not found"
// @Router /api/v1/runs/{id} [get]
func getDeploymentRun(store *Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid run ID"})
			return
		}

		run, err := store.GetDeploymentRun(c.Request.Context(), id)
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, ErrorResponse{Error: "run not found"})
			} else {
				c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
			}
			return
		}

		c.JSON(http.StatusOK, run)
	}
}

// getDeploymentRunLogs retrieves logs for a specific deployment run
// @Summary Get deployment run logs
// @Description Get logs for a specific deployment run
// @Tags deployments
// @Accept json
// @Produce json
// @Param id path string true "Run ID (UUID)"
// @Success 200 {object} map[string]interface{} "Deployment run logs"
// @Failure 400 {object} ErrorResponse "Invalid run ID"
// @Failure 404 {object} ErrorResponse "Run not found"
// @Router /api/v1/runs/{id}/logs [get]
func getDeploymentRunLogs(store *Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid run ID"})
			return
		}

		run, err := store.GetDeploymentRun(c.Request.Context(), id)
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, ErrorResponse{Error: "run not found"})
			} else {
				c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
			}
			return
		}

		// For now, return the output as logs
		// In a full implementation, this would stream real-time logs
		logs := map[string]interface{}{
			"run_id":       run.ID,
			"output":       run.Output,
			"error":        run.Error,
			"started_at":   run.StartedAt,
			"completed_at": run.CompletedAt,
			"status":       run.Status,
		}

		c.JSON(http.StatusOK, logs)
	}
}

// deleteDeploymentRun deletes a deployment run
// @Summary Delete a deployment run
// @Description Delete a deployment run
// @Tags deployments
// @Accept json
// @Produce json
// @Param id path string true "Run ID (UUID)"
// @Success 200 {object} MessageResponse "Run deleted successfully"
// @Failure 400 {object} ErrorResponse "Invalid run ID"
// @Failure 404 {object} ErrorResponse "Run not found"
// @Router /api/v1/runs/{id} [delete]
func deleteDeploymentRun(store *Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid run ID"})
			return
		}

		if err := store.DeleteDeploymentRun(c.Request.Context(), id); err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, ErrorResponse{Error: "run not found"})
			} else {
				c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
			}
			return
		}

		c.JSON(http.StatusOK, MessageResponse{Message: "run deleted successfully"})
	}
}

// listAllDeploymentRuns lists all deployment runs
// @Summary List all deployment runs
// @Description Get a list of all deployment runs across all deployments
// @Tags deployments
// @Accept json
// @Produce json
// @Param status query string false "Filter by run status"
// @Param offset query int false "Number of runs to skip" default(0)
// @Param limit query int false "Maximum number of runs to return" default(20)
// @Success 200 {object} DeploymentRunListResponse "List of deployment runs"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /api/v1/runs [get]
func listAllDeploymentRuns(store *Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		var status *RunStatus
		if s := c.Query("status"); s != "" {
			statusVal := RunStatus(s)
			status = &statusVal
		}

		offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
		limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

		if limit > 100 {
			limit = 100
		}

		runs, total, err := store.ListDeploymentRuns(c.Request.Context(), nil, status, offset, limit)
		if err != nil {
			c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
			return
		}

		c.JSON(http.StatusOK, DeploymentRunListResponse{
			Runs:   runs,
			Total:  total,
			Offset: offset,
			Limit:  limit,
		})
	}
}

// UploadDeploymentResponse represents the response after uploading a deployment package
// @Description Response containing storage information for an uploaded deployment package
// @name UploadDeploymentResponse
// @Schema
// @public
// @tags deployments

type UploadDeploymentResponse struct {
	PackageURL  string `json:"package_url"`
	PackageHash string `json:"package_hash"`
}

// uploadDeployment handles deployment package uploads
// @Summary Upload deployment package
// @Description Upload a deployment package and receive storage reference details
// @Tags deployments
// @Accept multipart/form-data
// @Produce json
// @Param file formData file true "Deployment package"
// @Success 200 {object} UploadDeploymentResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/deployments/upload [post]
func uploadDeployment(deps Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		file, header, err := c.Request.FormFile("file")
		if err != nil {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "file is required"})
			return
		}
		defer file.Close()

		id := uuid.New()
		key := fmt.Sprintf("deployments/%s/%s", id.String(), header.Filename)
		hasher := sha256.New()
		reader := io.TeeReader(file, hasher)
		if err := deps.Storage.Save(c.Request.Context(), key, reader); err != nil {
			c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
			return
		}
		hash := hex.EncodeToString(hasher.Sum(nil))
		c.JSON(http.StatusOK, UploadDeploymentResponse{PackageURL: key, PackageHash: hash})
	}
}
