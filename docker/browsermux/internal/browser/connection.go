package browser

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/gorilla/websocket"
)

type BrowserInfo struct {
	URL            string    `json:"url"`
	Version        string    `json:"version"`
	UserAgent      string    `json:"user_agent"`
	StartTime      time.Time `json:"start_time"`
	ConnectionTime time.Time `json:"connection_time"`
	Status         string    `json:"status"`
}

// GetBrowserInfo retrieves browser information and ensures that the WebSocket URL
// uses the same host and port as the provided browserURL
func GetBrowserInfo(browserURL string) (*BrowserInfo, error) {
	// Parse the input browser URL to extract host and port for later use
	parsedBrowserURL, err := url.Parse(browserURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse browser URL: %w", err)
	}

	// If the browser URL is already a WebSocket URL, convert it to HTTP
	baseURL := browserURL
	if len(baseURL) > 3 && baseURL[:3] == "ws:" {
		baseURL = "http:" + baseURL[3:]
	} else if len(baseURL) > 4 && baseURL[:4] == "wss:" {
		baseURL = "https:" + baseURL[4:]
	}

	// If the URL ends with a path like /devtools/browser, strip it off to get the base URL
	if lastIndex := lastIndexOf(baseURL, "/devtools/"); lastIndex != -1 {
		baseURL = baseURL[:lastIndex]
	}

	// Ensure the URL doesn't end with a trailing slash
	if baseURL[len(baseURL)-1] == '/' {
		baseURL = baseURL[:len(baseURL)-1]
	}

	// Add /json/version to get browser info
	infoURL := baseURL + "/json/version"

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	// Make request
	resp, err := client.Get(infoURL)
	if err != nil {
		return nil, fmt.Errorf("failed to get browser info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get browser info: HTTP %d", resp.StatusCode)
	}

	// Parse response
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to parse browser info: %w", err)
	}

	// Extract relevant fields
	info := &BrowserInfo{
		URL:            browserURL, // This will be replaced with webSocketDebuggerUrl if available
		ConnectionTime: time.Now(),
		Status:         "connected",
	}

	if v, ok := result["Browser"]; ok {
		info.Version = fmt.Sprintf("%v", v)
	}

	if v, ok := result["User-Agent"]; ok {
		info.UserAgent = fmt.Sprintf("%v", v)
	}

	// Get the actual WebSocket URL - this is critical
	if v, ok := result["webSocketDebuggerUrl"]; ok {
		wsURL := fmt.Sprintf("%v", v)
		if wsURL != "" {
			// Replace the host and port in the WebSocket URL with the ones from browserURL
			wsURL = replaceHostPort(wsURL, parsedBrowserURL.Host)
			info.URL = wsURL
		} else {
			return nil, fmt.Errorf("browser returned empty webSocketDebuggerUrl")
		}
	} else {
		return nil, fmt.Errorf("webSocketDebuggerUrl not found in browser info response")
	}

	return info, nil
}

// Helper function to find the last index of a substring
func lastIndexOf(s, substr string) int {
	for i := len(s) - len(substr); i >= 0; i-- {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

// replaceHostPort replaces the host and port in a URL with the specified host
func replaceHostPort(originalURL string, newHost string) string {
	parsedURL, err := url.Parse(originalURL)
	if err != nil {
		return originalURL
	}

	// Replace the host (including port if present)
	parsedURL.Host = newHost

	return parsedURL.String()
}

func TestBrowserConnection(browserURL string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	dialer := websocket.Dialer{
		HandshakeTimeout: timeout,
	}

	conn, _, err := dialer.DialContext(ctx, browserURL, nil)
	if err != nil {
		return fmt.Errorf("failed to connect to browser: %w", err)
	}

	conn.Close()

	return nil
}
