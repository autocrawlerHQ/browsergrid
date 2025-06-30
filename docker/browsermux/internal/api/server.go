package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"

	"browsermux/internal/api/middleware"
	"browsermux/internal/browser"
	"browsermux/internal/config"
	"browsermux/internal/events"
	"browsermux/internal/webhook"
)

type Server struct {
	router          *mux.Router
	server          *http.Server
	cdpProxy        *browser.CDPProxy
	webhookManager  webhook.Manager
	eventDispatcher events.Dispatcher
	browserBaseURL  string
	frontendBaseURL string
	config          *config.Config
}

func NewServer(cdpProxy *browser.CDPProxy, webhookManager webhook.Manager, eventDispatcher events.Dispatcher, port string, cfg *config.Config) *Server {
	router := mux.NewRouter()

	browserBaseURL := cdpProxy.GetConfig().BrowserURL

	if strings.HasPrefix(browserBaseURL, "ws:") {
		browserBaseURL = "http:" + browserBaseURL[3:]
	} else if strings.HasPrefix(browserBaseURL, "wss:") {
		browserBaseURL = "https:" + browserBaseURL[4:]
	} else if !strings.HasPrefix(browserBaseURL, "http:") && !strings.HasPrefix(browserBaseURL, "https:") {
		browserBaseURL = "http://" + browserBaseURL
	}

	if lastIndex := strings.LastIndex(browserBaseURL, "/devtools/"); lastIndex != -1 {
		browserBaseURL = browserBaseURL[:lastIndex]
	}

	browserBaseURL = strings.TrimSuffix(browserBaseURL, "/")

	frontendBaseURL := strings.TrimSuffix(cfg.FrontendURL, "/")

	server := &Server{
		router:          router,
		cdpProxy:        cdpProxy,
		webhookManager:  webhookManager,
		eventDispatcher: eventDispatcher,
		browserBaseURL:  browserBaseURL,
		frontendBaseURL: frontendBaseURL,
		config:          cfg,
		server: &http.Server{
			Addr:    ":" + port,
			Handler: router,
		},
	}

	server.setupRoutes()

	return server
}

func (s *Server) Start() error {
	log.Printf("Starting API server on %s", s.server.Addr)
	log.Printf("Proxying browser at %s through frontend at %s", s.browserBaseURL, s.frontendBaseURL)
	return s.server.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}

func (s *Server) setupRoutes() {
	s.router.Use(middleware.Logging)
	s.router.Use(middleware.Recovery)

	s.router.HandleFunc("/json/version", s.handleJSONVersion).Methods("GET", "POST")
	s.router.HandleFunc("/json", s.handleJSONList).Methods("GET", "POST")
	s.router.HandleFunc("/json/list", s.handleJSONList).Methods("GET", "POST")
	s.router.HandleFunc("/json/new", s.handleJSONNew).Methods("GET", "POST")
	s.router.HandleFunc("/json/activate/{targetId}", s.handleJSONActivate).Methods("GET", "POST")
	s.router.HandleFunc("/json/close/{targetId}", s.handleJSONClose).Methods("GET", "POST")
	s.router.HandleFunc("/json/protocol", s.handleJSONProtocol).Methods("GET", "POST")

	api := s.router.PathPrefix("/api").Subrouter()

	browserHandler := browser.NewHandler(s.cdpProxy)
	browserHandler.RegisterRoutes(api)

	webhookHandler := webhook.NewHandler(s.webhookManager)
	webhookHandler.RegisterRoutes(api)

	s.router.HandleFunc("/devtools/{path:.*}", browserHandler.HandleWebSocket)

	s.router.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}).Methods("GET")
}

func (s *Server) handleJSONVersion(w http.ResponseWriter, r *http.Request) {
	var reqBody io.Reader
	if r.Body != nil {
		reqBody = r.Body
	}

	req, err := http.NewRequest(r.Method, s.browserBaseURL+"/json/version", reqBody)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error creating request: %v", err), http.StatusInternalServerError)
		return
	}

	for key, values := range r.Header {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}

	if r.Header.Get("Content-Type") != "" {
		req.Header.Set("Content-Type", r.Header.Get("Content-Type"))
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error connecting to browser: %v", err), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		for key, values := range resp.Header {
			for _, value := range values {
				w.Header().Add(key, value)
			}
		}
		w.WriteHeader(resp.StatusCode)
		io.Copy(w, resp.Body)
		return
	}

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

	if wsURL, ok := result["webSocketDebuggerUrl"].(string); ok {
		parsedURL, err := url.Parse(wsURL)
		if err == nil {
			frontendURL, _ := url.Parse(s.frontendBaseURL)
			newWsURL := "ws://" + frontendURL.Host + parsedURL.Path
			result["webSocketDebuggerUrl"] = newWsURL
		}
	}

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

func (s *Server) handleJSONList(w http.ResponseWriter, r *http.Request) {
	var reqBody io.Reader
	if r.Body != nil {
		reqBody = r.Body
	}

	req, err := http.NewRequest(r.Method, s.browserBaseURL+"/json/list", reqBody)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error creating request: %v", err), http.StatusInternalServerError)
		return
	}

	for key, values := range r.Header {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}

	if r.Header.Get("Content-Type") != "" {
		req.Header.Set("Content-Type", r.Header.Get("Content-Type"))
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error connecting to browser: %v", err), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		for key, values := range resp.Header {
			for _, value := range values {
				w.Header().Add(key, value)
			}
		}
		w.WriteHeader(resp.StatusCode)
		io.Copy(w, resp.Body)
		return
	}

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

	for i := range targets {
		if wsURL, ok := targets[i]["webSocketDebuggerUrl"].(string); ok {
			parsedURL, err := url.Parse(wsURL)
			if err == nil {
				frontendURL, _ := url.Parse(s.frontendBaseURL)
				newWsURL := "ws://" + frontendURL.Host + parsedURL.Path
				targets[i]["webSocketDebuggerUrl"] = newWsURL
			}
		}

		if devtoolsURL, ok := targets[i]["devtoolsFrontendUrl"].(string); ok {
			parsedURL, err := url.Parse(s.browserBaseURL)
			if err == nil {
				frontendURL, _ := url.Parse(s.frontendBaseURL)
				devtoolsURL = strings.Replace(devtoolsURL, parsedURL.Host, frontendURL.Host, -1)
				targets[i]["devtoolsFrontendUrl"] = devtoolsURL
			}
		}
	}

	for key, values := range resp.Header {
		if key != "Content-Length" {
			for _, value := range values {
				w.Header().Add(key, value)
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	json.NewEncoder(w).Encode(targets)
}
func (s *Server) handleJSONNew(w http.ResponseWriter, r *http.Request) {
	browserURL := s.browserBaseURL + "/json/new"
	if r.URL.RawQuery != "" {
		browserURL += "?" + r.URL.RawQuery
	}

	var reqBody io.Reader
	if r.Body != nil {
		reqBody = r.Body
	}

	req, err := http.NewRequest(r.Method, browserURL, reqBody)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error creating request: %v", err), http.StatusInternalServerError)
		return
	}

	for key, values := range r.Header {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}

	if r.Header.Get("Content-Type") != "" {
		req.Header.Set("Content-Type", r.Header.Get("Content-Type"))
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error connecting to browser: %v", err), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	s.proxyJSONResponse(w, resp)
}

func (s *Server) handleJSONActivate(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	targetID := vars["targetId"]

	var reqBody io.Reader
	if r.Body != nil {
		reqBody = r.Body
	}

	req, err := http.NewRequest(r.Method, s.browserBaseURL+"/json/activate/"+targetID, reqBody)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error creating request: %v", err), http.StatusInternalServerError)
		return
	}

	for key, values := range r.Header {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}

	if r.Header.Get("Content-Type") != "" {
		req.Header.Set("Content-Type", r.Header.Get("Content-Type"))
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error connecting to browser: %v", err), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

func (s *Server) handleJSONClose(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	targetID := vars["targetId"]

	var reqBody io.Reader
	if r.Body != nil {
		reqBody = r.Body
	}

	req, err := http.NewRequest(r.Method, s.browserBaseURL+"/json/close/"+targetID, reqBody)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error creating request: %v", err), http.StatusInternalServerError)
		return
	}

	for key, values := range r.Header {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}

	if r.Header.Get("Content-Type") != "" {
		req.Header.Set("Content-Type", r.Header.Get("Content-Type"))
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error connecting to browser: %v", err), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

func (s *Server) handleJSONProtocol(w http.ResponseWriter, r *http.Request) {
	var reqBody io.Reader
	if r.Body != nil {
		reqBody = r.Body
	}

	req, err := http.NewRequest(r.Method, s.browserBaseURL+"/json/protocol", reqBody)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error creating request: %v", err), http.StatusInternalServerError)
		return
	}

	for key, values := range r.Header {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}

	if r.Header.Get("Content-Type") != "" {
		req.Header.Set("Content-Type", r.Header.Get("Content-Type"))
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error connecting to browser: %v", err), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

func (s *Server) proxyJSONResponse(w http.ResponseWriter, resp *http.Response) {
	if resp.StatusCode != http.StatusOK {
		for key, values := range resp.Header {
			for _, value := range values {
				w.Header().Add(key, value)
			}
		}
		w.WriteHeader(resp.StatusCode)
		io.Copy(w, resp.Body)
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error reading browser response: %v", err), http.StatusInternalServerError)
		return
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		var resultArray []map[string]interface{}
		if err := json.Unmarshal(body, &resultArray); err != nil {
			for key, values := range resp.Header {
				for _, value := range values {
					w.Header().Add(key, value)
				}
			}
			w.WriteHeader(resp.StatusCode)
			w.Write(body)
			return
		}

		for i := range resultArray {
			s.rewriteWebSocketURLs(resultArray[i])
		}

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

	s.rewriteWebSocketURLs(result)

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

func (s *Server) rewriteWebSocketURLs(data map[string]interface{}) {
	wsKeys := []string{"webSocketDebuggerUrl", "devtoolsFrontendUrl"}

	for _, key := range wsKeys {
		if strVal, ok := data[key].(string); ok {
			parsedBrowserURL, err := url.Parse(s.browserBaseURL)
			if err == nil {
				parsedFrontendURL, _ := url.Parse(s.frontendBaseURL)

				newVal := strings.Replace(strVal, parsedBrowserURL.Host, parsedFrontendURL.Host, -1)

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

// handleVNCProxy handles proxying VNC requests to the browser container
func (s *Server) handleVNCProxy(w http.ResponseWriter, r *http.Request) {
	// Strip /vnc prefix from the path
	targetPath := strings.TrimPrefix(r.URL.Path, "/vnc")
	if targetPath == "" {
		targetPath = "/"
	}

	// Check if this is a WebSocket upgrade request
	if websocket.IsWebSocketUpgrade(r) {
		s.handleVNCWebSocket(w, r, targetPath)
		return
	}

	// Handle regular HTTP requests
	targetURL, err := url.Parse(s.vncBaseURL + targetPath)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error parsing VNC URL: %v", err), http.StatusInternalServerError)
		return
	}

	// Preserve query parameters
	targetURL.RawQuery = r.URL.RawQuery

	// Create reverse proxy
	proxy := httputil.NewSingleHostReverseProxy(targetURL)

	// Modify the request
	proxy.Director = func(req *http.Request) {
		req.URL.Scheme = targetURL.Scheme
		req.URL.Host = targetURL.Host
		req.URL.Path = targetPath
		req.URL.RawQuery = r.URL.RawQuery
		req.Host = targetURL.Host

		// Copy headers
		for key, values := range r.Header {
			for _, value := range values {
				req.Header.Add(key, value)
			}
		}
	}

	// Handle errors
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		log.Printf("VNC proxy error: %v", err)
		http.Error(w, fmt.Sprintf("VNC proxy error: %v", err), http.StatusBadGateway)
	}

	proxy.ServeHTTP(w, r)
}

// handleVNCWebSocket handles WebSocket connections for VNC
func (s *Server) handleVNCWebSocket(w http.ResponseWriter, r *http.Request, targetPath string) {
	// Parse the target WebSocket URL
	vncWSURL := strings.Replace(s.vncBaseURL, "http://", "ws://", 1)
	vncWSURL = strings.Replace(vncWSURL, "https://", "wss://", 1)

	targetURL := vncWSURL + targetPath
	if r.URL.RawQuery != "" {
		targetURL += "?" + r.URL.RawQuery
	}

	// Connect to the VNC server
	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	// Copy headers for the upstream connection
	headers := http.Header{}
	for key, values := range r.Header {
		if key != "Connection" && key != "Upgrade" && key != "Sec-Websocket-Key" &&
			key != "Sec-Websocket-Version" && key != "Sec-Websocket-Extensions" {
			for _, value := range values {
				headers.Add(key, value)
			}
		}
	}

	upstreamConn, _, err := dialer.Dial(targetURL, headers)
	if err != nil {
		log.Printf("Failed to connect to VNC WebSocket: %v", err)
		http.Error(w, fmt.Sprintf("Failed to connect to VNC: %v", err), http.StatusBadGateway)
		return
	}
	defer upstreamConn.Close()

	// Upgrade the client connection
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true // Allow all origins for VNC
		},
	}

	clientConn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Failed to upgrade client connection: %v", err)
		return
	}
	defer clientConn.Close()

	// Proxy messages between client and VNC server
	errChan := make(chan error, 2)

	// Client to VNC server
	go func() {
		for {
			messageType, data, err := clientConn.ReadMessage()
			if err != nil {
				errChan <- fmt.Errorf("client read error: %v", err)
				return
			}

			if err := upstreamConn.WriteMessage(messageType, data); err != nil {
				errChan <- fmt.Errorf("upstream write error: %v", err)
				return
			}
		}
	}()

	// VNC server to client
	go func() {
		for {
			messageType, data, err := upstreamConn.ReadMessage()
			if err != nil {
				errChan <- fmt.Errorf("upstream read error: %v", err)
				return
			}

			if err := clientConn.WriteMessage(messageType, data); err != nil {
				errChan <- fmt.Errorf("client write error: %v", err)
				return
			}
		}
	}()

	// Wait for an error or connection close
	select {
	case err := <-errChan:
		log.Printf("VNC WebSocket proxy error: %v", err)
	}
}
