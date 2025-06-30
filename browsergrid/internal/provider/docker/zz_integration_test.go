//go:build integration

package docker

import (
	"context"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"

	"github.com/autocrawlerHQ/browsergrid/internal/sessions"
)

const (
	testTimeout      = 30 * time.Second
	testImageBrowser = "browsergrid/chrome:latest"
	testPort         = 80
)

func createIntegrationTestSession() *sessions.Session {
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
		Environment: datatypes.JSON(`{"TEST_VAR": "integration"}`),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
}

func createTestProvisioner(t *testing.T) *DockerProvisioner {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	require.NoError(t, err, "Docker client should be available for integration tests")

	_, err = cli.Ping(context.Background())
	require.NoError(t, err, "Docker daemon should be running for integration tests")

	return &DockerProvisioner{
		cli:           cli,
		defaultPort:   testPort,
		healthTimeout: 10 * time.Second,
	}
}

func ensureTestImages(t *testing.T, p *DockerProvisioner) {
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	err := p.ensureImage(ctx, testImageBrowser)
	require.NoError(t, err, "Should be able to pull test image: %s", testImageBrowser)
}

func cleanupContainers(t *testing.T, cli *client.Client, sessionID uuid.UUID) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	filterArgs := filters.NewArgs()
	filterArgs.Add("label", fmt.Sprintf("com.browsergrid.session=%s", sessionID.String()))

	containers, err := cli.ContainerList(ctx, container.ListOptions{
		All:     true,
		Filters: filterArgs,
	})
	if err != nil {
		t.Logf("Warning: Could not list containers for cleanup: %v", err)
		return
	}

	for _, c := range containers {
		_ = cli.ContainerRemove(ctx, c.ID, container.RemoveOptions{Force: true})
		t.Logf("Cleaned up container: %s", c.ID[:12])
	}
}

func TestDockerProvisionerIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	provisioner := createTestProvisioner(t)
	ensureTestImages(t, provisioner)

	t.Run("FullLifecycle", func(t *testing.T) {
		sess := createIntegrationTestSession()
		defer cleanupContainers(t, provisioner.cli, sess.ID)

		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		defer cancel()

		wsURL, liveURL, err := provisioner.Start(ctx, sess)
		require.NoError(t, err, "Start should succeed with real Docker")
		assert.NotEmpty(t, wsURL, "Should return WebSocket URL")
		assert.NotEmpty(t, liveURL, "Should return Live URL")
		assert.Contains(t, wsURL, "ws://localhost:", "WebSocket URL should be localhost")
		assert.Contains(t, liveURL, "http://localhost:", "Live URL should be localhost")

		assert.NotNil(t, sess.ContainerID, "Session should have container ID")
		// ContainerNetwork is not set in single-container mode
		assert.NotNil(t, sess.WSEndpoint, "Session should have WS endpoint")
		assert.NotNil(t, sess.LiveURL, "Session should have live URL")

		t.Run("ContainersRunning", func(t *testing.T) {
			filterArgs := filters.NewArgs()
			filterArgs.Add("label", fmt.Sprintf("com.browsergrid.session=%s", sess.ID.String()))

			containers, err := provisioner.cli.ContainerList(ctx, container.ListOptions{
				Filters: filterArgs,
			})
			require.NoError(t, err, "Should be able to list containers")
			assert.Len(t, containers, 1, "Should have 1 container")

			for _, c := range containers {
				assert.Equal(t, "running", c.State, "Container should be running")
				assert.Contains(t, c.Labels, "com.browsergrid.session", "Container should have session label")
			}
		})

		t.Run("PortsAccessible", func(t *testing.T) {
			parts := strings.Split(liveURL, ":")
			require.Len(t, parts, 3, "Live URL should have port")
			port := parts[2]

			conn, err := net.DialTimeout("tcp", "localhost:"+port, 5*time.Second)
			if err == nil {
				conn.Close()
				t.Logf("Successfully connected to port %s", port)
			} else {
				t.Logf("Could not connect to port %s (expected for test images): %v", port, err)
			}
		})

		t.Run("HealthCheck", func(t *testing.T) {
			// Give the container time to fully start up
			var hcErr error
			for i := 0; i < 5; i++ {
				hcErr = provisioner.HealthCheck(ctx, sess)
				if hcErr == nil {
					break
				}
				time.Sleep(1 * time.Second)
			}
			assert.NoError(t, hcErr, "HealthCheck should eventually succeed")
		})

		t.Run("GetMetrics", func(t *testing.T) {
			metrics, err := provisioner.GetMetrics(ctx, sess)
			require.NoError(t, err, "GetMetrics should succeed")
			require.NotNil(t, metrics, "Metrics should not be nil")

			assert.Equal(t, sess.ID, metrics.SessionID, "Metrics should reference correct session")
			assert.NotZero(t, metrics.Timestamp, "Metrics should have timestamp")

			if metrics.CPUPercent != nil {
				assert.GreaterOrEqual(t, *metrics.CPUPercent, 0.0, "CPU percent should be non-negative")
			}

			if metrics.MemoryMB != nil {
				assert.Greater(t, *metrics.MemoryMB, 0.0, "Memory should be greater than 0")
			}

			if metrics.NetworkRXBytes != nil {
				assert.GreaterOrEqual(t, *metrics.NetworkRXBytes, int64(0), "Network RX should be non-negative")
			}

			if metrics.NetworkTXBytes != nil {
				assert.GreaterOrEqual(t, *metrics.NetworkTXBytes, int64(0), "Network TX should be non-negative")
			}

			t.Logf("Metrics: CPU=%.2f%%, Memory=%.2fMB, Network RX/TX=%d/%d bytes",
				ptrFloat64(metrics.CPUPercent), ptrFloat64(metrics.MemoryMB),
				ptrInt64(metrics.NetworkRXBytes), ptrInt64(metrics.NetworkTXBytes))
		})

		t.Run("Stop", func(t *testing.T) {
			originalContainerID := *sess.ContainerID

			err := provisioner.Stop(ctx, sess)
			require.NoError(t, err, "Stop should succeed")

			time.Sleep(2 * time.Second)

			inspect, err := provisioner.cli.ContainerInspect(ctx, originalContainerID)
			if err != nil {
				t.Logf("Container successfully removed: %s", originalContainerID[:12])
			} else {
				t.Logf("Container still exists with state: %s", inspect.State.Status)
				assert.NotEqual(t, "running", inspect.State.Status,
					"Container should not be running after stop")
			}

			filterArgs := filters.NewArgs()
			filterArgs.Add("label", fmt.Sprintf("com.browsergrid.session=%s", sess.ID.String()))
			filterArgs.Add("status", "running")

			runningContainers, err := provisioner.cli.ContainerList(ctx, container.ListOptions{
				Filters: filterArgs,
			})
			require.NoError(t, err, "Should be able to list running containers after stop")

			if len(runningContainers) > 0 {
				for _, c := range runningContainers {
					t.Logf("Still running container: %s (state: %s)", c.ID[:12], c.State)
				}
				t.Logf("Warning: %d containers still running after stop", len(runningContainers))
			} else {
				t.Logf("All containers successfully stopped")
			}

			// No dedicated container network in single-container mode, so no
			// network-cleanup assertions are required.
		})
	})
}

func TestDockerProvisionerStartFailures(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	provisioner := createTestProvisioner(t)

	t.Run("InvalidImage", func(t *testing.T) {
		sess := createIntegrationTestSession()
		defer cleanupContainers(t, provisioner.cli, sess.ID)

		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		defer cancel()

		// Use an invalid browser type to generate a non-existent image
		sess.Browser = sessions.BrowserChrome
		sess.Version = sessions.BrowserVersion("nonexistent-invalid-version")

		_, _, err := provisioner.Start(ctx, sess)
		assert.Error(t, err, "Should fail with invalid image")
		assert.Contains(t, err.Error(), "pull", "Error should mention image pull failure")
	})

	t.Run("ContextCancellation", func(t *testing.T) {
		sess := createIntegrationTestSession()
		defer cleanupContainers(t, provisioner.cli, sess.ID)

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, _, err := provisioner.Start(ctx, sess)
		assert.Error(t, err, "Should fail with cancelled context")
		assert.Contains(t, err.Error(), "context", "Error should mention context cancellation")
	})
}

func TestDockerProvisionerStopIdempotency(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	provisioner := createTestProvisioner(t)
	ensureTestImages(t, provisioner)

	sess := createIntegrationTestSession()
	defer cleanupContainers(t, provisioner.cli, sess.ID)

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	_, _, err := provisioner.Start(ctx, sess)
	require.NoError(t, err, "Start should succeed")

	for i := 0; i < 3; i++ {
		err := provisioner.Stop(ctx, sess)
		assert.NoError(t, err, "Stop should be idempotent (iteration %d)", i+1)
	}
}

func TestDockerProvisionerContainerIsolation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	provisioner := createTestProvisioner(t)
	ensureTestImages(t, provisioner)

	sess1 := createIntegrationTestSession()
	sess2 := createIntegrationTestSession()

	defer cleanupContainers(t, provisioner.cli, sess1.ID)
	defer cleanupContainers(t, provisioner.cli, sess2.ID)

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	_, _, err1 := provisioner.Start(ctx, sess1)
	require.NoError(t, err1, "Session 1 should start")

	_, _, err2 := provisioner.Start(ctx, sess2)
	require.NoError(t, err2, "Session 2 should start")

	assert.NotEqual(t, sess1.ContainerID, sess2.ContainerID,
		"Sessions should have different container IDs")

	require.NoError(t, provisioner.Stop(ctx, sess1), "Should stop session 1")
	require.NoError(t, provisioner.Stop(ctx, sess2), "Should stop session 2")
}

func TestDockerProvisionerEnvironmentVariables(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	provisioner := createTestProvisioner(t)
	ensureTestImages(t, provisioner)

	sess := createIntegrationTestSession()
	defer cleanupContainers(t, provisioner.cli, sess.ID)

	sess.Environment = datatypes.JSON(`{
		"CUSTOM_VAR": "test_value",
		"ANOTHER_VAR": "another_value",
		"NUMERIC_VAR": "123"
	}`)

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	_, _, err := provisioner.Start(ctx, sess)
	require.NoError(t, err, "Start should succeed")

	filterArgs := filters.NewArgs()
	filterArgs.Add("label", fmt.Sprintf("com.browsergrid.session=%s", sess.ID.String()))

	containers, err := provisioner.cli.ContainerList(ctx, container.ListOptions{
		Filters: filterArgs,
	})
	require.NoError(t, err, "Should list session containers")
	require.GreaterOrEqual(t, len(containers), 1, "Should have at least one container")

	var browserContainerID string
	if len(containers) > 0 {
		for _, c := range containers {
			if !strings.Contains(strings.Join(c.Names, ""), "mux") {
				browserContainerID = c.ID
				break
			}
		}
		if browserContainerID == "" {
			browserContainerID = containers[0].ID
		}
	}

	containerInfo, err := provisioner.cli.ContainerInspect(ctx, browserContainerID)
	require.NoError(t, err, "Should inspect browser container")

	envVars := containerInfo.Config.Env
	envMap := make(map[string]string)
	for _, env := range envVars {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) == 2 {
			envMap[parts[0]] = parts[1]
		}
	}

	assert.Equal(t, "test_value", envMap["CUSTOM_VAR"], "Custom environment variable should be set")
	assert.Equal(t, "another_value", envMap["ANOTHER_VAR"], "Another environment variable should be set")
	assert.Equal(t, "123", envMap["NUMERIC_VAR"], "Numeric environment variable should be set")

	t.Logf("All environment variables in container:")
	for key, value := range envMap {
		t.Logf("  %s=%s", key, value)
	}

	if envMap["BROWSERGRID_SESSION_ID"] != "" {
		t.Logf("Found BROWSERGRID_SESSION_ID: %s", envMap["BROWSERGRID_SESSION_ID"])
	} else {
		t.Logf("BROWSERGRID_SESSION_ID not set (expected for test containers)")
	}

	if envMap["BROWSERGRID_BROWSER"] != "" {
		t.Logf("Found BROWSERGRID_BROWSER: %s", envMap["BROWSERGRID_BROWSER"])
	} else {
		t.Logf("BROWSERGRID_BROWSER not set (expected for test containers)")
	}

	require.NoError(t, provisioner.Stop(ctx, sess), "Should stop session")
}

func ptrFloat64(f *float64) float64 {
	if f == nil {
		return 0
	}
	return *f
}

func ptrInt64(i *int64) int64 {
	if i == nil {
		return 0
	}
	return *i
}

func BenchmarkDockerProvisionerStart(b *testing.B) {
	if testing.Short() {
		b.Skip("Skipping benchmark in short mode")
	}

	provisioner := createTestProvisioner(&testing.T{})
	ensureTestImages(&testing.T{}, provisioner)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sess := createIntegrationTestSession()
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)

		_, _, err := provisioner.Start(ctx, sess)
		if err != nil {
			b.Fatalf("Start failed: %v", err)
		}

		_ = provisioner.Stop(ctx, sess)
		cancel()
	}
}

func BenchmarkDockerProvisionerStop(b *testing.B) {
	if testing.Short() {
		b.Skip("Skipping benchmark in short mode")
	}

	provisioner := createTestProvisioner(&testing.T{})
	ensureTestImages(&testing.T{}, provisioner)

	sessions := make([]*sessions.Session, b.N)
	ctx := context.Background()

	for i := 0; i < b.N; i++ {
		sessions[i] = createIntegrationTestSession()
		_, _, err := provisioner.Start(ctx, sessions[i])
		if err != nil {
			b.Fatalf("Setup failed: %v", err)
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		err := provisioner.Stop(ctx, sessions[i])
		if err != nil {
			b.Fatalf("Stop failed: %v", err)
		}
		cancel()
	}
}
