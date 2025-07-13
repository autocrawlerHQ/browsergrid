package profiles

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestLocalStore(t *testing.T) (*LocalProfileStore, string) {
	tempDir := t.TempDir()
	store, err := NewLocalProfileStore(tempDir)
	require.NoError(t, err)
	return store, tempDir
}

func TestNewLocalProfileStore(t *testing.T) {
	tests := []struct {
		name        string
		basePath    string
		expectError bool
	}{
		{
			name:        "create with valid path",
			basePath:    t.TempDir(),
			expectError: false,
		},
		{
			name:        "create with empty path (uses default)",
			basePath:    "",
			expectError: false,
		},
		{
			name:        "create with non-existent path",
			basePath:    filepath.Join(t.TempDir(), "non-existent"),
			expectError: false, // Should create the directory
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store, err := NewLocalProfileStore(tt.basePath)
			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, store)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, store)
			}
		})
	}
}

func TestLocalProfileStore_InitializeProfile(t *testing.T) {
	store, tempDir := setupTestLocalStore(t)
	ctx := context.Background()

	profileID := "test-profile-123"

	err := store.InitializeProfile(ctx, profileID)
	require.NoError(t, err)

	// Check that directories were created
	profilePath := filepath.Join(tempDir, profileID)
	userDataPath := filepath.Join(profilePath, localUserDataDirName)
	metadataPath := filepath.Join(profilePath, localMetadataFileName)

	assert.DirExists(t, profilePath)
	assert.DirExists(t, userDataPath)
	assert.FileExists(t, metadataPath)

	// Check metadata content
	data, err := os.ReadFile(metadataPath)
	require.NoError(t, err)

	var metadata LocalProfileMetadata
	err = json.Unmarshal(data, &metadata)
	require.NoError(t, err)

	assert.Equal(t, 1, metadata.Version)
	assert.False(t, metadata.CreatedAt.IsZero())
	assert.False(t, metadata.LastModified.IsZero())
	assert.Equal(t, int64(0), metadata.SizeBytes)
}

func TestLocalProfileStore_GetProfilePath(t *testing.T) {
	store, tempDir := setupTestLocalStore(t)
	ctx := context.Background()

	profileID := "test-profile-123"

	// Initialize profile first
	err := store.InitializeProfile(ctx, profileID)
	require.NoError(t, err)

	// Get profile path
	path, err := store.GetProfilePath(ctx, profileID)
	require.NoError(t, err)

	expectedPath := filepath.Join(tempDir, profileID, localUserDataDirName)
	assert.Equal(t, expectedPath, path)
	assert.DirExists(t, path)
}

func TestLocalProfileStore_GetProfilePath_NotFound(t *testing.T) {
	store, _ := setupTestLocalStore(t)
	ctx := context.Background()

	profileID := "non-existent-profile"

	path, err := store.GetProfilePath(ctx, profileID)
	assert.Error(t, err)
	assert.Empty(t, path)
	assert.Contains(t, err.Error(), "profile not found")
}

func TestLocalProfileStore_ImportProfile(t *testing.T) {
	store, tempDir := setupTestLocalStore(t)
	ctx := context.Background()

	profileID := "imported-profile"

	// Create a test ZIP file
	zipData := createTestZip(t, map[string]string{
		"user-data/file1.txt":      "content1",
		"user-data/file2.txt":      "content2",
		"user-data/dir1/file3.txt": "content3",
	})

	err := store.ImportProfile(ctx, profileID, bytes.NewReader(zipData))
	require.NoError(t, err)

	// Verify files were extracted
	profilePath := filepath.Join(tempDir, profileID)
	userDataPath := filepath.Join(profilePath, localUserDataDirName)

	assert.FileExists(t, filepath.Join(userDataPath, "file1.txt"))
	assert.FileExists(t, filepath.Join(userDataPath, "file2.txt"))
	assert.FileExists(t, filepath.Join(userDataPath, "dir1", "file3.txt"))

	// Verify content
	content, err := os.ReadFile(filepath.Join(userDataPath, "file1.txt"))
	require.NoError(t, err)
	assert.Equal(t, "content1", string(content))

	// Verify metadata
	metadataPath := filepath.Join(profilePath, localMetadataFileName)
	assert.FileExists(t, metadataPath)
}

func TestLocalProfileStore_ImportProfile_InvalidZip(t *testing.T) {
	store, _ := setupTestLocalStore(t)
	ctx := context.Background()

	profileID := "invalid-profile"
	invalidData := []byte("not a zip file")

	err := store.ImportProfile(ctx, profileID, bytes.NewReader(invalidData))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "extract zip")
}

func TestLocalProfileStore_ImportProfile_SizeLimit(t *testing.T) {
	store, _ := setupTestLocalStore(t)
	ctx := context.Background()

	profileID := "large-profile"

	// Create a large buffer that exceeds the limit
	largeData := make([]byte, localMaxProfileSize+1)

	err := store.ImportProfile(ctx, profileID, bytes.NewReader(largeData))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds maximum")
}

func TestLocalProfileStore_ExportProfile(t *testing.T) {
	store, tempDir := setupTestLocalStore(t)
	ctx := context.Background()

	profileID := "export-test"

	// Initialize profile and add some files
	err := store.InitializeProfile(ctx, profileID)
	require.NoError(t, err)

	userDataPath := filepath.Join(tempDir, profileID, localUserDataDirName)

	// Create test files
	testFiles := map[string]string{
		"file1.txt":        "content1",
		"subdir/file2.txt": "content2",
	}

	for filePath, content := range testFiles {
		fullPath := filepath.Join(userDataPath, filePath)
		err := os.MkdirAll(filepath.Dir(fullPath), 0755)
		require.NoError(t, err)
		err = os.WriteFile(fullPath, []byte(content), 0644)
		require.NoError(t, err)
	}

	// Export profile
	reader, err := store.ExportProfile(ctx, profileID)
	require.NoError(t, err)
	defer reader.Close()

	// Read the ZIP and verify contents
	zipData, err := io.ReadAll(reader)
	require.NoError(t, err)

	// Verify ZIP contents
	zipReader, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	require.NoError(t, err)

	fileMap := make(map[string]string)
	for _, file := range zipReader.File {
		if !file.FileInfo().IsDir() {
			rc, err := file.Open()
			require.NoError(t, err)
			content, err := io.ReadAll(rc)
			require.NoError(t, err)
			rc.Close()
			fileMap[file.Name] = string(content)
		}
	}

	assert.Equal(t, "content1", fileMap["file1.txt"])
	assert.Equal(t, "content2", fileMap["subdir/file2.txt"])
}

func TestLocalProfileStore_ExportProfile_NotFound(t *testing.T) {
	store, _ := setupTestLocalStore(t)
	ctx := context.Background()

	profileID := "non-existent"

	reader, err := store.ExportProfile(ctx, profileID)
	assert.Error(t, err)
	assert.Nil(t, reader)
	assert.Contains(t, err.Error(), "profile not found")
}

func TestLocalProfileStore_DeleteProfile(t *testing.T) {
	store, tempDir := setupTestLocalStore(t)
	ctx := context.Background()

	profileID := "delete-test"

	// Initialize profile
	err := store.InitializeProfile(ctx, profileID)
	require.NoError(t, err)

	profilePath := filepath.Join(tempDir, profileID)
	assert.DirExists(t, profilePath)

	// Delete profile
	err = store.DeleteProfile(ctx, profileID)
	require.NoError(t, err)

	// Verify deletion
	assert.NoDirExists(t, profilePath)
}

func TestLocalProfileStore_DeleteProfile_NotFound(t *testing.T) {
	store, _ := setupTestLocalStore(t)
	ctx := context.Background()

	profileID := "non-existent"

	// Should not error when deleting non-existent profile
	err := store.DeleteProfile(ctx, profileID)
	assert.NoError(t, err)
}

func TestLocalProfileStore_GetProfileSize(t *testing.T) {
	store, tempDir := setupTestLocalStore(t)
	ctx := context.Background()

	profileID := "size-test"

	// Initialize profile
	err := store.InitializeProfile(ctx, profileID)
	require.NoError(t, err)

	userDataPath := filepath.Join(tempDir, profileID, localUserDataDirName)

	// Create files with known sizes
	file1Content := "hello world"                         // 11 bytes
	file2Content := "test content longer than first file" // 34 bytes

	err = os.WriteFile(filepath.Join(userDataPath, "file1.txt"), []byte(file1Content), 0644)
	require.NoError(t, err)

	err = os.MkdirAll(filepath.Join(userDataPath, "subdir"), 0755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(userDataPath, "subdir", "file2.txt"), []byte(file2Content), 0644)
	require.NoError(t, err)

	// Get size
	size, err := store.GetProfileSize(ctx, profileID)
	require.NoError(t, err)

	expectedSize := int64(len(file1Content) + len(file2Content))
	assert.Equal(t, expectedSize, size)
}

func TestLocalProfileStore_ValidateProfile(t *testing.T) {
	store, _ := setupTestLocalStore(t)
	ctx := context.Background()

	profileID := "validate-test"

	// Initialize profile
	err := store.InitializeProfile(ctx, profileID)
	require.NoError(t, err)

	// Should be valid
	err = store.ValidateProfile(ctx, profileID)
	assert.NoError(t, err)
}

func TestLocalProfileStore_ValidateProfile_Invalid(t *testing.T) {
	store, tempDir := setupTestLocalStore(t)
	ctx := context.Background()

	profileID := "invalid-test"

	// Create profile directory but not user-data
	profilePath := filepath.Join(tempDir, profileID)
	err := os.MkdirAll(profilePath, 0755)
	require.NoError(t, err)

	// Should be invalid
	err = store.ValidateProfile(ctx, profileID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "user-data directory not found")
}

func TestLocalProfileStore_ValidateProfile_UserDataNotDirectory(t *testing.T) {
	store, tempDir := setupTestLocalStore(t)
	ctx := context.Background()

	profileID := "invalid-userdata-test"

	// Create profile directory
	profilePath := filepath.Join(tempDir, profileID)
	err := os.MkdirAll(profilePath, 0755)
	require.NoError(t, err)

	// Create user-data as a file instead of directory
	userDataPath := filepath.Join(profilePath, localUserDataDirName)
	err = os.WriteFile(userDataPath, []byte("not a directory"), 0644)
	require.NoError(t, err)

	// Should be invalid
	err = store.ValidateProfile(ctx, profileID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "user-data is not a directory")
}

func TestLocalProfileStore_SaveProfileData(t *testing.T) {
	store, tempDir := setupTestLocalStore(t)
	ctx := context.Background()

	profileID := "save-test"

	// Initialize profile
	err := store.InitializeProfile(ctx, profileID)
	require.NoError(t, err)

	userDataPath := filepath.Join(tempDir, profileID, localUserDataDirName)

	// Add some data to the profile
	err = os.WriteFile(filepath.Join(userDataPath, "test.txt"), []byte("test content"), 0644)
	require.NoError(t, err)

	// Save profile data (should update metadata)
	err = store.SaveProfileData(ctx, profileID, "")
	require.NoError(t, err)

	// Check that metadata was updated
	profilePath := filepath.Join(tempDir, profileID)
	metadataPath := filepath.Join(profilePath, localMetadataFileName)

	data, err := os.ReadFile(metadataPath)
	require.NoError(t, err)

	var metadata LocalProfileMetadata
	err = json.Unmarshal(data, &metadata)
	require.NoError(t, err)

	assert.Greater(t, metadata.SizeBytes, int64(0))
	assert.False(t, metadata.LastModified.IsZero())
}

func TestLocalProfileStore_ExtractZip_Security(t *testing.T) {
	store, _ := setupTestLocalStore(t)
	ctx := context.Background()

	profileID := "security-test"

	// Create a malicious ZIP with directory traversal
	zipData := createMaliciousZip(t)

	err := store.ImportProfile(ctx, profileID, bytes.NewReader(zipData))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid file path")
}

func TestLocalProfileStore_CalculateDirectorySize(t *testing.T) {
	store, _ := setupTestLocalStore(t)

	// Create test directory structure in a temporary location
	tempDir := t.TempDir()
	testDir := filepath.Join(tempDir, "size-test")
	err := os.MkdirAll(filepath.Join(testDir, "subdir"), 0755)
	require.NoError(t, err)

	// Create files with known sizes
	files := map[string]string{
		"file1.txt":        "hello",  // 5 bytes
		"file2.txt":        "world!", // 6 bytes
		"subdir/file3.txt": "test",   // 4 bytes
	}

	expectedSize := int64(0)
	for fileName, content := range files {
		filePath := filepath.Join(testDir, fileName)
		err := os.WriteFile(filePath, []byte(content), 0644)
		require.NoError(t, err)
		expectedSize += int64(len(content))
	}

	// Calculate size
	size, err := store.calculateDirectorySize(testDir)
	require.NoError(t, err)
	assert.Equal(t, expectedSize, size)
}

func TestLocalProfileStore_ReadWriteMetadata(t *testing.T) {
	store, tempDir := setupTestLocalStore(t)

	profilePath := filepath.Join(tempDir, "metadata-test")
	err := os.MkdirAll(profilePath, 0755)
	require.NoError(t, err)

	// Write metadata
	originalMetadata := LocalProfileMetadata{
		Version:      1,
		CreatedAt:    time.Now().UTC().Truncate(time.Second),
		LastModified: time.Now().UTC().Truncate(time.Second),
		SizeBytes:    12345,
		Browser:      "chrome",
	}

	err = store.writeMetadata(profilePath, originalMetadata)
	require.NoError(t, err)

	// Read metadata back
	readMetadata, err := store.readMetadata(profilePath)
	require.NoError(t, err)

	assert.Equal(t, originalMetadata.Version, readMetadata.Version)
	assert.Equal(t, originalMetadata.CreatedAt, readMetadata.CreatedAt)
	assert.Equal(t, originalMetadata.LastModified, readMetadata.LastModified)
	assert.Equal(t, originalMetadata.SizeBytes, readMetadata.SizeBytes)
	assert.Equal(t, originalMetadata.Browser, readMetadata.Browser)
}

// Helper functions

func createTestZip(t *testing.T, files map[string]string) []byte {
	var buf bytes.Buffer
	zipWriter := zip.NewWriter(&buf)

	for fileName, content := range files {
		writer, err := zipWriter.Create(fileName)
		require.NoError(t, err)
		_, err = writer.Write([]byte(content))
		require.NoError(t, err)
	}

	err := zipWriter.Close()
	require.NoError(t, err)

	return buf.Bytes()
}

func createMaliciousZip(t *testing.T) []byte {
	var buf bytes.Buffer
	zipWriter := zip.NewWriter(&buf)

	// Try to create a file outside the extraction directory
	writer, err := zipWriter.Create("../../../etc/passwd")
	require.NoError(t, err)
	_, err = writer.Write([]byte("malicious content"))
	require.NoError(t, err)

	err = zipWriter.Close()
	require.NoError(t, err)

	return buf.Bytes()
}
