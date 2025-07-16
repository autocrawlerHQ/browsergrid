package profiles

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"path"
	"strings"
	"time"

	"github.com/autocrawlerHQ/browsergrid/internal/storage"
)

type StorageProfileStore struct {
	storage storage.Storage
	prefix  string
}

func NewStorageProfileStore(storage storage.Storage) *StorageProfileStore {
	return &StorageProfileStore{
		storage: storage,
		prefix:  "profiles",
	}
}

func (s *StorageProfileStore) InitializeProfile(ctx context.Context, profileID string) error {
	prefsKey := s.profileKey(profileID, "user-data/Default/Preferences")
	if err := s.storage.Put(ctx, prefsKey, strings.NewReader("{}"),
		storage.WithContentType("application/json")); err != nil {
		return fmt.Errorf("store preferences: %w", err)
	}

	return nil
}

func (s *StorageProfileStore) ImportProfile(ctx context.Context, profileID string, zipData io.Reader) error {
	buf := new(bytes.Buffer)
	size, err := io.Copy(buf, zipData)
	if err != nil {
		return fmt.Errorf("read zip data: %w", err)
	}

	if size > maxProfileSize {
		return fmt.Errorf("profile size %d exceeds maximum %d", size, maxProfileSize)
	}

	zipReader, err := zip.NewReader(bytes.NewReader(buf.Bytes()), size)
	if err != nil {
		return fmt.Errorf("open zip: %w", err)
	}

	for _, file := range zipReader.File {
		if file.FileInfo().IsDir() {
			continue
		}

		reader, err := file.Open()
		if err != nil {
			return fmt.Errorf("open zip file %s: %w", file.Name, err)
		}

		key := s.profileKey(profileID, "user-data/"+file.Name)
		if err := s.storage.Put(ctx, key, reader,
			storage.WithContentType(detectContentType(file.Name))); err != nil {
			reader.Close()
			return fmt.Errorf("store file %s: %w", file.Name, err)
		}
		reader.Close()
	}

	return nil
}

func (s *StorageProfileStore) GetProfilePath(ctx context.Context, profileID string) (string, error) {
	baseKey := s.profileKey(profileID, "user-data")
	objects, err := s.storage.List(ctx, baseKey)
	if err != nil {
		return "", fmt.Errorf("check profile existence: %w", err)
	}
	if len(objects) == 0 {
		return "", fmt.Errorf("profile not found: %s", profileID)
	}

	url, err := s.storage.SignedURL(ctx, baseKey,
		storage.WithMethod("GET"),
		storage.WithExpires(24*time.Hour))
	if err != nil {
		return baseKey, nil
	}

	return url, nil
}

func (s *StorageProfileStore) SaveProfileData(ctx context.Context, profileID string, sourcePath string) error {
	return nil
}

func (s *StorageProfileStore) ExportProfile(ctx context.Context, profileID string) (io.ReadCloser, error) {
	prefix := s.profileKey(profileID, "user-data/")
	objects, err := s.storage.List(ctx, prefix)
	if err != nil {
		return nil, fmt.Errorf("list profile files: %w", err)
	}

	if len(objects) == 0 {
		return nil, fmt.Errorf("profile is empty")
	}

	buf := new(bytes.Buffer)
	zipWriter := zip.NewWriter(buf)

	for _, obj := range objects {
		relPath := path.Base(obj.Key)

		reader, err := s.storage.Get(ctx, obj.Key)
		if err != nil {
			zipWriter.Close()
			return nil, fmt.Errorf("read file %s: %w", obj.Key, err)
		}

		writer, err := zipWriter.Create(relPath)
		if err != nil {
			reader.Close()
			zipWriter.Close()
			return nil, fmt.Errorf("create zip entry: %w", err)
		}

		if _, err := io.Copy(writer, reader); err != nil {
			reader.Close()
			zipWriter.Close()
			return nil, fmt.Errorf("write to zip: %w", err)
		}
		reader.Close()
	}

	if err := zipWriter.Close(); err != nil {
		return nil, fmt.Errorf("close zip: %w", err)
	}

	return io.NopCloser(bytes.NewReader(buf.Bytes())), nil
}

func (s *StorageProfileStore) DeleteProfile(ctx context.Context, profileID string) error {
	prefix := s.profileKey(profileID, "")
	objects, err := s.storage.List(ctx, prefix)
	if err != nil {
		return fmt.Errorf("list profile objects: %w", err)
	}

	for _, obj := range objects {
		if err := s.storage.Delete(ctx, obj.Key); err != nil {
			return fmt.Errorf("delete object %s: %w", obj.Key, err)
		}
	}

	return nil
}

func (s *StorageProfileStore) GetProfileSize(ctx context.Context, profileID string) (int64, error) {
	prefix := s.profileKey(profileID, "")
	objects, err := s.storage.List(ctx, prefix)
	if err != nil {
		return 0, fmt.Errorf("list profile objects: %w", err)
	}

	var totalSize int64
	for _, obj := range objects {
		totalSize += obj.Size
	}

	return totalSize, nil
}

func (s *StorageProfileStore) ValidateProfile(ctx context.Context, profileID string) error {
	requiredFiles := []string{
		"user-data/Default/Preferences",
	}

	for _, file := range requiredFiles {
		key := s.profileKey(profileID, file)
		exists, err := s.storage.Exists(ctx, key)
		if err != nil {
			return fmt.Errorf("check file %s: %w", file, err)
		}
		if !exists {
			return fmt.Errorf("missing required file: %s", file)
		}
	}

	return nil
}

func (s *StorageProfileStore) profileKey(profileID, suffix string) string {
	if suffix == "" {
		return path.Join(s.prefix, profileID)
	}
	return path.Join(s.prefix, profileID, suffix)
}

func detectContentType(filename string) string {
	switch path.Ext(filename) {
	case ".json":
		return "application/json"
	case ".db", ".sqlite":
		return "application/x-sqlite3"
	case ".log":
		return "text/plain"
	default:
		return "application/octet-stream"
	}
}
