package tasks

import (
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
)

// Task type names
const (
	TypeSessionStart       = "session:start"
	TypeSessionStop        = "session:stop"
	TypeSessionHealthCheck = "session:health_check"
	TypeSessionTimeout     = "session:timeout"
	TypePoolScale          = "pool:scale"
	TypeCleanupExpired     = "cleanup:expired"
	TypeCleanupOrphaned    = "cleanup:orphaned"
)

// SessionStartPayload is the payload for session start tasks
type SessionStartPayload struct {
	SessionID          uuid.UUID `json:"session_id"`
	WorkPoolID         uuid.UUID `json:"work_pool_id"`
	MaxSessionDuration int       `json:"max_session_duration"` // seconds
	RedisAddr          string    `json:"redis_addr"`
	QueueName          string    `json:"queue_name"`
}

func (p *SessionStartPayload) Marshal() ([]byte, error) {
	return json.Marshal(p)
}

func (p *SessionStartPayload) Unmarshal(data []byte) error {
	return json.Unmarshal(data, p)
}

// NewSessionStartTask creates a new session start task
func NewSessionStartTask(payload SessionStartPayload) (*asynq.Task, error) {
	data, err := payload.Marshal()
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}
	return asynq.NewTask(TypeSessionStart, data), nil
}

// SessionStopPayload is the payload for session stop tasks
type SessionStopPayload struct {
	SessionID uuid.UUID `json:"session_id"`
	Reason    string    `json:"reason"` // "completed", "timeout", "failed", "user_requested"
}

func (p *SessionStopPayload) Marshal() ([]byte, error) {
	return json.Marshal(p)
}

func (p *SessionStopPayload) Unmarshal(data []byte) error {
	return json.Unmarshal(data, p)
}

// NewSessionStopTask creates a new session stop task
func NewSessionStopTask(payload SessionStopPayload) (*asynq.Task, error) {
	data, err := payload.Marshal()
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}
	return asynq.NewTask(TypeSessionStop, data), nil
}

// SessionHealthCheckPayload is the payload for health check tasks
type SessionHealthCheckPayload struct {
	SessionID uuid.UUID `json:"session_id"`
	RedisAddr string    `json:"redis_addr"`
}

func (p *SessionHealthCheckPayload) Marshal() ([]byte, error) {
	return json.Marshal(p)
}

func (p *SessionHealthCheckPayload) Unmarshal(data []byte) error {
	return json.Unmarshal(data, p)
}

// NewSessionHealthCheckTask creates a new health check task
func NewSessionHealthCheckTask(payload SessionHealthCheckPayload) (*asynq.Task, error) {
	data, err := payload.Marshal()
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}
	return asynq.NewTask(TypeSessionHealthCheck, data), nil
}

// SessionTimeoutPayload is the payload for timeout tasks
type SessionTimeoutPayload struct {
	SessionID uuid.UUID `json:"session_id"`
}

func (p *SessionTimeoutPayload) Marshal() ([]byte, error) {
	return json.Marshal(p)
}

func (p *SessionTimeoutPayload) Unmarshal(data []byte) error {
	return json.Unmarshal(data, p)
}

// NewSessionTimeoutTask creates a new timeout task
func NewSessionTimeoutTask(payload SessionTimeoutPayload) (*asynq.Task, error) {
	data, err := payload.Marshal()
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}
	return asynq.NewTask(TypeSessionTimeout, data), nil
}

// PoolScalePayload is the payload for pool scaling tasks
type PoolScalePayload struct {
	WorkPoolID      uuid.UUID `json:"work_pool_id"`
	DesiredSessions int       `json:"desired_sessions"`
}

func (p *PoolScalePayload) Marshal() ([]byte, error) {
	return json.Marshal(p)
}

func (p *PoolScalePayload) Unmarshal(data []byte) error {
	return json.Unmarshal(data, p)
}

// NewPoolScaleTask creates a new pool scale task
func NewPoolScaleTask(payload PoolScalePayload) (*asynq.Task, error) {
	data, err := payload.Marshal()
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}
	return asynq.NewTask(TypePoolScale, data), nil
}

// CleanupExpiredPayload is the payload for cleanup tasks
type CleanupExpiredPayload struct {
	MaxAge int `json:"max_age"` // hours
}

func (p *CleanupExpiredPayload) Marshal() ([]byte, error) {
	return json.Marshal(p)
}

func (p *CleanupExpiredPayload) Unmarshal(data []byte) error {
	return json.Unmarshal(data, p)
}

// NewCleanupExpiredTask creates a new cleanup task
func NewCleanupExpiredTask(payload CleanupExpiredPayload) (*asynq.Task, error) {
	data, err := payload.Marshal()
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}
	return asynq.NewTask(TypeCleanupExpired, data), nil
}

// GetQueueForTask returns the appropriate queue name for a task type
func GetQueueForTask(taskType string) string {
	switch taskType {
	case TypeSessionStop:
		return "critical"
	case TypeSessionStart, TypeSessionHealthCheck:
		return "default"
	case TypeSessionTimeout, TypePoolScale, TypeCleanupExpired, TypeCleanupOrphaned:
		return "low"
	default:
		return "default"
	}
}

// GetTaskPriority returns the priority for a task type (higher = more important)
func GetTaskPriority(taskType string) int {
	switch taskType {
	case TypeSessionStop:
		return 10
	case TypeSessionStart:
		return 8
	case TypeSessionHealthCheck:
		return 5
	case TypeSessionTimeout:
		return 3
	case TypePoolScale, TypeCleanupExpired, TypeCleanupOrphaned:
		return 1
	default:
		return 5
	}
}
