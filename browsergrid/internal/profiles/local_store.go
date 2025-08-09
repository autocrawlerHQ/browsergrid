package profiles

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	defaultLocalProfilesPath = "/var/lib/browsergrid/profiles"
	localMetadataFileName    = "metadata.json"
	localUserDataDirName     = "user-data"
	localMaxProfileSize      = 1 << 30 // 1GB
)

// LocalProfileStore implements ProfileStore using local filesystem
type LocalProfileStore struct {
	basePath string
}

// NewLocalProfileStore creates a new local filesystem profile store
func NewLocalProfileStore(basePath string) (*LocalProfileStore, error) {
	if basePath == "" {
		basePath = defaultLocalProfilesPath
	}

	// Ensure base directory exists
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, fmt.Errorf("create base directory: %w", err)
	}

	return &LocalProfileStore{
		basePath: basePath,
	}, nil
}

// InitializeProfile creates an empty profile directory
func (s *LocalProfileStore) InitializeProfile(ctx context.Context, profileID string) error {
	profilePath := s.getProfilePath(profileID)

	// Create profile directory
	if err := os.MkdirAll(profilePath, 0755); err != nil {
		return fmt.Errorf("create profile directory: %w", err)
	}

	// Create user-data directory
	userDataPath := filepath.Join(profilePath, localUserDataDirName)
	if err := os.MkdirAll(userDataPath, 0755); err != nil {
		return fmt.Errorf("create user-data directory: %w", err)
	}

	// Ensure proper ownership for browser containers (UID 1000)
	// This is necessary when running in Docker where the browser containers
	// run as browseruser (UID 1000) but the server may run as root
	if err := s.ensureProperOwnership(profilePath, userDataPath); err != nil {
		// Log but don't fail - ownership issues are common in Docker
		fmt.Printf("Warning: Could not set proper ownership for profile %s: %v\n", profileID, err)
	}

	// Write initial metadata
	metadata := LocalProfileMetadata{
		Version:      1,
		CreatedAt:    time.Now(),
		LastModified: time.Now(),
		SizeBytes:    0,
	}

	return s.writeMetadata(profilePath, metadata)
}

// ImportProfile imports profile data from a ZIP archive
func (s *LocalProfileStore) ImportProfile(ctx context.Context, profileID string, zipData io.Reader) error {
	profilePath := s.getProfilePath(profileID)

	// Create profile directory
	if err := os.MkdirAll(profilePath, 0755); err != nil {
		return fmt.Errorf("create profile directory: %w", err)
	}

	// Create temporary file for ZIP
	tmpFile, err := os.CreateTemp("", "profile-import-*.zip")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())

	// Copy ZIP data to temp file
	size, err := io.Copy(tmpFile, zipData)
	if err != nil {
		return fmt.Errorf("copy zip data: %w", err)
	}

	if size > localMaxProfileSize {
		return fmt.Errorf("profile size %d exceeds maximum %d", size, localMaxProfileSize)
	}

	// Close and reopen for reading
	tmpFile.Close()

	// Extract ZIP to profile directory
	if err := s.extractZip(tmpFile.Name(), profilePath); err != nil {
		os.RemoveAll(profilePath) // Cleanup on error
		return fmt.Errorf("extract zip: %w", err)
	}

	// Validate extracted profile
	if err := s.validateProfileStructure(profilePath); err != nil {
		os.RemoveAll(profilePath) // Cleanup on error
		return fmt.Errorf("invalid profile structure: %w", err)
	}

	// Ensure proper ownership for browser containers
	userDataPath := filepath.Join(profilePath, localUserDataDirName)
	if err := s.ensureProperOwnership(profilePath, userDataPath); err != nil {
		// Log but don't fail - ownership issues are common in Docker
		fmt.Printf("Warning: Could not set proper ownership for imported profile %s: %v\n", profileID, err)
	}

	// Update metadata
	metadata := LocalProfileMetadata{
		Version:      1,
		CreatedAt:    time.Now(),
		LastModified: time.Now(),
		SizeBytes:    size,
	}

	return s.writeMetadata(profilePath, metadata)
}

// GetProfilePath returns the filesystem path for mounting
func (s *LocalProfileStore) GetProfilePath(ctx context.Context, profileID string) (string, error) {
	profilePath := s.getProfilePath(profileID)
	userDataPath := filepath.Join(profilePath, localUserDataDirName)

	// Check if profile exists
	if _, err := os.Stat(userDataPath); os.IsNotExist(err) {
		return "", fmt.Errorf("profile not found: %s", profileID)
	}

	return userDataPath, nil
}

// SaveProfileData is a no-op for local store since data is already persisted via volume mounts
func (s *LocalProfileStore) SaveProfileData(ctx context.Context, profileID string, sourcePath string) error {
	// With direct volume mounting, the data is already persisted in the correct location
	// We just need to update the metadata
	profilePath := s.getProfilePath(profileID)
	userDataPath := filepath.Join(profilePath, localUserDataDirName)

	// Calculate current size
	size, _ := s.calculateDirectorySize(userDataPath)

	// Update metadata
	metadata := LocalProfileMetadata{
		Version:      1,
		LastModified: time.Now(),
		SizeBytes:    size,
	}

	// Read existing metadata to preserve created_at
	if existing, err := s.readMetadata(profilePath); err == nil {
		metadata.CreatedAt = existing.CreatedAt
	}

	return s.writeMetadata(profilePath, metadata)
}

// ExportProfile exports profile data as a ZIP archive
func (s *LocalProfileStore) ExportProfile(ctx context.Context, profileID string) (io.ReadCloser, error) {
	profilePath := s.getProfilePath(profileID)
	userDataPath := filepath.Join(profilePath, localUserDataDirName)

	// Check if profile exists
	if _, err := os.Stat(userDataPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("profile not found: %s", profileID)
	}

	// Create temporary ZIP file
	tmpFile, err := os.CreateTemp("", "profile-export-*.zip")
	if err != nil {
		return nil, fmt.Errorf("create temp file: %w", err)
	}

	// Create ZIP archive from user-data directory
	if err := s.createZip(userDataPath, tmpFile); err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		return nil, fmt.Errorf("create zip: %w", err)
	}

	// Seek to beginning
	if _, err := tmpFile.Seek(0, 0); err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		return nil, fmt.Errorf("seek file: %w", err)
	}

	// Return a reader that cleans up the temp file when closed
	return &localExportReader{
		File:     tmpFile,
		tempPath: tmpFile.Name(),
	}, nil
}

// DeleteProfile removes profile data
func (s *LocalProfileStore) DeleteProfile(ctx context.Context, profileID string) error {
	profilePath := s.getProfilePath(profileID)

	// Check if profile exists
	if _, err := os.Stat(profilePath); os.IsNotExist(err) {
		return nil // Already deleted
	}

	return os.RemoveAll(profilePath)
}

// GetProfileSize returns the size of profile data in bytes
func (s *LocalProfileStore) GetProfileSize(ctx context.Context, profileID string) (int64, error) {
	profilePath := s.getProfilePath(profileID)
	userDataPath := filepath.Join(profilePath, localUserDataDirName)

	return s.calculateDirectorySize(userDataPath)
}

// ValidateProfile checks if profile data is valid
func (s *LocalProfileStore) ValidateProfile(ctx context.Context, profileID string) error {
	profilePath := s.getProfilePath(profileID)
	return s.validateProfileStructure(profilePath)
}

// Helper methods

func (s *LocalProfileStore) getProfilePath(profileID string) string {
	return filepath.Join(s.basePath, profileID)
}

func (s *LocalProfileStore) ensureProperOwnership(profilePath, userDataPath string) error {
	// Try to change ownership to browseruser (UID 1000, GID 1000)
	// This is the standard UID/GID used in browser containers
	const browserUID = 1000
	const browserGID = 1000

	// Change ownership of profile directory
	if err := os.Chown(profilePath, browserUID, browserGID); err != nil {
		return fmt.Errorf("chown profile directory: %w", err)
	}

	// Change ownership of user-data directory
	if err := os.Chown(userDataPath, browserUID, browserGID); err != nil {
		return fmt.Errorf("chown user-data directory: %w", err)
	}

	// Walk through and fix ownership of all existing files/directories
	return filepath.Walk(userDataPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		return os.Chown(path, browserUID, browserGID)
	})
}

func (s *LocalProfileStore) writeMetadata(profilePath string, metadata LocalProfileMetadata) error {
	metadataPath := filepath.Join(profilePath, localMetadataFileName)

	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}

	return os.WriteFile(metadataPath, data, 0644)
}

func (s *LocalProfileStore) readMetadata(profilePath string) (*LocalProfileMetadata, error) {
	metadataPath := filepath.Join(profilePath, localMetadataFileName)

	data, err := os.ReadFile(metadataPath)
	if err != nil {
		return nil, fmt.Errorf("read metadata: %w", err)
	}

	var metadata LocalProfileMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, fmt.Errorf("unmarshal metadata: %w", err)
	}

	return &metadata, nil
}

func (s *LocalProfileStore) validateProfileStructure(profilePath string) error {
	userDataPath := filepath.Join(profilePath, localUserDataDirName)

	// Check if user-data directory exists
	info, err := os.Stat(userDataPath)
	if err != nil {
		return fmt.Errorf("user-data directory not found")
	}

	if !info.IsDir() {
		return fmt.Errorf("user-data is not a directory")
	}

	return nil
}

func (s *LocalProfileStore) calculateDirectorySize(path string) (int64, error) {
	var size int64

	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})

	return size, err
}

func (s *LocalProfileStore) extractZip(zipPath, destPath string) error {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer reader.Close()

	for _, file := range reader.File {
		path := filepath.Join(destPath, file.Name)

		// Prevent directory traversal
		if !strings.HasPrefix(path, filepath.Clean(destPath)+string(os.PathSeparator)) {
			return fmt.Errorf("invalid file path: %s", file.Name)
		}

		if file.FileInfo().IsDir() {
			os.MkdirAll(path, file.Mode())
			continue
		}

		fileReader, err := file.Open()
		if err != nil {
			return err
		}
		defer fileReader.Close()

		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return err
		}

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

func (s *LocalProfileStore) createZip(sourcePath string, zipFile *os.File) error {
	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	return filepath.Walk(sourcePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(sourcePath, path)
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		// Normalize path separators to forward slashes for ZIP format
		relPath = strings.ReplaceAll(relPath, string(filepath.Separator), "/")

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

// LocalProfileMetadata represents metadata stored with each profile
type LocalProfileMetadata struct {
	Version      int       `json:"version"`
	CreatedAt    time.Time `json:"created_at"`
	LastModified time.Time `json:"last_modified"`
	SizeBytes    int64     `json:"size_bytes"`
	Browser      string    `json:"browser,omitempty"`
}

// localExportReader wraps an os.File and deletes the temp file on Close
type localExportReader struct {
	*os.File
	tempPath string
}

func (r *localExportReader) Close() error {
	err := r.File.Close()
	os.Remove(r.tempPath)
	return err
}
