package browser

import (
	"bytes"
	"log"
	"time"

	"github.com/gorilla/websocket"

	"browsermux/internal/cdp"
	"browsermux/internal/events"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 512 * 1024
)

func (c *Client) ToModel() *ClientDTO {
	return &ClientDTO{
		ID:        c.ID,
		Connected: c.Connected,
		Metadata:  c.Metadata,
		CreatedAt: c.CreatedAt,
	}
}

func NewClient(id string, conn *websocket.Conn, dispatcher events.Dispatcher, cdpProxy *CDPProxy, metadata map[string]interface{}) *Client {
	return &Client{
		ID:         id,
		Conn:       conn,
		Send:       make(chan []byte, 256),
		Dispatcher: dispatcher,
		CDPProxy:   cdpProxy,
		Metadata:   metadata,
		CreatedAt:  time.Now(),
		Connected:  true,
	}
}

func (c *Client) Start() {
	go c.ReadPump()
	go c.WritePump()

	c.Dispatcher.Dispatch(events.Event{
		Type:       events.EventClientConnected,
		SourceType: "client",
		SourceID:   c.ID,
		Timestamp:  time.Now(),
		Params:     c.Metadata,
	})
}

func (c *Client) Close() error {
	c.Connected = false

	c.Dispatcher.Dispatch(events.Event{
		Type:       events.EventClientDisconnected,
		SourceType: "client",
		SourceID:   c.ID,
		Timestamp:  time.Now(),
	})

	return c.Conn.Close()
}

func (c *Client) ReadPump() {
	defer func() {
		if err := c.CDPProxy.RemoveClient(c.ID); err != nil {
			log.Printf("Error removing client %s: %v", c.ID, err)
		}
	}()

	c.Conn.SetReadLimit(maxMessageSize)
	c.Conn.SetReadDeadline(time.Now().Add(pongWait))
	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, message, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("Read error: %v", err)
			}
			break
		}

		c.processMessage(message)
	}
}

func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.Send:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.Conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}

			w.Write(message)

			n := len(c.Send)
			for i := 0; i < n; i++ {
				w.Write(bytes.TrimSpace([]byte{'\n'}))
				w.Write(<-c.Send)
			}

			if err := w.Close(); err != nil {
				return
			}
		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *Client) SendMessage(message []byte) error {
	select {
	case c.Send <- message:
		return nil
	default:
		return c.Conn.Close()
	}
}

func (c *Client) processMessage(message []byte) {
	cdpMsg, err := cdp.ParseMessage(message)
	if err == nil {
		log.Printf("Received message from client %s: %s (id: %s)", c.ID, cdpMsg.Method, cdpMsg.ID)

		c.Dispatcher.Dispatch(events.Event{
			Type:       events.EventCDPCommand,
			Method:     cdpMsg.Method,
			Params:     cdpMsg.Params,
			SourceType: "client",
			SourceID:   c.ID,
			Timestamp:  time.Now(),
		})
	} else {
		log.Printf("Received message from client %s: %s", c.ID, string(message))
	}

	c.CDPProxy.HandleClientMessage(c.ID, message)
}
