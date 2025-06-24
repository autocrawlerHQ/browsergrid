package cdp

import (
	"encoding/json"
	"reflect"
	"strings"
)

func ParseMessage(data []byte) (*Message, error) {
	msg := &Message{}
	if err := json.Unmarshal(data, msg); err != nil {
		return nil, err
	}
	return msg, nil
}

func MatchesFilter(msg *Message, methodFilter string, paramsFilter map[string]interface{}) bool {
	if methodFilter != "*" && methodFilter != msg.Method {
		return false
	}

	if len(paramsFilter) == 0 {
		return true
	}

	for key, expectedValue := range paramsFilter {
		parts := strings.Split(key, ".")
		actualValue := interface{}(msg.Params)

		for _, part := range parts {
			if m, ok := actualValue.(map[string]interface{}); ok {
				if v, exists := m[part]; exists {
					actualValue = v
				} else {
					return false
				}
			} else {
				return false
			}
		}

		if !reflect.DeepEqual(actualValue, expectedValue) {
			return false
		}
	}

	return true
}
