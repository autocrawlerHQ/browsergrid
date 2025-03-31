package webhook_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/mux"

	"browsermux/internal/events"
	"browsermux/internal/webhook"
)

// webhookHandler is a test HTTP handler that records webhook calls
type webhookHandler struct {
	calls        []map[string]interface{}
	statusCode   int
	responseBody string
	delay        time.Duration
	Server       *httptest.Server
}

func newWebhookHandler() *webhookHandler {
	h := &webhookHandler{
		calls:        make([]map[string]interface{}, 0),
		statusCode:   200,
		responseBody: `{"status": "ok"}`,
	}
	h.Server = httptest.NewServer(h)
	return h
}

func (h *webhookHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Delay response if configured
	if h.delay > 0 {
		time.Sleep(h.delay)
	}

	// Record request body
	var body map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&body); err == nil {
		h.calls = append(h.calls, body)
	}

	// Return configured response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(h.statusCode)
	w.Write([]byte(h.responseBody))
}

// setupTestServer creates a test server with the webhook system configured
func setupTestServer(t *testing.T) (*httptest.Server, *webhook.Handler, *webhookHandler, events.Dispatcher) {
	// Create test webhook handler
	webhookReceiver := newWebhookHandler()

	// Create event dispatcher
	dispatcher := events.NewDispatcher()

	// Create webhook manager
	webhookManager := webhook.NewManager(dispatcher)

	// Set up API router just for the webhook endpoints
	router := mux.NewRouter()
	webhookHandler := webhook.NewHandler(webhookManager)
	webhookHandler.RegisterRoutes(router.PathPrefix("/api").Subrouter())

	// Create test server
	server := httptest.NewServer(router)

	return server, webhookHandler, webhookReceiver, dispatcher
}

// TestWebhookRegistrationAndExecution tests the full webhook lifecycle
func TestWebhookRegistrationAndExecution(t *testing.T) {
	// Set up test environment
	server, _, webhookReceiver, dispatcher := setupTestServer(t)
	defer server.Close()
	defer webhookReceiver.Server.Close()

	// Create a webhook via the API
	webhookURL := fmt.Sprintf("%s/api/webhooks", server.URL)
	webhookConfig := webhook.WebhookConfig{
		Name:        "Test Webhook",
		URL:         webhookReceiver.Server.URL,
		EventMethod: "Page.loadEventFired",
		Timing:      webhook.WebhookTimingAfterEvent,
		Headers:     map[string]string{"X-Test": "true"},
		Timeout:     5,
		MaxRetries:  1,
		Enabled:     true,
	}

	// Register the webhook
	reqBody, _ := json.Marshal(webhookConfig)
	resp, err := http.Post(webhookURL, "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		t.Fatalf("Failed to create webhook: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("Expected status code %d, got %d", http.StatusCreated, resp.StatusCode)
	}

	var createdWebhook webhook.WebhookConfig
	err = json.NewDecoder(resp.Body).Decode(&createdWebhook)
	if err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
	if createdWebhook.ID == "" {
		t.Fatal("Created webhook ID is empty")
	}

	// Emit an event to trigger the webhook
	eventParams := map[string]interface{}{
		"timestamp": time.Now().Unix(),
	}

	// Dispatch the event
	dispatcher.Dispatch(events.Event{
		Type:       events.EventCDPEvent,
		Method:     "Page.loadEventFired",
		Params:     eventParams,
		SourceID:   "test-client",
		SourceType: "client",
		Timestamp:  time.Now(),
	})

	// Wait for webhook execution
	time.Sleep(100 * time.Millisecond)

	// Verify webhook was called
	if len(webhookReceiver.calls) != 1 {
		t.Fatalf("Expected 1 webhook call, got %d", len(webhookReceiver.calls))
	}

	if len(webhookReceiver.calls) > 0 {
		call := webhookReceiver.calls[0]
		if call["event"] != "Page.loadEventFired" {
			t.Errorf("Expected event 'Page.loadEventFired', got '%v'", call["event"])
		}
		if call["client_id"] != "test-client" {
			t.Errorf("Expected client_id 'test-client', got '%v'", call["client_id"])
		}
		if call["data"] == nil {
			t.Error("Expected data to be non-nil")
		}
	}

	// Check webhook executions via API
	executionsURL := fmt.Sprintf("%s/api/webhook-executions?webhook_id=%s", server.URL, createdWebhook.ID)
	resp, err = http.Get(executionsURL)
	if err != nil {
		t.Fatalf("Failed to get executions: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status code %d, got %d", http.StatusOK, resp.StatusCode)
	}

	var executionsResp struct {
		Executions []*webhook.WebhookExecution `json:"executions"`
	}
	err = json.NewDecoder(resp.Body).Decode(&executionsResp)
	if err != nil {
		t.Fatalf("Failed to decode executions: %v", err)
	}

	if len(executionsResp.Executions) != 1 {
		t.Fatalf("Expected 1 execution, got %d", len(executionsResp.Executions))
	}

	if len(executionsResp.Executions) > 0 {
		exec := executionsResp.Executions[0]
		if exec.Status != webhook.WebhookStatusSuccess {
			t.Errorf("Expected status %s, got %s", webhook.WebhookStatusSuccess, exec.Status)
		}
		if exec.WebhookID != createdWebhook.ID {
			t.Errorf("Expected webhook ID %s, got %s", createdWebhook.ID, exec.WebhookID)
		}
		if exec.ClientID != "test-client" {
			t.Errorf("Expected client ID 'test-client', got '%s'", exec.ClientID)
		}
	}
}

// TestWebhookRetries tests the webhook retry mechanism
func TestWebhookRetries(t *testing.T) {
	// Set up test environment
	server, _, webhookReceiver, dispatcher := setupTestServer(t)
	defer server.Close()
	defer webhookReceiver.Server.Close()

	// Configure webhook handler to fail initially
	webhookReceiver.statusCode = 500
	webhookReceiver.responseBody = `{"error": "Server error"}`

	// Create a webhook via the API
	webhookURL := fmt.Sprintf("%s/api/webhooks", server.URL)
	webhookConfig := webhook.WebhookConfig{
		Name:        "Retry Test Webhook",
		URL:         webhookReceiver.Server.URL,
		EventMethod: "Network.requestWillBeSent",
		Timing:      webhook.WebhookTimingBeforeEvent,
		Timeout:     1,
		MaxRetries:  2, // Should retry twice
		Enabled:     true,
	}

	// Register the webhook
	reqBody, _ := json.Marshal(webhookConfig)
	resp, err := http.Post(webhookURL, "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		t.Fatalf("Failed to create webhook: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("Expected status code %d, got %d", http.StatusCreated, resp.StatusCode)
	}

	var createdWebhook webhook.WebhookConfig
	err = json.NewDecoder(resp.Body).Decode(&createdWebhook)
	if err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Emit an event to trigger the webhook
	eventParams := map[string]interface{}{
		"requestId": "123",
		"request":   map[string]interface{}{"url": "https://example.com"},
	}

	// Dispatch the event
	dispatcher.Dispatch(events.Event{
		Type:       events.EventCDPCommand, // Before event
		Method:     "Network.requestWillBeSent",
		Params:     eventParams,
		SourceID:   "test-client",
		SourceType: "client",
		Timestamp:  time.Now(),
	})

	// Let first attempt and first retry fail
	time.Sleep(50 * time.Millisecond)
	if len(webhookReceiver.calls) < 1 {
		t.Fatalf("Expected at least 1 call, got %d", len(webhookReceiver.calls))
	}

	// Make next retry succeed
	webhookReceiver.statusCode = 200
	webhookReceiver.responseBody = `{"status": "ok"}`

	// Wait for retries to complete
	time.Sleep(3 * time.Second)

	// Should have been called 2-3 times (initial + retries)
	if len(webhookReceiver.calls) < 2 {
		t.Fatalf("Expected at least 2 calls, got %d", len(webhookReceiver.calls))
	}

	// Check final execution status
	executionsURL := fmt.Sprintf("%s/api/webhook-executions?webhook_id=%s", server.URL, createdWebhook.ID)
	resp, err = http.Get(executionsURL)
	if err != nil {
		t.Fatalf("Failed to get executions: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status code %d, got %d", http.StatusOK, resp.StatusCode)
	}

	var executionsResp struct {
		Executions []*webhook.WebhookExecution `json:"executions"`
	}
	err = json.NewDecoder(resp.Body).Decode(&executionsResp)
	if err != nil {
		t.Fatalf("Failed to decode executions: %v", err)
	}

	if len(executionsResp.Executions) < 1 {
		t.Fatal("Expected at least one execution")
	}
}

// TestWebhookTimeout tests that webhooks timeout properly
func TestWebhookTimeout(t *testing.T) {
	// Set up test environment
	server, _, webhookReceiver, dispatcher := setupTestServer(t)
	defer server.Close()
	defer webhookReceiver.Server.Close()

	// Configure webhook handler to delay response
	webhookReceiver.delay = 2 * time.Second

	// Create a webhook with a short timeout
	webhookURL := fmt.Sprintf("%s/api/webhooks", server.URL)
	webhookConfig := webhook.WebhookConfig{
		Name:        "Timeout Test Webhook",
		URL:         webhookReceiver.Server.URL,
		EventMethod: "Runtime.consoleAPICalled",
		Timing:      webhook.WebhookTimingAfterEvent,
		Timeout:     1, // 1 second timeout
		MaxRetries:  0,
		Enabled:     true,
	}

	// Register the webhook
	reqBody, _ := json.Marshal(webhookConfig)
	resp, err := http.Post(webhookURL, "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		t.Fatalf("Failed to create webhook: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("Expected status code %d, got %d", http.StatusCreated, resp.StatusCode)
	}

	var createdWebhook webhook.WebhookConfig
	err = json.NewDecoder(resp.Body).Decode(&createdWebhook)
	if err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Emit an event to trigger the webhook
	eventParams := map[string]interface{}{
		"type":   "log",
		"args":   []interface{}{"Test log message"},
		"levels": []string{"log"},
	}

	// Dispatch the event
	dispatcher.Dispatch(events.Event{
		Type:       events.EventCDPEvent, // After event
		Method:     "Runtime.consoleAPICalled",
		Params:     eventParams,
		SourceID:   "test-client",
		SourceType: "client",
		Timestamp:  time.Now(),
	})

	// Wait for execution and timeout
	time.Sleep(2 * time.Second)

	// Check execution status
	executionsURL := fmt.Sprintf("%s/api/webhook-executions?webhook_id=%s", server.URL, createdWebhook.ID)
	resp, err = http.Get(executionsURL)
	if err != nil {
		t.Fatalf("Failed to get executions: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status code %d, got %d", http.StatusOK, resp.StatusCode)
	}

	var executionsResp struct {
		Executions []*webhook.WebhookExecution `json:"executions"`
	}
	err = json.NewDecoder(resp.Body).Decode(&executionsResp)
	if err != nil {
		t.Fatalf("Failed to decode executions: %v", err)
	}

	if len(executionsResp.Executions) < 1 {
		t.Fatal("Expected at least one execution")
	}

	// The execution should have failed with a timeout
	if len(executionsResp.Executions) > 0 {
		exec := executionsResp.Executions[0]
		if exec.Status != webhook.WebhookStatusFailed {
			t.Errorf("Expected status %s, got %s", webhook.WebhookStatusFailed, exec.Status)
		}
		if exec.Error == "" || !contains(exec.Error, "timeout") {
			t.Errorf("Expected timeout error, got: %s", exec.Error)
		}
	}
}

// TestWebhookFilteringByEvent tests that webhooks are only triggered for matching events
func TestWebhookFilteringByEvent(t *testing.T) {
	// Set up test environment
	server, _, webhookReceiver, dispatcher := setupTestServer(t)
	defer server.Close()
	defer webhookReceiver.Server.Close()

	// Create a webhook for a specific event
	webhookURL := fmt.Sprintf("%s/api/webhooks", server.URL)
	webhookConfig := webhook.WebhookConfig{
		Name:        "Specific Event Webhook",
		URL:         webhookReceiver.Server.URL,
		EventMethod: "Page.frameStartedLoading", // Only for this event
		Timing:      webhook.WebhookTimingAfterEvent,
		Enabled:     true,
	}

	// Register the webhook
	reqBody, _ := json.Marshal(webhookConfig)
	resp, err := http.Post(webhookURL, "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		t.Fatalf("Failed to create webhook: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("Expected status code %d, got %d", http.StatusCreated, resp.StatusCode)
	}

	// Emit a non-matching event
	dispatcher.Dispatch(events.Event{
		Type:       events.EventCDPEvent,
		Method:     "Page.frameNavigated",
		Params:     map[string]interface{}{"frame": map[string]interface{}{"id": "123"}},
		SourceID:   "test-client",
		SourceType: "client",
		Timestamp:  time.Now(),
	})

	// Wait briefly
	time.Sleep(50 * time.Millisecond)

	// Webhook should not have been called
	if len(webhookReceiver.calls) != 0 {
		t.Fatalf("Expected 0 calls, got %d", len(webhookReceiver.calls))
	}

	// Now emit a matching event
	dispatcher.Dispatch(events.Event{
		Type:       events.EventCDPEvent,
		Method:     "Page.frameStartedLoading",
		Params:     map[string]interface{}{"frameId": "123"},
		SourceID:   "test-client",
		SourceType: "client",
		Timestamp:  time.Now(),
	})

	// Wait for execution
	time.Sleep(50 * time.Millisecond)

	// Webhook should have been called once
	if len(webhookReceiver.calls) != 1 {
		t.Fatalf("Expected 1 call, got %d", len(webhookReceiver.calls))
	}
}

// TestWebhookDisabling tests that disabled webhooks are not triggered
func TestWebhookDisabling(t *testing.T) {
	// Set up test environment
	server, _, webhookReceiver, dispatcher := setupTestServer(t)
	defer server.Close()
	defer webhookReceiver.Server.Close()

	// Create a webhook that's initially enabled
	webhookURL := fmt.Sprintf("%s/api/webhooks", server.URL)
	webhookConfig := webhook.WebhookConfig{
		Name:        "Disabling Test Webhook",
		URL:         webhookReceiver.Server.URL,
		EventMethod: "*", // Match any event
		Timing:      webhook.WebhookTimingAfterEvent,
		Enabled:     true,
	}

	// Register the webhook
	reqBody, _ := json.Marshal(webhookConfig)
	resp, err := http.Post(webhookURL, "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		t.Fatalf("Failed to create webhook: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("Expected status code %d, got %d", http.StatusCreated, resp.StatusCode)
	}

	var createdWebhook webhook.WebhookConfig
	err = json.NewDecoder(resp.Body).Decode(&createdWebhook)
	if err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Emit an event
	dispatcher.Dispatch(events.Event{
		Type:       events.EventCDPEvent,
		Method:     "Page.loadEventFired",
		Params:     map[string]interface{}{},
		SourceID:   "test-client",
		SourceType: "client",
		Timestamp:  time.Now(),
	})

	// Wait for execution
	time.Sleep(50 * time.Millisecond)

	// Webhook should have been called
	if len(webhookReceiver.calls) != 1 {
		t.Fatalf("Expected 1 call, got %d", len(webhookReceiver.calls))
	}

	// Now disable the webhook
	updateURL := fmt.Sprintf("%s/api/webhooks/%s", server.URL, createdWebhook.ID)
	updateBody, _ := json.Marshal(map[string]interface{}{
		"enabled": false,
	})
	req, _ := http.NewRequest("PUT", updateURL, bytes.NewBuffer(updateBody))
	req.Header.Set("Content-Type", "application/json")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to update webhook: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status code %d, got %d", http.StatusOK, resp.StatusCode)
	}

	// Clear previous calls
	webhookReceiver.calls = make([]map[string]interface{}, 0)

	// Emit another event
	dispatcher.Dispatch(events.Event{
		Type:       events.EventCDPEvent,
		Method:     "Page.loadEventFired",
		Params:     map[string]interface{}{},
		SourceID:   "test-client",
		SourceType: "client",
		Timestamp:  time.Now(),
	})

	// Wait for potential execution
	time.Sleep(50 * time.Millisecond)

	// Webhook should NOT have been called again
	if len(webhookReceiver.calls) != 0 {
		t.Fatalf("Expected 0 calls, got %d", len(webhookReceiver.calls))
	}
}

// TestCDPEventIntegration tests that CDP events trigger webhooks properly
func TestCDPEventIntegration(t *testing.T) {
	// Set up test environment
	server, _, webhookReceiver, dispatcher := setupTestServer(t)
	defer server.Close()
	defer webhookReceiver.Server.Close()

	// Create two webhooks with different timings
	webhookURL := fmt.Sprintf("%s/api/webhooks", server.URL)

	// Before event webhook
	beforeWebhookConfig := webhook.WebhookConfig{
		Name:        "Before CDP Event Webhook",
		URL:         webhookReceiver.Server.URL + "/before",
		EventMethod: "Network.requestWillBeSent",
		Timing:      webhook.WebhookTimingBeforeEvent,
		Headers:     map[string]string{"X-Webhook-Type": "before"},
		Enabled:     true,
	}

	// After event webhook
	afterWebhookConfig := webhook.WebhookConfig{
		Name:        "After CDP Event Webhook",
		URL:         webhookReceiver.Server.URL + "/after",
		EventMethod: "Network.responseReceived",
		Timing:      webhook.WebhookTimingAfterEvent,
		Headers:     map[string]string{"X-Webhook-Type": "after"},
		Enabled:     true,
	}

	// Register the webhooks
	reqBody, _ := json.Marshal(beforeWebhookConfig)
	resp, err := http.Post(webhookURL, "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		t.Fatalf("Failed to create before webhook: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("Expected status code %d, got %d", http.StatusCreated, resp.StatusCode)
	}

	reqBody, _ = json.Marshal(afterWebhookConfig)
	resp, err = http.Post(webhookURL, "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		t.Fatalf("Failed to create after webhook: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("Expected status code %d, got %d", http.StatusCreated, resp.StatusCode)
	}

	// Simulate CDP command (should trigger before event webhook)
	beforeEventParams := map[string]interface{}{
		"requestId": "req123",
		"request": map[string]interface{}{
			"url":    "https://example.com",
			"method": "GET",
		},
	}

	dispatcher.Dispatch(events.Event{
		Type:       events.EventCDPCommand,
		Method:     "Network.requestWillBeSent",
		Params:     beforeEventParams,
		SourceID:   "client1",
		SourceType: "client",
		Timestamp:  time.Now(),
	})

	// Simulate CDP event (should trigger after event webhook)
	afterEventParams := map[string]interface{}{
		"requestId": "req123",
		"response": map[string]interface{}{
			"status":     200,
			"statusText": "OK",
		},
	}

	dispatcher.Dispatch(events.Event{
		Type:       events.EventCDPEvent,
		Method:     "Network.responseReceived",
		Params:     afterEventParams,
		SourceID:   "client1",
		SourceType: "client",
		Timestamp:  time.Now(),
	})

	// Wait for executions
	time.Sleep(100 * time.Millisecond)

	// Check that both webhooks were called
	if len(webhookReceiver.calls) != 2 {
		t.Fatalf("Expected 2 webhook calls, got %d", len(webhookReceiver.calls))
	}

	// Verify call contents
	var beforeCall, afterCall map[string]interface{}
	for _, call := range webhookReceiver.calls {
		if event, ok := call["event"].(string); ok {
			if event == "Network.requestWillBeSent" {
				beforeCall = call
			} else if event == "Network.responseReceived" {
				afterCall = call
			}
		}
	}

	if beforeCall == nil {
		t.Fatal("Before event webhook was not called")
	}
	if afterCall == nil {
		t.Fatal("After event webhook was not called")
	}

	// Verify before call data
	beforeData, ok := beforeCall["data"].(map[string]interface{})
	if !ok {
		t.Fatal("Before webhook data is not a map")
	}
	requestId, ok := beforeData["requestId"]
	if !ok || requestId != "req123" {
		t.Errorf("Expected requestId 'req123', got %v", requestId)
	}

	// Verify after call data
	afterData, ok := afterCall["data"].(map[string]interface{})
	if !ok {
		t.Fatal("After webhook data is not a map")
	}
	response, ok := afterData["response"].(map[string]interface{})
	if !ok {
		t.Fatal("Response data is missing or not a map")
	}
	status, ok := response["status"]
	if !ok || status != float64(200) {
		t.Errorf("Expected status 200, got %v", status)
	}
}

// TestWebhookSerialization tests webhook serialization/deserialization
func TestWebhookSerialization(t *testing.T) {
	// Set up test environment
	server, _, webhookReceiver, _ := setupTestServer(t)
	defer server.Close()
	defer webhookReceiver.Server.Close()

	// Create a webhook with all fields populated
	webhookURL := fmt.Sprintf("%s/api/webhooks", server.URL)
	webhookConfig := webhook.WebhookConfig{
		Name:        "Serialization Test Webhook",
		URL:         webhookReceiver.Server.URL,
		EventMethod: "Page.frameNavigated",
		EventParams: map[string]interface{}{
			"url": "https://example.com*", // URL pattern
		},
		Timing:     webhook.WebhookTimingAfterEvent,
		Headers:    map[string]string{"X-Test": "true", "Authorization": "Bearer token123"},
		Timeout:    10,
		MaxRetries: 3,
		Enabled:    true,
	}

	// Register the webhook
	reqBody, _ := json.Marshal(webhookConfig)
	resp, err := http.Post(webhookURL, "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		t.Fatalf("Failed to create webhook: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("Expected status code %d, got %d", http.StatusCreated, resp.StatusCode)
	}

	var createdWebhook webhook.WebhookConfig
	err = json.NewDecoder(resp.Body).Decode(&createdWebhook)
	if err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
	if createdWebhook.ID == "" {
		t.Fatal("Created webhook ID is empty")
	}

	// Retrieve the webhook to check serialization
	getURL := fmt.Sprintf("%s/api/webhooks/%s", server.URL, createdWebhook.ID)
	resp, err = http.Get(getURL)
	if err != nil {
		t.Fatalf("Failed to get webhook: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status code %d, got %d", http.StatusOK, resp.StatusCode)
	}

	var retrievedWebhook webhook.WebhookConfig
	err = json.NewDecoder(resp.Body).Decode(&retrievedWebhook)
	if err != nil {
		t.Fatalf("Failed to decode webhook: %v", err)
	}

	// Verify all fields were preserved
	if retrievedWebhook.ID != createdWebhook.ID {
		t.Errorf("ID mismatch: expected %s, got %s", createdWebhook.ID, retrievedWebhook.ID)
	}
	if retrievedWebhook.Name != webhookConfig.Name {
		t.Errorf("Name mismatch: expected %s, got %s", webhookConfig.Name, retrievedWebhook.Name)
	}
	if retrievedWebhook.URL != webhookConfig.URL {
		t.Errorf("URL mismatch: expected %s, got %s", webhookConfig.URL, retrievedWebhook.URL)
	}
	if retrievedWebhook.EventMethod != webhookConfig.EventMethod {
		t.Errorf("EventMethod mismatch: expected %s, got %s", webhookConfig.EventMethod, retrievedWebhook.EventMethod)
	}
	if retrievedWebhook.Timing != webhookConfig.Timing {
		t.Errorf("Timing mismatch: expected %s, got %s", webhookConfig.Timing, retrievedWebhook.Timing)
	}
	if retrievedWebhook.Timeout != webhookConfig.Timeout {
		t.Errorf("Timeout mismatch: expected %d, got %d", webhookConfig.Timeout, retrievedWebhook.Timeout)
	}
	if retrievedWebhook.MaxRetries != webhookConfig.MaxRetries {
		t.Errorf("MaxRetries mismatch: expected %d, got %d", webhookConfig.MaxRetries, retrievedWebhook.MaxRetries)
	}
	if retrievedWebhook.Enabled != webhookConfig.Enabled {
		t.Errorf("Enabled mismatch: expected %t, got %t", webhookConfig.Enabled, retrievedWebhook.Enabled)
	}

	// Check headers (map comparison)
	if len(retrievedWebhook.Headers) != len(webhookConfig.Headers) {
		t.Errorf("Headers count mismatch: expected %d, got %d", len(webhookConfig.Headers), len(retrievedWebhook.Headers))
	}
	for k, v := range webhookConfig.Headers {
		if retrievedWebhook.Headers[k] != v {
			t.Errorf("Header mismatch for key %s: expected %s, got %s", k, v, retrievedWebhook.Headers[k])
		}
	}

	// Check event params (map comparison)
	if len(retrievedWebhook.EventParams) != len(webhookConfig.EventParams) {
		t.Errorf("EventParams count mismatch: expected %d, got %d", len(webhookConfig.EventParams), len(retrievedWebhook.EventParams))
	}
	for k, v := range webhookConfig.EventParams {
		if retrievedValue, ok := retrievedWebhook.EventParams[k]; !ok || retrievedValue != v {
			t.Errorf("EventParam mismatch for key %s: expected %v, got %v", k, v, retrievedValue)
		}
	}

	// Verify timestamps
	if retrievedWebhook.CreatedAt.IsZero() {
		t.Error("CreatedAt timestamp is zero")
	}
	if retrievedWebhook.UpdatedAt.IsZero() {
		t.Error("UpdatedAt timestamp is zero")
	}

	// Update the webhook
	updateURL := fmt.Sprintf("%s/api/webhooks/%s", server.URL, createdWebhook.ID)
	updateBody, _ := json.Marshal(map[string]interface{}{
		"name":    "Updated Webhook Name",
		"enabled": false,
		"timeout": 15,
		"headers": map[string]string{"X-Test": "updated", "New-Header": "value"},
	})
	req, _ := http.NewRequest("PUT", updateURL, bytes.NewBuffer(updateBody))
	req.Header.Set("Content-Type", "application/json")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to update webhook: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status code %d, got %d", http.StatusOK, resp.StatusCode)
	}

	var updatedWebhook webhook.WebhookConfig
	err = json.NewDecoder(resp.Body).Decode(&updatedWebhook)
	if err != nil {
		t.Fatalf("Failed to decode updated webhook: %v", err)
	}

	// Verify updates were applied
	if updatedWebhook.Name != "Updated Webhook Name" {
		t.Errorf("Name not updated: expected 'Updated Webhook Name', got %s", updatedWebhook.Name)
	}
	if updatedWebhook.Enabled != false {
		t.Error("Enabled not updated to false")
	}
	if updatedWebhook.Timeout != 15 {
		t.Errorf("Timeout not updated: expected 15, got %d", updatedWebhook.Timeout)
	}

	// Original fields should be preserved
	if updatedWebhook.URL != webhookConfig.URL {
		t.Errorf("URL should be unchanged, expected %s, got %s", webhookConfig.URL, updatedWebhook.URL)
	}
	if updatedWebhook.EventMethod != webhookConfig.EventMethod {
		t.Errorf("EventMethod should be unchanged, expected %s, got %s", webhookConfig.EventMethod, updatedWebhook.EventMethod)
	}
}

// TestEventParamFiltering tests that webhooks can filter by event parameters
func TestEventParamFiltering(t *testing.T) {
	// Set up test environment
	server, _, webhookReceiver, dispatcher := setupTestServer(t)
	defer server.Close()
	defer webhookReceiver.Server.Close()

	// Create a webhook that filters based on event parameters
	webhookURL := fmt.Sprintf("%s/api/webhooks", server.URL)
	webhookConfig := webhook.WebhookConfig{
		Name:        "Param Filtering Webhook",
		URL:         webhookReceiver.Server.URL,
		EventMethod: "Network.requestWillBeSent",
		EventParams: map[string]interface{}{
			"request.url": "*example.com*", // URL pattern matching
		},
		Timing:  webhook.WebhookTimingBeforeEvent,
		Enabled: true,
	}

	// Register the webhook
	reqBody, _ := json.Marshal(webhookConfig)
	resp, err := http.Post(webhookURL, "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		t.Fatalf("Failed to create webhook: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("Expected status code %d, got %d", http.StatusCreated, resp.StatusCode)
	}

	// Emit non-matching event (wrong URL)
	nonMatchingParams := map[string]interface{}{
		"requestId": "req1",
		"request": map[string]interface{}{
			"url":    "https://othersite.com/path",
			"method": "GET",
		},
	}

	dispatcher.Dispatch(events.Event{
		Type:       events.EventCDPCommand,
		Method:     "Network.requestWillBeSent",
		Params:     nonMatchingParams,
		SourceID:   "client1",
		SourceType: "client",
		Timestamp:  time.Now(),
	})

	// Wait for processing
	time.Sleep(50 * time.Millisecond)

	// Webhook should not have been called
	if len(webhookReceiver.calls) != 0 {
		t.Fatalf("Expected 0 calls, got %d", len(webhookReceiver.calls))
	}

	// Emit matching event (URL contains example.com)
	matchingParams := map[string]interface{}{
		"requestId": "req2",
		"request": map[string]interface{}{
			"url":    "https://api.example.com/resources",
			"method": "GET",
		},
	}

	dispatcher.Dispatch(events.Event{
		Type:       events.EventCDPCommand,
		Method:     "Network.requestWillBeSent",
		Params:     matchingParams,
		SourceID:   "client1",
		SourceType: "client",
		Timestamp:  time.Now(),
	})

	// Wait for processing
	time.Sleep(50 * time.Millisecond)

	// Webhook should have been called once
	if len(webhookReceiver.calls) != 1 {
		t.Fatalf("Expected 1 call, got %d", len(webhookReceiver.calls))
	}

	// Verify the matched parameters
	if len(webhookReceiver.calls) > 0 {
		call := webhookReceiver.calls[0]
		data, ok := call["data"].(map[string]interface{})
		if !ok {
			t.Fatal("Call data is not a map")
		}

		requestData, ok := data["request"].(map[string]interface{})
		if !ok {
			t.Fatal("Request data is not a map")
		}

		url, ok := requestData["url"].(string)
		if !ok || url != "https://api.example.com/resources" {
			t.Errorf("Expected URL 'https://api.example.com/resources', got %v", url)
		}
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return s != "" && substr != "" && len(s) >= len(substr) && s != substr && strings.Contains(s, substr)
}
