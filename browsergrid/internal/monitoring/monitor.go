package monitoring

import (
	"fmt"
	"math"
	"time"

	"github.com/hibiken/asynq"
)

// WorkerMonitor provides visibility into running workers without database registration
type WorkerMonitor struct {
	inspector *asynq.Inspector
} // @name WorkerMonitor

func NewWorkerMonitor(redisOpt asynq.RedisClientOpt) *WorkerMonitor {
	return &WorkerMonitor{
		inspector: asynq.NewInspector(redisOpt),
	}
}

// GetServers returns information about active Asynq servers (workers)
func (m *WorkerMonitor) GetServers() ([]*asynq.ServerInfo, error) {
	return m.inspector.Servers()
}

// GetQueueStats returns statistics for all queues
func (m *WorkerMonitor) GetQueueStats() (map[string]*asynq.QueueInfo, error) {
	queues, err := m.inspector.Queues()
	if err != nil {
		return nil, err
	}

	stats := make(map[string]*asynq.QueueInfo)
	for _, q := range queues {
		info, err := m.inspector.GetQueueInfo(q)
		if err != nil {
			continue
		}
		stats[q] = info
	}

	return stats, nil
}

// GetServerStats returns aggregated server statistics
func (m *WorkerMonitor) GetServerStats() (*ServerStats, error) {
	servers, err := m.inspector.Servers()
	if err != nil {
		return nil, err
	}

	stats := &ServerStats{
		TotalServers:  len(servers),
		ActiveServers: 0,
		TotalQueues:   make(map[string]int),
	}

	for _, s := range servers {
		if s.Status == "active" {
			stats.ActiveServers++
		}

		// Count queues per server
		for q := range s.Queues {
			stats.TotalQueues[q]++
		}
	}

	return stats, nil
}

// IsHealthy checks if the worker pool is healthy
func (m *WorkerMonitor) IsHealthy(minServers int) (bool, string) {
	servers, err := m.inspector.Servers()
	if err != nil {
		return false, fmt.Sprintf("Failed to get servers: %v", err)
	}

	if len(servers) < minServers {
		return false, fmt.Sprintf("Not enough servers: %d < %d", len(servers), minServers)
	}

	// Check if queues are backing up
	queues, err := m.inspector.Queues()
	if err != nil {
		return false, fmt.Sprintf("Failed to get queues: %v", err)
	}

	for _, q := range queues {
		info, err := m.inspector.GetQueueInfo(q)
		if err != nil {
			continue
		}

		// Alert if too many tasks are pending
		if info.Pending > 1000 {
			return false, fmt.Sprintf("Queue %s has too many pending tasks: %d", q, info.Pending)
		}

		// Alert if too many tasks are in retry
		if info.Retry > 100 {
			return false, fmt.Sprintf("Queue %s has too many retry tasks: %d", q, info.Retry)
		}

		// Alert if too many tasks are archived (failed)
		if info.Archived > 500 {
			return false, fmt.Sprintf("Queue %s has too many archived tasks: %d", q, info.Archived)
		}
	}

	return true, "Healthy"
}

// GetQueueHealth returns health status for each queue
func (m *WorkerMonitor) GetQueueHealth() (map[string]QueueHealth, error) {
	queues, err := m.inspector.Queues()
	if err != nil {
		return nil, err
	}

	health := make(map[string]QueueHealth)
	for _, q := range queues {
		info, err := m.inspector.GetQueueInfo(q)
		if err != nil {
			health[q] = QueueHealth{
				Status:  "error",
				Message: fmt.Sprintf("Failed to get queue info: %v", err),
			}
			continue
		}

		queueHealth := QueueHealth{
			Status:    "healthy",
			Message:   "Queue is operating normally",
			Pending:   info.Pending,
			Active:    info.Active,
			Scheduled: info.Scheduled,
			Retry:     info.Retry,
			Archived:  info.Archived,
			Completed: info.Completed,
			Paused:    info.Paused,
		}

		// Determine health status based on queue metrics
		if info.Paused {
			queueHealth.Status = "paused"
			queueHealth.Message = "Queue is paused"
		} else if info.Pending > 1000 {
			queueHealth.Status = "warning"
			queueHealth.Message = fmt.Sprintf("High pending tasks: %d", info.Pending)
		} else if info.Retry > 100 {
			queueHealth.Status = "warning"
			queueHealth.Message = fmt.Sprintf("High retry tasks: %d", info.Retry)
		} else if info.Archived > 500 {
			queueHealth.Status = "critical"
			queueHealth.Message = fmt.Sprintf("High archived tasks: %d", info.Archived)
		}

		health[q] = queueHealth
	}

	return health, nil
}

// GetTaskInfo returns information about tasks in various states
func (m *WorkerMonitor) GetTaskInfo(queue string) (*TaskInfo, error) {
	info := &TaskInfo{
		Queue: queue,
	}

	// Get pending tasks (limited sample)
	pending, err := m.inspector.ListPendingTasks(queue, asynq.PageSize(10), asynq.Page(0))
	if err != nil {
		return nil, fmt.Errorf("failed to get pending tasks: %w", err)
	}
	info.PendingTasks = pending

	// Get active tasks
	active, err := m.inspector.ListActiveTasks(queue, asynq.PageSize(10), asynq.Page(0))
	if err != nil {
		return nil, fmt.Errorf("failed to get active tasks: %w", err)
	}
	info.ActiveTasks = active

	// Get scheduled tasks
	scheduled, err := m.inspector.ListScheduledTasks(queue, asynq.PageSize(10), asynq.Page(0))
	if err != nil {
		return nil, fmt.Errorf("failed to get scheduled tasks: %w", err)
	}
	info.ScheduledTasks = scheduled

	// Get retry tasks
	retry, err := m.inspector.ListRetryTasks(queue, asynq.PageSize(10), asynq.Page(0))
	if err != nil {
		return nil, fmt.Errorf("failed to get retry tasks: %w", err)
	}
	info.RetryTasks = retry

	// Get archived tasks
	archived, err := m.inspector.ListArchivedTasks(queue, asynq.PageSize(10), asynq.Page(0))
	if err != nil {
		return nil, fmt.Errorf("failed to get archived tasks: %w", err)
	}
	info.ArchivedTasks = archived

	return info, nil
}

// ServerStats represents aggregated server statistics
type ServerStats struct {
	TotalServers  int            `json:"total_servers"`
	ActiveServers int            `json:"active_servers"`
	TotalQueues   map[string]int `json:"total_queues"` // queue name -> count of servers processing it
} // @name ServerStats

// QueueHealth represents health status of a queue
type QueueHealth struct {
	Status    string `json:"status"` // "healthy", "warning", "critical", "error", "paused"
	Message   string `json:"message"`
	Pending   int    `json:"pending"`
	Active    int    `json:"active"`
	Scheduled int    `json:"scheduled"`
	Retry     int    `json:"retry"`
	Archived  int    `json:"archived"`
	Completed int    `json:"completed"`
	Paused    bool   `json:"paused"`
} // @name QueueHealth

// TaskInfo provides detailed information about tasks in a queue
type TaskInfo struct {
	Queue          string            `json:"queue"`
	PendingTasks   []*asynq.TaskInfo `json:"pending_tasks"`
	ActiveTasks    []*asynq.TaskInfo `json:"active_tasks"`
	ScheduledTasks []*asynq.TaskInfo `json:"scheduled_tasks"`
	RetryTasks     []*asynq.TaskInfo `json:"retry_tasks"`
	ArchivedTasks  []*asynq.TaskInfo `json:"archived_tasks"`
} // @name TaskInfo

// MonitoringInfo provides comprehensive monitoring information
type MonitoringInfo struct {
	Servers     []*asynq.ServerInfo         `json:"servers"`
	QueueStats  map[string]*asynq.QueueInfo `json:"queue_stats"`
	QueueHealth map[string]QueueHealth      `json:"queue_health"`
	ServerStats *ServerStats                `json:"server_stats"`
	Timestamp   time.Time                   `json:"timestamp"`
} // @name MonitoringInfo

// GetMonitoringInfo returns comprehensive monitoring information
func (m *WorkerMonitor) GetMonitoringInfo() (*MonitoringInfo, error) {
	servers, err := m.GetServers()
	if err != nil {
		return nil, fmt.Errorf("failed to get servers: %w", err)
	}

	queueStats, err := m.GetQueueStats()
	if err != nil {
		return nil, fmt.Errorf("failed to get queue stats: %w", err)
	}

	queueHealth, err := m.GetQueueHealth()
	if err != nil {
		return nil, fmt.Errorf("failed to get queue health: %w", err)
	}

	serverStats, err := m.GetServerStats()
	if err != nil {
		return nil, fmt.Errorf("failed to get server stats: %w", err)
	}

	return &MonitoringInfo{
		Servers:     servers,
		QueueStats:  queueStats,
		QueueHealth: queueHealth,
		ServerStats: serverStats,
		Timestamp:   time.Now(),
	}, nil
}

// GetSchedulerEntries returns scheduled/periodic tasks
func (m *WorkerMonitor) GetSchedulerEntries() ([]*asynq.SchedulerEntry, error) {
	return m.inspector.SchedulerEntries()
}

// DeleteAllArchivedTasks deletes all archived tasks from a queue
func (m *WorkerMonitor) DeleteAllArchivedTasks(queue string) (int, error) {
	deleted, err := m.inspector.DeleteAllArchivedTasks(queue)
	if err != nil {
		return 0, err
	}
	return deleted, nil
}

// DeleteAllRetryTasks deletes all retry tasks from a queue
func (m *WorkerMonitor) DeleteAllRetryTasks(queue string) (int, error) {
	deleted, err := m.inspector.DeleteAllRetryTasks(queue)
	if err != nil {
		return 0, err
	}
	return deleted, nil
}

// PauseQueue pauses processing of a queue
func (m *WorkerMonitor) PauseQueue(queue string) error {
	return m.inspector.PauseQueue(queue)
}

// UnpauseQueue unpauses processing of a queue
func (m *WorkerMonitor) UnpauseQueue(queue string) error {
	return m.inspector.UnpauseQueue(queue)
}

// GetMetrics returns time-series metrics for the specified time range
func (m *WorkerMonitor) GetMetrics(timeRange MetricsTimeRange, queues []string) (*SystemMetrics, error) {
	// In a real implementation, this would fetch from a time-series database
	// For now, we'll generate sample data based on current stats

	now := time.Now()
	duration := m.parseTimeRange(timeRange)
	interval := m.getInterval(timeRange)

	metrics := &SystemMetrics{
		TasksProcessed: make([]MetricsDataPoint, 0),
		TasksFailed:    make([]MetricsDataPoint, 0),
		ErrorRate:      make([]MetricsDataPoint, 0),
		QueueMetrics:   make(map[string]QueueMetrics),
	}

	// Generate time series data points
	for t := now.Add(-duration); t.Before(now); t = t.Add(interval) {
		// In production, fetch from metrics store
		processed := m.generateMetricValue(0.27, 0.36, t)
		failed := m.generateMetricValue(0.019, 0.038, t)
		errorRate := failed / (processed + failed) * 100

		metrics.TasksProcessed = append(metrics.TasksProcessed, MetricsDataPoint{
			Timestamp: t,
			Value:     processed,
		})
		metrics.TasksFailed = append(metrics.TasksFailed, MetricsDataPoint{
			Timestamp: t,
			Value:     failed,
		})
		metrics.ErrorRate = append(metrics.ErrorRate, MetricsDataPoint{
			Timestamp: t,
			Value:     errorRate,
		})
	}

	// Add queue-specific metrics if requested
	if len(queues) > 0 {
		for _, queue := range queues {
			queueMetrics := QueueMetrics{
				Queue:          queue,
				TasksProcessed: make([]MetricsDataPoint, 0),
				TasksFailed:    make([]MetricsDataPoint, 0),
				ErrorRate:      make([]MetricsDataPoint, 0),
				QueueSize:      make([]MetricsDataPoint, 0),
			}

			// Generate queue-specific data
			for t := now.Add(-duration); t.Before(now); t = t.Add(interval) {
				processed := m.generateMetricValue(0.1, 0.2, t)
				failed := m.generateMetricValue(0.01, 0.02, t)
				size := m.generateMetricValue(100, 200, t)

				queueMetrics.TasksProcessed = append(queueMetrics.TasksProcessed, MetricsDataPoint{
					Timestamp: t,
					Value:     processed,
				})
				queueMetrics.TasksFailed = append(queueMetrics.TasksFailed, MetricsDataPoint{
					Timestamp: t,
					Value:     failed,
				})
				queueMetrics.ErrorRate = append(queueMetrics.ErrorRate, MetricsDataPoint{
					Timestamp: t,
					Value:     failed / (processed + failed) * 100,
				})
				queueMetrics.QueueSize = append(queueMetrics.QueueSize, MetricsDataPoint{
					Timestamp: t,
					Value:     size,
				})
			}

			metrics.QueueMetrics[queue] = queueMetrics
		}
	}

	return metrics, nil
}

// GetQueueStatsExtended returns extended queue statistics
func (m *WorkerMonitor) GetQueueStatsExtended() ([]QueueStatsExtended, error) {
	queues, err := m.inspector.Queues()
	if err != nil {
		return nil, err
	}

	stats := make([]QueueStatsExtended, 0, len(queues))

	for _, q := range queues {
		info, err := m.inspector.GetQueueInfo(q)
		if err != nil {
			continue
		}

		// Calculate additional metrics
		totalTasks := info.Active + info.Pending + info.Scheduled + info.Retry + info.Archived
		memoryUsage := m.calculateMemoryUsage(totalTasks)

		errorRate := 0.0
		if info.Completed > 0 {
			errorRate = float64(info.Failed) / float64(info.Completed+info.Failed) * 100
		}

		state := "run"
		if info.Paused {
			state = "paused"
		}

		stats = append(stats, QueueStatsExtended{
			Queue:       q,
			State:       state,
			Size:        totalTasks,
			MemoryUsage: memoryUsage,
			Processed:   info.Completed,
			Failed:      info.Failed,
			ErrorRate:   errorRate,
			Active:      info.Active,
			Pending:     info.Pending,
			Scheduled:   info.Scheduled,
			Retry:       info.Retry,
			Archived:    info.Archived,
		})
	}

	return stats, nil
}

// Helper methods
func (m *WorkerMonitor) parseTimeRange(tr MetricsTimeRange) time.Duration {
	switch tr {
	case TimeRange5Min:
		return 5 * time.Minute
	case TimeRange30Min:
		return 30 * time.Minute
	case TimeRange1Hour:
		return time.Hour
	case TimeRange6Hour:
		return 6 * time.Hour
	case TimeRange1Day:
		return 24 * time.Hour
	case TimeRange7Day:
		return 7 * 24 * time.Hour
	default:
		return 30 * time.Minute
	}
}

func (m *WorkerMonitor) getInterval(tr MetricsTimeRange) time.Duration {
	switch tr {
	case TimeRange5Min:
		return 10 * time.Second
	case TimeRange30Min:
		return time.Minute
	case TimeRange1Hour:
		return 2 * time.Minute
	case TimeRange6Hour:
		return 10 * time.Minute
	case TimeRange1Day:
		return 30 * time.Minute
	case TimeRange7Day:
		return 2 * time.Hour
	default:
		return time.Minute
	}
}

func (m *WorkerMonitor) generateMetricValue(min, max float64, t time.Time) float64 {
	// Simple sine wave with noise for demo
	// In production, fetch from metrics store
	base := min + (max-min)/2
	amplitude := (max - min) / 2
	noise := (float64(t.Unix()%7) - 3.5) / 35.0
	return base + amplitude*math.Sin(float64(t.Unix())/300) + noise
}

func (m *WorkerMonitor) calculateMemoryUsage(taskCount int) string {
	// Estimate memory usage based on task count
	// Assume ~1KB per task average
	bytes := taskCount * 1024

	if bytes < 1024*1024 {
		return fmt.Sprintf("%.1f KB", float64(bytes)/1024)
	} else if bytes < 1024*1024*1024 {
		return fmt.Sprintf("%.1f MB", float64(bytes)/(1024*1024))
	}
	return fmt.Sprintf("%.1f GB", float64(bytes)/(1024*1024*1024))
}
