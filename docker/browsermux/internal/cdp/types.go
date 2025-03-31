package cdp

import (
	"encoding/json"
)

// Message represents a Chrome DevTools Protocol message
type Message struct {
	ID     int                    `json:"id,omitempty"`
	Method string                 `json:"method,omitempty"`
	Params map[string]interface{} `json:"params,omitempty"`
	Result json.RawMessage        `json:"result,omitempty"`
	Error  *Error                 `json:"error,omitempty"`
}

// Error represents a CDP error
type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// IsCommand returns true if the message is a command (has an ID and Method)
func (m *Message) IsCommand() bool {
	return m.ID != 0 && m.Method != ""
}

// IsEvent returns true if the message is an event (has a Method but no ID)
func (m *Message) IsEvent() bool {
	return m.ID == 0 && m.Method != ""
}

// IsResponse returns true if the message is a response (has an ID but no Method)
func (m *Message) IsResponse() bool {
	return m.ID != 0 && m.Method == ""
}
