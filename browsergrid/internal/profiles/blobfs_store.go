package profiles

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// BlobFSProfileStore is a reference blob-like backend that keeps a compressed ZIP
// per profile at BlobPath and extracts to CachePath for mounting. It demonstrates
// a remote-blob-style contract without coupling to a specific cloud SDK. Swapping
// BlobFS with S3/Azure is a matter of implementing the same methods using cloud SDKs.
type BlobFSProfileStore struct {
	blobPath  string
	cachePath string
}

// NewBlobFSProfileStore creates a new blob-like profile store on filesystem
func NewBlobFSProfileStore(opts StorageOptions) (*BlobFSProfileStore, error) {
	if opts.BlobPath == "" {
		opts.BlobPath = "/var/lib/browsergrid/profile-blobs"
	}
	if opts.CachePath == "" {
		opts.CachePath = "/var/lib/browsergrid/profile-cache"
	}

	if err := os.MkdirAll(opts.BlobPath, 0755); err != nil {
		return nil, fmt.Errorf("create blob path: %w", err)
	}
	if err := os.MkdirAll(opts.CachePath, 0755); err != nil {
		return nil, fmt.Errorf("create cache path: %w", err)
	}

	s := &BlobFSProfileStore{blobPath: opts.BlobPath, cachePath: opts.CachePath}
	// Register in the backend registry when package is imported
	RegisterBackend(BackendBlobFS, func(opts StorageOptions) (ProfileStore, error) {
		return NewBlobFSProfileStore(opts)
	})
	return s, nil
}

// InitializeProfile creates an empty extracted cache dir and writes an empty zip
func (s *BlobFSProfileStore) InitializeProfile(ctx context.Context, profileID string) error {
	// Create empty cache dir
	mountPath := s.mountPath(profileID)
	if err := os.MkdirAll(filepath.Join(mountPath, "user-data"), 0755); err != nil {
		return fmt.Errorf("create cache: %w", err)
	}
	// Save initial ZIP artifact (empty)
	return s.SaveProfileData(ctx, profileID, mountPath)
}

// ImportProfile takes a ZIP stream and persists it as the canonical blob, then refreshes cache
func (s *BlobFSProfileStore) ImportProfile(ctx context.Context, profileID string, zipData io.Reader) error {
	// Store canonical ZIP
	zipPath := s.zipPath(profileID)
	if err := os.MkdirAll(filepath.Dir(zipPath), 0755); err != nil {
		return fmt.Errorf("prepare blob dir: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(zipPath), "profile-*.zip")
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	defer func() { tmp.Close(); os.Remove(tmp.Name()) }()
	if _, err := io.Copy(tmp, zipData); err != nil {
		return fmt.Errorf("write temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp.Name(), zipPath); err != nil {
		return fmt.Errorf("persist zip: %w", err)
	}
	// Refresh cache
	mountPath := s.mountPath(profileID)
	_ = os.RemoveAll(mountPath)
	if err := os.MkdirAll(mountPath, 0755); err != nil {
		return fmt.Errorf("prepare cache: %w", err)
	}
	return s.extractZip(zipPath, mountPath)
}

// GetProfilePath returns the extracted cache dir for mounting into engines
func (s *BlobFSProfileStore) GetProfilePath(ctx context.Context, profileID string) (string, error) {
	mount := s.mountPath(profileID)
	if _, err := os.Stat(filepath.Join(mount, "user-data")); os.IsNotExist(err) {
		// Attempt lazy hydrate from blob
		if err := s.hydrateFromZip(profileID); err != nil {
			return "", fmt.Errorf("profile not found or hydrate failed: %w", err)
		}
	}
	return filepath.Join(mount, "user-data"), nil
}

// SaveProfileData zips the current cache dir and writes it to blob storage
// sourcePath, if non-empty, will be copied over the cache's user-data before zipping
func (s *BlobFSProfileStore) SaveProfileData(ctx context.Context, profileID string, sourcePath string) error {
	mount := s.mountPath(profileID)
	userData := filepath.Join(mount, "user-data")
	if err := os.MkdirAll(userData, 0755); err != nil {
		return err
	}
	if sourcePath != "" {
		// Best-effort copy from sourcePath into userData
		if err := copyDir(sourcePath, userData); err != nil {
			return fmt.Errorf("copy source: %w", err)
		}
	}
	// Create ZIP
	zipPath := s.zipPath(profileID)
	if err := os.MkdirAll(filepath.Dir(zipPath), 0755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(zipPath), "profile-*.zip")
	if err != nil {
		return err
	}
	if err := s.createZip(userData, tmp); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return err
	}
	if _, err := tmp.Seek(0, 0); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	if err := os.Rename(tmp.Name(), zipPath); err != nil {
		return err
	}
	return nil
}

// ExportProfile returns the canonical ZIP as a reader
func (s *BlobFSProfileStore) ExportProfile(ctx context.Context, profileID string) (io.ReadCloser, error) {
	zipPath := s.zipPath(profileID)
	f, err := os.Open(zipPath)
	if err != nil {
		return nil, err
	}
	return f, nil
}

// DeleteProfile deletes both blob and cache
func (s *BlobFSProfileStore) DeleteProfile(ctx context.Context, profileID string) error {
	_ = os.Remove(s.zipPath(profileID))
	return os.RemoveAll(s.mountPath(profileID))
}

// GetProfileSize returns the size of the current cache user-data
func (s *BlobFSProfileStore) GetProfileSize(ctx context.Context, profileID string) (int64, error) {
	return dirSize(filepath.Join(s.mountPath(profileID), "user-data"))
}

// ValidateProfile ensures cache folder integrity
func (s *BlobFSProfileStore) ValidateProfile(ctx context.Context, profileID string) error {
	path := filepath.Join(s.mountPath(profileID), "user-data")
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("user-data directory not found")
	}
	if !info.IsDir() {
		return fmt.Errorf("user-data is not a directory")
	}
	return nil
}

// helper methods
func (s *BlobFSProfileStore) zipPath(profileID string) string {
	return filepath.Join(s.blobPath, fmt.Sprintf("%s.zip", profileID))
}

func (s *BlobFSProfileStore) mountPath(profileID string) string {
	return filepath.Join(s.cachePath, profileID)
}

func (s *BlobFSProfileStore) hydrateFromZip(profileID string) error {
	zipPath := s.zipPath(profileID)
	mountPath := s.mountPath(profileID)
	if _, err := os.Stat(zipPath); err != nil {
		return err
	}
	if err := os.MkdirAll(mountPath, 0755); err != nil {
		return err
	}
	return s.extractZip(zipPath, mountPath)
}

func (s *BlobFSProfileStore) extractZip(zipPath, destPath string) error {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer reader.Close()

	for _, file := range reader.File {
		path := filepath.Join(destPath, file.Name)
		if !strings.HasPrefix(path, filepath.Clean(destPath)+string(os.PathSeparator)) {
			return fmt.Errorf("invalid file path: %s", file.Name)
		}
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(path, file.Mode()); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return err
		}
		rc, err := file.Open()
		if err != nil {
			return err
		}
		out, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
		if err != nil {
			rc.Close()
			return err
		}
		if _, err := io.Copy(out, rc); err != nil {
			rc.Close()
			out.Close()
			return err
		}
		rc.Close()
		out.Close()
	}
	return nil
}

func (s *BlobFSProfileStore) createZip(sourceUserData string, zipFile *os.File) error {
	zw := zip.NewWriter(zipFile)
	defer zw.Close()
	return filepath.Walk(sourceUserData, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(sourceUserData, path)
		if err != nil {
			return err
		}
		rel = strings.ReplaceAll(rel, string(filepath.Separator), "/")
		rc, err := os.Open(path)
		if err != nil {
			return err
		}
		w, err := zw.Create(rel)
		if err != nil {
			rc.Close()
			return err
		}
		if _, err := io.Copy(w, rc); err != nil {
			rc.Close()
			return err
		}
		rc.Close()
		return nil
	})
}

func dirSize(root string) (int64, error) {
	var total int64
	err := filepath.Walk(root, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			total += info.Size()
		}
		return nil
	})
	return total, err
}

// copyDir best-effort recursive copy from src to dst
func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}
		// ensure parent
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode())
		if err != nil {
			in.Close()
			return err
		}
		if _, err := io.Copy(out, in); err != nil {
			in.Close()
			out.Close()
			return err
		}
		in.Close()
		out.Close()
		// preserve modtime
		return os.Chtimes(target, time.Now(), info.ModTime())
	})
}
