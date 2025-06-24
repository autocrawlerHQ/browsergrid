package workpool

import (
	"testing"
	"time"
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
		name                 string
		pool                 *WorkPool
		currentSessions      int
		pendingSessions      int
		availableWorkerSlots int
		expected             int
	}{
		{
			name: "create sessions to reach minimum",
			pool: &WorkPool{
				MinSize:        10,
				MaxConcurrency: 20,
				AutoScale:      true,
				Paused:         false,
			},
			currentSessions:      3,
			pendingSessions:      2,
			availableWorkerSlots: 10,
			expected:             5,
		},
		{
			name: "limited by worker capacity",
			pool: &WorkPool{
				MinSize:        10,
				MaxConcurrency: 20,
				AutoScale:      true,
				Paused:         false,
			},
			currentSessions:      2,
			pendingSessions:      1,
			availableWorkerSlots: 3,
			expected:             3,
		},
		{
			name: "limited by pool capacity",
			pool: &WorkPool{
				MinSize:        15,
				MaxConcurrency: 10,
				AutoScale:      true,
				Paused:         false,
			},
			currentSessions:      5,
			pendingSessions:      2,
			availableWorkerSlots: 20,
			expected:             3,
		},
		{
			name: "no sessions when already at minimum",
			pool: &WorkPool{
				MinSize:        10,
				MaxConcurrency: 20,
				AutoScale:      true,
				Paused:         false,
			},
			currentSessions:      6,
			pendingSessions:      4,
			availableWorkerSlots: 10,
			expected:             0,
		},
		{
			name: "no sessions when paused",
			pool: &WorkPool{
				MinSize:        10,
				MaxConcurrency: 20,
				AutoScale:      true,
				Paused:         true,
			},
			currentSessions:      2,
			pendingSessions:      1,
			availableWorkerSlots: 10,
			expected:             0,
		},
		{
			name: "no sessions when autoscale disabled",
			pool: &WorkPool{
				MinSize:        10,
				MaxConcurrency: 20,
				AutoScale:      false,
				Paused:         false,
			},
			currentSessions:      2,
			pendingSessions:      1,
			availableWorkerSlots: 10,
			expected:             0,
		},
		{
			name: "no worker slots available",
			pool: &WorkPool{
				MinSize:        10,
				MaxConcurrency: 20,
				AutoScale:      true,
				Paused:         false,
			},
			currentSessions:      2,
			pendingSessions:      1,
			availableWorkerSlots: 0,
			expected:             0,
		},
		{
			name: "edge case: negative calculation",
			pool: &WorkPool{
				MinSize:        5,
				MaxConcurrency: 3,
				AutoScale:      true,
				Paused:         false,
			},
			currentSessions:      2,
			pendingSessions:      2,
			availableWorkerSlots: 10,
			expected:             0,
		},
		{
			name: "edge case: zero minimum",
			pool: &WorkPool{
				MinSize:        0,
				MaxConcurrency: 10,
				AutoScale:      true,
				Paused:         false,
			},
			currentSessions:      0,
			pendingSessions:      0,
			availableWorkerSlots: 5,
			expected:             0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.pool.SessionsToCreate(tt.currentSessions, tt.pendingSessions, tt.availableWorkerSlots)
			if result != tt.expected {
				t.Errorf("SessionsToCreate() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestWorker_IsOnline(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name     string
		worker   *Worker
		ttl      time.Duration
		expected bool
	}{
		{
			name: "online with recent heartbeat",
			worker: &Worker{
				LastBeat: now.Add(-1 * time.Minute),
			},
			ttl:      5 * time.Minute,
			expected: true,
		},
		{
			name: "online at exact TTL boundary",
			worker: &Worker{
				LastBeat: now.Add(-5 * time.Minute),
			},
			ttl:      5 * time.Minute,
			expected: true,
		},
		{
			name: "offline past TTL",
			worker: &Worker{
				LastBeat: now.Add(-6 * time.Minute),
			},
			ttl:      5 * time.Minute,
			expected: false,
		},
		{
			name: "offline with very old heartbeat",
			worker: &Worker{
				LastBeat: now.Add(-1 * time.Hour),
			},
			ttl:      5 * time.Minute,
			expected: false,
		},
		{
			name: "online with zero TTL",
			worker: &Worker{
				LastBeat: now.Add(1 * time.Nanosecond),
			},
			ttl:      0,
			expected: true,
		},
		{
			name: "offline with zero TTL and old heartbeat",
			worker: &Worker{
				LastBeat: now.Add(-1 * time.Second),
			},
			ttl:      0,
			expected: false,
		},
		{
			name: "online with future heartbeat",
			worker: &Worker{
				LastBeat: now.Add(1 * time.Minute),
			},
			ttl:      5 * time.Minute,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.worker.IsOnline(tt.ttl)
			if result != tt.expected {
				t.Errorf("IsOnline() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestWorker_HasCapacity(t *testing.T) {
	tests := []struct {
		name     string
		worker   *Worker
		expected bool
	}{
		{
			name: "has capacity when under max slots",
			worker: &Worker{
				MaxSlots: 5,
				Active:   3,
				Paused:   false,
			},
			expected: true,
		},
		{
			name: "has capacity with zero active",
			worker: &Worker{
				MaxSlots: 5,
				Active:   0,
				Paused:   false,
			},
			expected: true,
		},
		{
			name: "no capacity when at max slots",
			worker: &Worker{
				MaxSlots: 5,
				Active:   5,
				Paused:   false,
			},
			expected: false,
		},
		{
			name: "no capacity when over max slots",
			worker: &Worker{
				MaxSlots: 5,
				Active:   7,
				Paused:   false,
			},
			expected: false,
		},
		{
			name: "no capacity when paused",
			worker: &Worker{
				MaxSlots: 5,
				Active:   2,
				Paused:   true,
			},
			expected: false,
		},
		{
			name: "no capacity when paused and at zero active",
			worker: &Worker{
				MaxSlots: 5,
				Active:   0,
				Paused:   true,
			},
			expected: false,
		},
		{
			name: "edge case: zero max slots",
			worker: &Worker{
				MaxSlots: 0,
				Active:   0,
				Paused:   false,
			},
			expected: false,
		},
		{
			name: "edge case: negative active",
			worker: &Worker{
				MaxSlots: 5,
				Active:   -1,
				Paused:   false,
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.worker.HasCapacity()
			if result != tt.expected {
				t.Errorf("HasCapacity() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestWorker_AvailableSlots(t *testing.T) {
	tests := []struct {
		name     string
		worker   *Worker
		expected int
	}{
		{
			name: "worker not paused with available slots",
			worker: &Worker{
				MaxSlots: 5,
				Active:   2,
				Paused:   false,
			},
			expected: 3,
		},
		{
			name: "worker paused",
			worker: &Worker{
				MaxSlots: 5,
				Active:   2,
				Paused:   true,
			},
			expected: 0,
		},
		{
			name: "worker at max capacity",
			worker: &Worker{
				MaxSlots: 3,
				Active:   3,
				Paused:   false,
			},
			expected: 0,
		},
		{
			name: "worker over capacity",
			worker: &Worker{
				MaxSlots: 3,
				Active:   5,
				Paused:   false,
			},
			expected: -2,
		},
		{
			name: "worker with negative active",
			worker: &Worker{
				MaxSlots: 5,
				Active:   -2,
				Paused:   false,
			},
			expected: 7,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.worker.AvailableSlots()
			if result != tt.expected {
				t.Errorf("AvailableSlots() = %v, expected %v", result, tt.expected)
			}
		})
	}
}
