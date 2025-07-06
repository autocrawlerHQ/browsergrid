package workpool

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWorkPool_NeedsScaling(t *testing.T) {
	tests := []struct {
		name            string
		pool            *WorkPool
		currentSessions int
		pendingSessions int
		expected        bool
	}{
		{
			name: "needs scaling when below minimum",
			pool: &WorkPool{
				MinSize:   5,
				AutoScale: true,
				Paused:    false,
			},
			currentSessions: 2,
			pendingSessions: 1,
			expected:        true,
		},
		{
			name: "no scaling when at minimum",
			pool: &WorkPool{
				MinSize:   5,
				AutoScale: true,
				Paused:    false,
			},
			currentSessions: 3,
			pendingSessions: 2,
			expected:        false,
		},
		{
			name: "no scaling when above minimum",
			pool: &WorkPool{
				MinSize:   5,
				AutoScale: true,
				Paused:    false,
			},
			currentSessions: 4,
			pendingSessions: 3,
			expected:        false,
		},
		{
			name: "no scaling when paused",
			pool: &WorkPool{
				MinSize:   5,
				AutoScale: true,
				Paused:    true,
			},
			currentSessions: 1,
			pendingSessions: 0,
			expected:        false,
		},
		{
			name: "no scaling when autoscale disabled",
			pool: &WorkPool{
				MinSize:   5,
				AutoScale: false,
				Paused:    false,
			},
			currentSessions: 1,
			pendingSessions: 0,
			expected:        false,
		},
		{
			name: "no scaling when both paused and autoscale disabled",
			pool: &WorkPool{
				MinSize:   5,
				AutoScale: false,
				Paused:    true,
			},
			currentSessions: 1,
			pendingSessions: 0,
			expected:        false,
		},
		{
			name: "edge case: zero minimum size",
			pool: &WorkPool{
				MinSize:   0,
				AutoScale: true,
				Paused:    false,
			},
			currentSessions: 0,
			pendingSessions: 0,
			expected:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.pool.NeedsScaling(tt.currentSessions, tt.pendingSessions)
			if result != tt.expected {
				t.Errorf("NeedsScaling() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestWorkPool_CanAcceptMore(t *testing.T) {
	tests := []struct {
		name            string
		pool            *WorkPool
		currentSessions int
		expected        bool
	}{
		{
			name: "can accept when under capacity",
			pool: &WorkPool{
				MaxConcurrency: 10,
				Paused:         false,
			},
			currentSessions: 5,
			expected:        true,
		},
		{
			name: "can accept when at zero",
			pool: &WorkPool{
				MaxConcurrency: 10,
				Paused:         false,
			},
			currentSessions: 0,
			expected:        true,
		},
		{
			name: "cannot accept when at capacity",
			pool: &WorkPool{
				MaxConcurrency: 10,
				Paused:         false,
			},
			currentSessions: 10,
			expected:        false,
		},
		{
			name: "cannot accept when over capacity",
			pool: &WorkPool{
				MaxConcurrency: 10,
				Paused:         false,
			},
			currentSessions: 15,
			expected:        false,
		},
		{
			name: "cannot accept when paused",
			pool: &WorkPool{
				MaxConcurrency: 10,
				Paused:         true,
			},
			currentSessions: 5,
			expected:        false,
		},
		{
			name: "edge case: zero capacity",
			pool: &WorkPool{
				MaxConcurrency: 0,
				Paused:         false,
			},
			currentSessions: 0,
			expected:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.pool.CanAcceptMore(tt.currentSessions)
			if result != tt.expected {
				t.Errorf("CanAcceptMore() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestWorkPool_SessionsToCreate(t *testing.T) {
	tests := []struct {
		name             string
		pool             WorkPool
		currentSessions  int
		pendingSessions  int
		expectedSessions int
	}{
		{
			name: "needs scaling - basic case",
			pool: WorkPool{
				MinSize:        5,
				MaxConcurrency: 10,
				AutoScale:      true,
				Paused:         false,
			},
			currentSessions:  2,
			pendingSessions:  0,
			expectedSessions: 3,
		},
		{
			name: "needs scaling - limited by max concurrency",
			pool: WorkPool{
				MinSize:        10,
				MaxConcurrency: 5,
				AutoScale:      true,
				Paused:         false,
			},
			currentSessions:  2,
			pendingSessions:  0,
			expectedSessions: 3,
		},
		{
			name: "needs scaling - has pending sessions",
			pool: WorkPool{
				MinSize:        10,
				MaxConcurrency: 20,
				AutoScale:      true,
				Paused:         false,
			},
			currentSessions:  2,
			pendingSessions:  3,
			expectedSessions: 5,
		},
		{
			name: "no scaling needed - at minimum",
			pool: WorkPool{
				MinSize:        5,
				MaxConcurrency: 10,
				AutoScale:      true,
				Paused:         false,
			},
			currentSessions:  5,
			pendingSessions:  0,
			expectedSessions: 0,
		},
		{
			name: "no scaling needed - above minimum",
			pool: WorkPool{
				MinSize:        5,
				MaxConcurrency: 10,
				AutoScale:      true,
				Paused:         false,
			},
			currentSessions:  7,
			pendingSessions:  0,
			expectedSessions: 0,
		},
		{
			name: "no scaling needed - at max concurrency",
			pool: WorkPool{
				MinSize:        5,
				MaxConcurrency: 10,
				AutoScale:      true,
				Paused:         false,
			},
			currentSessions:  10,
			pendingSessions:  0,
			expectedSessions: 0,
		},
		{
			name: "no scaling - paused",
			pool: WorkPool{
				MinSize:        5,
				MaxConcurrency: 10,
				AutoScale:      true,
				Paused:         true,
			},
			currentSessions:  2,
			pendingSessions:  0,
			expectedSessions: 0,
		},
		{
			name: "no scaling - auto scale disabled",
			pool: WorkPool{
				MinSize:        5,
				MaxConcurrency: 10,
				AutoScale:      false,
				Paused:         false,
			},
			currentSessions:  2,
			pendingSessions:  0,
			expectedSessions: 0,
		},
		{
			name: "partial scaling - some pending",
			pool: WorkPool{
				MinSize:        10,
				MaxConcurrency: 15,
				AutoScale:      true,
				Paused:         false,
			},
			currentSessions:  3,
			pendingSessions:  2,
			expectedSessions: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.pool.SessionsToCreate(tt.currentSessions, tt.pendingSessions)
			assert.Equal(t, tt.expectedSessions, result)
		})
	}
}
