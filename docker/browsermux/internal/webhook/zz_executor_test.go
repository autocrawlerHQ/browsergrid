package webhook

import (
	"browsermux/internal/events"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type mockEventDispatcher struct {
	events []events.Event
}

func (m *mockEventDispatcher) Register(eventType events.EventType, handler events.EventHandler) events.HandlerID {
	return events.HandlerID("mock-handler-id")
}

func (m *mockEventDispatcher) Unregister(eventType events.EventType, handlerID events.HandlerID) {
}

func (m *mockEventDispatcher) Dispatch(event events.Event) {
	m.events = append(m.events, event)
}

func TestNewExecutor(t *testing.T) {
	dispatcher := &mockEventDispatcher{}
	executor := NewExecutor(dispatcher)
	if executor == nil {
		t.Fatal("NewExecutor() returned nil")
	}
}

func TestExecuteWebhookSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Expected POST request, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Expected Content-Type application/json, got %s", r.Header.Get("Content-Type"))
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Success"))
	}))
	defer server.Close()

	dispatcher := &mockEventDispatcher{}
	executor := NewExecutor(dispatcher)

	webhook := &WebhookConfig{
		ID:         "test-webhook",
		URL:        server.URL,
		Timeout:    30,
		MaxRetries: 3,
		Headers:    map[string]string{"Authorization": "Bearer token"},
	}

	execution, err := executor.ExecuteWebhook(webhook, "client-123", "Page.navigate", map[string]interface{}{
		"url": "https://example.com",
	})

	if err != nil {
		t.Fatalf("ExecuteWebhook() error = %v", err)
	}

	if execution == nil {
		t.Fatal("ExecuteWebhook() returned nil execution")
	}

	if execution.WebhookID != webhook.ID {
		t.Errorf("Expected WebhookID %s, got %s", webhook.ID, execution.WebhookID)
	}

	if execution.ClientID != "client-123" {
		t.Errorf("Expected ClientID client-123, got %s", execution.ClientID)
	}

	if execution.CDPEvent != "Page.navigate" {
		t.Errorf("Expected CDPEvent Page.navigate, got %s", execution.CDPEvent)
	}

	time.Sleep(100 * time.Millisecond)

	finalExecution, err := executor.GetExecution(execution.ID)
	if err != nil {
		t.Fatalf("GetExecution() error = %v", err)
	}

	if finalExecution.Status != WebhookStatusSuccess {
		t.Errorf("Expected status success, got %s", finalExecution.Status)
	}
}

func TestExecuteWebhookWithRetries(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Internal Server Error"))
		} else {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("Success"))
		}
	}))
	defer server.Close()

	dispatcher := &mockEventDispatcher{}
	executor := NewExecutor(dispatcher)

	webhook := &WebhookConfig{
		ID:         "test-webhook",
		URL:        server.URL,
		Timeout:    30,
		MaxRetries: 2,
	}

	execution, err := executor.ExecuteWebhook(webhook, "client-123", "Page.navigate", map[string]interface{}{})
	if err != nil {
		t.Fatalf("ExecuteWebhook() error = %v", err)
	}

	time.Sleep(4 * time.Second)

	finalExecution, err := executor.GetExecution(execution.ID)
	if err != nil {
		t.Fatalf("GetExecution() error = %v", err)
	}

	if finalExecution.Status != WebhookStatusSuccess {
		t.Errorf("Expected status success after retries, got %s (error: %s)", finalExecution.Status, finalExecution.Error)
	}

	if finalExecution.RetryCount != 2 {
		t.Errorf("Expected retry count 2, got %d", finalExecution.RetryCount)
	}

	if callCount != 3 {
		t.Errorf("Expected 3 HTTP calls, got %d", callCount)
	}
}

func TestExecuteWebhookFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	}))
	defer server.Close()

	dispatcher := &mockEventDispatcher{}
	executor := NewExecutor(dispatcher)

	webhook := &WebhookConfig{
		ID:         "test-webhook",
		URL:        server.URL,
		Timeout:    30,
		MaxRetries: 1,
	}

	execution, err := executor.ExecuteWebhook(webhook, "client-123", "Page.navigate", map[string]interface{}{})
	if err != nil {
		t.Fatalf("ExecuteWebhook() error = %v", err)
	}

	time.Sleep(3 * time.Second)

	finalExecution, err := executor.GetExecution(execution.ID)
	if err != nil {
		t.Fatalf("GetExecution() error = %v", err)
	}

	if finalExecution.Status != WebhookStatusFailed {
		t.Errorf("Expected status failed, got %s", finalExecution.Status)
	}

	if finalExecution.RetryCount != 1 {
		t.Errorf("Expected retry count 1, got %d", finalExecution.RetryCount)
	}

	if !strings.Contains(finalExecution.Error, "HTTP error: 500") {
		t.Errorf("Expected error to contain 'HTTP error: 500', got %s", finalExecution.Error)
	}
}

func TestExecuteWebhookTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	dispatcher := &mockEventDispatcher{}
	executor := NewExecutor(dispatcher)

	webhook := &WebhookConfig{
		ID:         "test-webhook",
		URL:        server.URL,
		Timeout:    1,
		MaxRetries: 1,
	}

	execution, err := executor.ExecuteWebhook(webhook, "client-123", "Page.navigate", map[string]interface{}{})
	if err != nil {
		t.Fatalf("ExecuteWebhook() error = %v", err)
	}

	time.Sleep(3 * time.Second)

	finalExecution, err := executor.GetExecution(execution.ID)
	if err != nil {
		t.Fatalf("GetExecution() error = %v", err)
	}

	if finalExecution.Status != WebhookStatusFailed {
		t.Errorf("Expected status failed due to timeout, got %s", finalExecution.Status)
	}

	if !strings.Contains(finalExecution.Error, "Request failed") {
		t.Errorf("Expected error to contain 'Request failed', got %s", finalExecution.Error)
	}
}

func TestTestWebhook(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Test Success"))
	}))
	defer server.Close()

	dispatcher := &mockEventDispatcher{}
	executor := NewExecutor(dispatcher)

	webhook := &WebhookConfig{
		ID:         "test-webhook",
		URL:        server.URL,
		Timeout:    30,
		MaxRetries: 3,
	}

	execution, err := executor.TestWebhook(webhook, "client-123", "Page.navigate", map[string]interface{}{
		"url": "https://example.com",
	})

	if err != nil {
		t.Fatalf("TestWebhook() error = %v", err)
	}

	if execution.Status != WebhookStatusSuccess {
		t.Errorf("Expected status success, got %s", execution.Status)
	}

	if execution.ResponseBody != "Test Success" {
		t.Errorf("Expected response body 'Test Success', got %s", execution.ResponseBody)
	}

	if execution.RetryCount != 0 {
		t.Errorf("Expected retry count 0 for test webhook, got %d", execution.RetryCount)
	}
}

func TestGetExecutionNotFound(t *testing.T) {
	dispatcher := &mockEventDispatcher{}
	executor := NewExecutor(dispatcher)

	execution, err := executor.GetExecution("nonexistent")
	if err == nil {
		t.Fatal("Expected error for nonexistent execution")
	}
	if execution != nil {
		t.Fatal("Expected nil execution for nonexistent ID")
	}
	if !strings.Contains(err.Error(), "execution not found") {
		t.Errorf("Expected error to contain 'execution not found', got %s", err.Error())
	}
}

func TestListExecutions(t *testing.T) {
	dispatcher := &mockEventDispatcher{}
	executor := NewExecutor(dispatcher)

	executions := executor.ListExecutions("", "", "", 0)
	if len(executions) != 0 {
		t.Errorf("Expected 0 executions, got %d", len(executions))
	}

	testExecutor, ok := executor.(*defaultExecutor)
	if !ok {
		t.Fatal("Expected defaultExecutor type")
	}

	testExecutor.mu.Lock()
	testExecutor.executions["exec1"] = &WebhookExecution{
		ID:        "exec1",
		WebhookID: "webhook1",
		ClientID:  "client1",
		Status:    WebhookStatusSuccess,
	}
	testExecutor.executions["exec2"] = &WebhookExecution{
		ID:        "exec2",
		WebhookID: "webhook1",
		ClientID:  "client2",
		Status:    WebhookStatusFailed,
	}
	testExecutor.executions["exec3"] = &WebhookExecution{
		ID:        "exec3",
		WebhookID: "webhook2",
		ClientID:  "client1",
		Status:    WebhookStatusSuccess,
	}
	testExecutor.mu.Unlock()

	executions = executor.ListExecutions("webhook1", "", "", 0)
	if len(executions) != 2 {
		t.Errorf("Expected 2 executions for webhook1, got %d", len(executions))
	}

	executions = executor.ListExecutions("", "client1", "", 0)
	if len(executions) != 2 {
		t.Errorf("Expected 2 executions for client1, got %d", len(executions))
	}

	executions = executor.ListExecutions("", "", "success", 0)
	if len(executions) != 2 {
		t.Errorf("Expected 2 successful executions, got %d", len(executions))
	}

	executions = executor.ListExecutions("", "", "", 1)
	if len(executions) != 1 {
		t.Errorf("Expected 1 execution with limit, got %d", len(executions))
	}

	executions = executor.ListExecutions("webhook1", "client1", "", 0)
	if len(executions) != 1 {
		t.Errorf("Expected 1 execution with combined filters, got %d", len(executions))
	}
}

func TestExecuteWebhookCustomHeaders(t *testing.T) {
	var receivedHeaders http.Header
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders = r.Header
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Success"))
	}))
	defer server.Close()

	dispatcher := &mockEventDispatcher{}
	executor := NewExecutor(dispatcher)

	webhook := &WebhookConfig{
		ID:         "test-webhook",
		URL:        server.URL,
		Timeout:    30,
		MaxRetries: 0,
		Headers: map[string]string{
			"Authorization": "Bearer secret-token",
			"X-Custom":      "custom-value",
		},
	}

	execution, err := executor.TestWebhook(webhook, "client-123", "Page.navigate", map[string]interface{}{})
	if err != nil {
		t.Fatalf("TestWebhook() error = %v", err)
	}

	if execution.Status != WebhookStatusSuccess {
		t.Errorf("Expected status success, got %s", execution.Status)
	}

	if receivedHeaders.Get("Authorization") != "Bearer secret-token" {
		t.Errorf("Expected Authorization header, got %s", receivedHeaders.Get("Authorization"))
	}
	if receivedHeaders.Get("X-Custom") != "custom-value" {
		t.Errorf("Expected X-Custom header, got %s", receivedHeaders.Get("X-Custom"))
	}
	if receivedHeaders.Get("Content-Type") != "application/json" {
		t.Errorf("Expected Content-Type header, got %s", receivedHeaders.Get("Content-Type"))
	}
}

func TestExecuteWebhookPayloadStructure(t *testing.T) {
	var receivedPayload map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := json.NewDecoder(r.Body).Decode(&receivedPayload)
		if err != nil {
			t.Errorf("Failed to decode payload: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	dispatcher := &mockEventDispatcher{}
	executor := NewExecutor(dispatcher)

	webhook := &WebhookConfig{
		ID:      "test-webhook",
		URL:     server.URL,
		Timeout: 30,
	}

	eventData := map[string]interface{}{
		"url":    "https://example.com",
		"method": "GET",
	}

	execution, err := executor.TestWebhook(webhook, "client-123", "Page.navigate", eventData)
	if err != nil {
		t.Fatalf("TestWebhook() error = %v", err)
	}

	if execution.Status != WebhookStatusSuccess {
		t.Errorf("Expected status success, got %s", execution.Status)
	}

	if receivedPayload["event"] != "Page.navigate" {
		t.Errorf("Expected event 'Page.navigate', got %v", receivedPayload["event"])
	}
	if receivedPayload["client_id"] != "client-123" {
		t.Errorf("Expected client_id 'client-123', got %v", receivedPayload["client_id"])
	}
	if receivedPayload["data"] == nil {
		t.Error("Expected data field in payload")
	}
}
