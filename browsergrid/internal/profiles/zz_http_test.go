package profiles

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/autocrawlerHQ/browsergrid/internal/sessions"
)

// MockProfileStore implements ProfileStore for testing
type MockProfileStore struct {
	profiles map[string]*Profile
	errors   map[string]error
}

func NewMockProfileStore() *MockProfileStore {
	return &MockProfileStore{
		profiles: make(map[string]*Profile),
		errors:   make(map[string]error),
	}
}

func (m *MockProfileStore) InitializeProfile(ctx context.Context, profileID string) error {
	if err, exists := m.errors["InitializeProfile"]; exists {
		return err
	}
	return nil
}

func (m *MockProfileStore) ImportProfile(ctx context.Context, profileID string, zipData io.Reader) error {
	if err, exists := m.errors["ImportProfile"]; exists {
		return err
	}
	return nil
}

func (m *MockProfileStore) GetProfilePath(ctx context.Context, profileID string) (string, error) {
	if err, exists := m.errors["GetProfilePath"]; exists {
		return "", err
	}
	return fmt.Sprintf("/var/lib/browsergrid/profiles/%s/user-data", profileID), nil
}

func (m *MockProfileStore) SaveProfileData(ctx context.Context, profileID string, sourcePath string) error {
	if err, exists := m.errors["SaveProfileData"]; exists {
		return err
	}
	return nil
}

func (m *MockProfileStore) ExportProfile(ctx context.Context, profileID string) (io.ReadCloser, error) {
	if err, exists := m.errors["ExportProfile"]; exists {
		return nil, err
	}
	return io.NopCloser(strings.NewReader("mock profile data")), nil
}

func (m *MockProfileStore) DeleteProfile(ctx context.Context, profileID string) error {
	if err, exists := m.errors["DeleteProfile"]; exists {
		return err
	}
	return nil
}

func (m *MockProfileStore) GetProfileSize(ctx context.Context, profileID string) (int64, error) {
	if err, exists := m.errors["GetProfileSize"]; exists {
		return 0, err
	}
	return 1024, nil
}

func (m *MockProfileStore) ValidateProfile(ctx context.Context, profileID string) error {
	if err, exists := m.errors["ValidateProfile"]; exists {
		return err
	}
	return nil
}

func setupTestRouter(t *testing.T) (*gin.Engine, *Store, *MockProfileStore) {
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	err = db.AutoMigrate(&Profile{}, &sessions.Session{})
	require.NoError(t, err)

	store := NewStore(db)
	mockProfileStore := NewMockProfileStore()

	router := gin.New()
	api := router.Group("/api/v1")

	deps := Dependencies{
		DB:           db,
		Store:        store,
		ProfileStore: mockProfileStore,
	}

	RegisterRoutes(api, deps)

	return router, store, mockProfileStore
}

func TestCreateProfile(t *testing.T) {
	router, _, _ := setupTestRouter(t)

	tests := []struct {
		name           string
		requestBody    CreateProfileRequest
		expectedStatus int
		expectedError  string
	}{
		{
			name: "valid profile creation",
			requestBody: CreateProfileRequest{
				Name:        "test-profile",
				Description: "Test profile",
				Browser:     sessions.BrowserChrome,
			},
			expectedStatus: http.StatusCreated,
		},
		{
			name: "missing name",
			requestBody: CreateProfileRequest{
				Description: "Test profile",
				Browser:     sessions.BrowserChrome,
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Key: 'CreateProfileRequest.Name' Error:Field validation for 'Name' failed on the 'required' tag",
		},
		{
			name: "invalid browser",
			requestBody: CreateProfileRequest{
				Name:        "test-profile",
				Description: "Test profile",
				Browser:     "invalid-browser",
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Key: 'CreateProfileRequest.Browser' Error:Field validation for 'Browser' failed on the 'oneof' tag",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.requestBody)
			req := httptest.NewRequest("POST", "/api/v1/profiles", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedStatus == http.StatusCreated {
				var response Profile
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)

				assert.NotEqual(t, uuid.Nil, response.ID)
				assert.Equal(t, tt.requestBody.Name, response.Name)
				assert.Equal(t, tt.requestBody.Description, response.Description)
				assert.Equal(t, tt.requestBody.Browser, response.Browser)
				assert.Equal(t, StorageBackendLocal, response.StorageBackend)
			} else if tt.expectedError != "" {
				var errorResp ErrorResponse
				err := json.Unmarshal(w.Body.Bytes(), &errorResp)
				require.NoError(t, err)
				assert.Contains(t, errorResp.Error, tt.expectedError)
			}
		})
	}
}

func TestListProfiles(t *testing.T) {
	router, store, _ := setupTestRouter(t)
	ctx := context.Background()

	// Create test profiles
	profiles := []*Profile{
		{Name: "chrome-profile", Browser: sessions.BrowserChrome},
		{Name: "firefox-profile", Browser: sessions.BrowserFirefox},
		{Name: "chrome-profile-2", Browser: sessions.BrowserChrome},
	}

	for _, p := range profiles {
		err := store.CreateProfile(ctx, p)
		require.NoError(t, err)
	}

	tests := []struct {
		name           string
		query          string
		expectedStatus int
		expectedCount  int
	}{
		{
			name:           "list all profiles",
			query:          "",
			expectedStatus: http.StatusOK,
			expectedCount:  3,
		},
		{
			name:           "filter by browser",
			query:          "?browser=chrome",
			expectedStatus: http.StatusOK,
			expectedCount:  2,
		},
		{
			name:           "with pagination",
			query:          "?offset=1&limit=2",
			expectedStatus: http.StatusOK,
			expectedCount:  2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/v1/profiles"+tt.query, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			var response ProfileListResponse
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			assert.Equal(t, tt.expectedCount, len(response.Profiles))
			if tt.name == "filter by browser" {
				assert.Equal(t, 2, response.Total) // Filtered total
			} else {
				assert.Equal(t, 3, response.Total) // Unfiltered total
			}
		})
	}
}

func TestGetProfile(t *testing.T) {
	router, store, _ := setupTestRouter(t)
	ctx := context.Background()

	profile := &Profile{
		Name:        "test-profile",
		Description: "Test profile",
		Browser:     sessions.BrowserChrome,
	}

	err := store.CreateProfile(ctx, profile)
	require.NoError(t, err)

	tests := []struct {
		name           string
		profileID      string
		expectedStatus int
		expectedError  string
	}{
		{
			name:           "valid profile ID",
			profileID:      profile.ID.String(),
			expectedStatus: http.StatusOK,
		},
		{
			name:           "invalid UUID",
			profileID:      "invalid-uuid",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "invalid profile ID",
		},
		{
			name:           "non-existent profile",
			profileID:      uuid.New().String(),
			expectedStatus: http.StatusNotFound,
			expectedError:  "profile not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/v1/profiles/"+tt.profileID, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedStatus == http.StatusOK {
				var response Profile
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)

				assert.Equal(t, profile.ID, response.ID)
				assert.Equal(t, profile.Name, response.Name)
			} else if tt.expectedError != "" {
				var errorResp ErrorResponse
				err := json.Unmarshal(w.Body.Bytes(), &errorResp)
				require.NoError(t, err)
				assert.Contains(t, errorResp.Error, tt.expectedError)
			}
		})
	}
}

func TestUpdateProfile(t *testing.T) {
	router, store, _ := setupTestRouter(t)
	ctx := context.Background()

	profile := &Profile{
		Name:        "test-profile",
		Description: "Original description",
		Browser:     sessions.BrowserChrome,
	}

	err := store.CreateProfile(ctx, profile)
	require.NoError(t, err)

	tests := []struct {
		name           string
		profileID      string
		requestBody    UpdateProfileRequest
		expectedStatus int
		expectedError  string
	}{
		{
			name:      "valid update",
			profileID: profile.ID.String(),
			requestBody: UpdateProfileRequest{
				Name:        stringPtr("updated-name"),
				Description: stringPtr("Updated description"),
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:      "invalid UUID",
			profileID: "invalid-uuid",
			requestBody: UpdateProfileRequest{
				Name: stringPtr("updated-name"),
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "invalid profile ID",
		},
		{
			name:      "non-existent profile",
			profileID: uuid.New().String(),
			requestBody: UpdateProfileRequest{
				Name: stringPtr("updated-name"),
			},
			expectedStatus: http.StatusNotFound,
			expectedError:  "profile not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.requestBody)
			req := httptest.NewRequest("PATCH", "/api/v1/profiles/"+tt.profileID, bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedStatus == http.StatusOK {
				var response Profile
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)

				if tt.requestBody.Name != nil {
					assert.Equal(t, *tt.requestBody.Name, response.Name)
				}
				if tt.requestBody.Description != nil {
					assert.Equal(t, *tt.requestBody.Description, response.Description)
				}
			} else if tt.expectedError != "" {
				var errorResp ErrorResponse
				err := json.Unmarshal(w.Body.Bytes(), &errorResp)
				require.NoError(t, err)
				assert.Contains(t, errorResp.Error, tt.expectedError)
			}
		})
	}
}

func TestDeleteProfile(t *testing.T) {
	router, store, _ := setupTestRouter(t)
	ctx := context.Background()

	profile := &Profile{
		Name:        "test-profile",
		Description: "Test profile for deletion",
		Browser:     sessions.BrowserChrome,
	}

	err := store.CreateProfile(ctx, profile)
	require.NoError(t, err)

	tests := []struct {
		name           string
		profileID      string
		expectedStatus int
		expectedError  string
	}{
		{
			name:           "valid profile deletion",
			profileID:      profile.ID.String(),
			expectedStatus: http.StatusOK,
		},
		{
			name:           "invalid UUID",
			profileID:      "invalid-uuid",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "invalid profile ID",
		},
		{
			name:           "non-existent profile",
			profileID:      uuid.New().String(),
			expectedStatus: http.StatusNotFound,
			expectedError:  "profile not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("DELETE", "/api/v1/profiles/"+tt.profileID, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedStatus == http.StatusOK {
				var response MessageResponse
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)
				assert.Equal(t, "profile deleted successfully", response.Message)
			} else if tt.expectedError != "" {
				var errorResp ErrorResponse
				err := json.Unmarshal(w.Body.Bytes(), &errorResp)
				require.NoError(t, err)
				assert.Contains(t, errorResp.Error, tt.expectedError)
			}
		})
	}
}

func TestImportProfile(t *testing.T) {
	router, _, mockProfileStore := setupTestRouter(t)

	// Create a temporary ZIP file for testing
	tempDir := t.TempDir()
	zipPath := filepath.Join(tempDir, "test-profile.zip")

	// Create a simple ZIP file
	zipFile, err := os.Create(zipPath)
	require.NoError(t, err)
	defer zipFile.Close()

	// Write some test data to the ZIP
	zipWriter := zip.NewWriter(zipFile)
	file, err := zipWriter.Create("user-data/test.txt")
	require.NoError(t, err)
	file.Write([]byte("test data"))
	zipWriter.Close()

	tests := []struct {
		name           string
		formData       map[string]string
		filePath       string
		expectedStatus int
		expectedError  string
		setupMock      func()
	}{
		{
			name: "valid profile import",
			formData: map[string]string{
				"name":        "imported-profile-1",
				"description": "Imported profile",
				"browser":     "chrome",
			},
			filePath:       zipPath,
			expectedStatus: http.StatusCreated,
		},
		{
			name: "missing file",
			formData: map[string]string{
				"name":    "imported-profile-2",
				"browser": "chrome",
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "file is required",
		},
		{
			name: "import error",
			formData: map[string]string{
				"name":    "imported-profile-3",
				"browser": "chrome",
			},
			filePath:       zipPath,
			expectedStatus: http.StatusBadRequest,
			expectedError:  "failed to import profile",
			setupMock: func() {
				mockProfileStore.errors["ImportProfile"] = fmt.Errorf("import failed")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset mock errors
			mockProfileStore.errors = make(map[string]error)
			if tt.setupMock != nil {
				tt.setupMock()
			}

			var body bytes.Buffer
			writer := multipart.NewWriter(&body)

			// Add form fields
			for key, value := range tt.formData {
				writer.WriteField(key, value)
			}

			// Add file if specified
			if tt.filePath != "" {
				file, err := os.Open(tt.filePath)
				require.NoError(t, err)
				defer file.Close()

				part, err := writer.CreateFormFile("file", "test-profile.zip")
				require.NoError(t, err)

				_, err = io.Copy(part, file)
				require.NoError(t, err)
			}

			writer.Close()

			req := httptest.NewRequest("POST", "/api/v1/profiles/import", &body)
			req.Header.Set("Content-Type", writer.FormDataContentType())

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedStatus == http.StatusCreated {
				var response Profile
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)

				assert.NotEqual(t, uuid.Nil, response.ID)
				assert.Equal(t, tt.formData["name"], response.Name)
				assert.Equal(t, tt.formData["description"], response.Description)
				assert.Equal(t, sessions.Browser(tt.formData["browser"]), response.Browser)
			} else if tt.expectedError != "" {
				var errorResp ErrorResponse
				err := json.Unmarshal(w.Body.Bytes(), &errorResp)
				require.NoError(t, err)
				assert.Contains(t, errorResp.Error, tt.expectedError)
			}
		})
	}
}

func TestExportProfile(t *testing.T) {
	router, store, mockProfileStore := setupTestRouter(t)
	ctx := context.Background()

	profile := &Profile{
		Name:        "test-profile",
		Description: "Test profile for export",
		Browser:     sessions.BrowserChrome,
	}

	err := store.CreateProfile(ctx, profile)
	require.NoError(t, err)

	tests := []struct {
		name           string
		profileID      string
		expectedStatus int
		expectedError  string
		setupMock      func()
	}{
		{
			name:           "valid profile export",
			profileID:      profile.ID.String(),
			expectedStatus: http.StatusOK,
		},
		{
			name:           "invalid UUID",
			profileID:      "invalid-uuid",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "invalid profile ID",
		},
		{
			name:           "non-existent profile",
			profileID:      uuid.New().String(),
			expectedStatus: http.StatusNotFound,
			expectedError:  "profile not found",
		},
		{
			name:           "export error",
			profileID:      profile.ID.String(),
			expectedStatus: http.StatusInternalServerError,
			expectedError:  "failed to export profile",
			setupMock: func() {
				mockProfileStore.errors["ExportProfile"] = fmt.Errorf("export failed")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset mock errors
			mockProfileStore.errors = make(map[string]error)
			if tt.setupMock != nil {
				tt.setupMock()
			}

			req := httptest.NewRequest("GET", "/api/v1/profiles/"+tt.profileID+"/export", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedStatus == http.StatusOK {
				assert.Equal(t, "application/zip", w.Header().Get("Content-Type"))
				assert.Contains(t, w.Header().Get("Content-Disposition"), "attachment")
				assert.Contains(t, w.Header().Get("Content-Disposition"), profile.Name+".zip")
				assert.NotEmpty(t, w.Body.Bytes())
			} else if tt.expectedError != "" {
				var errorResp ErrorResponse
				err := json.Unmarshal(w.Body.Bytes(), &errorResp)
				require.NoError(t, err)
				assert.Contains(t, errorResp.Error, tt.expectedError)
			}
		})
	}
}

// Helper function to create string pointers
func stringPtr(s string) *string {
	return &s
}
