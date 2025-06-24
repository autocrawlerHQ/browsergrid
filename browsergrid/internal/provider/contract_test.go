//go:build provider

package provider

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"

	"github.com/autocrawlerHQ/browsergrid/internal/sessions"
	"github.com/autocrawlerHQ/browsergrid/internal/workpool"
)

type MockProvisioner struct {
	providerType workpool.ProviderType

	startError   error
	stopError    error
	healthError  error
	metricsError error
	startWSURL   string
	startLiveURL string
	metrics      *sessions.SessionMetrics

	startCalled   bool
	stopCalled    bool
	healthCalled  bool
	metricsCalled bool
	lastSession   *sessions.Session
}

func NewMockProvisioner(providerType workpool.ProviderType) *MockProvisioner {
	return &MockProvisioner{
		providerType: providerType,
		startWSURL:   "ws://localhost:8080",
		startLiveURL: "http://localhost:8080",
		metrics: &sessions.SessionMetrics{
			ID:             uuid.New(),
			SessionID:      uuid.New(),
			Timestamp:      time.Now(),
			CPUPercent:     floatPtr(25.5),
			MemoryMB:       floatPtr(512.0),
			NetworkRXBytes: int64Ptr(1024),
			NetworkTXBytes: int64Ptr(2048),
		},
	}
}

func (m *MockProvisioner) Start(ctx context.Context, sess *sessions.Session) (wsURL, liveURL string, err error) {
	m.startCalled = true
	m.lastSession = sess
	if m.startError != nil {
		return "", "", m.startError
	}
	return m.startWSURL, m.startLiveURL, nil
}

func (m *MockProvisioner) Stop(ctx context.Context, sess *sessions.Session) error {
	m.stopCalled = true
	m.lastSession = sess
	return m.stopError
}

func (m *MockProvisioner) HealthCheck(ctx context.Context, sess *sessions.Session) error {
	m.healthCalled = true
	m.lastSession = sess
	return m.healthError
}

func (m *MockProvisioner) GetMetrics(ctx context.Context, sess *sessions.Session) (*sessions.SessionMetrics, error) {
	m.metricsCalled = true
	m.lastSession = sess
	if m.metricsError != nil {
		return nil, m.metricsError
	}
	if sess != nil {
		m.metrics.SessionID = sess.ID
	}
	return m.metrics, nil
}

func (m *MockProvisioner) GetType() workpool.ProviderType {
	return m.providerType
}

func floatPtr(f float64) *float64 { return &f }
func int64Ptr(i int64) *int64     { return &i }

func createTestSession() *sessions.Session {
	return &sessions.Session{
		ID:              uuid.New(),
		Browser:         sessions.BrowserChrome,
		Version:         sessions.VerLatest,
		Headless:        true,
		OperatingSystem: sessions.OSLinux,
		Screen: sessions.ScreenConfig{
			Width:  1920,
			Height: 1080,
			DPI:    96,
			Scale:  1.0,
		},
		Status:      sessions.StatusPending,
		Environment: datatypes.JSON(`{}`),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
}

func TestProvisionerContract(t *testing.T) {
	providerTypes := []workpool.ProviderType{
		workpool.ProviderDocker,
		workpool.ProviderACI,
		workpool.ProviderLocal,
	}

	for _, providerType := range providerTypes {
		t.Run(string(providerType), func(t *testing.T) {
			mock := NewMockProvisioner(providerType)
			testProvisionerInterface(t, mock)
		})
	}
}

func TestRegisteredProvidersContract(t *testing.T) {
	registeredTypes := DefaultFactory.GetRegisteredTypes()

	if len(registeredTypes) == 0 {
		t.Skip("No providers registered in DefaultFactory - skipping registered provider contract tests")
	}

	for _, providerType := range registeredTypes {
		t.Run("registered_"+string(providerType), func(t *testing.T) {
			p, ok := DefaultFactory.Get(providerType)
			require.True(t, ok, "Should be able to get registered provider")
			require.NotNil(t, p, "Registered provider should not be nil")

			assert.Equal(t, providerType, p.GetType(), "Provider should return correct type")

			ctx := context.Background()
			sess := createTestSession()

			assert.NotPanics(t, func() {
				_, _, _ = p.Start(ctx, sess)
			}, "Start should not panic")

			assert.NotPanics(t, func() {
				_ = p.Stop(ctx, sess)
			}, "Stop should not panic")

			assert.NotPanics(t, func() {
				_ = p.HealthCheck(ctx, sess)
			}, "HealthCheck should not panic")

			assert.NotPanics(t, func() {
				_, _ = p.GetMetrics(ctx, sess)
			}, "GetMetrics should not panic")
		})
	}
}

func testProvisionerInterface(t *testing.T, p Provisioner) {
	ctx := context.Background()
	sess := createTestSession()

	t.Run("GetType", func(t *testing.T) {
		providerType := p.GetType()
		assert.NotEmpty(t, providerType, "GetType should return non-empty provider type")

		validTypes := []workpool.ProviderType{
			workpool.ProviderDocker,
			workpool.ProviderACI,
			workpool.ProviderLocal,
		}
		assert.Contains(t, validTypes, providerType, "GetType should return a valid provider type")
	})

	t.Run("Start_Success", func(t *testing.T) {
		wsURL, liveURL, err := p.Start(ctx, sess)

		require.NoError(t, err, "Start should not return error on success")
		assert.NotEmpty(t, wsURL, "Start should return non-empty WebSocket URL")
		assert.NotEmpty(t, liveURL, "Start should return non-empty Live URL")

		assert.Contains(t, wsURL, "://", "WebSocket URL should contain protocol")
		assert.Contains(t, liveURL, "://", "Live URL should contain protocol")

		if mock, ok := p.(*MockProvisioner); ok {
			assert.True(t, mock.startCalled, "Start method should be called")
			assert.Equal(t, sess, mock.lastSession, "Start should receive the correct session")
		}
	})

	t.Run("Start_Error", func(t *testing.T) {
		if mock, ok := p.(*MockProvisioner); ok {
			mock.startError = errors.New("mock start error")

			wsURL, liveURL, err := p.Start(ctx, sess)

			assert.Error(t, err, "Start should return error when configured")
			assert.Empty(t, wsURL, "Start should return empty WebSocket URL on error")
			assert.Empty(t, liveURL, "Start should return empty Live URL on error")
		}
	})

	t.Run("Stop_Success", func(t *testing.T) {
		err := p.Stop(ctx, sess)

		assert.NoError(t, err, "Stop should not return error on success")

		if mock, ok := p.(*MockProvisioner); ok {
			assert.True(t, mock.stopCalled, "Stop method should be called")
			assert.Equal(t, sess, mock.lastSession, "Stop should receive the correct session")
		}
	})

	t.Run("Stop_Idempotent", func(t *testing.T) {
		err1 := p.Stop(ctx, sess)
		err2 := p.Stop(ctx, sess)

		assert.NoError(t, err1, "First Stop call should not return error")
		assert.NoError(t, err2, "Second Stop call should not return error (idempotent)")
	})

	t.Run("HealthCheck_Success", func(t *testing.T) {
		sessWithEndpoints := createTestSession()
		wsURL := "ws://localhost:8080"
		liveURL := "http://localhost:8080"
		sessWithEndpoints.WSEndpoint = &wsURL
		sessWithEndpoints.LiveURL = &liveURL

		err := p.HealthCheck(ctx, sessWithEndpoints)

		assert.NoError(t, err, "HealthCheck should not return error when healthy")

		if mock, ok := p.(*MockProvisioner); ok {
			assert.True(t, mock.healthCalled, "HealthCheck method should be called")
			assert.Equal(t, sessWithEndpoints, mock.lastSession, "HealthCheck should receive the correct session")
		}
	})

	t.Run("HealthCheck_Error", func(t *testing.T) {
		if mock, ok := p.(*MockProvisioner); ok {
			mock.healthError = errors.New("mock health error")

			err := p.HealthCheck(ctx, sess)

			assert.Error(t, err, "HealthCheck should return error when configured")
		}
	})

	t.Run("GetMetrics_Success", func(t *testing.T) {
		metrics, err := p.GetMetrics(ctx, sess)

		require.NoError(t, err, "GetMetrics should not return error on success")
		require.NotNil(t, metrics, "GetMetrics should return non-nil metrics")

		assert.NotEqual(t, uuid.Nil, metrics.ID, "Metrics should have valid ID")
		assert.Equal(t, sess.ID, metrics.SessionID, "Metrics should reference correct session")
		assert.NotZero(t, metrics.Timestamp, "Metrics should have timestamp")

		if metrics.CPUPercent != nil {
			assert.GreaterOrEqual(t, *metrics.CPUPercent, 0.0, "CPU percent should be non-negative")
			assert.LessOrEqual(t, *metrics.CPUPercent, 100.0*16, "CPU percent should be reasonable (allowing for multi-core)")
		}

		if metrics.MemoryMB != nil {
			assert.GreaterOrEqual(t, *metrics.MemoryMB, 0.0, "Memory MB should be non-negative")
			assert.LessOrEqual(t, *metrics.MemoryMB, 32*1024.0, "Memory MB should be reasonable (max 32GB)")
		}

		if mock, ok := p.(*MockProvisioner); ok {
			assert.True(t, mock.metricsCalled, "GetMetrics method should be called")
			assert.Equal(t, sess, mock.lastSession, "GetMetrics should receive the correct session")
		}
	})

	t.Run("GetMetrics_Error", func(t *testing.T) {
		if mock, ok := p.(*MockProvisioner); ok {
			mock.metricsError = errors.New("mock metrics error")

			metrics, err := p.GetMetrics(ctx, sess)

			assert.Error(t, err, "GetMetrics should return error when configured")
			assert.Nil(t, metrics, "GetMetrics should return nil metrics on error")
		}
	})
}

func TestProvisionerContextCancellation(t *testing.T) {
	mock := NewMockProvisioner(workpool.ProviderDocker)
	sess := createTestSession()

	t.Run("Start_ContextCancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		wsURL, liveURL, err := mock.Start(ctx, sess)

		if err != nil {
			assert.Contains(t, err.Error(), "context", "Context cancellation should be handled gracefully")
		}
		_ = wsURL
		_ = liveURL
	})

	t.Run("Start_ContextTimeout", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
		defer cancel()

		time.Sleep(1 * time.Millisecond)

		wsURL, liveURL, err := mock.Start(ctx, sess)

		if err != nil {
			assert.Contains(t, err.Error(), "context", "Context timeout should be handled gracefully")
		}
		_ = wsURL
		_ = liveURL
	})
}

func TestFactoryContract(t *testing.T) {
	t.Run("NewFactory", func(t *testing.T) {
		factory := NewFactory()

		assert.NotNil(t, factory, "NewFactory should return non-nil factory")
		assert.NotNil(t, factory.providers, "Factory should have providers map")
		assert.Empty(t, factory.GetRegisteredTypes(), "New factory should have no registered types")
	})

	t.Run("Register_And_Get", func(t *testing.T) {
		factory := NewFactory()
		mock := NewMockProvisioner(workpool.ProviderDocker)

		factory.Register(workpool.ProviderDocker, mock)

		p, ok := factory.Get(workpool.ProviderDocker)
		assert.True(t, ok, "Get should return true for registered provider")
		assert.Equal(t, mock, p, "Get should return the registered provisioner")

		p2, ok2 := factory.Get(workpool.ProviderACI)
		assert.False(t, ok2, "Get should return false for unregistered provider")
		assert.Nil(t, p2, "Get should return nil for unregistered provider")
	})

	t.Run("GetRegisteredTypes", func(t *testing.T) {
		factory := NewFactory()

		types := factory.GetRegisteredTypes()
		assert.Empty(t, types, "Empty factory should return no types")

		factory.Register(workpool.ProviderDocker, NewMockProvisioner(workpool.ProviderDocker))
		factory.Register(workpool.ProviderACI, NewMockProvisioner(workpool.ProviderACI))

		types = factory.GetRegisteredTypes()
		assert.Len(t, types, 2, "Factory should return correct number of registered types")
		assert.Contains(t, types, workpool.ProviderDocker, "Should contain Docker provider")
		assert.Contains(t, types, workpool.ProviderACI, "Should contain ACI provider")
	})

	t.Run("DefaultFactory", func(t *testing.T) {
		assert.NotNil(t, DefaultFactory, "DefaultFactory should be initialized")

		p, ok := FromString("docker")
		if ok {
			assert.Equal(t, workpool.ProviderDocker, p.GetType(), "FromString should return correct provider type")
		}

		p2, ok2 := FromString("invalid")
		assert.False(t, ok2, "FromString should return false for invalid provider")
		assert.Nil(t, p2, "FromString should return nil for invalid provider")
	})
}

func TestSessionValidation(t *testing.T) {
	mock := NewMockProvisioner(workpool.ProviderDocker)
	ctx := context.Background()

	t.Run("NilSession", func(t *testing.T) {
		assert.NotPanics(t, func() {
			_, _, _ = mock.Start(ctx, nil)
			_ = mock.Stop(ctx, nil)
			_ = mock.HealthCheck(ctx, nil)
			_, _ = mock.GetMetrics(ctx, nil)
		}, "Provider methods should handle nil session gracefully")
	})

	t.Run("MinimalSession", func(t *testing.T) {
		minimalSess := &sessions.Session{
			ID:              uuid.New(),
			Browser:         sessions.BrowserChrome,
			Version:         sessions.VerLatest,
			OperatingSystem: sessions.OSLinux,
			Screen: sessions.ScreenConfig{
				Width:  1920,
				Height: 1080,
			},
		}

		assert.NotPanics(t, func() {
			_, _, _ = mock.Start(ctx, minimalSess)
			_ = mock.Stop(ctx, minimalSess)
			_ = mock.HealthCheck(ctx, minimalSess)
			_, _ = mock.GetMetrics(ctx, minimalSess)
		}, "Provider methods should handle minimal session gracefully")
	})
}

func TestProvisionerLifecycle(t *testing.T) {
	mock := NewMockProvisioner(workpool.ProviderDocker)
	ctx := context.Background()
	sess := createTestSession()

	t.Run("FullLifecycle", func(t *testing.T) {
		wsURL, liveURL, err := mock.Start(ctx, sess)
		require.NoError(t, err, "Start should succeed")
		assert.NotEmpty(t, wsURL, "Should return WebSocket URL")
		assert.NotEmpty(t, liveURL, "Should return Live URL")

		sess.WSEndpoint = &wsURL
		sess.LiveURL = &liveURL
		err = mock.HealthCheck(ctx, sess)
		assert.NoError(t, err, "HealthCheck should succeed after Start")

		metrics, err := mock.GetMetrics(ctx, sess)
		require.NoError(t, err, "GetMetrics should succeed")
		assert.NotNil(t, metrics, "Should return metrics")

		err = mock.Stop(ctx, sess)
		assert.NoError(t, err, "Stop should succeed")

		assert.True(t, mock.startCalled, "Start should have been called")
		assert.True(t, mock.healthCalled, "HealthCheck should have been called")
		assert.True(t, mock.metricsCalled, "GetMetrics should have been called")
		assert.True(t, mock.stopCalled, "Stop should have been called")
	})
}
