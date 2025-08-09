package deployments

import (
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// Runtime represents supported deployment runtimes
// @Description Supported deployment runtimes
type Runtime string //@name Runtime

const (
	RuntimeNode   Runtime = "node"
	RuntimePython Runtime = "python"
)

// DeploymentStatus represents the current status of a deployment
// @Description Current status of a deployment
type DeploymentStatus string //@name DeploymentStatus

const (
	StatusActive     DeploymentStatus = "active"
	StatusInactive   DeploymentStatus = "inactive"
	StatusDeploying  DeploymentStatus = "deploying"
	StatusFailed     DeploymentStatus = "failed"
	StatusDeprecated DeploymentStatus = "deprecated"
)

// RunStatus represents the current status of a deployment run
// @Description Current status of a deployment run
type RunStatus string //@name RunStatus

const (
	RunStatusPending   RunStatus = "pending"
	RunStatusRunning   RunStatus = "running"
	RunStatusCompleted RunStatus = "completed"
	RunStatusFailed    RunStatus = "failed"
	RunStatusCancelled RunStatus = "cancelled"
)

// DeploymentConfig represents deployment configuration
// @Description Deployment configuration settings
type DeploymentConfig struct {
	Concurrency     int                    `json:"concurrency,omitempty"`
	MaxRetries      int                    `json:"max_retries,omitempty"`
	TimeoutSeconds  int                    `json:"timeout_seconds,omitempty"`
	Environment     map[string]string      `json:"environment,omitempty"`
	Schedule        string                 `json:"schedule,omitempty"`
	ResourceLimits  map[string]interface{} `json:"resource_limits,omitempty"`
	TriggerEvents   []string               `json:"trigger_events,omitempty"`
	BrowserRequests []BrowserRequest       `json:"browser_requests,omitempty"`
}

// BrowserRequest represents a browser session request for deployment
// @Description Browser session configuration for deployment
type BrowserRequest struct {
	Browser         string                 `json:"browser,omitempty"`
	Version         string                 `json:"version,omitempty"`
	Headless        bool                   `json:"headless,omitempty"`
	OperatingSystem string                 `json:"operating_system,omitempty"`
	Screen          map[string]interface{} `json:"screen,omitempty"`
	Environment     map[string]string      `json:"environment,omitempty"`
	ProfileID       *uuid.UUID             `json:"profile_id,omitempty"`
}

// Deployment represents a packaged automation script
// @Description Deployment package with configuration and metadata
type Deployment struct {
	ID          uuid.UUID        `json:"id" gorm:"type:uuid;primary_key"`
	Name        string           `json:"name" gorm:"not null;index"`
	Description string           `json:"description"`
	Version     string           `json:"version" gorm:"not null"`
	Runtime     Runtime          `json:"runtime" gorm:"not null"`
	PackageURL  string           `json:"package_url" gorm:"not null"`
	PackageHash string           `json:"package_hash" gorm:"not null"`
	Config      datatypes.JSON   `json:"config"`
	Status      DeploymentStatus `json:"status" gorm:"not null;default:active"`
	CreatedAt   time.Time        `json:"created_at" gorm:"not null"`
	UpdatedAt   time.Time        `json:"updated_at" gorm:"not null"`

	// Relationships
	Runs []DeploymentRun `json:"runs,omitempty" gorm:"foreignKey:DeploymentID;constraint:OnDelete:CASCADE"`

	// Computed fields
	TotalRuns      int        `json:"total_runs" gorm:"-"`
	SuccessfulRuns int        `json:"successful_runs" gorm:"-"`
	FailedRuns     int        `json:"failed_runs" gorm:"-"`
	LastRunAt      *time.Time `json:"last_run_at" gorm:"-"`
}

func (Deployment) TableName() string {
	return "deployments"
}

// BeforeCreate hook to ensure UUID is set and defaults are applied
func (d *Deployment) BeforeCreate(tx *gorm.DB) error {
	// Validate required fields
	if d.Name == "" {
		return errors.New("deployment name cannot be empty")
	}

	if d.ID == uuid.Nil {
		d.ID = uuid.New()
	}

	now := time.Now()
	if d.CreatedAt.IsZero() {
		d.CreatedAt = now
	}
	if d.UpdatedAt.IsZero() {
		d.UpdatedAt = now
	}

	if d.Config == nil {
		d.Config = datatypes.JSON("{}")
	}

	if d.Status == "" {
		d.Status = StatusActive
	}

	return nil
}

// BeforeUpdate hook to update the UpdatedAt timestamp
func (d *Deployment) BeforeUpdate(tx *gorm.DB) error {
	d.UpdatedAt = time.Now()
	return nil
}

// DeploymentRun represents an execution of a deployment
// @Description Deployment run with execution details and results
type DeploymentRun struct {
	ID           uuid.UUID      `json:"id" gorm:"type:uuid;primary_key"`
	DeploymentID uuid.UUID      `json:"deployment_id" gorm:"type:uuid;not null;index"`
	SessionID    *uuid.UUID     `json:"session_id,omitempty" gorm:"type:uuid;index"`
	Status       RunStatus      `json:"status" gorm:"not null;default:pending"`
	StartedAt    time.Time      `json:"started_at" gorm:"not null"`
	CompletedAt  *time.Time     `json:"completed_at,omitempty"`
	Output       datatypes.JSON `json:"output,omitempty"`
	Error        *string        `json:"error,omitempty"`
	CreatedAt    time.Time      `json:"created_at" gorm:"not null"`
	UpdatedAt    time.Time      `json:"updated_at" gorm:"not null"`

	// Relationships
	Deployment *Deployment `json:"deployment,omitempty" gorm:"foreignKey:DeploymentID"`

	// Computed fields
	Duration *int64 `json:"duration_seconds,omitempty" gorm:"-"`
}

func (DeploymentRun) TableName() string {
	return "deployment_runs"
}

// BeforeCreate hook to ensure UUID is set and defaults are applied
func (dr *DeploymentRun) BeforeCreate(tx *gorm.DB) error {
	if dr.ID == uuid.Nil {
		dr.ID = uuid.New()
	}

	now := time.Now()
	if dr.CreatedAt.IsZero() {
		dr.CreatedAt = now
	}
	if dr.UpdatedAt.IsZero() {
		dr.UpdatedAt = now
	}
	if dr.StartedAt.IsZero() {
		dr.StartedAt = now
	}

	if dr.Output == nil {
		dr.Output = datatypes.JSON("{}")
	}

	if dr.Status == "" {
		dr.Status = RunStatusPending
	}

	return nil
}

// BeforeUpdate hook to update the UpdatedAt timestamp
func (dr *DeploymentRun) BeforeUpdate(tx *gorm.DB) error {
	dr.UpdatedAt = time.Now()
	return nil
}

// AfterFind hook to calculate duration
func (dr *DeploymentRun) AfterFind(tx *gorm.DB) error {
	if dr.CompletedAt != nil {
		duration := int64(dr.CompletedAt.Sub(dr.StartedAt).Seconds())
		dr.Duration = &duration
	}
	return nil
}

// IsTerminal returns true if the run is in a terminal state
func (dr *DeploymentRun) IsTerminal() bool {
	return dr.Status == RunStatusCompleted ||
		dr.Status == RunStatusFailed ||
		dr.Status == RunStatusCancelled
}

// IsRunning returns true if the run is currently executing
func (dr *DeploymentRun) IsRunning() bool {
	return dr.Status == RunStatusRunning
}
