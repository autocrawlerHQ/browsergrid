package deployments

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

// ArtifactStore defines how deployment packages are stored and fetched.
// Implementations can use S3, Azure Blob, GCS, or simple HTTP/local files.
type ArtifactStore interface {
	// Fetch downloads the artifact identified by URL into a local file path and returns that path
	Fetch(ctx context.Context, url string, expectedHash string) (localPath string, err error)
}

// HTTPArtifactStore is a simple implementation that fetches artifacts via HTTP/HTTPS.
// This is provider-agnostic and can be swapped with S3/Azure implementations.
type HTTPArtifactStore struct {
	WorkDir string
}

func NewHTTPArtifactStore(workDir string) *HTTPArtifactStore {
	if workDir == "" {
		workDir = "/tmp/deployments"
	}
	return &HTTPArtifactStore{WorkDir: workDir}
}

func (s *HTTPArtifactStore) Fetch(ctx context.Context, url string, expectedHash string) (string, error) {
	dir := filepath.Join(s.WorkDir, "packages")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download failed: status %d", resp.StatusCode)
	}
	f, err := os.CreateTemp(dir, "artifact-*.zip")
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		os.Remove(f.Name())
		return "", err
	}
	if err := f.Close(); err != nil {
		os.Remove(f.Name())
		return "", err
	}
	// TODO: verify expectedHash when provided
	return f.Name(), nil
}
