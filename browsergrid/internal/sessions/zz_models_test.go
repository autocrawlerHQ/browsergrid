package sessions

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestSession_IsInPool(t *testing.T) {
	tests := []struct {
		name     string
		session  Session
		expected bool
	}{
		{
			name: "pooled available session with nil claimed_by",
			session: Session{
				IsPooled:  true,
				Status:    StatusAvailable,
				ClaimedBy: nil,
			},
			expected: true,
		},
		{
			name: "pooled available session with empty claimed_by",
			session: Session{
				IsPooled:  true,
				Status:    StatusAvailable,
				ClaimedBy: stringPtr(""),
			},
			expected: false,
		},
		{
			name: "pooled claimed session",
			session: Session{
				IsPooled:  true,
				Status:    StatusAvailable,
				ClaimedBy: stringPtr("user123"),
			},
			expected: false,
		},
		{
			name: "non-pooled available session",
			session: Session{
				IsPooled:  false,
				Status:    StatusAvailable,
				ClaimedBy: nil,
			},
			expected: false,
		},
		{
			name: "pooled non-available session",
			session: Session{
				IsPooled:  true,
				Status:    StatusRunning,
				ClaimedBy: nil,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.session.IsInPool()
			if result != tt.expected {
				t.Errorf("IsInPool() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestSession_IsClaimed(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name     string
		session  Session
		expected bool
	}{
		{
			name: "claimed session with both fields set",
			session: Session{
				ClaimedBy: stringPtr("user123"),
				ClaimedAt: &now,
			},
			expected: true,
		},
		{
			name: "session with claimed_by but no claimed_at",
			session: Session{
				ClaimedBy: stringPtr("user123"),
				ClaimedAt: nil,
			},
			expected: false,
		},
		{
			name: "session with claimed_at but no claimed_by",
			session: Session{
				ClaimedBy: nil,
				ClaimedAt: &now,
			},
			expected: false,
		},
		{
			name: "unclaimed session",
			session: Session{
				ClaimedBy: nil,
				ClaimedAt: nil,
			},
			expected: false,
		},
		{
			name: "session with empty claimed_by string",
			session: Session{
				ClaimedBy: stringPtr(""),
				ClaimedAt: &now,
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.session.IsClaimed()
			if result != tt.expected {
				t.Errorf("IsClaimed() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestSession_CanBeClaimed(t *testing.T) {
	tests := []struct {
		name     string
		session  Session
		expected bool
	}{
		{
			name: "available pooled session",
			session: Session{
				IsPooled:  true,
				Status:    StatusAvailable,
				ClaimedBy: nil,
			},
			expected: true,
		},
		{
			name: "claimed pooled session",
			session: Session{
				IsPooled:  true,
				Status:    StatusAvailable,
				ClaimedBy: stringPtr("user123"),
			},
			expected: false,
		},
		{
			name: "non-pooled session",
			session: Session{
				IsPooled:  false,
				Status:    StatusAvailable,
				ClaimedBy: nil,
			},
			expected: false,
		},
		{
			name: "pooled running session",
			session: Session{
				IsPooled:  true,
				Status:    StatusRunning,
				ClaimedBy: nil,
			},
			expected: false,
		},
		{
			name: "pooled pending session",
			session: Session{
				IsPooled:  true,
				Status:    StatusPending,
				ClaimedBy: nil,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.session.CanBeClaimed()
			if result != tt.expected {
				t.Errorf("CanBeClaimed() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestPool_NeedsScaling(t *testing.T) {
	tests := []struct {
		name     string
		pool     Pool
		expected bool
	}{
		{
			name: "needs scaling - available less than min",
			pool: Pool{
				AutoScale:     true,
				Enabled:       true,
				MinSize:       3,
				AvailableSize: 1,
			},
			expected: true,
		},
		{
			name: "no scaling needed - available equals min",
			pool: Pool{
				AutoScale:     true,
				Enabled:       true,
				MinSize:       3,
				AvailableSize: 3,
			},
			expected: false,
		},
		{
			name: "no scaling needed - available greater than min",
			pool: Pool{
				AutoScale:     true,
				Enabled:       true,
				MinSize:       3,
				AvailableSize: 5,
			},
			expected: false,
		},
		{
			name: "auto scale disabled",
			pool: Pool{
				AutoScale:     false,
				Enabled:       true,
				MinSize:       3,
				AvailableSize: 1,
			},
			expected: false,
		},
		{
			name: "pool disabled",
			pool: Pool{
				AutoScale:     true,
				Enabled:       false,
				MinSize:       3,
				AvailableSize: 1,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.pool.NeedsScaling()
			if result != tt.expected {
				t.Errorf("NeedsScaling() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestPool_CanAcceptMore(t *testing.T) {
	tests := []struct {
		name     string
		pool     Pool
		expected bool
	}{
		{
			name: "can accept more - current less than max",
			pool: Pool{
				Enabled:     true,
				MaxSize:     10,
				CurrentSize: 5,
			},
			expected: true,
		},
		{
			name: "cannot accept more - current equals max",
			pool: Pool{
				Enabled:     true,
				MaxSize:     10,
				CurrentSize: 10,
			},
			expected: false,
		},
		{
			name: "cannot accept more - current greater than max",
			pool: Pool{
				Enabled:     true,
				MaxSize:     10,
				CurrentSize: 15,
			},
			expected: false,
		},
		{
			name: "pool disabled",
			pool: Pool{
				Enabled:     false,
				MaxSize:     10,
				CurrentSize: 5,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.pool.CanAcceptMore()
			if result != tt.expected {
				t.Errorf("CanAcceptMore() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestSessionTableName(t *testing.T) {
	session := Session{}
	expected := "sessions"
	result := session.TableName()
	if result != expected {
		t.Errorf("TableName() = %v, want %v", result, expected)
	}
}

func TestSessionEventTableName(t *testing.T) {
	event := SessionEvent{}
	expected := "session_events"
	result := event.TableName()
	if result != expected {
		t.Errorf("TableName() = %v, want %v", result, expected)
	}
}

func TestSessionMetricsTableName(t *testing.T) {
	metrics := SessionMetrics{}
	expected := "session_metrics"
	result := metrics.TableName()
	if result != expected {
		t.Errorf("TableName() = %v, want %v", result, expected)
	}
}

func TestPoolTableName(t *testing.T) {
	pool := Pool{}
	expected := "session_pools"
	result := pool.TableName()
	if result != expected {
		t.Errorf("TableName() = %v, want %v", result, expected)
	}
}

func TestSessionValidation(t *testing.T) {
	tests := []struct {
		name    string
		session Session
		valid   bool
	}{
		{
			name: "valid session",
			session: Session{
				ID:              uuid.New(),
				Browser:         BrowserChrome,
				Version:         VerLatest,
				OperatingSystem: OSLinux,
				Screen: ScreenConfig{
					Width:  1920,
					Height: 1080,
					DPI:    96,
					Scale:  1.0,
				},
				Status: StatusPending,
			},
			valid: true,
		},
		{
			name: "session with proxy config",
			session: Session{
				ID:              uuid.New(),
				Browser:         BrowserFirefox,
				Version:         VerStable,
				OperatingSystem: OSWindows,
				Screen: ScreenConfig{
					Width:  1280,
					Height: 720,
					DPI:    96,
					Scale:  1.0,
				},
				Proxy: ProxyConfig{
					URL:      "http://proxy.example.com:8080",
					Username: "user",
					Password: "pass",
				},
				Status: StatusPending,
			},
			valid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.session.ID == uuid.Nil && tt.valid {
				t.Error("Expected valid session to have non-nil UUID")
			}
			if tt.session.Browser == "" && tt.valid {
				t.Error("Expected valid session to have browser specified")
			}
			if tt.session.Screen.Width <= 0 && tt.valid {
				t.Error("Expected valid session to have positive screen width")
			}
		})
	}
}

func stringPtr(s string) *string {
	return &s
}
