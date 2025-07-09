package api

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
			requestHost := s.getRequestHost(r)
			newWsURL := "ws://" + requestHost + parsedURL.Path
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

	requestHost := s.getRequestHost(r)
	for i := range targets {
		s.rewriteWebSocketURLsWithHost(targets[i], requestHost)
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

	s.proxyJSONResponse(w, r, resp)
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

func (s *Server) proxyJSONResponse(w http.ResponseWriter, r *http.Request, resp *http.Response) {
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

		requestHost := s.getRequestHost(r)
		for i := range resultArray {
			s.rewriteWebSocketURLsWithHost(resultArray[i], requestHost)
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

	requestHost := s.getRequestHost(r)
	s.rewriteWebSocketURLsWithHost(result, requestHost)

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

func (s *Server) getRequestHost(r *http.Request) string {
	host := r.Host
	if host == "" {
		host = r.Header.Get("Host")
	}
	if host == "" {
		if parsedURL, err := url.Parse(s.frontendBaseURL); err == nil {
			host = parsedURL.Host
		}
	}
	return host
}

func (s *Server) rewriteWebSocketURLsWithHost(data map[string]interface{}, requestHost string) {
	wsKeys := []string{"webSocketDebuggerUrl", "devtoolsFrontendUrl"}

	for _, key := range wsKeys {
		if strVal, ok := data[key].(string); ok {
			parsedBrowserURL, err := url.Parse(s.browserBaseURL)
			if err == nil {
				newVal := strings.Replace(strVal, parsedBrowserURL.Host, requestHost, -1)

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
