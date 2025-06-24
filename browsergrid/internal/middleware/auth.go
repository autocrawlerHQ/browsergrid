package middleware

import (
	"crypto/subtle"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

const (
	headerAPIKey  = "X-API-Key"
	queryAPIKey   = "api_key"
	authHeaderKey = "Authorization"
)

// publicPaths defines routes that don't require authentication
var publicPaths = []string{
	"/health",
	"/swagger/",
}

// isPublicPath checks if the given path should be accessible without authentication
func isPublicPath(path string) bool {
	for _, publicPath := range publicPaths {
		if path == publicPath || strings.HasPrefix(path, publicPath) {
			return true
		}
	}
	return false
}

func Auth() gin.HandlerFunc {
	expected := os.Getenv("BROWSERGRID_API_KEY")
	if expected == "" {
		expected = os.Getenv("API_KEY")
	}

	expectedKey := expected

	return func(c *gin.Context) {
		if isPublicPath(c.Request.URL.Path) {
			c.Next()
			return
		}

		if expectedKey == "" {
			c.Next()
			return
		}

		provided := extractKey(c)

		if subtle.ConstantTimeCompare([]byte(provided), []byte(expectedKey)) == 1 {
			c.Next()
			return
		}

		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
	}
}

func extractKey(c *gin.Context) string {
	if v := c.GetHeader(headerAPIKey); v != "" {
		return v
	}

	if v := c.GetHeader(authHeaderKey); v != "" {
		parts := strings.Fields(v)
		if len(parts) == 1 {
			return parts[0]
		}
		if len(parts) == 2 {
			return parts[1]
		}
	}

	if v := c.Query(queryAPIKey); v != "" {
		return v
	}
	return ""
}
