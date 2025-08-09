package tasks

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSessionStartPayload(t *testing.T) {
	payload := SessionStartPayload{
		SessionID:          uuid.New(),
		WorkPoolID:         uuid.New(),
		MaxSessionDuration: 1800,
		RedisAddr:          "localhost:6379",
		QueueName:          "default",
	}

	data, err := payload.Marshal()
	require.NoError(t, err)
	assert.NotEmpty(t, data)

	var decoded SessionStartPayload
	err = decoded.Unmarshal(data)
	require.NoError(t, err)
	assert.Equal(t, payload.SessionID, decoded.SessionID)
	assert.Equal(t, payload.WorkPoolID, decoded.WorkPoolID)
	assert.Equal(t, payload.MaxSessionDuration, decoded.MaxSessionDuration)
}

func TestNewSessionStartTask(t *testing.T) {
	payload := SessionStartPayload{
		SessionID:  uuid.New(),
		WorkPoolID: uuid.New(),
		RedisAddr:  "localhost:6379",
		QueueName:  "default",
	}

	task, err := NewSessionStartTask(payload)
	require.NoError(t, err)
	assert.NotNil(t, task)
	assert.Equal(t, TypeSessionStart, task.Type())
	assert.NotEmpty(t, task.Payload())
}

func TestSessionStopPayload(t *testing.T) {
	tests := []struct {
		name    string
		payload SessionStopPayload
	}{
		{
			name: "user requested stop",
			payload: SessionStopPayload{
				SessionID: uuid.New(),
				Reason:    "user_requested",
			},
		},
		{
			name: "timeout stop",
			payload: SessionStopPayload{
				SessionID: uuid.New(),
				Reason:    "timeout",
			},
		},
		{
			name: "failed stop",
			payload: SessionStopPayload{
				SessionID: uuid.New(),
				Reason:    "failed",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := tt.payload.Marshal()
			require.NoError(t, err)

			var decoded SessionStopPayload
			err = decoded.Unmarshal(data)
			require.NoError(t, err)
			assert.Equal(t, tt.payload.SessionID, decoded.SessionID)
			assert.Equal(t, tt.payload.Reason, decoded.Reason)
		})
	}
}

func TestGetQueueForTask(t *testing.T) {
	tests := []struct {
		taskType string
		expected string
	}{
		{TypeSessionStop, "critical"},
		{TypeSessionStart, "default"},
		{TypeSessionHealthCheck, "default"},
		{TypeSessionTimeout, "low"},
		{TypePoolScale, "low"},
		{TypeCleanupExpired, "low"},
		{TypeCleanupOrphaned, "low"},
		{"unknown", "default"},
	}

	for _, tt := range tests {
		t.Run(tt.taskType, func(t *testing.T) {
			queue := GetQueueForTask(tt.taskType)
			assert.Equal(t, tt.expected, queue)
		})
	}
}

func TestGetTaskPriority(t *testing.T) {
	tests := []struct {
		taskType string
		expected int
	}{
		{TypeSessionStop, 10},
		{TypeSessionStart, 8},
		{TypeSessionHealthCheck, 5},
		{TypeSessionTimeout, 3},
		{TypePoolScale, 1},
		{TypeCleanupExpired, 1},
		{TypeCleanupOrphaned, 1},
		{"unknown", 5},
	}

	for _, tt := range tests {
		t.Run(tt.taskType, func(t *testing.T) {
			priority := GetTaskPriority(tt.taskType)
			assert.Equal(t, tt.expected, priority)
		})
	}
}

func TestPoolScalePayload(t *testing.T) {
	payload := PoolScalePayload{
		WorkPoolID:      uuid.New(),
		DesiredSessions: 5,
	}

	data, err := payload.Marshal()
	require.NoError(t, err)

	var decoded PoolScalePayload
	err = decoded.Unmarshal(data)
	require.NoError(t, err)
	assert.Equal(t, payload.WorkPoolID, decoded.WorkPoolID)
	assert.Equal(t, payload.DesiredSessions, decoded.DesiredSessions)

	task, err := NewPoolScaleTask(payload)
	require.NoError(t, err)
	assert.Equal(t, TypePoolScale, task.Type())
}

func TestCleanupExpiredPayload(t *testing.T) {
	payload := CleanupExpiredPayload{
		MaxAge: 24,
	}

	data, err := payload.Marshal()
	require.NoError(t, err)

	var decoded CleanupExpiredPayload
	err = decoded.Unmarshal(data)
	require.NoError(t, err)
	assert.Equal(t, payload.MaxAge, decoded.MaxAge)

	task, err := NewCleanupExpiredTask(payload)
	require.NoError(t, err)
	assert.Equal(t, TypeCleanupExpired, task.Type())
}
