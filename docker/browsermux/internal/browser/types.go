package browser

import (
	"browsermux/internal/events"
	"time"

	"github.com/gorilla/websocket"
)

type Client struct {
	ID         string
	Conn       *websocket.Conn
	Send       chan []byte
	Dispatcher events.Dispatcher
	CDPProxy   *CDPProxy
	Metadata   map[string]interface{}
	CreatedAt  time.Time
	Connected  bool
}

type ClientDTO struct {
	ID        string                 `json:"id"`
	Conn      *websocket.Conn        `json:"-"`
	Messages  chan []byte            `json:"-"`
	Connected bool                   `json:"connected"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt time.Time              `json:"created_at"`
}

type ClientManager interface {
	AddClient(conn *websocket.Conn, metadata map[string]interface{}) (string, error)
	RemoveClient(clientID string) error
	GetClients() []*ClientDTO
	GetClientCount() int
}

type ConnectionManager interface {
	Connect() error
	Disconnect() error
	IsConnected() bool
	GetInfo() (*BrowserInfo, error)
}

type MessageHandler interface {
	HandleClientMessage(clientID string, message []byte) error
	HandleBrowserMessage(message []byte) error
}
