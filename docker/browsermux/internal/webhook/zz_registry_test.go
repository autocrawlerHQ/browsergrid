package webhook

import (
	"fmt"
	"testing"
	"time"
)

func TestNewRegistry(t *testing.T) {
	registry := NewRegistry()
	if registry == nil {
		t.Fatal("NewRegistry() returned nil")
	}
}

func TestRegisterWebhook(t *testing.T) {
	registry := NewRegistry()

	config := WebhookConfig{
		Name:        "Test Webhook",
		URL:         "https://example.com/webhook",
		EventMethod: "Page.navigate",
		Timing:      WebhookTimingBeforeEvent,
		Headers:     map[string]string{"Authorization": "Bearer token"},
		Timeout:     30,
		MaxRetries:  3,
		Enabled:     true,
	}

	id, err := registry.RegisterWebhook(config)
	if err != nil {
		t.Fatalf("RegisterWebhook() error = %v", err)
	}

	if id == "" {
		t.Fatal("RegisterWebhook() returned empty ID")
	}

	webhook, err := registry.GetWebhook(id)
	if err != nil {
		t.Fatalf("GetWebhook() error = %v", err)
	}

	if webhook.Name != config.Name {
		t.Errorf("Expected name %s, got %s", config.Name, webhook.Name)
	}
	if webhook.URL != config.URL {
		t.Errorf("Expected URL %s, got %s", config.URL, webhook.URL)
	}
	if webhook.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set")
	}
	if webhook.UpdatedAt.IsZero() {
		t.Error("UpdatedAt should be set")
	}
}

func TestRegisterWebhookWithID(t *testing.T) {
	registry := NewRegistry()

	expectedID := "custom-webhook-id"
	config := WebhookConfig{
		ID:          expectedID,
		Name:        "Test Webhook",
		URL:         "https://example.com/webhook",
		EventMethod: "Page.navigate",
		Timing:      WebhookTimingBeforeEvent,
		Enabled:     true,
	}

	id, err := registry.RegisterWebhook(config)
	if err != nil {
		t.Fatalf("RegisterWebhook() error = %v", err)
	}

	if id != expectedID {
		t.Errorf("Expected ID %s, got %s", expectedID, id)
	}
}

func TestGetWebhookNotFound(t *testing.T) {
	registry := NewRegistry()

	webhook, err := registry.GetWebhook("nonexistent")
	if err == nil {
		t.Fatal("Expected error for nonexistent webhook")
	}
	if webhook != nil {
		t.Fatal("Expected nil webhook for nonexistent ID")
	}
	if err != ErrWebhookNotFound {
		t.Errorf("Expected ErrWebhookNotFound, got %v", err)
	}
}

func TestUpdateWebhook(t *testing.T) {
	registry := NewRegistry()

	config := WebhookConfig{
		Name:        "Original Name",
		URL:         "https://example.com/original",
		EventMethod: "Page.navigate",
		Timing:      WebhookTimingBeforeEvent,
		Enabled:     true,
	}

	id, err := registry.RegisterWebhook(config)
	if err != nil {
		t.Fatalf("RegisterWebhook() error = %v", err)
	}

	updates := map[string]interface{}{
		"name": "Updated Name",
		"url":  "https://example.com/updated",
	}

	originalWebhook, _ := registry.GetWebhook(id)
	originalUpdateTime := originalWebhook.UpdatedAt

	time.Sleep(time.Millisecond)

	updatedWebhook, err := registry.UpdateWebhook(id, updates)
	if err != nil {
		t.Fatalf("UpdateWebhook() error = %v", err)
	}

	if updatedWebhook.Name != "Updated Name" {
		t.Errorf("Expected name 'Updated Name', got %s", updatedWebhook.Name)
	}
	if updatedWebhook.URL != "https://example.com/updated" {
		t.Errorf("Expected URL 'https://example.com/updated', got %s", updatedWebhook.URL)
	}
	if !updatedWebhook.UpdatedAt.After(originalUpdateTime) {
		t.Error("UpdatedAt should be updated")
	}
}

func TestUpdateWebhookNotFound(t *testing.T) {
	registry := NewRegistry()

	updates := map[string]interface{}{
		"name": "Updated Name",
	}

	webhook, err := registry.UpdateWebhook("nonexistent", updates)
	if err == nil {
		t.Fatal("Expected error for nonexistent webhook")
	}
	if webhook != nil {
		t.Fatal("Expected nil webhook for nonexistent ID")
	}
	if err != ErrWebhookNotFound {
		t.Errorf("Expected ErrWebhookNotFound, got %v", err)
	}
}

func TestDeleteWebhook(t *testing.T) {
	registry := NewRegistry()

	config := WebhookConfig{
		Name:        "Test Webhook",
		URL:         "https://example.com/webhook",
		EventMethod: "Page.navigate",
		Timing:      WebhookTimingBeforeEvent,
		Enabled:     true,
	}

	id, err := registry.RegisterWebhook(config)
	if err != nil {
		t.Fatalf("RegisterWebhook() error = %v", err)
	}

	err = registry.DeleteWebhook(id)
	if err != nil {
		t.Fatalf("DeleteWebhook() error = %v", err)
	}

	webhook, err := registry.GetWebhook(id)
	if err == nil {
		t.Fatal("Expected error after deletion")
	}
	if webhook != nil {
		t.Fatal("Expected nil webhook after deletion")
	}
}

func TestDeleteWebhookNotFound(t *testing.T) {
	registry := NewRegistry()

	err := registry.DeleteWebhook("nonexistent")
	if err == nil {
		t.Fatal("Expected error for nonexistent webhook")
	}
	if err != ErrWebhookNotFound {
		t.Errorf("Expected ErrWebhookNotFound, got %v", err)
	}
}

func TestListWebhooks(t *testing.T) {
	registry := NewRegistry()

	webhooks := registry.ListWebhooks()
	if len(webhooks) != 0 {
		t.Errorf("Expected 0 webhooks, got %d", len(webhooks))
	}

	configs := []WebhookConfig{
		{
			Name:        "Webhook 1",
			URL:         "https://example.com/webhook1",
			EventMethod: "Page.navigate",
			Timing:      WebhookTimingBeforeEvent,
			Enabled:     true,
		},
		{
			Name:        "Webhook 2",
			URL:         "https://example.com/webhook2",
			EventMethod: "Runtime.consoleAPICalled",
			Timing:      WebhookTimingAfterEvent,
			Enabled:     true,
		},
	}

	for _, config := range configs {
		_, err := registry.RegisterWebhook(config)
		if err != nil {
			t.Fatalf("RegisterWebhook() error = %v", err)
		}
	}

	webhooks = registry.ListWebhooks()
	if len(webhooks) != 2 {
		t.Errorf("Expected 2 webhooks, got %d", len(webhooks))
	}
}

func TestFindMatchingWebhooks(t *testing.T) {
	registry := NewRegistry()

	configs := []WebhookConfig{
		{
			ID:          "before-navigate",
			Name:        "Before Navigate",
			URL:         "https://example.com/before-navigate",
			EventMethod: "Page.navigate",
			Timing:      WebhookTimingBeforeEvent,
			Enabled:     true,
		},
		{
			ID:          "after-navigate",
			Name:        "After Navigate",
			URL:         "https://example.com/after-navigate",
			EventMethod: "Page.navigate",
			Timing:      WebhookTimingAfterEvent,
			Enabled:     true,
		},
		{
			ID:          "console-log",
			Name:        "Console Log",
			URL:         "https://example.com/console",
			EventMethod: "Runtime.consoleAPICalled",
			Timing:      WebhookTimingAfterEvent,
			Enabled:     true,
		},
		{
			ID:          "disabled-webhook",
			Name:        "Disabled",
			URL:         "https://example.com/disabled",
			EventMethod: "Page.navigate",
			Timing:      WebhookTimingBeforeEvent,
			Enabled:     false,
		},
		{
			ID:          "wildcard-webhook",
			Name:        "Wildcard",
			URL:         "https://example.com/wildcard",
			EventMethod: "*",
			Timing:      WebhookTimingAfterEvent,
			Enabled:     true,
		},
	}

	for _, config := range configs {
		_, err := registry.RegisterWebhook(config)
		if err != nil {
			t.Fatalf("RegisterWebhook() error = %v", err)
		}
	}

	tests := []struct {
		method   string
		params   map[string]interface{}
		timing   WebhookTiming
		expected []string
	}{
		{
			method:   "Page.navigate",
			params:   map[string]interface{}{},
			timing:   WebhookTimingBeforeEvent,
			expected: []string{"before-navigate"},
		},
		{
			method:   "Page.navigate",
			params:   map[string]interface{}{},
			timing:   WebhookTimingAfterEvent,
			expected: []string{"after-navigate", "wildcard-webhook"},
		},
		{
			method:   "Runtime.consoleAPICalled",
			params:   map[string]interface{}{},
			timing:   WebhookTimingAfterEvent,
			expected: []string{"console-log", "wildcard-webhook"},
		},
		{
			method:   "NonExistent.method",
			params:   map[string]interface{}{},
			timing:   WebhookTimingAfterEvent,
			expected: []string{"wildcard-webhook"},
		},
		{
			method:   "Page.navigate",
			params:   map[string]interface{}{},
			timing:   WebhookTiming("invalid-timing"),
			expected: []string{},
		},
	}

	for _, test := range tests {
		matches := registry.FindMatchingWebhooks(test.method, test.params, test.timing)

		if len(matches) != len(test.expected) {
			t.Errorf("For method %s, timing %s: expected %d matches, got %d",
				test.method, test.timing, len(test.expected), len(matches))
			continue
		}

		matchIDs := make([]string, len(matches))
		for i, match := range matches {
			matchIDs[i] = match.ID
		}

		for _, expectedID := range test.expected {
			found := false
			for _, matchID := range matchIDs {
				if matchID == expectedID {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Expected to find webhook %s in matches for method %s, timing %s",
					expectedID, test.method, test.timing)
			}
		}
	}
}

func TestWebhookError(t *testing.T) {
	err := NewError("test error message")
	if err.Error() != "test error message" {
		t.Errorf("Expected 'test error message', got %s", err.Error())
	}
}

func TestRegistryConcurrency(t *testing.T) {
	registry := NewRegistry()

	done := make(chan bool, 10)

	for i := 0; i < 5; i++ {
		go func(idx int) {
			config := WebhookConfig{
				Name:        fmt.Sprintf("Webhook %d", idx),
				URL:         fmt.Sprintf("https://example.com/webhook%d", idx),
				EventMethod: "Page.navigate",
				Timing:      WebhookTimingBeforeEvent,
				Enabled:     true,
			}
			_, err := registry.RegisterWebhook(config)
			if err != nil {
				t.Errorf("Concurrent RegisterWebhook() error = %v", err)
			}
			done <- true
		}(i)
	}

	for i := 0; i < 5; i++ {
		go func() {
			registry.ListWebhooks()
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}
