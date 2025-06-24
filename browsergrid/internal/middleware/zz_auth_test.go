package middleware

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		apiKey         string
		requestSetup   func(req *http.Request)
		expectedStatus int
	}{
		{
			name:   "no auth configured - should pass",
			apiKey: "",
			requestSetup: func(req *http.Request) {
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:   "valid X-API-Key header",
			apiKey: "test-secret",
			requestSetup: func(req *http.Request) {
				req.Header.Set("X-API-Key", "test-secret")
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:   "valid Authorization Bearer",
			apiKey: "test-secret",
			requestSetup: func(req *http.Request) {
				req.Header.Set("Authorization", "Bearer test-secret")
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:   "valid Authorization BrowsergridKey",
			apiKey: "test-secret",
			requestSetup: func(req *http.Request) {
				req.Header.Set("Authorization", "BrowsergridKey test-secret")
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:   "valid query parameter",
			apiKey: "test-secret",
			requestSetup: func(req *http.Request) {
				q := req.URL.Query()
				q.Add("api_key", "test-secret")
				req.URL.RawQuery = q.Encode()
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:   "invalid key",
			apiKey: "test-secret",
			requestSetup: func(req *http.Request) {
				req.Header.Set("X-API-Key", "wrong-key")
			},
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:   "missing key when required",
			apiKey: "test-secret",
			requestSetup: func(req *http.Request) {
			},
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:   "health endpoint bypasses auth",
			apiKey: "test-secret",
			requestSetup: func(req *http.Request) {
			},
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.apiKey != "" {
				os.Setenv("BROWSERGRID_API_KEY", tt.apiKey)
			} else {
				os.Unsetenv("BROWSERGRID_API_KEY")
				os.Unsetenv("API_KEY")
			}

			r := gin.New()
			r.Use(Auth())

			r.GET("/health", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"status": "ok"})
			})
			r.GET("/api/v1/test", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"message": "authenticated"})
			})

			var path string
			if tt.name == "health endpoint bypasses auth" {
				path = "/health"
			} else {
				path = "/api/v1/test"
			}

			req := httptest.NewRequest("GET", path, nil)
			tt.requestSetup(req)

			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedStatus == http.StatusUnauthorized {
				assert.Contains(t, w.Body.String(), "unauthorized")
			}
		})
	}
}

func TestExtractKey(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name     string
		setup    func(c *gin.Context)
		expected string
	}{
		{
			name: "X-API-Key header",
			setup: func(c *gin.Context) {
				c.Request.Header.Set("X-API-Key", "header-key")
			},
			expected: "header-key",
		},
		{
			name: "Authorization Bearer",
			setup: func(c *gin.Context) {
				c.Request.Header.Set("Authorization", "Bearer bearer-key")
			},
			expected: "bearer-key",
		},
		{
			name: "Authorization raw token",
			setup: func(c *gin.Context) {
				c.Request.Header.Set("Authorization", "raw-token")
			},
			expected: "raw-token",
		},
		{
			name: "Query parameter",
			setup: func(c *gin.Context) {
				c.Request.URL.RawQuery = "api_key=query-key"
			},
			expected: "query-key",
		},
		{
			name: "No key provided",
			setup: func(c *gin.Context) {
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test", nil)
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = req

			tt.setup(c)

			result := extractKey(c)
			assert.Equal(t, tt.expected, result)
		})
	}
}
