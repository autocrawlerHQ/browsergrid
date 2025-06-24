package webhook

import (
	"testing"
)

func TestNewManager(t *testing.T) {
	dispatcher := &mockEventDispatcher{}
	manager := NewManager(dispatcher)
	if manager == nil {
		t.Fatal("NewManager() returned nil")
	}
}

func TestManagerRegisterWebhook(t *testing.T) {
	dispatcher := &mockEventDispatcher{}
	manager := NewManager(dispatcher)

	config := WebhookConfig{
		Name:        "Test Webhook",
		URL:         "https://example.com/webhook",
		EventMethod: "Page.navigate",
		Timing:      WebhookTimingBeforeEvent,
		Enabled:     true,
	}

	id, err := manager.RegisterWebhook(config)
	if err != nil {
		t.Fatalf("RegisterWebhook() error = %v", err)
	}

	if id == "" {
		t.Fatal("RegisterWebhook() returned empty ID")
	}

	webhook, err := manager.GetWebhook(id)
	if err != nil {
		t.Fatalf("GetWebhook() error = %v", err)
	}

	if webhook.Name != config.Name {
		t.Errorf("Expected name %s, got %s", config.Name, webhook.Name)
	}
}

func TestManagerGetWebhook(t *testing.T) {
	dispatcher := &mockEventDispatcher{}
	manager := NewManager(dispatcher)

	config := WebhookConfig{
		Name:        "Test Webhook",
		URL:         "https://example.com/webhook",
		EventMethod: "Page.navigate",
		Timing:      WebhookTimingBeforeEvent,
		Enabled:     true,
	}

	id, err := manager.RegisterWebhook(config)
	if err != nil {
		t.Fatalf("RegisterWebhook() error = %v", err)
	}

	webhook, err := manager.GetWebhook(id)
	if err != nil {
		t.Fatalf("GetWebhook() error = %v", err)
	}

	if webhook.ID != id {
		t.Errorf("Expected ID %s, got %s", id, webhook.ID)
	}
}

func TestManagerGetWebhookNotFound(t *testing.T) {
	dispatcher := &mockEventDispatcher{}
	manager := NewManager(dispatcher)

	webhook, err := manager.GetWebhook("nonexistent")
	if err == nil {
		t.Fatal("Expected error for nonexistent webhook")
	}
	if webhook != nil {
		t.Fatal("Expected nil webhook for nonexistent ID")
	}
}

func TestManagerUpdateWebhook(t *testing.T) {
	dispatcher := &mockEventDispatcher{}
	manager := NewManager(dispatcher)

	config := WebhookConfig{
		Name:        "Original Name",
		URL:         "https://example.com/original",
		EventMethod: "Page.navigate",
		Timing:      WebhookTimingBeforeEvent,
		Enabled:     true,
	}

	id, err := manager.RegisterWebhook(config)
	if err != nil {
		t.Fatalf("RegisterWebhook() error = %v", err)
	}

	updates := map[string]interface{}{
		"name": "Updated Name",
		"url":  "https://example.com/updated",
	}

	updatedWebhook, err := manager.UpdateWebhook(id, updates)
	if err != nil {
		t.Fatalf("UpdateWebhook() error = %v", err)
	}

	if updatedWebhook.Name != "Updated Name" {
		t.Errorf("Expected name 'Updated Name', got %s", updatedWebhook.Name)
	}
	if updatedWebhook.URL != "https://example.com/updated" {
		t.Errorf("Expected URL 'https://example.com/updated', got %s", updatedWebhook.URL)
	}
}

func TestManagerDeleteWebhook(t *testing.T) {
	dispatcher := &mockEventDispatcher{}
	manager := NewManager(dispatcher)

	config := WebhookConfig{
		Name:        "Test Webhook",
		URL:         "https://example.com/webhook",
		EventMethod: "Page.navigate",
		Timing:      WebhookTimingBeforeEvent,
		Enabled:     true,
	}

	id, err := manager.RegisterWebhook(config)
	if err != nil {
		t.Fatalf("RegisterWebhook() error = %v", err)
	}

	err = manager.DeleteWebhook(id)
	if err != nil {
		t.Fatalf("DeleteWebhook() error = %v", err)
	}

	webhook, err := manager.GetWebhook(id)
	if err == nil {
		t.Fatal("Expected error after deletion")
	}
	if webhook != nil {
		t.Fatal("Expected nil webhook after deletion")
	}
}

func TestManagerListWebhooks(t *testing.T) {
	dispatcher := &mockEventDispatcher{}
	manager := NewManager(dispatcher)

	webhooks := manager.ListWebhooks()
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
		_, err := manager.RegisterWebhook(config)
		if err != nil {
			t.Fatalf("RegisterWebhook() error = %v", err)
		}
	}

	webhooks = manager.ListWebhooks()
	if len(webhooks) != 2 {
		t.Errorf("Expected 2 webhooks, got %d", len(webhooks))
	}
}

func TestManagerTriggerWebhooks(t *testing.T) {
	dispatcher := &mockEventDispatcher{}
	manager := NewManager(dispatcher)

	config := WebhookConfig{
		Name:        "Test Webhook",
		URL:         "https://httpbin.org/post",
		EventMethod: "Page.navigate",
		Timing:      WebhookTimingBeforeEvent,
		Timeout:     1,
		MaxRetries:  0,
		Enabled:     true,
	}

	id, err := manager.RegisterWebhook(config)
	if err != nil {
		t.Fatalf("RegisterWebhook() error = %v", err)
	}

	executions := manager.TriggerWebhooks("Page.navigate", map[string]interface{}{
		"url": "https://example.com",
	}, "client-123", WebhookTimingBeforeEvent)

	if len(executions) != 1 {
		t.Errorf("Expected 1 execution, got %d", len(executions))
	}

	execution := executions[0]
	if execution.WebhookID != id {
		t.Errorf("Expected WebhookID %s, got %s", id, execution.WebhookID)
	}
	if execution.ClientID != "client-123" {
		t.Errorf("Expected ClientID client-123, got %s", execution.ClientID)
	}
	if execution.CDPEvent != "Page.navigate" {
		t.Errorf("Expected CDPEvent Page.navigate, got %s", execution.CDPEvent)
	}
}

func TestManagerTriggerWebhooksNoMatches(t *testing.T) {
	dispatcher := &mockEventDispatcher{}
	manager := NewManager(dispatcher)

	config := WebhookConfig{
		Name:        "Test Webhook",
		URL:         "https://example.com/webhook",
		EventMethod: "Page.navigate",
		Timing:      WebhookTimingAfterEvent,
		Enabled:     true,
	}

	_, err := manager.RegisterWebhook(config)
	if err != nil {
		t.Fatalf("RegisterWebhook() error = %v", err)
	}

	executions := manager.TriggerWebhooks("Page.navigate", map[string]interface{}{
		"url": "https://example.com",
	}, "client-123", WebhookTimingBeforeEvent)

	if len(executions) != 0 {
		t.Errorf("Expected 0 executions, got %d", len(executions))
	}
}

func TestManagerTriggerWebhooksDisabled(t *testing.T) {
	dispatcher := &mockEventDispatcher{}
	manager := NewManager(dispatcher)

	config := WebhookConfig{
		Name:        "Disabled Webhook",
		URL:         "https://example.com/webhook",
		EventMethod: "Page.navigate",
		Timing:      WebhookTimingBeforeEvent,
		Enabled:     false,
	}

	_, err := manager.RegisterWebhook(config)
	if err != nil {
		t.Fatalf("RegisterWebhook() error = %v", err)
	}

	executions := manager.TriggerWebhooks("Page.navigate", map[string]interface{}{
		"url": "https://example.com",
	}, "client-123", WebhookTimingBeforeEvent)

	if len(executions) != 0 {
		t.Errorf("Expected 0 executions for disabled webhook, got %d", len(executions))
	}
}

func TestManagerGetExecution(t *testing.T) {
	dispatcher := &mockEventDispatcher{}
	manager := NewManager(dispatcher)

	execution, err := manager.GetExecution("nonexistent")
	if err == nil {
		t.Fatal("Expected error for nonexistent execution")
	}
	if execution != nil {
		t.Fatal("Expected nil execution for nonexistent ID")
	}
}

func TestManagerListExecutions(t *testing.T) {
	dispatcher := &mockEventDispatcher{}
	manager := NewManager(dispatcher)

	executions := manager.ListExecutions("", "", "", 0)
	if len(executions) != 0 {
		t.Errorf("Expected 0 executions, got %d", len(executions))
	}
}

func TestManagerTestWebhook(t *testing.T) {
	dispatcher := &mockEventDispatcher{}
	manager := NewManager(dispatcher)

	config := WebhookConfig{
		Name:        "Test Webhook",
		URL:         "https://httpbin.org/post",
		EventMethod: "Page.navigate",
		Timing:      WebhookTimingBeforeEvent,
		Timeout:     1,
		MaxRetries:  0,
		Enabled:     true,
	}

	id, err := manager.RegisterWebhook(config)
	if err != nil {
		t.Fatalf("RegisterWebhook() error = %v", err)
	}

	execution, err := manager.TestWebhook(id, "client-123", "Page.navigate", map[string]interface{}{
		"url": "https://example.com",
	})

	if err != nil {
		t.Fatalf("TestWebhook() error = %v", err)
	}

	if execution.WebhookID != id {
		t.Errorf("Expected WebhookID %s, got %s", id, execution.WebhookID)
	}
	if execution.ClientID != "client-123" {
		t.Errorf("Expected ClientID client-123, got %s", execution.ClientID)
	}
}

func TestManagerTestWebhookNotFound(t *testing.T) {
	dispatcher := &mockEventDispatcher{}
	manager := NewManager(dispatcher)

	execution, err := manager.TestWebhook("nonexistent", "client-123", "Page.navigate", map[string]interface{}{})
	if err == nil {
		t.Fatal("Expected error for nonexistent webhook")
	}
	if execution != nil {
		t.Fatal("Expected nil execution for nonexistent webhook")
	}
}
