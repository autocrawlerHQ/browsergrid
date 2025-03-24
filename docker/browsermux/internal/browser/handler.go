package browser

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for now
	},
}

// Handler handles browser-related API requests
type Handler struct {
	clientManager     ClientManager
	connectionManager ConnectionManager
}

// NewHandler creates a new browser handler
func NewHandler(cdpProxy *CDPProxy) *Handler {
	return &Handler{
		clientManager:     cdpProxy,
		connectionManager: cdpProxy,
	}
}

// RegisterRoutes registers all handler routes on the provided router
func (h *Handler) RegisterRoutes(router *mux.Router) {
	router.HandleFunc("/browser", h.GetBrowserInfo).Methods("GET")
	router.HandleFunc("/clients", h.GetClients).Methods("GET")
}

// GetBrowserInfo returns information about the browser
func (h *Handler) GetBrowserInfo(w http.ResponseWriter, r *http.Request) {
	info, err := h.connectionManager.GetInfo()
	if err != nil {
		http.Error(w, "Failed to get browser info: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Add client count
	data := map[string]interface{}{
		"browser": info,
		"clients": h.clientManager.GetClientCount(),
		"status":  h.connectionManager.IsConnected(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

// GetClients returns a list of connected clients
func (h *Handler) GetClients(w http.ResponseWriter, r *http.Request) {
	clients := h.clientManager.GetClients()

	data := map[string]interface{}{
		"clients": clients,
		"count":   len(clients),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

// HandleWebSocket handles WebSocket connections
func (h *Handler) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Extract path variables from URL
	vars := mux.Vars(r)
	path := vars["path"]

	log.Printf("WebSocket connection request for path: %s", path)

	// Upgrade HTTP connection to WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Error upgrading connection to WebSocket: %v", err)
		return
	}

	// Extract client metadata from request
	metadata := extractClientMetadata(r)

	// Add path information to metadata
	metadata["path"] = path

	// Extract target ID if this is a session-based endpoint
	if strings.HasPrefix(path, "page/") {
		parts := strings.Split(path, "/")
		if len(parts) > 1 {
			metadata["target_id"] = parts[1]
		}
	}

	// Add client to CDP Proxy
	clientID, err := h.clientManager.AddClient(conn, metadata)
	if err != nil {
		log.Printf("Error adding client: %v", err)
		conn.Close()
		return
	}

	log.Printf("Client %s connected with path %s", clientID, path)
}

// extractClientMetadata extracts metadata from the request
func extractClientMetadata(r *http.Request) map[string]interface{} {
	metadata := make(map[string]interface{})

	// Add user agent
	metadata["user_agent"] = r.UserAgent()

	// Add remote address
	metadata["remote_addr"] = r.RemoteAddr

	// Add any query parameters
	query := r.URL.Query()
	for key, values := range query {
		if len(values) > 0 {
			metadata[key] = values[0]
		}
	}

	return metadata
}
