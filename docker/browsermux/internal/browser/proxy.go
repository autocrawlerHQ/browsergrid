package browser

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	"browsermux/internal/cdp"
	"browsermux/internal/events"
)

// Ensure CDPProxy implements all required interfaces
var _ ClientManager = (*CDPProxy)(nil)
var _ ConnectionManager = (*CDPProxy)(nil)
var _ MessageHandler = (*CDPProxy)(nil)

// CDPProxy manages clients and browser connections
type CDPProxy struct {
	browserConn     *websocket.Conn
	clients         map[string]*Client
	eventDispatcher events.Dispatcher
	config          CDPProxyConfig
	browserMessages chan []byte
	mu              sync.RWMutex
	connected       bool
	shutdown        chan struct{}
}

// CDPProxyConfig holds configuration options for the CDP proxy
type CDPProxyConfig struct {
	BrowserURL        string
	MaxMessageSize    int
	ConnectionTimeout time.Duration
}

// GetConfig returns the proxy configuration
func (p *CDPProxy) GetConfig() CDPProxyConfig {
	return p.config
}

// DefaultConfig returns a default configuration for the CDP proxy
func DefaultConfig() CDPProxyConfig {
	return CDPProxyConfig{
		BrowserURL:        "ws://localhost:9222/devtools/browser",
		MaxMessageSize:    1024 * 1024,
		ConnectionTimeout: 10 * time.Second,
	}
}

// NewCDPProxy creates a new CDP proxy instance
func NewCDPProxy(dispatcher events.Dispatcher, config CDPProxyConfig) (*CDPProxy, error) {
	p := &CDPProxy{
		clients:         make(map[string]*Client),
		eventDispatcher: dispatcher,
		config:          config,
		browserMessages: make(chan []byte, 100),
		shutdown:        make(chan struct{}),
	}

	if err := p.Connect(); err != nil {
		return nil, fmt.Errorf("CDP proxy initialization error: %w", err)
	}

	// Start message handlers
	go p.processBrowserMessages()
	go p.processClientMessages()

	return p, nil
}

// Connect establishes a connection to the browser
func (p *CDPProxy) Connect() error {
	// Get the actual WebSocket URL from browser info endpoint
	log.Printf("Attempting to connect to browser at %s", p.config.BrowserURL)
	browserInfo, err := GetBrowserInfo(p.config.BrowserURL)
	if err != nil {
		return fmt.Errorf("failed to get browser info: %w", err)
	}

	// Use the actual WebSocket URL from the browser info
	actualBrowserURL := browserInfo.URL
	log.Printf("Using browser WebSocket URL: %s (transformed from original endpoint)", actualBrowserURL)

	if err := p.connectToBrowser(actualBrowserURL); err != nil {
		return fmt.Errorf("browser connection error: %w", err)
	}
	return nil
}

// Disconnect closes the browser connection
func (p *CDPProxy) Disconnect() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.browserConn != nil {
		if err := p.browserConn.Close(); err != nil {
			return fmt.Errorf("error closing browser connection: %w", err)
		}
		p.browserConn = nil
	}

	p.connected = false
	return nil
}

// IsConnected returns the connection status
func (p *CDPProxy) IsConnected() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.connected
}

// GetInfo returns information about the browser
func (p *CDPProxy) GetInfo() (*BrowserInfo, error) {
	return GetBrowserInfo(p.config.BrowserURL)
}

// HandleClientMessage processes a message from a client
func (p *CDPProxy) HandleClientMessage(clientID string, message []byte) error {
	p.mu.RLock()
	if !p.connected || p.browserConn == nil {
		p.mu.RUnlock()
		return fmt.Errorf("browser not connected")
	}
	p.mu.RUnlock()

	if err := p.browserConn.WriteMessage(websocket.TextMessage, message); err != nil {
		log.Printf("Error sending message to browser: %v", err)
		return fmt.Errorf("failed to send message to browser: %w", err)
	}
	return nil
}

// HandleBrowserMessage processes a message from the browser
func (p *CDPProxy) HandleBrowserMessage(message []byte) error {
	cdpMsg, err := cdp.ParseMessage(message)
	if err == nil && cdpMsg.IsEvent() {
		p.eventDispatcher.Dispatch(events.Event{
			Type:       events.EventCDPEvent,
			Method:     cdpMsg.Method,
			Params:     cdpMsg.Params,
			SourceType: "browser",
			Timestamp:  time.Now(),
		})
	}

	p.mu.RLock()
	for _, client := range p.clients {
		if client.Connected {
			select {
			case client.Send <- message:
			default:
				log.Printf("Client %s message buffer full, dropping message", client.ID)
			}
		}
	}
	p.mu.RUnlock()
	return nil
}

func (p *CDPProxy) connectToBrowser(browserURL string) error {
	dialer := websocket.Dialer{
		HandshakeTimeout: p.config.ConnectionTimeout,
	}

	conn, _, err := dialer.Dial(browserURL, nil)
	if err != nil {
		return fmt.Errorf("websocket connection error: %w", err)
	}

	p.browserConn = conn
	p.connected = true

	p.browserConn.SetReadLimit(int64(p.config.MaxMessageSize))

	log.Printf("Connected to browser at %s", browserURL)

	return nil
}

func (p *CDPProxy) AddClient(conn *websocket.Conn, metadata map[string]interface{}) (string, error) {
	clientID := uuid.New().String()

	client := NewClient(clientID, conn, p.eventDispatcher, p, metadata)

	p.mu.Lock()
	p.clients[clientID] = client
	p.mu.Unlock()

	p.eventDispatcher.Dispatch(events.Event{
		Type:       events.EventClientConnected,
		SourceID:   clientID,
		SourceType: "client",
		Timestamp:  time.Now(),
		Params: map[string]interface{}{
			"client_id": clientID,
			"metadata":  metadata,
		},
	})

	go p.handleClientMessages(client)
	go p.sendMessagesToClient(client)

	return clientID, nil
}

func (p *CDPProxy) RemoveClient(clientID string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	client, exists := p.clients[clientID]
	if !exists {
		return fmt.Errorf("client %s not found", clientID)
	}

	close(client.Send)

	delete(p.clients, clientID)

	p.eventDispatcher.Dispatch(events.Event{
		Type:       events.EventClientDisconnected,
		SourceID:   clientID,
		SourceType: "client",
		Timestamp:  time.Now(),
		Params: map[string]interface{}{
			"client_id": clientID,
		},
	})

	log.Printf("Removed client %s, remaining clients: %d", clientID, len(p.clients))

	return nil
}

func (p *CDPProxy) GetClientCount() int {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return len(p.clients)
}

func (p *CDPProxy) GetClients() []*ClientDTO {
	p.mu.RLock()
	defer p.mu.RUnlock()

	clients := make([]*ClientDTO, 0, len(p.clients))
	for _, client := range p.clients {
		clients = append(clients, client.ToModel())
	}

	return clients
}

func (p *CDPProxy) Shutdown() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	close(p.shutdown)

	if p.browserConn != nil {
		p.browserConn.Close()
		p.connected = false
	}

	for clientID, client := range p.clients {
		if client.Connected {
			client.Conn.Close()
			client.Connected = false
		}
		delete(p.clients, clientID)
	}

	log.Println("CDP Proxy shutdown complete")

	return nil
}

func (p *CDPProxy) handleClientMessages(client *Client) {
	defer func() {
		p.RemoveClient(client.ID)
	}()

	client.Conn.SetReadLimit(int64(p.config.MaxMessageSize))

	for {
		_, message, err := client.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("Client %s error: %v", client.ID, err)
			}
			break
		}

		cdpMsg, err := cdp.ParseMessage(message)
		if err == nil && cdpMsg.IsCommand() {
			p.eventDispatcher.Dispatch(events.Event{
				Type:       events.EventCDPCommand,
				Method:     cdpMsg.Method,
				Params:     cdpMsg.Params,
				SourceID:   client.ID,
				SourceType: "client",
				Timestamp:  time.Now(),
			})
		}

		select {
		case p.browserMessages <- message:
		case <-p.shutdown:
			return
		}
	}
}

func (p *CDPProxy) sendMessagesToClient(client *Client) {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
	}()

	for {
		select {
		case message, ok := <-client.Send:
			if !ok {
				return
			}

			err := client.Conn.WriteMessage(websocket.TextMessage, message)
			if err != nil {
				log.Printf("Error sending message to client %s: %v", client.ID, err)
				return
			}
		case <-ticker.C:
			if err := client.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				log.Printf("Error sending ping to client %s: %v", client.ID, err)
				return
			}
		case <-p.shutdown:
			return
		}
	}
}

func (p *CDPProxy) processBrowserMessages() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Recovered in processBrowserMessages: %v", r)
		}
	}()

	for {
		select {
		case <-p.shutdown:
			return
		default:
			_, message, err := p.browserConn.ReadMessage()
			if err != nil {
				log.Printf("Error reading from browser: %v", err)

				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					if err := p.reconnectToBrowser(); err != nil {
						log.Printf("Failed to reconnect to browser: %v", err)
						time.Sleep(5 * time.Second)
					}
				}
				continue
			}

			p.processBrowserMessage(message)
		}
	}
}

func (p *CDPProxy) processClientMessages() {
	for {
		select {
		case message := <-p.browserMessages:
			if err := p.browserConn.WriteMessage(websocket.TextMessage, message); err != nil {
				log.Printf("Error sending message to browser: %v", err)

				if err := p.reconnectToBrowser(); err != nil {
					log.Printf("Failed to reconnect to browser: %v", err)
				}
			}

		case <-p.shutdown:
			return
		}
	}
}

func (p *CDPProxy) processBrowserMessage(message []byte) {
	cdpMsg, err := cdp.ParseMessage(message)
	if err == nil && cdpMsg.IsEvent() {
		p.eventDispatcher.Dispatch(events.Event{
			Type:       events.EventCDPEvent,
			Method:     cdpMsg.Method,
			Params:     cdpMsg.Params,
			SourceType: "browser",
			Timestamp:  time.Now(),
		})
	}

	p.mu.RLock()
	for _, client := range p.clients {
		if client.Connected {
			select {
			case client.Send <- message:
			default:
				log.Printf("Client %s message buffer full, dropping message", client.ID)
			}
		}
	}
	p.mu.RUnlock()
}

func (p *CDPProxy) reconnectToBrowser() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.browserConn != nil {
		p.browserConn.Close()
	}

	// Get the latest WebSocket URL before reconnecting
	log.Printf("Attempting to reconnect to browser at %s", p.config.BrowserURL)
	browserInfo, err := GetBrowserInfo(p.config.BrowserURL)
	if err != nil {
		p.connected = false
		return fmt.Errorf("failed to get browser info for reconnection: %w", err)
	}

	// Use the actual WebSocket URL from the browser info
	actualBrowserURL := browserInfo.URL
	log.Printf("Reconnecting using WebSocket URL: %s (transformed from original endpoint)", actualBrowserURL)

	dialer := websocket.Dialer{
		HandshakeTimeout: p.config.ConnectionTimeout,
	}

	p.browserConn, _, err = dialer.Dial(actualBrowserURL, nil)
	if err != nil {
		p.connected = false
		return fmt.Errorf("failed to reconnect to browser: %w", err)
	}

	p.connected = true
	p.browserConn.SetReadLimit(int64(p.config.MaxMessageSize))

	log.Printf("Reconnected to browser at %s", actualBrowserURL)

	return nil
}
