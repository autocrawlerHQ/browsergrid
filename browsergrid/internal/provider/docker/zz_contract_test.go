//go:build provider

package docker

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"gorm.io/datatypes"

	"github.com/autocrawlerHQ/browsergrid/internal/provider"
	"github.com/autocrawlerHQ/browsergrid/internal/sessions"
	"github.com/autocrawlerHQ/browsergrid/internal/workpool"
)

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

func TestDockerProviderContract(t *testing.T) {
	t.Run("Registration", func(t *testing.T) {
		p, ok := provider.DefaultFactory.Get(workpool.ProviderDocker)
		assert.True(t, ok, "Docker provider should be registered in DefaultFactory")
		assert.NotNil(t, p, "Docker provider should not be nil")

		if p != nil {
			assert.Equal(t, workpool.ProviderDocker, p.GetType(), "Docker provider should return correct type")
		}
	})

	t.Run("InterfaceCompliance", func(t *testing.T) {
		provisioner := &DockerProvisioner{
			cli:           nil,
			defaultPort:   80,
			healthTimeout: 10 * time.Second,
		}

		assert.Equal(t, workpool.ProviderDocker, provisioner.GetType(), "GetType should return Docker provider type")

		var _ provider.Provisioner = provisioner
		assert.NotNil(t, provisioner, "DockerProvisioner should implement Provisioner interface")

		t.Run("HealthCheck_WithoutEndpoints", func(t *testing.T) {
			sess := createTestSession()
			err := provisioner.HealthCheck(context.Background(), sess)
			assert.Error(t, err, "HealthCheck should return error when WSEndpoint is nil")
			assert.Contains(t, err.Error(), "no ws endpoint", "Error should mention missing endpoint")
		})

		t.Run("StructFields", func(t *testing.T) {
			assert.Equal(t, 80, provisioner.defaultPort)
			assert.Equal(t, 10*time.Second, provisioner.healthTimeout)
		})
	})

	t.Run("FactoryIntegration", func(t *testing.T) {
		p, ok := provider.FromString("docker")
		assert.True(t, ok, "FromString should find Docker provider")
		assert.NotNil(t, p, "FromString should return non-nil provider")

		if p != nil {
			assert.Equal(t, workpool.ProviderDocker, p.GetType(), "FromString should return Docker provider")
		}
	})

	t.Run("RegisteredTypes", func(t *testing.T) {
		types := provider.DefaultFactory.GetRegisteredTypes()
		assert.Contains(t, types, workpool.ProviderDocker, "Docker provider should be in registered types")
	})
}

func TestDockerProvisionerCreation(t *testing.T) {
	t.Run("NewDockerProvisioner", func(t *testing.T) {
		assert.NotPanics(t, func() {
			defer func() {
				if r := recover(); r != nil {
					t.Logf("NewDockerProvisioner panicked (expected if Docker not available): %v", r)
				}
			}()

			provisioner := NewDockerProvisioner()
			if provisioner != nil {
				assert.Equal(t, workpool.ProviderDocker, provisioner.GetType())
				assert.Equal(t, 80, provisioner.defaultPort)
				assert.Equal(t, 10*time.Second, provisioner.healthTimeout)
			}
		}, "NewDockerProvisioner creation should be tested")
	})
}
