package monitoring

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/hibiken/asynq"
)

// RegisterRoutes registers monitoring endpoints
func RegisterRoutes(rg *gin.RouterGroup, redisOpt asynq.RedisClientOpt) {
	monitor := NewWorkerMonitor(redisOpt)

	// Overview endpoint
	rg.GET("/monitoring", getMonitoringInfo(monitor))

	// Specific endpoints
	rg.GET("/monitoring/servers", getServers(monitor))
	rg.GET("/monitoring/queues", getQueues(monitor))
	rg.GET("/monitoring/queues/:name", getQueueDetails(monitor))
	rg.GET("/monitoring/queues/:name/tasks", getQueueTasks(monitor))
	rg.GET("/monitoring/health", getHealth(monitor))
	rg.GET("/monitoring/scheduler", getSchedulerEntries(monitor))

	// Management endpoints
	rg.POST("/monitoring/queues/:name/pause", pauseQueue(monitor))
	rg.POST("/monitoring/queues/:name/unpause", unpauseQueue(monitor))
	rg.DELETE("/monitoring/queues/:name/archived", deleteArchivedTasks(monitor))
	rg.DELETE("/monitoring/queues/:name/retry", deleteRetryTasks(monitor))
}

// getMonitoringInfo returns comprehensive monitoring information
// @Summary Get monitoring information
// @Description Get comprehensive monitoring information including servers, queues, and health
// @Tags monitoring
// @Accept json
// @Produce json
// @Success 200 {object} MonitoringInfo
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/monitoring [get]
func getMonitoringInfo(monitor *WorkerMonitor) gin.HandlerFunc {
	return func(c *gin.Context) {
		info, err := monitor.GetMonitoringInfo()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, info)
	}
}

// getServers returns information about Asynq servers
// @Summary Get server information
// @Description Get information about all Asynq servers (workers)
// @Tags monitoring
// @Accept json
// @Produce json
// @Success 200 {array} asynq.ServerInfo
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/monitoring/servers [get]
func getServers(monitor *WorkerMonitor) gin.HandlerFunc {
	return func(c *gin.Context) {
		servers, err := monitor.GetServers()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"servers": servers,
			"total":   len(servers),
		})
	}
}

// getQueues returns queue statistics
// @Summary Get queue statistics
// @Description Get statistics for all queues
// @Tags monitoring
// @Accept json
// @Produce json
// @Success 200 {object} map[string]asynq.QueueInfo
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/monitoring/queues [get]
func getQueues(monitor *WorkerMonitor) gin.HandlerFunc {
	return func(c *gin.Context) {
		stats, err := monitor.GetQueueStats()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		health, err := monitor.GetQueueHealth()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"stats":  stats,
			"health": health,
		})
	}
}

// getQueueDetails returns detailed information about a specific queue
// @Summary Get queue details
// @Description Get detailed information about a specific queue
// @Tags monitoring
// @Accept json
// @Produce json
// @Param name path string true "Queue name"
// @Success 200 {object} asynq.QueueInfo
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/monitoring/queues/{name} [get]
func getQueueDetails(monitor *WorkerMonitor) gin.HandlerFunc {
	return func(c *gin.Context) {
		queueName := c.Param("name")

		info, err := monitor.inspector.GetQueueInfo(queueName)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "queue not found"})
			return
		}

		health, err := monitor.GetQueueHealth()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"info":   info,
			"health": health[queueName],
		})
	}
}

// getQueueTasks returns tasks in various states for a queue
// @Summary Get queue tasks
// @Description Get sample of tasks in various states for a queue
// @Tags monitoring
// @Accept json
// @Produce json
// @Param name path string true "Queue name"
// @Success 200 {object} TaskInfo
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/monitoring/queues/{name}/tasks [get]
func getQueueTasks(monitor *WorkerMonitor) gin.HandlerFunc {
	return func(c *gin.Context) {
		queueName := c.Param("name")

		taskInfo, err := monitor.GetTaskInfo(queueName)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, taskInfo)
	}
}

// getHealth returns health status
// @Summary Get health status
// @Description Get health status of the worker system
// @Tags monitoring
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 503 {object} map[string]interface{}
// @Router /api/v1/monitoring/health [get]
func getHealth(monitor *WorkerMonitor) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Default minimum servers required
		minServers := 1
		if v := c.Query("min_servers"); v != "" {
			// Parse min_servers from query param if provided
			fmt.Sscanf(v, "%d", &minServers)
		}

		healthy, message := monitor.IsHealthy(minServers)

		status := http.StatusOK
		if !healthy {
			status = http.StatusServiceUnavailable
		}

		c.JSON(status, gin.H{
			"healthy": healthy,
			"message": message,
		})
	}
}

// getSchedulerEntries returns scheduled/periodic tasks
// @Summary Get scheduler entries
// @Description Get all scheduled and periodic tasks
// @Tags monitoring
// @Accept json
// @Produce json
// @Success 200 {array} asynq.SchedulerEntry
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/monitoring/scheduler [get]
func getSchedulerEntries(monitor *WorkerMonitor) gin.HandlerFunc {
	return func(c *gin.Context) {
		entries, err := monitor.GetSchedulerEntries()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"entries": entries,
			"total":   len(entries),
		})
	}
}

// pauseQueue pauses processing of a queue
// @Summary Pause queue
// @Description Pause processing of a specific queue
// @Tags monitoring
// @Accept json
// @Produce json
// @Param name path string true "Queue name"
// @Success 200 {object} MessageResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/monitoring/queues/{name}/pause [post]
func pauseQueue(monitor *WorkerMonitor) gin.HandlerFunc {
	return func(c *gin.Context) {
		queueName := c.Param("name")

		if err := monitor.PauseQueue(queueName); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("Queue %s paused", queueName)})
	}
}

// unpauseQueue unpauses processing of a queue
// @Summary Unpause queue
// @Description Resume processing of a paused queue
// @Tags monitoring
// @Accept json
// @Produce json
// @Param name path string true "Queue name"
// @Success 200 {object} MessageResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/monitoring/queues/{name}/unpause [post]
func unpauseQueue(monitor *WorkerMonitor) gin.HandlerFunc {
	return func(c *gin.Context) {
		queueName := c.Param("name")

		if err := monitor.UnpauseQueue(queueName); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("Queue %s unpaused", queueName)})
	}
}

// deleteArchivedTasks deletes all archived tasks from a queue
// @Summary Delete archived tasks
// @Description Delete all archived (failed) tasks from a queue
// @Tags monitoring
// @Accept json
// @Produce json
// @Param name path string true "Queue name"
// @Success 200 {object} map[string]interface{}
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/monitoring/queues/{name}/archived [delete]
func deleteArchivedTasks(monitor *WorkerMonitor) gin.HandlerFunc {
	return func(c *gin.Context) {
		queueName := c.Param("name")

		deleted, err := monitor.DeleteAllArchivedTasks(queueName)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message": fmt.Sprintf("Deleted %d archived tasks from queue %s", deleted, queueName),
			"deleted": deleted,
		})
	}
}

// deleteRetryTasks deletes all retry tasks from a queue
// @Summary Delete retry tasks
// @Description Delete all retry tasks from a queue
// @Tags monitoring
// @Accept json
// @Produce json
// @Param name path string true "Queue name"
// @Success 200 {object} map[string]interface{}
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/monitoring/queues/{name}/retry [delete]
func deleteRetryTasks(monitor *WorkerMonitor) gin.HandlerFunc {
	return func(c *gin.Context) {
		queueName := c.Param("name")

		deleted, err := monitor.DeleteAllRetryTasks(queueName)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message": fmt.Sprintf("Deleted %d retry tasks from queue %s", deleted, queueName),
			"deleted": deleted,
		})
	}
}

// ErrorResponse represents an error response
// @Description Error response
type ErrorResponse struct {
	Error string `json:"error" example:"Internal server error"`
}

// MessageResponse represents a simple message response
// @Description Simple message response
type MessageResponse struct {
	Message string `json:"message" example:"Operation successful"`
}
