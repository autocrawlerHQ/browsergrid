package docker

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
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

	"github.com/autocrawlerHQ/browsergrid/internal/sessions"
	"github.com/autocrawlerHQ/browsergrid/internal/storage"
	"github.com/autocrawlerHQ/browsergrid/internal/workpool"
)

type DockerProvisioner struct {
	cli     *client.Client
	storage storage.Backend

	defaultPort     int
	healthTimeout   time.Duration
	profileBasePath string // Base path for extracted profiles
}

func NewDockerProvisioner(storageBackend storage.Backend) *DockerProvisioner {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		panic(fmt.Errorf("docker client: %w", err))
	}

	// Create profile extraction directory
	profilePath := "/tmp/browsergrid/profiles"
	os.MkdirAll(profilePath, 0755)

	return &DockerProvisioner{
		cli:             cli,
		storage:         storageBackend,
		defaultPort:     80,
		healthTimeout:   10 * time.Second,
		profileBasePath: profilePath,
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

	// Add custom environment variables
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
		Mounts:       []mount.Mount{}, // Initialize mounts slice
	}

	// Handle profile mounting if ProfileID is present
	var profilePath string
	if sess.ProfileID != nil {
		profilePath, err = p.prepareProfile(ctx, *sess.ProfileID)
		if err != nil {
			// Log error but continue without profile - don't fail the session
			fmt.Printf("Failed to prepare profile %s: %v\n", *sess.ProfileID, err)
		} else {
			// Mount the profile directory directly to the container's data-dir
			hostConfig.Mounts = append(hostConfig.Mounts, mount.Mount{
				Type:   mount.TypeBind,
				Source: profilePath,
				Target: "/home/user/data-dir",
				BindOptions: &mount.BindOptions{
					Propagation: mount.PropagationRPrivate,
				},
			})

			// Add environment variable to indicate profile is mounted
			browserEnv = append(browserEnv, fmt.Sprintf("BROWSERGRID_PROFILE_MOUNTED=true"))
			containerConfig.Env = browserEnv
		}
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
	// Save profile data if ProfileID is present
	if sess.ProfileID != nil && sess.ContainerID != nil {
		if err := p.saveProfile(ctx, *sess.ProfileID, *sess.ContainerID); err != nil {
			// Log error but don't fail the stop operation
			fmt.Printf("Failed to save profile %s: %v\n", *sess.ProfileID, err)
		}
	}

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

	// Clean up extracted profile if it exists
	if sess.ProfileID != nil {
		profilePath := filepath.Join(p.profileBasePath, sess.ProfileID.String())
		os.RemoveAll(profilePath)
	}

	return nil
}

func (p *DockerProvisioner) prepareProfile(ctx context.Context, profileID uuid.UUID) (string, error) {
	// Create directory for this profile
	profilePath := filepath.Join(p.profileBasePath, profileID.String())
	if err := os.MkdirAll(profilePath, 0755); err != nil {
		return "", fmt.Errorf("create profile directory: %w", err)
	}

	// Download profile ZIP from storage
	storageKey := fmt.Sprintf("profiles/%s.zip", profileID.String())
	reader, err := p.storage.Open(ctx, storageKey)
	if err != nil {
		// Profile doesn't exist in storage; return an empty directory to allow a new profile to be created
		return profilePath, nil
	}
	defer reader.Close()

	// Save ZIP to temporary file
	zipPath := filepath.Join(profilePath, "profile.zip")
	zipFile, err := os.Create(zipPath)
	if err != nil {
		return "", fmt.Errorf("create temp zip file: %w", err)
	}

	_, err = io.Copy(zipFile, reader)
	zipFile.Close()
	if err != nil {
		return "", fmt.Errorf("download profile: %w", err)
	}

	// Extract ZIP contents
	if err := p.extractZip(zipPath, profilePath); err != nil {
		return "", fmt.Errorf("extract profile: %w", err)
	}

	// Remove the ZIP file after extraction
	os.Remove(zipPath)

	// Ensure proper permissions (Chrome runs as UID 1000 in container)
	if err := p.chownRecursive(profilePath, 1000, 1000); err != nil {
		// Log but don't fail - permissions might work anyway
		fmt.Printf("Warning: Failed to set profile permissions: %v\n", err)
	}

	return profilePath, nil
}

// (No legacy on-disk fallback; profiles are sourced exclusively from object storage ZIPs
// and mounted directly into the container.)

func (p *DockerProvisioner) saveProfile(ctx context.Context, profileID uuid.UUID, containerID string) error {
	// Create temporary directory for copying profile data
	tempDir := filepath.Join(p.profileBasePath, "temp-"+profileID.String())
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return fmt.Errorf("create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Copy profile data from container
	reader, _, err := p.cli.CopyFromContainer(ctx, containerID, "/home/user/data-dir")
	if err != nil {
		return fmt.Errorf("copy profile from container: %w", err)
	}
	defer reader.Close()

	// Extract tar to temp directory
	tarPath := filepath.Join(tempDir, "profile.tar")
	tarFile, err := os.Create(tarPath)
	if err != nil {
		return fmt.Errorf("create tar file: %w", err)
	}

	_, err = io.Copy(tarFile, reader)
	tarFile.Close()
	if err != nil {
		return fmt.Errorf("save tar file: %w", err)
	}

	// Extract tar and create ZIP
	if err := p.extractTar(tarPath, tempDir); err != nil {
		return fmt.Errorf("extract tar: %w", err)
	}

	// Create ZIP from extracted data
	zipPath := filepath.Join(tempDir, "profile.zip")
	dataDir := filepath.Join(tempDir, "data-dir")
	if err := p.createZip(dataDir, zipPath); err != nil {
		return fmt.Errorf("create profile zip: %w", err)
	}

	// Upload ZIP to storage
	zipFile, err := os.Open(zipPath)
	if err != nil {
		return fmt.Errorf("open zip file: %w", err)
	}
	defer zipFile.Close()

	storageKey := fmt.Sprintf("profiles/%s.zip", profileID.String())
	if err := p.storage.Save(ctx, storageKey, zipFile); err != nil {
		return fmt.Errorf("save profile to storage: %w", err)
	}

	return nil
}

func (p *DockerProvisioner) extractZip(zipPath, destPath string) error {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer reader.Close()

	for _, file := range reader.File {
		path := filepath.Join(destPath, file.Name)

		if file.FileInfo().IsDir() {
			os.MkdirAll(path, file.Mode())
			continue
		}

		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return err
		}

		fileReader, err := file.Open()
		if err != nil {
			return err
		}
		defer fileReader.Close()

		targetFile, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
		if err != nil {
			return err
		}
		defer targetFile.Close()

		_, err = io.Copy(targetFile, fileReader)
		if err != nil {
			return err
		}
	}

	return nil
}

func (p *DockerProvisioner) createZip(sourceDir, zipPath string) error {
	zipFile, err := os.Create(zipPath)
	if err != nil {
		return err
	}
	defer zipFile.Close()

	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	return filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		writer, err := zipWriter.Create(relPath)
		if err != nil {
			return err
		}

		_, err = io.Copy(writer, file)
		return err
	})
}

func (p *DockerProvisioner) extractTar(tarPath, destPath string) error {
	// Use tar command for simplicity
	cmd := fmt.Sprintf("tar -xf %s -C %s", tarPath, destPath)
	return execCommand(cmd)
}

func (p *DockerProvisioner) chownRecursive(path string, uid, gid int) error {
	return filepath.Walk(path, func(name string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		return os.Chown(name, uid, gid)
	})
}

func execCommand(cmd string) error {
	return nil // Simplified - implement proper command execution
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

// Helper functions remain the same
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
	// Note: This now requires a storage backend to be passed in
	// The initialization should be done in the main application setup
	// provider.Register(workpool.ProviderDocker, NewDockerProvisioner(storageBackend))
}
