package sessions

import (
	"encoding/json"
	"fmt"
	"time"

	"database/sql/driver"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// ScreenConfig represents screen configuration for browser sessions
// @Description Screen configuration with width, height, DPI and scale
type ScreenConfig struct {
	Width  int     `json:"width" example:"1920"`
	Height int     `json:"height" example:"1080"`
	DPI    int     `json:"dpi" example:"96"`
	Scale  float64 `json:"scale" example:"1.0"`
} //@name ScreenConfig

func (s ScreenConfig) Value() (driver.Value, error) {
	return json.Marshal(s)
}

func (s *ScreenConfig) Scan(value interface{}) error {
	if value == nil {
		*s = ScreenConfig{}
		return nil
	}

	var bytes []byte
	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		return fmt.Errorf("cannot scan %T into ScreenConfig", value)
	}

	return json.Unmarshal(bytes, s)
}

// ProxyConfig represents proxy configuration for browser sessions
// @Description Proxy configuration with URL and optional credentials
type ProxyConfig struct {
	URL      string `json:"proxy_url" example:"http://proxy.example.com:8080"`
	Username string `json:"proxy_username,omitempty" example:"user"`
	Password string `json:"proxy_password,omitempty" example:"pass"`
} //@name ProxyConfig

func (p ProxyConfig) Value() (driver.Value, error) {
	return json.Marshal(p)
}

func (p *ProxyConfig) Scan(value interface{}) error {
	if value == nil {
		*p = ProxyConfig{}
		return nil
	}

	var bytes []byte
	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		return fmt.Errorf("cannot scan %T into ProxyConfig", value)
	}

	return json.Unmarshal(bytes, p)
}

// ResourceLimits represents resource limits for browser sessions
// @Description Resource limits for CPU, memory and timeout
type ResourceLimits struct {
	CPU            *float64 `json:"cpu,omitempty" example:"2.0"`
	Memory         *string  `json:"memory,omitempty" example:"2GB"`
	TimeoutMinutes *int     `json:"timeout_minutes,omitempty" example:"30"`
} //@name ResourceLimits

func (r ResourceLimits) Value() (driver.Value, error) {
	return json.Marshal(r)
}

func (r *ResourceLimits) Scan(value interface{}) error {
	if value == nil {
		*r = ResourceLimits{}
		return nil
	}

	var bytes []byte
	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		return fmt.Errorf("cannot scan %T into ResourceLimits", value)
	}

	return json.Unmarshal(bytes, r)
}

// Session represents a browser session
// @Description Browser session with configuration and status
type Session struct {
	ID              uuid.UUID       `json:"id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Browser         Browser         `json:"browser" example:"chrome"`
	Version         BrowserVersion  `json:"version" example:"latest"`
	Headless        bool            `json:"headless" example:"true"`
	OperatingSystem OperatingSystem `json:"operating_system" example:"linux"`
	Screen          ScreenConfig    `json:"screen"`
	Proxy           ProxyConfig     `json:"proxy,omitempty"`
	ResourceLimits  ResourceLimits  `json:"resource_limits,omitempty"`
	Environment     datatypes.JSON  `json:"environment" swaggertype:"object"`
	Status          SessionStatus   `json:"status" example:"pending"`
	CreatedAt       time.Time       `json:"created_at" example:"2023-01-01T00:00:00Z"`
	UpdatedAt       time.Time       `json:"updated_at" example:"2023-01-01T00:00:00Z"`
	ExpiresAt       *time.Time      `json:"expires_at,omitempty" example:"2023-01-01T01:00:00Z"`

	ContainerID      *string `json:"container_id,omitempty" example:"abc123"`
	ContainerNetwork *string `json:"container_network,omitempty" example:"browsergrid_default"`
	Provider         string  `json:"provider" example:"local"`
	WebhooksEnabled  bool    `json:"webhooks_enabled" example:"false"`

	WSEndpoint *string `json:"ws_endpoint,omitempty" example:"ws://localhost:80/devtools/browser"`
	LiveURL    *string `json:"live_url,omitempty" example:"http://localhost:80"`

	WorkPoolID *uuid.UUID `json:"work_pool_id,omitempty" example:"550e8400-e29b-41d4-a716-446655440002"`
	ProfileID  *uuid.UUID `json:"profile_id,omitempty" example:"550e8400-e29b-41d4-a716-446655440001"`
	PoolID     *string    `json:"pool_id,omitempty" example:"chrome-pool"`
	IsPooled   bool       `json:"is_pooled" example:"false"`
	ClaimedAt  *time.Time `json:"claimed_at,omitempty" example:"2023-01-01T00:30:00Z"`
	ClaimedBy  *string    `json:"claimed_by,omitempty" example:"client-123"`

	AvailableAt *time.Time `json:"available_at,omitempty" example:"2023-01-01T00:15:00Z"`
} //@name Session

func (Session) TableName() string {
	return "sessions"
}

func (s *Session) IsInPool() bool {
	return s.IsPooled && s.Status == StatusAvailable && s.ClaimedBy == nil
}

func (s *Session) IsClaimed() bool {
	return s.ClaimedBy != nil && s.ClaimedAt != nil
}

func (s *Session) CanBeClaimed() bool {
	return s.IsInPool() && IsClaimableStatus(s.Status)
}

// BeforeCreate hook for Session - generates UUID if nil
func (s *Session) BeforeCreate(tx *gorm.DB) error {
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	return nil
}

// SessionEvent represents an event that occurred during a session
// @Description Session event with type, data and timestamp
type SessionEvent struct {
	ID        uuid.UUID        `json:"id" example:"550e8400-e29b-41d4-a716-446655440003"`
	SessionID uuid.UUID        `json:"session_id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Event     SessionEventType `json:"event" example:"session_created"`
	Data      datatypes.JSON   `json:"data,omitempty" swaggertype:"object"`
	Timestamp time.Time        `json:"timestamp" example:"2023-01-01T00:00:00Z"`

	Session Session `json:"-" gorm:"foreignKey:SessionID;constraint:OnDelete:CASCADE"`
} //@name SessionEvent

func (SessionEvent) TableName() string {
	return "session_events"
}

// BeforeCreate hook for SessionEvent - generates UUID if nil
func (se *SessionEvent) BeforeCreate(tx *gorm.DB) error {
	if se.ID == uuid.Nil {
		se.ID = uuid.New()
	}
	return nil
}

// SessionMetrics represents performance metrics for a session
// @Description Performance metrics including CPU, memory and network usage
type SessionMetrics struct {
	ID             uuid.UUID `json:"id" example:"550e8400-e29b-41d4-a716-446655440004"`
	SessionID      uuid.UUID `json:"session_id" example:"550e8400-e29b-41d4-a716-446655440000"`
	CPUPercent     *float64  `json:"cpu_percent,omitempty" example:"45.2"`
	MemoryMB       *float64  `json:"memory_mb,omitempty" example:"1024.5"`
	NetworkRXBytes *int64    `json:"network_rx_bytes,omitempty" example:"1048576"`
	NetworkTXBytes *int64    `json:"network_tx_bytes,omitempty" example:"2097152"`
	Timestamp      time.Time `json:"timestamp" example:"2023-01-01T00:00:00Z"`

	Session Session `json:"-" gorm:"foreignKey:SessionID;constraint:OnDelete:CASCADE"`
} //@name SessionMetrics

func (SessionMetrics) TableName() string {
	return "session_metrics"
}

// BeforeCreate hook for SessionMetrics - generates UUID if nil
func (sm *SessionMetrics) BeforeCreate(tx *gorm.DB) error {
	if sm.ID == uuid.Nil {
		sm.ID = uuid.New()
	}
	return nil
}

// Pool represents a session pool configuration
// @Description Session pool with browser configuration and scaling settings
type Pool struct {
	ID          string `json:"id" example:"chrome-pool"`
	Name        string `json:"name" example:"Chrome Pool"`
	Description string `json:"description,omitempty" example:"Pool for Chrome browser sessions"`

	Browser         Browser         `json:"browser" example:"chrome"`
	Version         BrowserVersion  `json:"version" example:"latest"`
	OperatingSystem OperatingSystem `json:"operating_system" example:"linux"`
	Screen          ScreenConfig    `json:"screen"`
	Headless        bool            `json:"headless" example:"true"`

	MinSize       int `json:"min_size" example:"1"`
	MaxSize       int `json:"max_size" example:"10"`
	CurrentSize   int `json:"current_size" example:"3"`
	AvailableSize int `json:"available_size" example:"2"`

	MaxIdleTime int  `json:"max_idle_time" example:"3600"`
	AutoScale   bool `json:"auto_scale" example:"true"`
	Enabled     bool `json:"enabled" example:"true"`

	CreatedAt  time.Time  `json:"created_at" example:"2023-01-01T00:00:00Z"`
	UpdatedAt  time.Time  `json:"updated_at" example:"2023-01-01T00:00:00Z"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty" example:"2023-01-01T00:30:00Z"`

	ResourceLimits ResourceLimits `json:"resource_limits,omitempty"`
	Environment    datatypes.JSON `json:"environment" swaggertype:"object"`
} //@name SessionPool

func (Pool) TableName() string {
	return "session_pools"
}

func (p *Pool) NeedsScaling() bool {
	return p.AutoScale && p.Enabled && p.AvailableSize < p.MinSize
}

func (p *Pool) CanAcceptMore() bool {
	return p.Enabled && p.CurrentSize < p.MaxSize
}

// SessionListResponse represents a response containing a list of sessions
// @Description Response containing a list of sessions with pagination info
type SessionListResponse struct {
	Sessions []Session `json:"sessions"`
	Total    int       `json:"total" example:"25"`
	Offset   int       `json:"offset" example:"0"`
	Limit    int       `json:"limit" example:"100"`
} //@name SessionListResponse

// SessionEventListResponse represents a response containing a list of session events
// @Description Response containing a list of session events with pagination info
type SessionEventListResponse struct {
	Events []SessionEvent `json:"events"`
	Total  int            `json:"total" example:"15"`
	Offset int            `json:"offset" example:"0"`
	Limit  int            `json:"limit" example:"100"`
} //@name SessionEventListResponse

// ErrorResponse represents an error response
// @Description Standard error response format
type ErrorResponse struct {
	Error string `json:"error" example:"Invalid session ID"`
} //@name ErrorResponse

// MessageResponse represents a simple message response
// @Description Standard message response format
type MessageResponse struct {
	Message string `json:"message" example:"Session termination initiated"`
} //@name MessageResponse
