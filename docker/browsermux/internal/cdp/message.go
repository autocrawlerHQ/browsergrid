package cdp

import (
	"encoding/json"
	"reflect"
	"strings"
)

// ParseMessage parses a raw JSON message into a CDP Message
func ParseMessage(data []byte) (*Message, error) {
	msg := &Message{}
	if err := json.Unmarshal(data, msg); err != nil {
		return nil, err
	}
	return msg, nil
}

// MatchesFilter checks if a message matches the given filter criteria
func MatchesFilter(msg *Message, methodFilter string, paramsFilter map[string]interface{}) bool {
	// Check method match (exact match or wildcard)
	if methodFilter != "*" && methodFilter != msg.Method {
		return false
	}

	// If no param filters, it's a match
	if len(paramsFilter) == 0 {
		return true
	}

	// Check all param filters
	for key, expectedValue := range paramsFilter {
		// Handle nested parameters with dot notation (e.g., "response.url")
		parts := strings.Split(key, ".")
		actualValue := interface{}(msg.Params)

		// Navigate through nested objects
		for _, part := range parts {
			if m, ok := actualValue.(map[string]interface{}); ok {
				if v, exists := m[part]; exists {
					actualValue = v
				} else {
					return false // Path doesn't exist
				}
			} else {
				return false // Not a map, can't navigate further
			}
		}

		// Check if values match
		if !reflect.DeepEqual(actualValue, expectedValue) {
			return false
		}
	}

	return true
}
