package cdp

import (
	"encoding/json"
)

type Message struct {
	ID     int                    `json:"id,omitempty"`
	Method string                 `json:"method,omitempty"`
	Params map[string]interface{} `json:"params,omitempty"`
	Result json.RawMessage        `json:"result,omitempty"`
	Error  *Error                 `json:"error,omitempty"`
}

type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (m *Message) IsCommand() bool {
	return m.ID != 0 && m.Method != ""
}

func (m *Message) IsEvent() bool {
	return m.ID == 0 && m.Method != ""
}

func (m *Message) IsResponse() bool {
	return m.ID != 0 && m.Method == ""
}
