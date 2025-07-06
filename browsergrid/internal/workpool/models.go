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

func (wp *WorkPool) SessionsToCreate(currentSessions, pendingSessions int) int {
	if !wp.NeedsScaling(currentSessions, pendingSessions) {
		return 0
	}

	totalSessions := currentSessions + pendingSessions
	needed := wp.MinSize - totalSessions

	if currentSessions+pendingSessions+needed > wp.MaxConcurrency {
		needed = wp.MaxConcurrency - currentSessions - pendingSessions
	}

	if needed < 0 {
		needed = 0
	}

	return needed
}

// WorkPoolListResponse represents a response containing a list of work pools
// @Description Response containing a list of work pools
type WorkPoolListResponse struct {
	Pools []WorkPool `json:"pools"`
	Total int        `json:"total" example:"5"`
} //@name WorkPoolListResponse

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
