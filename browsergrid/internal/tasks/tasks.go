package tasks

import (
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
)

const (
	TypeSessionStart       = "session:start"
	TypeSessionStop        = "session:stop"
	TypeSessionHealthCheck = "session:health_check"
	TypeSessionTimeout     = "session:timeout"
	TypePoolScale          = "pool:scale"
	TypeCleanupExpired     = "cleanup:expired"
	TypeCleanupOrphaned    = "cleanup:orphaned"
)

type SessionStartPayload struct {
	SessionID          uuid.UUID `json:"session_id"`
	WorkPoolID         uuid.UUID `json:"work_pool_id"`
	MaxSessionDuration int       `json:"max_session_duration"`
	RedisAddr          string    `json:"redis_addr"`
	QueueName          string    `json:"queue_name"`
}

func (p *SessionStartPayload) Marshal() ([]byte, error) {
	return json.Marshal(p)
}

func (p *SessionStartPayload) Unmarshal(data []byte) error {
	return json.Unmarshal(data, p)
}

func NewSessionStartTask(payload SessionStartPayload) (*asynq.Task, error) {
	data, err := payload.Marshal()
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}
	return asynq.NewTask(TypeSessionStart, data), nil
}

type SessionStopPayload struct {
	SessionID uuid.UUID `json:"session_id"`
	Reason    string    `json:"reason"`
}

func (p *SessionStopPayload) Marshal() ([]byte, error) {
	return json.Marshal(p)
}

func (p *SessionStopPayload) Unmarshal(data []byte) error {
	return json.Unmarshal(data, p)
}

func NewSessionStopTask(payload SessionStopPayload) (*asynq.Task, error) {
	data, err := payload.Marshal()
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}
	return asynq.NewTask(TypeSessionStop, data), nil
}

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

func NewSessionHealthCheckTask(payload SessionHealthCheckPayload) (*asynq.Task, error) {
	data, err := payload.Marshal()
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}
	return asynq.NewTask(TypeSessionHealthCheck, data), nil
}

type SessionTimeoutPayload struct {
	SessionID uuid.UUID `json:"session_id"`
}

func (p *SessionTimeoutPayload) Marshal() ([]byte, error) {
	return json.Marshal(p)
}

func (p *SessionTimeoutPayload) Unmarshal(data []byte) error {
	return json.Unmarshal(data, p)
}

func NewSessionTimeoutTask(payload SessionTimeoutPayload) (*asynq.Task, error) {
	data, err := payload.Marshal()
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}
	return asynq.NewTask(TypeSessionTimeout, data), nil
}

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

func NewPoolScaleTask(payload PoolScalePayload) (*asynq.Task, error) {
	data, err := payload.Marshal()
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}
	return asynq.NewTask(TypePoolScale, data), nil
}

type CleanupExpiredPayload struct {
	MaxAge int `json:"max_age"`
}

func (p *CleanupExpiredPayload) Marshal() ([]byte, error) {
	return json.Marshal(p)
}

func (p *CleanupExpiredPayload) Unmarshal(data []byte) error {
	return json.Unmarshal(data, p)
}

func NewCleanupExpiredTask(payload CleanupExpiredPayload) (*asynq.Task, error) {
	data, err := payload.Marshal()
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}
	return asynq.NewTask(TypeCleanupExpired, data), nil
}

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
