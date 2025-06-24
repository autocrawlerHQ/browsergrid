package events

import (
	"time"
)

type EventType string

const (
	EventCDPCommand EventType = "cdp.command"
	EventCDPEvent   EventType = "cdp.event"

	EventClientConnected    EventType = "client.connected"
	EventClientDisconnected EventType = "client.disconnected"

	EventWebhookRegistered EventType = "webhook.registered"
	EventWebhookTriggered  EventType = "webhook.triggered"
	EventWebhookExecuted   EventType = "webhook.executed"
)

type Event struct {
	Type       EventType              `json:"type"`
	Method     string                 `json:"method,omitempty"`
	Params     map[string]interface{} `json:"params,omitempty"`
	SourceType string                 `json:"source_type,omitempty"`
	SourceID   string                 `json:"source_id,omitempty"`
	Timestamp  time.Time              `json:"timestamp"`
}

type EventHandler func(Event)
