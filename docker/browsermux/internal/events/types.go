package events

// todo should this be in internal/models?
import (
	"time"
)

// EventType represents the type of system event
type EventType string

const (
	// CDP events
	EventCDPCommand EventType = "cdp.command"
	EventCDPEvent   EventType = "cdp.event"
	
	// Client events
	EventClientConnected    EventType = "client.connected"
	EventClientDisconnected EventType = "client.disconnected"
	
	// Webhook events
	EventWebhookRegistered EventType = "webhook.registered"
	EventWebhookTriggered  EventType = "webhook.triggered"
	EventWebhookExecuted   EventType = "webhook.executed"
)

// Event represents a system event
type Event struct {
	Type       EventType              `json:"type"`
	Method     string                 `json:"method,omitempty"`
	Params     map[string]interface{} `json:"params,omitempty"`
	SourceType string                 `json:"source_type,omitempty"` // "client", "browser", "system"
	SourceID   string                 `json:"source_id,omitempty"`
	Timestamp  time.Time              `json:"timestamp"`
}

type EventHandler func(Event)