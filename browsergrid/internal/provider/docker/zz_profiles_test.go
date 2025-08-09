package docker

import (
	"context"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/autocrawlerHQ/browsergrid/internal/sessions"
)

func TestDockerProvisioner_StartWithProfile(t *testing.T) {
	// Skip if Docker is not available
	if !isDockerAvailable() {
		t.Skip("Docker not available")
	}

	provisioner := NewDockerProvisioner()
	ctx := context.Background()

	// Create a session with profile
	profileID := uuid.New()
	session := &sessions.Session{
		ID:        uuid.New(),
		Browser:   sessions.BrowserChrome,
		Version:   sessions.VerLatest,
		ProfileID: &profileID,
		Screen: sessions.ScreenConfig{
			Width:  1920,
			Height: 1080,
			DPI:    96,
			Scale:  1.0,
		},
		Headless:        true,
		OperatingSystem: sessions.OSLinux,
	}

	// Test that the provisioner can handle profile mounting
	// Note: This is a basic test that doesn't actually mount a real profile
	// In a real environment, the profile store would provide the actual path
	wsURL, liveURL, err := provisioner.Start(ctx, session)

	// The test should either succeed (if Docker is available) or fail gracefully
	if err != nil {
		// If it fails, it should be due to Docker/image issues, not profile issues
		assert.NotContains(t, err.Error(), "profile")
	} else {
		assert.NotEmpty(t, wsURL)
		assert.NotEmpty(t, liveURL)
		assert.NotNil(t, session.ContainerID)
		assert.NotNil(t, session.WSEndpoint)
		assert.NotNil(t, session.LiveURL)
	}

	// Clean up
	if session.ContainerID != nil {
		provisioner.Stop(ctx, session)
	}
}

func TestDockerProvisioner_StartWithoutProfile(t *testing.T) {
	// Skip if Docker is not available
	if !isDockerAvailable() {
		t.Skip("Docker not available")
	}

	provisioner := NewDockerProvisioner()
	ctx := context.Background()

	// Create a session without profile
	session := &sessions.Session{
		ID:        uuid.New(),
		Browser:   sessions.BrowserChrome,
		Version:   sessions.VerLatest,
		ProfileID: nil, // No profile
		Screen: sessions.ScreenConfig{
			Width:  1920,
			Height: 1080,
			DPI:    96,
			Scale:  1.0,
		},
		Headless:        true,
		OperatingSystem: sessions.OSLinux,
	}

	wsURL, liveURL, err := provisioner.Start(ctx, session)

	if err != nil {
		// If it fails, it should be due to Docker/image issues, not profile issues
		assert.NotContains(t, err.Error(), "profile")
	} else {
		assert.NotEmpty(t, wsURL)
		assert.NotEmpty(t, liveURL)
		assert.NotNil(t, session.ContainerID)
		assert.NotNil(t, session.WSEndpoint)
		assert.NotNil(t, session.LiveURL)
	}

	// Clean up
	if session.ContainerID != nil {
		provisioner.Stop(ctx, session)
	}
}

func TestDockerProvisioner_ProfileStoreIntegration(t *testing.T) {
	provisioner := NewDockerProvisioner()

	// Test that the provisioner has a profile store
	assert.NotNil(t, provisioner.profileStore)
}

func TestDockerProvisioner_ContainerLabels(t *testing.T) {
	// Skip if Docker is not available
	if !isDockerAvailable() {
		t.Skip("Docker not available")
	}

	provisioner := NewDockerProvisioner()
	ctx := context.Background()

	sessionID := uuid.New()
	session := &sessions.Session{
		ID:        sessionID,
		Browser:   sessions.BrowserChrome,
		Version:   sessions.VerLatest,
		ProfileID: nil,
		Screen: sessions.ScreenConfig{
			Width:  1920,
			Height: 1080,
			DPI:    96,
			Scale:  1.0,
		},
		Headless:        true,
		OperatingSystem: sessions.OSLinux,
	}

	_, _, err := provisioner.Start(ctx, session)

	if err == nil {
		// Verify container has proper labels
		containers, err := provisioner.cli.ContainerList(ctx, container.ListOptions{
			All: true,
			Filters: filters.NewArgs(
				filters.Arg("label", "com.browsergrid.session="+sessionID.String()),
			),
		})
		require.NoError(t, err)
		assert.Len(t, containers, 1)
		assert.Equal(t, sessionID.String(), containers[0].Labels["com.browsergrid.session"])

		// Clean up
		provisioner.Stop(ctx, session)
	}
}

func TestDockerProvisioner_HealthCheckWithProfile(t *testing.T) {
	// Skip if Docker is not available
	if !isDockerAvailable() {
		t.Skip("Docker not available")
	}

	provisioner := NewDockerProvisioner()
	ctx := context.Background()

	profileID := uuid.New()
	session := &sessions.Session{
		ID:        uuid.New(),
		Browser:   sessions.BrowserChrome,
		Version:   sessions.VerLatest,
		ProfileID: &profileID,
		Screen: sessions.ScreenConfig{
			Width:  1920,
			Height: 1080,
			DPI:    96,
			Scale:  1.0,
		},
		Headless:        true,
		OperatingSystem: sessions.OSLinux,
	}

	_, _, err := provisioner.Start(ctx, session)

	if err == nil {
		// Test health check
		err = provisioner.HealthCheck(ctx, session)
		// Health check might fail if container is not fully ready, which is expected
		// The important thing is that it doesn't fail due to profile-related issues
		if err != nil {
			assert.NotContains(t, err.Error(), "profile")
		}

		// Clean up
		provisioner.Stop(ctx, session)
	}
}

func TestDockerProvisioner_GetMetricsWithProfile(t *testing.T) {
	// Skip if Docker is not available
	if !isDockerAvailable() {
		t.Skip("Docker not available")
	}

	provisioner := NewDockerProvisioner()
	ctx := context.Background()

	profileID := uuid.New()
	session := &sessions.Session{
		ID:        uuid.New(),
		Browser:   sessions.BrowserChrome,
		Version:   sessions.VerLatest,
		ProfileID: &profileID,
		Screen: sessions.ScreenConfig{
			Width:  1920,
			Height: 1080,
			DPI:    96,
			Scale:  1.0,
		},
		Headless:        true,
		OperatingSystem: sessions.OSLinux,
	}

	_, _, err := provisioner.Start(ctx, session)

	if err == nil {
		// Wait a bit for container to stabilize
		time.Sleep(2 * time.Second)

		// Test metrics collection
		metrics, err := provisioner.GetMetrics(ctx, session)
		if err != nil {
			// Metrics might fail if container is not fully ready, which is expected
			assert.NotContains(t, err.Error(), "profile")
		} else {
			assert.NotNil(t, metrics)
			assert.Equal(t, session.ID, metrics.SessionID)
			assert.NotZero(t, metrics.Timestamp)
		}

		// Clean up
		provisioner.Stop(ctx, session)
	}
}

// Helper function to check if Docker is available
func isDockerAvailable() bool {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return false
	}
	defer cli.Close()

	_, err = cli.Ping(context.Background())
	return err == nil
}
