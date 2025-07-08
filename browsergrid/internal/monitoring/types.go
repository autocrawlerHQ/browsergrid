package monitoring

import "time"

// MetricsTimeRange represents the time range for metrics
type MetricsTimeRange string

const (
	TimeRange5Min  MetricsTimeRange = "5m"
	TimeRange30Min MetricsTimeRange = "30m"
	TimeRange1Hour MetricsTimeRange = "1h"
	TimeRange6Hour MetricsTimeRange = "6h"
	TimeRange1Day  MetricsTimeRange = "1d"
	TimeRange7Day  MetricsTimeRange = "7d"
)

// MetricsDataPoint represents a single data point in time series
type MetricsDataPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Value     float64   `json:"value"`
}

// QueueMetrics represents time-series metrics for queues
type QueueMetrics struct {
	Queue          string             `json:"queue"`
	TasksProcessed []MetricsDataPoint `json:"tasks_processed"`
	TasksFailed    []MetricsDataPoint `json:"tasks_failed"`
	ErrorRate      []MetricsDataPoint `json:"error_rate"`
	QueueSize      []MetricsDataPoint `json:"queue_size"`
}

// SystemMetrics represents overall system metrics
type SystemMetrics struct {
	TasksProcessed []MetricsDataPoint      `json:"tasks_processed"`
	TasksFailed    []MetricsDataPoint      `json:"tasks_failed"`
	ErrorRate      []MetricsDataPoint      `json:"error_rate"`
	QueueMetrics   map[string]QueueMetrics `json:"queue_metrics,omitempty"`
}

// QueueStatsExtended extends QueueInfo with additional metrics
type QueueStatsExtended struct {
	Queue        string    `json:"queue"`
	State        string    `json:"state"` // "run", "paused"
	Size         int       `json:"size"`
	MemoryUsage  string    `json:"memory_usage"`
	Processed    int       `json:"processed"`
	Failed       int       `json:"failed"`
	ErrorRate    float64   `json:"error_rate"`
	Active       int       `json:"active"`
	Pending      int       `json:"pending"`
	Scheduled    int       `json:"scheduled"`
	Retry        int       `json:"retry"`
	Archived     int       `json:"archived"`
	LastActivity time.Time `json:"last_activity,omitempty"`
}
