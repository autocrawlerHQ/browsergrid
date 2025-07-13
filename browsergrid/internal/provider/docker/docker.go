package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"github.com/google/uuid"

	"github.com/autocrawlerHQ/browsergrid/internal/profiles"
	"github.com/autocrawlerHQ/browsergrid/internal/provider"
	"github.com/autocrawlerHQ/browsergrid/internal/sessions"
	"github.com/autocrawlerHQ/browsergrid/internal/workpool"
)

type DockerProvisioner struct {
	cli *client.Client

	defaultPort   int
	healthTimeout time.Duration
	profileStore  *profiles.LocalProfileStore
}

func NewDockerProvisioner() *DockerProvisioner {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		panic(fmt.Errorf("docker client: %w", err))
	}

	profileStore, err := profiles.NewLocalProfileStore("")
	if err != nil {
		panic(fmt.Errorf("profile store: %w", err))
	}

	return &DockerProvisioner{
		cli:           cli,
		defaultPort:   80,
		healthTimeout: 10 * time.Second,
		profileStore:  profileStore,
	}
}

func (p *DockerProvisioner) GetType() workpool.ProviderType { return workpool.ProviderDocker }

func (p *DockerProvisioner) keepContainers() bool {
	return strings.ToLower(os.Getenv("BROWSERGRID_KEEP_CONTAINERS")) == "true"
}

func (p *DockerProvisioner) Start(
	ctx context.Context,
	sess *sessions.Session,
) (wsURL, liveURL string, err error) {
	shortID := sess.ID.String()[:8]

	browserImage := fmt.Sprintf("browsergrid/%s:%s",
		string(sess.Browser), defaultStr(string(sess.Version), "latest"))

	if err := p.ensureImage(ctx, browserImage); err != nil {
		return "", "", err
	}

	browserCName := "bg-browser-" + shortID
	browserEnv := []string{
		fmt.Sprintf("HEADLESS=%t", sess.Headless),
		fmt.Sprintf("RESOLUTION_WIDTH=%d", sess.Screen.Width),
		fmt.Sprintf("RESOLUTION_HEIGHT=%d", sess.Screen.Height),
	}
	var envMap map[string]string
	if sess.Environment != nil {
		if err := json.Unmarshal(sess.Environment, &envMap); err == nil {
			for k, v := range envMap {
				browserEnv = append(browserEnv, k+"="+v)
			}
		}
	}

	// Prepare container configuration
	containerConfig := &container.Config{
		Image: browserImage,
		Env:   browserEnv,
		Labels: map[string]string{
			"com.browsergrid.session": sess.ID.String(),
		},
		Hostname: "browser",
		ExposedPorts: natSet(
			fmt.Sprintf("%d/tcp", p.defaultPort),
		),
	}

	hostConfig := &container.HostConfig{
		AutoRemove:   false,
		PortBindings: natMap(p.defaultPort, 0),
		Tmpfs:        map[string]string{"/dev/shm": "rw,size=2g"},
	}

	// Mount profile if specified
	if sess.ProfileID != nil {
		log.Printf("[DOCKER] Session %s requires profile %s", shortID, sess.ProfileID)

		profilePath, err := p.profileStore.GetProfilePath(ctx, sess.ProfileID.String())
		if err != nil {
			log.Printf("[DOCKER] Failed to get profile path for %s: %v", sess.ProfileID, err)
			return "", "", fmt.Errorf("failed to get profile path: %w", err)
		}

		// Check if profile path exists and is accessible
		if _, err := os.Stat(profilePath); os.IsNotExist(err) {
			log.Printf("[DOCKER] Profile path does not exist: %s", profilePath)
			return "", "", fmt.Errorf("profile path does not exist: %s", profilePath)
		} else if err != nil {
			log.Printf("[DOCKER] Cannot access profile path %s: %v", profilePath, err)
			return "", "", fmt.Errorf("cannot access profile path: %w", err)
		}

		// When running in Docker, use volume mount instead of bind mount
		// because the Docker daemon can't access paths inside containers
		if runningInDocker() {
			// Use volume mount to share the profile data volume
			// The volume name includes the Docker Compose project prefix
			volumeName := os.Getenv("BROWSERGRID_PROFILE_VOLUME_NAME")
			if volumeName == "" {
				volumeName = "browsergrid-server_profile_data"
			}
			hostConfig.Mounts = append(hostConfig.Mounts, mount.Mount{
				Type:   mount.TypeVolume,
				Source: volumeName,
				Target: "/var/lib/browsergrid/profiles",
			})

			// Set environment variable to indicate which profile to use
			containerConfig.Env = append(containerConfig.Env, fmt.Sprintf("BROWSERGRID_PROFILE_ID=%s", sess.ProfileID.String()))

			log.Printf("[DOCKER] Mounting profile volume for profile %s in container %s", sess.ProfileID, shortID)
		} else {
			// For non-containerized workers, use bind mount
			hostConfig.Mounts = append(hostConfig.Mounts, mount.Mount{
				Type:   mount.TypeBind,
				Source: profilePath,
				Target: "/home/user/data-dir",
			})

			log.Printf("[DOCKER] Mounting profile %s from %s to container %s", sess.ProfileID, profilePath, shortID)
		}
	} else {
		log.Printf("[DOCKER] Session %s has no profile, using default browser data directory", shortID)
	}

	browserResp, err := p.cli.ContainerCreate(ctx,
		containerConfig,
		hostConfig,
		nil,
		nil,
		browserCName,
	)
	if err != nil {
		return "", "", fmt.Errorf("create browser container: %w", err)
	}

	if err := p.cli.ContainerStart(ctx, browserResp.ID, container.StartOptions{}); err != nil {
		return p.abortStart(ctx, browserResp.ID, fmt.Errorf("start browser: %w", err))
	}

	// Wait for browser to be healthy
	hostPort, err := p.waitForContainer(ctx, browserResp.ID)
	if err != nil {
		return p.abortStart(ctx, browserResp.ID, err)
	}

	// Use host-mapped address that is reachable from inside a container
	hostName := "localhost"
	if runningInDocker() {
		hostName = "host.docker.internal"
	}

	sess.ContainerID = &browserResp.ID
	sess.WSEndpoint = strPtr(fmt.Sprintf("ws://%s:%d", hostName, hostPort))
	sess.LiveURL = strPtr(fmt.Sprintf("http://%s:%d", hostName, hostPort))

	return *sess.WSEndpoint, *sess.LiveURL, nil
}

func (p *DockerProvisioner) Stop(ctx context.Context, sess *sessions.Session) error {
	if sess.ContainerID != nil {
		// Find all containers with the session label and remove them
		filterArgs := filters.NewArgs()
		filterArgs.Add("label", fmt.Sprintf("com.browsergrid.session=%s", sess.ID.String()))

		containers, err := p.cli.ContainerList(ctx, container.ListOptions{
			All:     true,
			Filters: filterArgs,
		})
		if err == nil && !p.keepContainers() {
			for _, c := range containers {
				_ = p.cli.ContainerRemove(ctx, c.ID, container.RemoveOptions{Force: true})
			}
		}

		// Fallback: try to remove the stored container ID directly
		if !p.keepContainers() {
			_ = p.cli.ContainerRemove(ctx, *sess.ContainerID, container.RemoveOptions{Force: true})
		}
	}
	return nil
}

func (p *DockerProvisioner) HealthCheck(ctx context.Context, sess *sessions.Session) error {
	if sess.WSEndpoint == nil {
		return fmt.Errorf("no ws endpoint recorded")
	}
	u := "http://" + wsToHTTP(*sess.WSEndpoint) + "/health"
	cli := &http.Client{Timeout: 3 * time.Second}

	resp, err := cli.Get(u)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unhealthy: %s", resp.Status)
	}
	return nil
}

func (p *DockerProvisioner) GetMetrics(
	ctx context.Context,
	sess *sessions.Session,
) (*sessions.SessionMetrics, error) {
	if sess.ContainerID == nil {
		return nil, fmt.Errorf("no container id recorded")
	}
	stats, err := p.cli.ContainerStatsOneShot(ctx, *sess.ContainerID)
	if err != nil {
		return nil, err
	}
	defer stats.Body.Close()

	var v container.StatsResponse
	if err := json.NewDecoder(stats.Body).Decode(&v); err != nil {
		return nil, err
	}

	cpuPct := cpuPercentUnix(v)
	memMB := float64(v.MemoryStats.Usage) / (1024 * 1024)

	return &sessions.SessionMetrics{
		ID:             uuid.New(),
		SessionID:      sess.ID,
		Timestamp:      time.Now(),
		CPUPercent:     &cpuPct,
		MemoryMB:       &memMB,
		NetworkRXBytes: int64Ptr(int64(v.Networks["eth0"].RxBytes)),
		NetworkTXBytes: int64Ptr(int64(v.Networks["eth0"].TxBytes)),
	}, nil
}

func (p *DockerProvisioner) ensureImage(ctx context.Context, imageName string) error {
	// Always try to pull the latest image. If the pull fails because the image
	// isn't available in a remote registry, but it exists locally, continue.
	rd, err := p.cli.ImagePull(ctx, imageName, image.PullOptions{})
	if err != nil {
		// If the image exists locally, ignore the pull error.
		if _, inspectErr := p.cli.ImageInspect(ctx, imageName); inspectErr == nil {
			return nil
		}
		return fmt.Errorf("pull %s: %w", imageName, err)
	}
	defer rd.Close()
	_, _ = io.Copy(io.Discard, rd)
	return nil
}

func (p *DockerProvisioner) waitForContainer(ctx context.Context, containerID string) (hostPort int, err error) {
	deadline := time.Now().Add(p.healthTimeout)

	for time.Now().Before(deadline) {
		inspect, err := p.cli.ContainerInspect(ctx, containerID)
		if err != nil {
			return 0, err
		}

		// Check if container is running
		if !inspect.State.Running {
			time.Sleep(200 * time.Millisecond)
			continue
		}

		for pc := range inspect.NetworkSettings.Ports {
			if pc.Int() == p.defaultPort && len(inspect.NetworkSettings.Ports[pc]) > 0 {
				hostPortStr := inspect.NetworkSettings.Ports[pc][0].HostPort
				hostPort, _ := strconv.Atoi(hostPortStr)
				if hostPort > 0 {
					// Don't try to dial from inside the worker container
					// Just return the port once Docker has assigned it
					return hostPort, nil
				}
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	return 0, fmt.Errorf("browser container did not become ready within %s", p.healthTimeout)
}

func (p *DockerProvisioner) abortStart(
	ctx context.Context,
	browserID string,
	rootErr error,
) (string, string, error) {
	if !p.keepContainers() {
		_ = p.cli.ContainerRemove(ctx, browserID, container.RemoveOptions{Force: true})
	}
	return "", "", rootErr
}

func natSet(ports ...string) nat.PortSet {
	ps := nat.PortSet{}
	for _, p := range ports {
		ps[nat.Port(p)] = struct{}{}
	}
	return ps
}

func natMap(containerPort int, hostPort int) nat.PortMap {
	pm := nat.PortMap{}
	cp := nat.Port(strconv.Itoa(containerPort) + "/tcp")
	pm[cp] = []nat.PortBinding{{
		HostIP:   "0.0.0.0",
		HostPort: strconv.Itoa(hostPort),
	}}
	return pm
}

func strPtr(s string) *string { return &s }

func int64Ptr(i int64) *int64 { return &i }

func defaultStr(s, d string) string {
	if s == "" {
		return d
	}
	return s
}

func cpuPercentUnix(v container.StatsResponse) float64 {
	cpuDelta := float64(v.CPUStats.CPUUsage.TotalUsage) - float64(v.PreCPUStats.CPUUsage.TotalUsage)
	sysDelta := float64(v.CPUStats.SystemUsage) - float64(v.PreCPUStats.SystemUsage)
	if sysDelta > 0 && cpuDelta > 0 {
		return (cpuDelta / sysDelta) * float64(len(v.CPUStats.CPUUsage.PercpuUsage)) * 100.0
	}
	return 0.0
}

func wsToHTTP(ws string) string { return strings.TrimPrefix(ws, "ws://") }

// runningInDocker detects if the current process is running inside a Docker container.
func runningInDocker() bool {
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return true
	}
	if data, err := os.ReadFile("/proc/1/cgroup"); err == nil {
		if strings.Contains(string(data), "docker") || strings.Contains(string(data), "kubepods") {
			return true
		}
	}
	return false
}

func init() {
	provider.Register(workpool.ProviderDocker, NewDockerProvisioner())
}
