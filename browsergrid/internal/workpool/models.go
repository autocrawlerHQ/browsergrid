package workpool

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// ProviderType represents the type of provider for a work pool
// @Description Type of provider for a work pool
type ProviderType string //@name ProviderType

const (
	ProviderDocker ProviderType = "docker"
	ProviderACI    ProviderType = "azure_aci"
	ProviderLocal  ProviderType = "local"
)

// WorkPool represents a work pool for managing browser workers
// @Description Work pool configuration for managing browser workers
type WorkPool struct {
	ID          uuid.UUID    `json:"id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Name        string       `json:"name" example:"Chrome Workers"`
	Description string       `json:"description" example:"Pool for Chrome browser workers"`
	Provider    ProviderType `json:"provider" example:"docker"`

	MinSize            int  `json:"min_size" example:"0"`
	MaxConcurrency     int  `json:"max_concurrency" example:"10"`
	MaxIdleTime        int  `json:"max_idle_time" example:"1800"`
	MaxSessionDuration int  `json:"max_session_duration" example:"1800"`
	AutoScale          bool `json:"auto_scale" example:"true"`
	Paused             bool `json:"paused" example:"false"`

	DefaultPriority int    `json:"default_priority" example:"0"`
	QueueStrategy   string `json:"queue_strategy" example:"fifo"`

	DefaultEnv   datatypes.JSON `json:"default_env" swaggertype:"object"`
	DefaultImage *string        `json:"default_image" example:"browsergrid/chrome:latest"`

	CreatedAt time.Time `json:"created_at" example:"2023-01-01T00:00:00Z"`
	UpdatedAt time.Time `json:"updated_at" example:"2023-01-01T00:00:00Z"`
} //@name WorkPool

func (WorkPool) TableName() string {
	return "work_pools"
}

func (wp *WorkPool) NeedsScaling(currentSessions, pendingSessions int) bool {
	if wp.Paused || !wp.AutoScale {
		return false
	}

	totalSessions := currentSessions + pendingSessions
	return totalSessions < wp.MinSize
}

func (wp *WorkPool) CanAcceptMore(currentSessions int) bool {
	return !wp.Paused && currentSessions < wp.MaxConcurrency
}

func (wp *WorkPool) SessionsToCreate(currentSessions, pendingSessions, availableWorkerSlots int) int {
	if !wp.NeedsScaling(currentSessions, pendingSessions) {
		return 0
	}

	totalSessions := currentSessions + pendingSessions
	needed := wp.MinSize - totalSessions

	if needed > availableWorkerSlots {
		needed = availableWorkerSlots
	}

	if currentSessions+pendingSessions+needed > wp.MaxConcurrency {
		needed = wp.MaxConcurrency - currentSessions - pendingSessions
	}

	if needed < 0 {
		needed = 0
	}

	return needed
}

// Worker represents a browser worker instance
// @Description Worker instance that handles browser sessions
type Worker struct {
	ID       uuid.UUID    `json:"id" example:"550e8400-e29b-41d4-a716-446655440001"`
	PoolID   uuid.UUID    `json:"pool_id" example:"550e8400-e29b-41d4-a716-446655440000" gorm:"uniqueIndex:idx_workers_pool_hostname"`
	Name     string       `json:"name" example:"worker-chrome-001"`
	Hostname string       `json:"hostname" example:"browsergrid-worker-1" gorm:"uniqueIndex:idx_workers_pool_hostname"`
	Provider ProviderType `json:"provider" example:"docker"`

	MaxSlots  int       `json:"max_slots" example:"1"`
	Active    int       `json:"active" example:"0"`
	LastBeat  time.Time `json:"last_beat" example:"2023-01-01T00:00:00Z"`
	StartedAt time.Time `json:"started_at" example:"2023-01-01T00:00:00Z"`
	Paused    bool      `json:"paused" example:"false"`

	Pool WorkPool `json:"-" gorm:"foreignKey:PoolID;constraint:OnDelete:CASCADE"`
} //@name Worker

func (Worker) TableName() string {
	return "workers"
}

func (w *Worker) IsOnline(ttl time.Duration) bool {
	return time.Since(w.LastBeat) <= ttl
}

func (w *Worker) HasCapacity() bool {
	return !w.Paused && w.Active < w.MaxSlots
}

func (w *Worker) AvailableSlots() int {
	if w.Paused {
		return 0
	}
	return w.MaxSlots - w.Active
}

// WorkPoolListResponse represents a response containing a list of work pools
// @Description Response containing a list of work pools
type WorkPoolListResponse struct {
	Pools []WorkPool `json:"pools"`
	Total int        `json:"total" example:"5"`
} //@name WorkPoolListResponse

// WorkerListResponse represents a response containing a list of workers
// @Description Response containing a list of workers
type WorkerListResponse struct {
	Workers []Worker `json:"workers"`
	Total   int      `json:"total" example:"10"`
} //@name WorkerListResponse

// WorkerHeartbeatRequest represents a heartbeat request from a worker
// @Description Heartbeat data with active session count
type WorkerHeartbeatRequest struct {
	Active int `json:"active" example:"2" validate:"min=0"`
} //@name WorkerHeartbeatRequest

// WorkerPauseRequest represents a request to pause or resume a worker
// @Description Pause configuration for a worker
type WorkerPauseRequest struct {
	Paused bool `json:"paused" example:"true"`
} //@name WorkerPauseRequest

// ScalingRequest represents scaling parameters for a work pool
// @Description Scaling parameters for updating work pool configuration
type ScalingRequest struct {
	MinSize            *int  `json:"min_size,omitempty" example:"1" validate:"min=0"`
	MaxConcurrency     *int  `json:"max_concurrency,omitempty" example:"10" validate:"min=1"`
	MaxIdleTime        *int  `json:"max_idle_time,omitempty" example:"1800" validate:"min=0"`
	MaxSessionDuration *int  `json:"max_session_duration,omitempty" example:"1800" validate:"min=60"`
	AutoScale          *bool `json:"auto_scale,omitempty" example:"true"`
	Paused             *bool `json:"paused,omitempty" example:"false"`
} //@name ScalingRequest

// MessageResponse represents a simple message response
// @Description Simple message response
type MessageResponse struct {
	Message string `json:"message" example:"Operation completed successfully"`
} //@name MessageResponse

// ScalingResponse represents a scaling operation response
// @Description Response from a scaling operation with updated parameters
type ScalingResponse struct {
	Message string                 `json:"message" example:"pool scaled"`
	Updates map[string]interface{} `json:"updates"`
} //@name ScalingResponse

// ErrorResponse represents an error response
// @Description Error response with details
type ErrorResponse struct {
	Error string `json:"error" example:"Validation failed"`
} //@name ErrorResponse
