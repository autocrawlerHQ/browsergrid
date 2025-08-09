package profiles

import (
	"fmt"
	"os"
)

// BackendType represents a storage backend for profiles
type BackendType string

const (
	BackendLocal  BackendType = "local"
	BackendBlobFS BackendType = "blobfs" // example blob-like backend using filesystem for persistence
)

// StorageOptions configures the profile storage backend
type StorageOptions struct {
	Backend BackendType

	// Local options
	LocalPath string

	// BlobFS options (example blob store persisted on filesystem)
	BlobPath  string // where compressed profiles (ZIPs) are stored
	CachePath string // where profiles are extracted for mounting
}

// Factory creates a ProfileStore from options
type Factory func(opts StorageOptions) (ProfileStore, error)

var backendRegistry = map[BackendType]Factory{}

// RegisterBackend registers a profile storage backend
func RegisterBackend(t BackendType, f Factory) {
	backendRegistry[t] = f
}

// NewFromOptions creates a ProfileStore from the provided options
func NewFromOptions(opts StorageOptions) (ProfileStore, error) {
	if f, ok := backendRegistry[opts.Backend]; ok {
		return f(opts)
	}
	return nil, fmt.Errorf("unknown profile storage backend: %s", opts.Backend)
}

// ResolveFromEnv builds StorageOptions from environment variables and returns a ProfileStore.
//
// Environment:
// - BROWSERGRID_PROFILE_STORAGE: local | blobfs (default: local)
// - BROWSERGRID_PROFILES_PATH: base path for local backend (default: /var/lib/browsergrid/profiles)
// - BROWSERGRID_PROFILE_BLOB_PATH: directory to persist ZIPs for blobfs backend (default: /var/lib/browsergrid/profile-blobs)
// - BROWSERGRID_PROFILE_CACHE_PATH: extraction/cache directory for blobfs backend (default: /var/lib/browsergrid/profile-cache)
func ResolveFromEnv() (ProfileStore, error) {
	backend := BackendType(getenvOr("BROWSERGRID_PROFILE_STORAGE", string(BackendLocal)))

	switch backend {
	case BackendLocal:
		return NewLocalProfileStore(getenvOr("BROWSERGRID_PROFILES_PATH", ""))
	case BackendBlobFS:
		opts := StorageOptions{
			Backend:   BackendBlobFS,
			BlobPath:  getenvOr("BROWSERGRID_PROFILE_BLOB_PATH", "/var/lib/browsergrid/profile-blobs"),
			CachePath: getenvOr("BROWSERGRID_PROFILE_CACHE_PATH", "/var/lib/browsergrid/profile-cache"),
		}
		return NewBlobFSProfileStore(opts)
	default:
		return nil, fmt.Errorf("unsupported profile storage backend: %s", backend)
	}
}

func getenvOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
