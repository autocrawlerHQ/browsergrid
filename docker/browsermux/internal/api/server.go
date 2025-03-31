// internal/api/server.go
package api

// TODO: reconnection is broken, we the browser disconnect/ shuts down, when we reconnect our user automation client hangs
// TODO: need to fix this
import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/mux"

	"browsermux/internal/api/middleware"
	"browsermux/internal/browser"
	"browsermux/internal/events"
	"browsermux/internal/webhook"
)

// Server is the API server
type Server struct {
	router          *mux.Router
	server          *http.Server
	cdpProxy        *browser.CDPProxy
	webhookManager  webhook.Manager
	eventDispatcher events.Dispatcher
	browserBaseURL  string
	serverBaseURL   string
}

// NewServer creates a new API server
func NewServer(cdpProxy *browser.CDPProxy, webhookManager webhook.Manager, eventDispatcher events.Dispatcher, port string) *Server {
	router := mux.NewRouter()

	// Extract browser base URL from the CDPProxy
	browserBaseURL := cdpProxy.GetConfig().BrowserURL

	// Convert WebSocket URL to HTTP if needed
	if strings.HasPrefix(browserBaseURL, "ws:") {
		browserBaseURL = "http:" + browserBaseURL[3:]
	} else if strings.HasPrefix(browserBaseURL, "wss:") {
		browserBaseURL = "https:" + browserBaseURL[4:]
	} else if !strings.HasPrefix(browserBaseURL, "http:") && !strings.HasPrefix(browserBaseURL, "https:") {
		// If no protocol is specified, assume HTTP
		browserBaseURL = "http://" + browserBaseURL
	}

	// Remove any path components like /devtools/browser
	if lastIndex := strings.LastIndex(browserBaseURL, "/devtools/"); lastIndex != -1 {
		browserBaseURL = browserBaseURL[:lastIndex]
	}

	// Ensure no trailing slash
	browserBaseURL = strings.TrimSuffix(browserBaseURL, "/")

	// Determine server base URL based on port
	serverBaseURL := fmt.Sprintf("http://localhost:%s", port)

	server := &Server{
		router:          router,
		cdpProxy:        cdpProxy,
		webhookManager:  webhookManager,
		eventDispatcher: eventDispatcher,
		browserBaseURL:  browserBaseURL,
		serverBaseURL:   serverBaseURL,
		server: &http.Server{
			Addr:    ":" + port,
			Handler: router,
		},
	}

	server.setupRoutes()

	return server
}

// Start starts the API server
func (s *Server) Start() error {
	log.Printf("Starting API server on %s", s.server.Addr)
	log.Printf("Proxying browser at %s to %s", s.browserBaseURL, s.serverBaseURL)
	return s.server.ListenAndServe()
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}

// setupRoutes configures all API routes
func (s *Server) setupRoutes() {
	// Apply common middleware
	s.router.Use(middleware.Logging)
	s.router.Use(middleware.Recovery)

	// Chrome DevTools Protocol HTTP endpoints
	s.router.HandleFunc("/json/version", s.handleJSONVersion).Methods("GET", "POST")
	s.router.HandleFunc("/json", s.handleJSONList).Methods("GET", "POST")
	s.router.HandleFunc("/json/list", s.handleJSONList).Methods("GET", "POST")
	s.router.HandleFunc("/json/new", s.handleJSONNew).Methods("GET", "POST")
	s.router.HandleFunc("/json/activate/{targetId}", s.handleJSONActivate).Methods("GET", "POST")
	s.router.HandleFunc("/json/close/{targetId}", s.handleJSONClose).Methods("GET", "POST")
	s.router.HandleFunc("/json/protocol", s.handleJSONProtocol).Methods("GET", "POST")

	// API routes
	api := s.router.PathPrefix("/api").Subrouter()

	// Register domain-specific routes
	browserHandler := browser.NewHandler(s.cdpProxy)
	browserHandler.RegisterRoutes(api)

	webhookHandler := webhook.NewHandler(s.webhookManager)
	webhookHandler.RegisterRoutes(api)

	// WebSocket route for client connections - special case for non-API routes
	s.router.HandleFunc("/devtools/{path:.*}", browserHandler.HandleWebSocket)

	// Health check
	s.router.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}).Methods("GET")
}

// handleJSONVersion handles /json/version endpoint
func (s *Server) handleJSONVersion(w http.ResponseWriter, r *http.Request) {
	// Forward request to the browser with appropriate method
	var reqBody io.Reader
	if r.Body != nil {
		reqBody = r.Body
	}

	req, err := http.NewRequest(r.Method, s.browserBaseURL+"/json/version", reqBody)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error creating request: %v", err), http.StatusInternalServerError)
		return
	}

	// Copy headers from original request
	for key, values := range r.Header {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}

	// Copy content type if present
	if r.Header.Get("Content-Type") != "" {
		req.Header.Set("Content-Type", r.Header.Get("Content-Type"))
	}

	// Send the request
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error connecting to browser: %v", err), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Forward the error response
		for key, values := range resp.Header {
			for _, value := range values {
				w.Header().Add(key, value)
			}
		}
		w.WriteHeader(resp.StatusCode)
		io.Copy(w, resp.Body)
		return
	}

	// Read and parse the response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error reading browser response: %v", err), http.StatusInternalServerError)
		return
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		http.Error(w, fmt.Sprintf("Error parsing browser response: %v", err), http.StatusInternalServerError)
		return
	}

	// Rewrite the WebSocket URLs to point to our server
	if wsURL, ok := result["webSocketDebuggerUrl"].(string); ok {
		// Parse the URL to extract the path
		parsedURL, err := url.Parse(wsURL)
		if err == nil {
			// Create new WebSocket URL with our server's host and the original path
			serverURL, _ := url.Parse(s.serverBaseURL)
			newWsURL := "ws://" + serverURL.Host + parsedURL.Path
			result["webSocketDebuggerUrl"] = newWsURL
		}
	}

	// Copy original headers except Content-Length which will change
	for key, values := range resp.Header {
		if key != "Content-Length" {
			for _, value := range values {
				w.Header().Add(key, value)
			}
		}
	}

	// Write the modified response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	json.NewEncoder(w).Encode(result)
}

// handleJSONList handles /json and /json/list endpoints
func (s *Server) handleJSONList(w http.ResponseWriter, r *http.Request) {
	// Forward request to the browser with appropriate method
	var reqBody io.Reader
	if r.Body != nil {
		reqBody = r.Body
	}

	req, err := http.NewRequest(r.Method, s.browserBaseURL+"/json/list", reqBody)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error creating request: %v", err), http.StatusInternalServerError)
		return
	}

	// Copy headers from original request
	for key, values := range r.Header {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}

	// Copy content type if present
	if r.Header.Get("Content-Type") != "" {
		req.Header.Set("Content-Type", r.Header.Get("Content-Type"))
	}

	// Send the request
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error connecting to browser: %v", err), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Forward the error response
		for key, values := range resp.Header {
			for _, value := range values {
				w.Header().Add(key, value)
			}
		}
		w.WriteHeader(resp.StatusCode)
		io.Copy(w, resp.Body)
		return
	}

	// Read and parse the response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error reading browser response: %v", err), http.StatusInternalServerError)
		return
	}

	var targets []map[string]interface{}
	if err := json.Unmarshal(body, &targets); err != nil {
		http.Error(w, fmt.Sprintf("Error parsing browser response: %v", err), http.StatusInternalServerError)
		return
	}

	// Rewrite the WebSocket URLs in all targets
	for i := range targets {
		if wsURL, ok := targets[i]["webSocketDebuggerUrl"].(string); ok {
			// Parse the URL to extract the path
			parsedURL, err := url.Parse(wsURL)
			if err == nil {
				// Create new WebSocket URL with our server's host and the original path
				serverURL, _ := url.Parse(s.serverBaseURL)
				newWsURL := "ws://" + serverURL.Host + parsedURL.Path
				targets[i]["webSocketDebuggerUrl"] = newWsURL
			}
		}

		// Also rewrite devtoolsFrontendUrl if present
		if frontendURL, ok := targets[i]["devtoolsFrontendUrl"].(string); ok {
			// Replace any WebSocket references in the frontend URL
			parsedURL, err := url.Parse(s.browserBaseURL)
			if err == nil {
				serverURL, _ := url.Parse(s.serverBaseURL)
				frontendURL = strings.Replace(frontendURL, parsedURL.Host, serverURL.Host, -1)
				targets[i]["devtoolsFrontendUrl"] = frontendURL
			}
		}
	}

	// Copy original headers except Content-Length which will change
	for key, values := range resp.Header {
		if key != "Content-Length" {
			for _, value := range values {
				w.Header().Add(key, value)
			}
		}
	}

	// Write the modified response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	json.NewEncoder(w).Encode(targets)
}

// handleJSONNew handles /json/new endpoint
func (s *Server) handleJSONNew(w http.ResponseWriter, r *http.Request) {
	// Forward request to the browser, including any query params
	browserURL := s.browserBaseURL + "/json/new"
	if r.URL.RawQuery != "" {
		browserURL += "?" + r.URL.RawQuery
	}

	// Create request with original method
	var reqBody io.Reader
	if r.Body != nil {
		reqBody = r.Body
	}

	req, err := http.NewRequest(r.Method, browserURL, reqBody)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error creating request: %v", err), http.StatusInternalServerError)
		return
	}

	// Copy headers from original request
	for key, values := range r.Header {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}

	// Copy content type if present
	if r.Header.Get("Content-Type") != "" {
		req.Header.Set("Content-Type", r.Header.Get("Content-Type"))
	}

	// Send the request
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error connecting to browser: %v", err), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	// Proxy the response with URL rewriting
	s.proxyJSONResponse(w, resp)
}

// handleJSONActivate handles /json/activate/{targetId} endpoint
func (s *Server) handleJSONActivate(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	targetID := vars["targetId"]

	// Create request with original method
	var reqBody io.Reader
	if r.Body != nil {
		reqBody = r.Body
	}

	req, err := http.NewRequest(r.Method, s.browserBaseURL+"/json/activate/"+targetID, reqBody)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error creating request: %v", err), http.StatusInternalServerError)
		return
	}

	// Copy headers from original request
	for key, values := range r.Header {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}

	// Copy content type if present
	if r.Header.Get("Content-Type") != "" {
		req.Header.Set("Content-Type", r.Header.Get("Content-Type"))
	}

	// Send the request
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error connecting to browser: %v", err), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	// Copy headers and status code
	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

// handleJSONClose handles /json/close/{targetId} endpoint
func (s *Server) handleJSONClose(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	targetID := vars["targetId"]

	// Create request with original method
	var reqBody io.Reader
	if r.Body != nil {
		reqBody = r.Body
	}

	req, err := http.NewRequest(r.Method, s.browserBaseURL+"/json/close/"+targetID, reqBody)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error creating request: %v", err), http.StatusInternalServerError)
		return
	}

	// Copy headers from original request
	for key, values := range r.Header {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}

	// Copy content type if present
	if r.Header.Get("Content-Type") != "" {
		req.Header.Set("Content-Type", r.Header.Get("Content-Type"))
	}

	// Send the request
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error connecting to browser: %v", err), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	// Copy headers and status code
	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

// handleJSONProtocol handles /json/protocol endpoint
func (s *Server) handleJSONProtocol(w http.ResponseWriter, r *http.Request) {
	// Create request with original method
	var reqBody io.Reader
	if r.Body != nil {
		reqBody = r.Body
	}

	req, err := http.NewRequest(r.Method, s.browserBaseURL+"/json/protocol", reqBody)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error creating request: %v", err), http.StatusInternalServerError)
		return
	}

	// Copy headers from original request
	for key, values := range r.Header {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}

	// Copy content type if present
	if r.Header.Get("Content-Type") != "" {
		req.Header.Set("Content-Type", r.Header.Get("Content-Type"))
	}

	// Send the request
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error connecting to browser: %v", err), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	// Copy headers and status code
	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

// proxyJSONResponse is a helper to proxy JSON responses with URL rewriting
func (s *Server) proxyJSONResponse(w http.ResponseWriter, resp *http.Response) {
	if resp.StatusCode != http.StatusOK {
		// Copy headers from original response
		for key, values := range resp.Header {
			for _, value := range values {
				w.Header().Add(key, value)
			}
		}
		w.WriteHeader(resp.StatusCode)
		io.Copy(w, resp.Body)
		return
	}

	// Read and parse the response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error reading browser response: %v", err), http.StatusInternalServerError)
		return
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		// If not a JSON object, try as an array
		var resultArray []map[string]interface{}
		if err := json.Unmarshal(body, &resultArray); err != nil {
			// Not JSON, just return as-is with original headers
			for key, values := range resp.Header {
				for _, value := range values {
					w.Header().Add(key, value)
				}
			}
			w.WriteHeader(resp.StatusCode)
			w.Write(body)
			return
		}

		// Process array of targets
		for i := range resultArray {
			s.rewriteWebSocketURLs(resultArray[i])
		}

		// Copy original headers except Content-Length which will change
		for key, values := range resp.Header {
			if key != "Content-Length" {
				for _, value := range values {
					w.Header().Add(key, value)
				}
			}
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(resp.StatusCode)
		json.NewEncoder(w).Encode(resultArray)
		return
	}

	// Rewrite WebSocket URLs in the object
	s.rewriteWebSocketURLs(result)

	// Copy original headers except Content-Length which will change
	for key, values := range resp.Header {
		if key != "Content-Length" {
			for _, value := range values {
				w.Header().Add(key, value)
			}
		}
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	json.NewEncoder(w).Encode(result)
}

// rewriteWebSocketURLs replaces browser URLs with server URLs in a map
func (s *Server) rewriteWebSocketURLs(data map[string]interface{}) {
	// Common keys that might contain WebSocket URLs
	wsKeys := []string{"webSocketDebuggerUrl", "devtoolsFrontendUrl"}

	for _, key := range wsKeys {
		if strVal, ok := data[key].(string); ok {
			// Parse browser URL to get host
			parsedBrowserURL, err := url.Parse(s.browserBaseURL)
			if err == nil {
				// Parse server URL
				parsedServerURL, _ := url.Parse(s.serverBaseURL)

				// Replace the host part
				newVal := strings.Replace(strVal, parsedBrowserURL.Host, parsedServerURL.Host, -1)

				// For WebSocket URLs, ensure protocol is ws:// or wss://
				if key == "webSocketDebuggerUrl" {
					if strings.HasPrefix(newVal, "http:") {
						newVal = "ws:" + newVal[5:]
					} else if strings.HasPrefix(newVal, "https:") {
						newVal = "wss:" + newVal[6:]
					}
				}

				data[key] = newVal
			}
		}
	}
}
