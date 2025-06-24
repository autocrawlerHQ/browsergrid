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
		return true
	},
}

type Handler struct {
	clientManager     ClientManager
	connectionManager ConnectionManager
}

func NewHandler(cdpProxy *CDPProxy) *Handler {
	return &Handler{
		clientManager:     cdpProxy,
		connectionManager: cdpProxy,
	}
}

func (h *Handler) RegisterRoutes(router *mux.Router) {
	router.HandleFunc("/browser", h.GetBrowserInfo).Methods("GET")
	router.HandleFunc("/clients", h.GetClients).Methods("GET")
}

func (h *Handler) GetBrowserInfo(w http.ResponseWriter, r *http.Request) {
	info, err := h.connectionManager.GetInfo()
	if err != nil {
		http.Error(w, "Failed to get browser info: "+err.Error(), http.StatusInternalServerError)
		return
	}

	data := map[string]interface{}{
		"browser": info,
		"clients": h.clientManager.GetClientCount(),
		"status":  h.connectionManager.IsConnected(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func (h *Handler) GetClients(w http.ResponseWriter, r *http.Request) {
	clients := h.clientManager.GetClients()

	data := map[string]interface{}{
		"clients": clients,
		"count":   len(clients),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func (h *Handler) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	path := vars["path"]

	log.Printf("WebSocket connection request for path: %s", path)

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Error upgrading connection to WebSocket: %v", err)
		return
	}

	metadata := extractClientMetadata(r)

	metadata["path"] = path

	if strings.HasPrefix(path, "page/") {
		parts := strings.Split(path, "/")
		if len(parts) > 1 {
			metadata["target_id"] = parts[1]
		}
	}

	clientID, err := h.clientManager.AddClient(conn, metadata)
	if err != nil {
		log.Printf("Error adding client: %v", err)
		conn.Close()
		return
	}

	log.Printf("Client %s connected with path %s", clientID, path)
}

func extractClientMetadata(r *http.Request) map[string]interface{} {
	metadata := make(map[string]interface{})

	metadata["user_agent"] = r.UserAgent()

	metadata["remote_addr"] = r.RemoteAddr

	query := r.URL.Query()
	for key, values := range query {
		if len(values) > 0 {
			metadata[key] = values[0]
		}
	}

	return metadata
}
