package browser

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
)

type mockClientManager struct {
	clients     []*ClientDTO
	clientCount int
}

func (m *mockClientManager) AddClient(conn *websocket.Conn, metadata map[string]interface{}) (string, error) {
	m.clientCount++
	return "test-client-id", nil
}

func (m *mockClientManager) RemoveClient(clientID string) error {
	m.clientCount--
	return nil
}

func (m *mockClientManager) GetClients() []*ClientDTO {
	return m.clients
}

func (m *mockClientManager) GetClientCount() int {
	return m.clientCount
}

type mockConnectionManager struct {
	connected   bool
	info        *BrowserInfo
	shouldError bool
}

func (m *mockConnectionManager) Connect() error {
	m.connected = true
	return nil
}

func (m *mockConnectionManager) Disconnect() error {
	m.connected = false
	return nil
}

func (m *mockConnectionManager) IsConnected() bool {
	return m.connected
}

func (m *mockConnectionManager) GetInfo() (*BrowserInfo, error) {
	if m.shouldError {
		return nil, http.ErrServerClosed
	}
	if m.info != nil {
		return m.info, nil
	}
	return &BrowserInfo{
		URL:       "ws://localhost:9222/devtools/browser",
		Version:   "Chrome/120.0.0.0",
		UserAgent: "Mozilla/5.0 Chrome/120.0.0.0",
		Status:    "connected",
	}, nil
}

func TestNewHandler(t *testing.T) {
	proxy := &CDPProxy{
		clients: make(map[string]*Client),
		config:  DefaultConfig(),
	}

	handler := NewHandler(proxy)

	if handler == nil {
		t.Fatal("NewHandler() returned nil")
	}

	if handler.clientManager == nil {
		t.Error("Handler clientManager should not be nil")
	}

	if handler.connectionManager == nil {
		t.Error("Handler connectionManager should not be nil")
	}
}

func TestHandlerGetBrowserInfo(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		clientManager := &mockClientManager{clientCount: 2}
		connectionManager := &mockConnectionManager{
			connected: true,
			info: &BrowserInfo{
				URL:       "ws://localhost:9222/devtools/browser",
				Version:   "Chrome/120.0.0.0",
				UserAgent: "Mozilla/5.0 Chrome/120.0.0.0",
				Status:    "connected",
			},
		}

		handler := &Handler{
			clientManager:     clientManager,
			connectionManager: connectionManager,
		}

		req := httptest.NewRequest("GET", "/browser", nil)
		rr := httptest.NewRecorder()

		handler.GetBrowserInfo(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
		}

		var response map[string]interface{}
		err := json.NewDecoder(rr.Body).Decode(&response)
		if err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if response["clients"] != float64(2) {
			t.Errorf("Expected 2 clients, got %v", response["clients"])
		}

		if response["status"] != true {
			t.Errorf("Expected status true, got %v", response["status"])
		}

		browserInfo, ok := response["browser"].(map[string]interface{})
		if !ok {
			t.Fatal("Expected browser info in response")
		}

		if browserInfo["version"] != "Chrome/120.0.0.0" {
			t.Errorf("Expected version Chrome/120.0.0.0, got %v", browserInfo["version"])
		}
	})

	t.Run("Error Getting Browser Info", func(t *testing.T) {
		clientManager := &mockClientManager{}
		connectionManager := &mockConnectionManager{
			shouldError: true,
		}

		handler := &Handler{
			clientManager:     clientManager,
			connectionManager: connectionManager,
		}

		req := httptest.NewRequest("GET", "/browser", nil)
		rr := httptest.NewRecorder()

		handler.GetBrowserInfo(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("Expected status %d, got %d", http.StatusInternalServerError, rr.Code)
		}
	})
}

func TestHandlerGetClients(t *testing.T) {
	clientManager := &mockClientManager{
		clients: []*ClientDTO{
			{
				ID:        "client-1",
				Connected: true,
				Metadata:  map[string]interface{}{"user_agent": "test-agent-1"},
			},
			{
				ID:        "client-2",
				Connected: true,
				Metadata:  map[string]interface{}{"user_agent": "test-agent-2"},
			},
		},
	}
	connectionManager := &mockConnectionManager{}

	handler := &Handler{
		clientManager:     clientManager,
		connectionManager: connectionManager,
	}

	req := httptest.NewRequest("GET", "/clients", nil)
	rr := httptest.NewRecorder()

	handler.GetClients(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var response map[string]interface{}
	err := json.NewDecoder(rr.Body).Decode(&response)
	if err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response["count"] != float64(2) {
		t.Errorf("Expected count 2, got %v", response["count"])
	}

	clients, ok := response["clients"].([]interface{})
	if !ok {
		t.Fatal("Expected clients array in response")
	}

	if len(clients) != 2 {
		t.Errorf("Expected 2 clients, got %d", len(clients))
	}
}

func TestHandlerRegisterRoutes(t *testing.T) {
	proxy := &CDPProxy{
		clients: make(map[string]*Client),
		config:  DefaultConfig(),
	}

	handler := NewHandler(proxy)
	router := mux.NewRouter()

	handler.RegisterRoutes(router)

	t.Run("Browser Info Route", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/browser", nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code == http.StatusNotFound {
			t.Error("Browser info route not registered")
		}
	})

	t.Run("Clients Route", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/clients", nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code == http.StatusNotFound {
			t.Error("Clients route not registered")
		}
	})
}

func TestExtractClientMetadata(t *testing.T) {
	t.Run("Basic Metadata", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("User-Agent", "test-browser/1.0")
		req.RemoteAddr = "192.168.1.100:12345"

		metadata := extractClientMetadata(req)

		if metadata["user_agent"] != "test-browser/1.0" {
			t.Errorf("Expected user_agent 'test-browser/1.0', got %v", metadata["user_agent"])
		}

		if metadata["remote_addr"] != "192.168.1.100:12345" {
			t.Errorf("Expected remote_addr '192.168.1.100:12345', got %v", metadata["remote_addr"])
		}
	})

	t.Run("Query Parameters", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test?session_id=123&debug=true", nil)

		metadata := extractClientMetadata(req)

		if metadata["session_id"] != "123" {
			t.Errorf("Expected session_id '123', got %v", metadata["session_id"])
		}

		if metadata["debug"] != "true" {
			t.Errorf("Expected debug 'true', got %v", metadata["debug"])
		}
	})

	t.Run("Multiple Query Values", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test?tags=tag1&tags=tag2", nil)

		metadata := extractClientMetadata(req)

		if metadata["tags"] != "tag1" {
			t.Errorf("Expected tags 'tag1', got %v", metadata["tags"])
		}
	})
}

func TestHandleWebSocket(t *testing.T) {
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				break
			}
			if err := conn.WriteMessage(websocket.TextMessage, message); err != nil {
				break
			}
		}
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Skip("Cannot establish WebSocket connection for test:", err)
	}
	defer conn.Close()

	testMessage := []byte("test message")
	err = conn.WriteMessage(websocket.TextMessage, testMessage)
	if err != nil {
		t.Fatalf("Error sending message: %v", err)
	}

	_, response, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("Error reading message: %v", err)
	}

	if string(response) != string(testMessage) {
		t.Errorf("Expected echo response %s, got %s", string(testMessage), string(response))
	}
}
