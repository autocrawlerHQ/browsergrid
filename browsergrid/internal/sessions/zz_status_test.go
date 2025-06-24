package sessions

import (
	"testing"
)

func TestStatusFromEvent(t *testing.T) {
	tests := []struct {
		event     SessionEventType
		expected  SessionStatus
		hasStatus bool
	}{
		{EvtSessionCreated, StatusPending, true},
		{EvtResourceAllocated, StatusPending, true},
		{EvtSessionStarting, StatusStarting, true},
		{EvtContainerStarted, StatusStarting, true},
		{EvtBrowserStarted, StatusStarting, true},
		{EvtSessionAvailable, StatusAvailable, true},
		{EvtPoolAdded, StatusAvailable, true},
		{EvtSessionClaimed, StatusClaimed, true},
		{EvtSessionAssigned, StatusRunning, true},
		{EvtSessionReady, StatusRunning, true},
		{EvtSessionActive, StatusRunning, true},
		{EvtSessionIdle, StatusIdle, true},
		{EvtSessionCompleted, StatusCompleted, true},
		{EvtSessionExpired, StatusExpired, true},
		{EvtSessionTimedOut, StatusTimedOut, true},
		{EvtSessionTerminated, StatusTerminated, true},
		{EvtStartupFailed, StatusFailed, true},
		{EvtResourceExhausted, StatusFailed, true},
		{EvtNetworkError, StatusFailed, true},
		{EvtBrowserCrashed, StatusCrashed, true},
		{EvtContainerCrashed, StatusCrashed, true},
		{EvtHeartbeat, "", false},
		{EvtStatusChanged, "", false},
		{EvtConfigUpdated, "", false},
		{EvtHealthCheck, "", false},
		{EvtPoolRemoved, "", false},
		{EvtPoolDrained, "", false},
	}

	for _, tt := range tests {
		t.Run(string(tt.event), func(t *testing.T) {
			status, hasStatus := statusFromEvent(tt.event)
			if hasStatus != tt.hasStatus {
				t.Errorf("statusFromEvent(%v) hasStatus = %v, want %v", tt.event, hasStatus, tt.hasStatus)
			}
			if status != tt.expected {
				t.Errorf("statusFromEvent(%v) status = %v, want %v", tt.event, status, tt.expected)
			}
		})
	}
}

func TestShouldUpdateStatus(t *testing.T) {
	tests := []struct {
		current  SessionStatus
		next     SessionStatus
		expected bool
	}{
		{StatusPending, StatusStarting, true},
		{StatusStarting, StatusAvailable, true},
		{StatusAvailable, StatusClaimed, true},
		{StatusClaimed, StatusRunning, true},
		{StatusRunning, StatusCompleted, true},
		{StatusRunning, StatusIdle, true},
		{StatusIdle, StatusRunning, true},
		{StatusAvailable, StatusClaimed, true},
		{StatusClaimed, StatusRunning, true},
		{StatusAvailable, StatusTerminated, true},
		{StatusAvailable, StatusExpired, true},
		{StatusPending, StatusRunning, true},
		{StatusStarting, StatusCompleted, true},
		{StatusRunning, StatusStarting, false},
		{StatusCompleted, StatusRunning, false},
		{StatusClaimed, StatusAvailable, false},
		{StatusStarting, StatusPending, false},
		{StatusCompleted, StatusRunning, false},
		{StatusFailed, StatusStarting, false},
		{StatusCrashed, StatusRunning, false},
		{StatusRunning, StatusRunning, false},
		{StatusPending, StatusPending, false},
		{StatusAvailable, StatusAvailable, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.current)+"->"+string(tt.next), func(t *testing.T) {
			result := shouldUpdateStatus(tt.current, tt.next)
			if result != tt.expected {
				t.Errorf("shouldUpdateStatus(%v, %v) = %v, want %v",
					tt.current, tt.next, result, tt.expected)
			}
		})
	}
}

func TestIsTerminalStatus(t *testing.T) {
	tests := []struct {
		status   SessionStatus
		expected bool
	}{
		{StatusPending, false},
		{StatusStarting, false},
		{StatusAvailable, false},
		{StatusClaimed, false},
		{StatusRunning, false},
		{StatusIdle, false},
		{StatusCompleted, true},
		{StatusFailed, true},
		{StatusExpired, true},
		{StatusCrashed, true},
		{StatusTimedOut, true},
		{StatusTerminated, true},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			result := IsTerminalStatus(tt.status)
			if result != tt.expected {
				t.Errorf("IsTerminalStatus(%v) = %v, want %v", tt.status, result, tt.expected)
			}
		})
	}
}

func TestIsPooledStatus(t *testing.T) {
	tests := []struct {
		status   SessionStatus
		expected bool
	}{
		{StatusAvailable, true},
		{StatusPending, false},
		{StatusClaimed, false},
		{StatusRunning, false},
		{StatusCompleted, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			result := IsPooledStatus(tt.status)
			if result != tt.expected {
				t.Errorf("IsPooledStatus(%v) = %v, want %v", tt.status, result, tt.expected)
			}
		})
	}
}

func TestIsClaimableStatus(t *testing.T) {
	tests := []struct {
		status   SessionStatus
		expected bool
	}{
		{StatusAvailable, true},
		{StatusPending, false},
		{StatusClaimed, false},
		{StatusRunning, false},
		{StatusCompleted, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			result := IsClaimableStatus(tt.status)
			if result != tt.expected {
				t.Errorf("IsClaimableStatus(%v) = %v, want %v", tt.status, result, tt.expected)
			}
		})
	}
}

func TestCanTransitionTo(t *testing.T) {
	tests := []struct {
		current  SessionStatus
		target   SessionStatus
		expected bool
	}{
		{StatusPending, StatusStarting, true},
		{StatusStarting, StatusAvailable, true},
		{StatusAvailable, StatusClaimed, true},
		{StatusClaimed, StatusRunning, true},
		{StatusRunning, StatusIdle, true},
		{StatusIdle, StatusRunning, true},
		{StatusRunning, StatusCompleted, true},
		{StatusAvailable, StatusTerminated, true},
		{StatusAvailable, StatusExpired, true},
		{StatusCompleted, StatusRunning, false},
		{StatusFailed, StatusStarting, false},
		{StatusCrashed, StatusRunning, false},
		{StatusRunning, StatusStarting, false},
		{StatusClaimed, StatusAvailable, false},
		{StatusStarting, StatusPending, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.current)+"->"+string(tt.target), func(t *testing.T) {
			result := CanTransitionTo(tt.current, tt.target)
			if result != tt.expected {
				t.Errorf("CanTransitionTo(%v, %v) = %v, want %v",
					tt.current, tt.target, result, tt.expected)
			}
		})
	}
}

func TestGetValidTransitions(t *testing.T) {
	tests := []struct {
		current    SessionStatus
		hasTargets bool
	}{
		{StatusPending, true},
		{StatusStarting, true},
		{StatusAvailable, true},
		{StatusClaimed, true},
		{StatusRunning, true},
		{StatusIdle, true},
		{StatusCompleted, false},
		{StatusFailed, false},
		{StatusExpired, false},
		{StatusCrashed, false},
		{StatusTimedOut, false},
		{StatusTerminated, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.current), func(t *testing.T) {
			transitions := GetValidTransitions(tt.current)
			hasTargets := len(transitions) > 0
			if hasTargets != tt.hasTargets {
				t.Errorf("GetValidTransitions(%v) hasTargets = %v, want %v (got %d transitions)",
					tt.current, hasTargets, tt.hasTargets, len(transitions))
			}
		})
	}
}

func TestGetPoolTransitions(t *testing.T) {
	tests := []struct {
		current         SessionStatus
		expectedTargets []SessionStatus
	}{
		{StatusAvailable, []SessionStatus{StatusClaimed, StatusTerminated, StatusExpired}},
		{StatusClaimed, []SessionStatus{StatusRunning, StatusTerminated}},
		{StatusPending, []SessionStatus{}},
		{StatusRunning, []SessionStatus{}},
		{StatusCompleted, []SessionStatus{}},
	}

	for _, tt := range tests {
		t.Run(string(tt.current), func(t *testing.T) {
			transitions := GetPoolTransitions(tt.current)
			if len(transitions) != len(tt.expectedTargets) {
				t.Errorf("GetPoolTransitions(%v) returned %d transitions, want %d",
					tt.current, len(transitions), len(tt.expectedTargets))
				return
			}

			for _, expected := range tt.expectedTargets {
				found := false
				for _, actual := range transitions {
					if actual == expected {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("GetPoolTransitions(%v) missing expected transition to %v",
						tt.current, expected)
				}
			}
		})
	}
}
