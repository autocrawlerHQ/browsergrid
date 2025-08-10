package profiles

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"github.com/autocrawlerHQ/browsergrid/internal/sessions"
	"github.com/autocrawlerHQ/browsergrid/internal/storage"
	_ "github.com/autocrawlerHQ/browsergrid/internal/storage/local"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func setupTestRouter(t *testing.T) (*gin.Engine, *Store) {
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	err = db.AutoMigrate(&Profile{}, &sessions.Session{})
	require.NoError(t, err)

	store := NewStore(db)

	router := gin.New()
	api := router.Group("/api/v1")

	backend, err := storage.New("local", map[string]string{"path": t.TempDir()})
	require.NoError(t, err)

	deps := Dependencies{
		DB:      db,
		Store:   store,
		Storage: backend,
	}

	RegisterRoutes(api, deps)

	return router, store
}

func TestCreateProfile(t *testing.T) {
	router, _ := setupTestRouter(t)

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
	router, store := setupTestRouter(t)
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
	router, store := setupTestRouter(t)
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
	router, store := setupTestRouter(t)
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
	router, store := setupTestRouter(t)
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
	router, _ := setupTestRouter(t)

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
		// Import errors are handled by validation and DB operations in current implementation
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// No mocks required

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

// Export route removed with profile store elimination

// Helper function to create string pointers
func stringPtr(s string) *string {
	return &s
}
