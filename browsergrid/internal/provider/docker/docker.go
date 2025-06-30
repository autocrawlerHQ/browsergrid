package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"github.com/google/uuid"

	"github.com/autocrawlerHQ/browsergrid/internal/provider"
	"github.com/autocrawlerHQ/browsergrid/internal/sessions"
	"github.com/autocrawlerHQ/browsergrid/internal/workpool"
)

type DockerProvisioner struct {
	cli *client.Client

	imageBrowserMux string
	defaultPortDev  int
	defaultPortMux  int
	healthTimeout   time.Duration
}

func NewDockerProvisioner() *DockerProvisioner {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		panic(fmt.Errorf("docker client: %w", err))
	}

	return &DockerProvisioner{
		cli:             cli,
		imageBrowserMux: "browsergrid/browsermux:latest",
		defaultPortDev:  9222,
		defaultPortMux:  8080,
		healthTimeout:   10 * time.Second,
	}
}

func (p *DockerProvisioner) GetType() workpool.ProviderType { return workpool.ProviderDocker }

// keepContainers returns true if containers should be kept for debugging
func (p *DockerProvisioner) keepContainers() bool {
	return strings.ToLower(os.Getenv("BROWSERGRID_KEEP_CONTAINERS")) == "true"
	// return true
}

func (p *DockerProvisioner) Start(
	ctx context.Context,
	sess *sessions.Session,
) (wsURL, liveURL string, err error) {
	shortID := sess.ID.String()[:8]
	networkName := "bg-" + shortID

	browserImage := fmt.Sprintf("browsergrid/%s:%s",
		string(sess.Browser), defaultStr(string(sess.Version), "latest"))

	for _, img := range []string{browserImage, p.imageBrowserMux} {
		if err := p.ensureImage(ctx, img); err != nil {
			return "", "", err
		}
	}

	var containerNetwork string
	if err := p.createNetwork(ctx, networkName); err != nil {
		return "", "", err
	}
	containerNetwork = networkName

	browserCName := "bg-browser-" + shortID
	browserEnv := []string{
		fmt.Sprintf("HEADLESS=%t", sess.Headless),
		fmt.Sprintf("DISPLAY_WIDTH=%d", sess.Screen.Width),
		fmt.Sprintf("DISPLAY_HEIGHT=%d", sess.Screen.Height),
	}
	var envMap map[string]string
	if sess.Environment != nil {
		if err := json.Unmarshal(sess.Environment, &envMap); err == nil {
			for k, v := range envMap {
				browserEnv = append(browserEnv, k+"="+v)
			}
		}
	}

	browserResp, err := p.cli.ContainerCreate(ctx,
		&container.Config{
			Image: browserImage,
			Env:   browserEnv,
			Labels: map[string]string{
				"com.browsergrid.session": sess.ID.String(),
			},
			Hostname: "browser",
			ExposedPorts: natSet(
				fmt.Sprintf("%d/tcp", p.defaultPortDev),
			),
		},
		&container.HostConfig{
			AutoRemove:      false,
			PublishAllPorts: false,
			Tmpfs:           map[string]string{"/dev/shm": "rw,size=2g"},
		},
		&network.NetworkingConfig{
			EndpointsConfig: map[string]*network.EndpointSettings{
				networkName: {Aliases: []string{"browser"}},
			},
		},
		nil,
		browserCName,
	)
	if err != nil {
		_ = p.cli.NetworkRemove(ctx, networkName)
		return "", "", fmt.Errorf("create browser container: %w", err)
	}

	muxCName := "bg-mux-" + shortID
	muxEnv := []string{
		fmt.Sprintf("PORT=%d", p.defaultPortMux),
		fmt.Sprintf("BROWSER_URL=http://localhost:%d", p.defaultPortDev),
	}

	muxResp, err := p.cli.ContainerCreate(ctx,
		&container.Config{
			Image: p.imageBrowserMux,
			Env:   muxEnv,
			Labels: map[string]string{
				"com.browsergrid.session": sess.ID.String(),
			},
			ExposedPorts: natSet(
				fmt.Sprintf("%d/tcp", p.defaultPortMux),
			),
		},
		&container.HostConfig{
			AutoRemove: false,
			PortBindings: natMap(
				p.defaultPortMux, 0),
		},
		&network.NetworkingConfig{
			EndpointsConfig: map[string]*network.EndpointSettings{
				networkName: {},
			},
		},
		nil,
		muxCName,
	)
	if err != nil {
		if !p.keepContainers() {
			_ = p.cli.ContainerRemove(ctx, browserResp.ID, container.RemoveOptions{Force: true})
		}
		_ = p.cli.NetworkRemove(ctx, networkName)
		return "", "", fmt.Errorf("create mux container: %w", err)
	}

	if err := p.cli.ContainerStart(ctx, browserResp.ID, container.StartOptions{}); err != nil {
		return p.abortStart(ctx, containerNetwork, browserResp.ID, muxResp.ID, fmt.Errorf("start browser: %w", err))
	}

	// Wait for browser to be healthy before starting mux
	if err := p.waitForBrowser(ctx, browserResp.ID); err != nil {
		return p.abortStart(ctx, containerNetwork, browserResp.ID, muxResp.ID, err)
	}

	if err := p.cli.ContainerStart(ctx, muxResp.ID, container.StartOptions{}); err != nil {
		return p.abortStart(ctx, containerNetwork, browserResp.ID, muxResp.ID, fmt.Errorf("start mux: %w", err))
	}

	hostPort, err := p.waitForMux(ctx, muxResp.ID)
	if err != nil {
		return p.abortStart(ctx, containerNetwork, browserResp.ID, muxResp.ID, err)
	}

	// Use host-mapped address that is reachable from inside a container
	hostName := "localhost"
	if runningInDocker() {
		hostName = "host.docker.internal"
	}

	sess.ContainerID = &browserResp.ID
	sess.ContainerNetwork = &containerNetwork
	sess.WSEndpoint = strPtr(fmt.Sprintf("ws://%s:%d", hostName, hostPort))
	sess.LiveURL = strPtr(fmt.Sprintf("http://%s:%d", hostName, hostPort))

	return *sess.WSEndpoint, *sess.LiveURL, nil
}

func (p *DockerProvisioner) Stop(ctx context.Context, sess *sessions.Session) error {
	if sess.ContainerID != nil {
		// Remove network first
		if sess.ContainerNetwork != nil {
			_ = p.cli.NetworkRemove(ctx, *sess.ContainerNetwork)
		}

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

func (p *DockerProvisioner) createNetwork(ctx context.Context, name string) error {
	_, err := p.cli.NetworkCreate(ctx, name, network.CreateOptions{
		Driver:     "bridge",
		Attachable: true,
	})
	if err != nil && strings.Contains(err.Error(), "already exists") {
		return nil
	}
	return err
}

func (p *DockerProvisioner) waitForMux(ctx context.Context, muxID string) (hostPort int, err error) {
	deadline := time.Now().Add(p.healthTimeout)

	for time.Now().Before(deadline) {
		inspect, err := p.cli.ContainerInspect(ctx, muxID)
		if err != nil {
			return 0, err
		}

		// Check if container is running
		if !inspect.State.Running {
			time.Sleep(200 * time.Millisecond)
			continue
		}

		for pc := range inspect.NetworkSettings.Ports {
			if pc.Int() == p.defaultPortMux && len(inspect.NetworkSettings.Ports[pc]) > 0 {
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
	return 0, fmt.Errorf("browsermux did not become ready within %s", p.healthTimeout)
}

func (p *DockerProvisioner) waitForBrowser(ctx context.Context, browserID string) error {
	deadline := time.Now().Add(p.healthTimeout)

	for time.Now().Before(deadline) {
		inspect, err := p.cli.ContainerInspect(ctx, browserID)
		if err != nil {
			return fmt.Errorf("inspect browser container: %w", err)
		}

		// Check if container is running and healthy
		if inspect.State.Running {
			if inspect.State.Health != nil {
				if inspect.State.Health.Status == "healthy" {
					return nil
				}
			} else {
				// If no health check defined, just check if it's been running for a bit
				if startedAt, err := time.Parse(time.RFC3339Nano, inspect.State.StartedAt); err == nil {
					if time.Since(startedAt) > 2*time.Second {
						return nil
					}
				}
			}
		}

		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("browser did not become healthy within %s", p.healthTimeout)
}

func (p *DockerProvisioner) abortStart(
	ctx context.Context,
	netName, browserID, muxID string,
	rootErr error,
) (string, string, error) {
	if !p.keepContainers() {
		_ = p.cli.ContainerRemove(ctx, browserID, container.RemoveOptions{Force: true})
		_ = p.cli.ContainerRemove(ctx, muxID, container.RemoveOptions{Force: true})
	}
	_ = p.cli.NetworkRemove(ctx, netName)
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
