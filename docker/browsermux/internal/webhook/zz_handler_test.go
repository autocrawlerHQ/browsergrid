package webhook

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"
)

type mockManager struct {
	webhooks   map[string]*WebhookConfig
	executions map[string]*WebhookExecution
}

func newMockManager() *mockManager {
	return &mockManager{
		webhooks:   make(map[string]*WebhookConfig),
		executions: make(map[string]*WebhookExecution),
	}
}

func (m *mockManager) RegisterWebhook(config WebhookConfig) (string, error) {
	if config.ID == "" {
		config.ID = "mock-webhook-id"
	}
	m.webhooks[config.ID] = &config
	return config.ID, nil
}

func (m *mockManager) GetWebhook(id string) (*WebhookConfig, error) {
	webhook, ok := m.webhooks[id]
	if !ok {
		return nil, ErrWebhookNotFound
	}
	return webhook, nil
}

func (m *mockManager) UpdateWebhook(id string, updates map[string]interface{}) (*WebhookConfig, error) {
	webhook, ok := m.webhooks[id]
	if !ok {
		return nil, ErrWebhookNotFound
	}
	if name, ok := updates["name"].(string); ok {
		webhook.Name = name
	}
	return webhook, nil
}

func (m *mockManager) DeleteWebhook(id string) error {
	if _, ok := m.webhooks[id]; !ok {
		return ErrWebhookNotFound
	}
	delete(m.webhooks, id)
	return nil
}

func (m *mockManager) ListWebhooks() []*WebhookConfig {
	var result []*WebhookConfig
	for _, webhook := range m.webhooks {
		result = append(result, webhook)
	}
	return result
}

func (m *mockManager) TriggerWebhooks(method string, params map[string]interface{}, clientID string, timing WebhookTiming) []*WebhookExecution {
	return []*WebhookExecution{}
}

func (m *mockManager) GetExecution(id string) (*WebhookExecution, error) {
	execution, ok := m.executions[id]
	if !ok {
		return nil, ErrWebhookNotFound
	}
	return execution, nil
}

func (m *mockManager) ListExecutions(webhookID, clientID, status string, limit int) []*WebhookExecution {
	var result []*WebhookExecution
	for _, execution := range m.executions {
		result = append(result, execution)
	}
	return result
}

func (m *mockManager) TestWebhook(webhookID, clientID, method string, params map[string]interface{}) (*WebhookExecution, error) {
	webhook, err := m.GetWebhook(webhookID)
	if err != nil {
		return nil, err
	}

	execution := &WebhookExecution{
		ID:        "test-execution-id",
		WebhookID: webhook.ID,
		ClientID:  clientID,
		CDPEvent:  method,
		Status:    WebhookStatusSuccess,
	}

	return execution, nil
}

func TestNewHandler(t *testing.T) {
	manager := newMockManager()
	handler := NewHandler(manager)
	if handler == nil {
		t.Fatal("NewHandler() returned nil")
	}
}

func TestCreateWebhook(t *testing.T) {
	manager := newMockManager()
	handler := NewHandler(manager)

	config := WebhookConfig{
		Name:        "Test Webhook",
		URL:         "https://example.com/webhook",
		EventMethod: "Page.navigate",
		Timing:      WebhookTimingBeforeEvent,
		Enabled:     true,
	}

	jsonData, _ := json.Marshal(config)
	req := httptest.NewRequest("POST", "/webhooks", bytes.NewReader(jsonData))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.CreateWebhook(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("Expected status %d, got %d", http.StatusCreated, rr.Code)
	}

	var response WebhookConfig
	err := json.NewDecoder(rr.Body).Decode(&response)
	if err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response.Name != config.Name {
		t.Errorf("Expected name %s, got %s", config.Name, response.Name)
	}
}

func TestCreateWebhookInvalidJSON(t *testing.T) {
	manager := newMockManager()
	handler := NewHandler(manager)

	req := httptest.NewRequest("POST", "/webhooks", strings.NewReader("invalid json"))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.CreateWebhook(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestHandlerListWebhooks(t *testing.T) {
	manager := newMockManager()
	handler := NewHandler(manager)

	config := WebhookConfig{
		ID:          "test-webhook",
		Name:        "Test Webhook",
		URL:         "https://example.com/webhook",
		EventMethod: "Page.navigate",
		Timing:      WebhookTimingBeforeEvent,
		Enabled:     true,
	}
	manager.webhooks["test-webhook"] = &config

	req := httptest.NewRequest("GET", "/webhooks", nil)
	rr := httptest.NewRecorder()
	handler.ListWebhooks(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var response map[string]interface{}
	err := json.NewDecoder(rr.Body).Decode(&response)
	if err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	webhooks, ok := response["webhooks"].([]interface{})
	if !ok {
		t.Fatal("Expected webhooks field in response")
	}

	if len(webhooks) != 1 {
		t.Errorf("Expected 1 webhook, got %d", len(webhooks))
	}
}

func TestGetWebhook(t *testing.T) {
	manager := newMockManager()
	handler := NewHandler(manager)

	config := WebhookConfig{
		ID:          "test-webhook",
		Name:        "Test Webhook",
		URL:         "https://example.com/webhook",
		EventMethod: "Page.navigate",
		Timing:      WebhookTimingBeforeEvent,
		Enabled:     true,
	}
	manager.webhooks["test-webhook"] = &config

	req := httptest.NewRequest("GET", "/webhooks/test-webhook", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "test-webhook"})

	rr := httptest.NewRecorder()
	handler.GetWebhook(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var response WebhookConfig
	err := json.NewDecoder(rr.Body).Decode(&response)
	if err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response.ID != "test-webhook" {
		t.Errorf("Expected ID test-webhook, got %s", response.ID)
	}
}

func TestHandlerGetWebhookNotFound(t *testing.T) {
	manager := newMockManager()
	handler := NewHandler(manager)

	req := httptest.NewRequest("GET", "/webhooks/nonexistent", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "nonexistent"})

	rr := httptest.NewRecorder()
	handler.GetWebhook(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}

func TestHandlerUpdateWebhook(t *testing.T) {
	manager := newMockManager()
	handler := NewHandler(manager)

	config := WebhookConfig{
		ID:          "test-webhook",
		Name:        "Original Name",
		URL:         "https://example.com/webhook",
		EventMethod: "Page.navigate",
		Timing:      WebhookTimingBeforeEvent,
		Enabled:     true,
	}
	manager.webhooks["test-webhook"] = &config

	updates := map[string]interface{}{
		"name": "Updated Name",
	}

	jsonData, _ := json.Marshal(updates)
	req := httptest.NewRequest("PUT", "/webhooks/test-webhook", bytes.NewReader(jsonData))
	req.Header.Set("Content-Type", "application/json")
	req = mux.SetURLVars(req, map[string]string{"id": "test-webhook"})

	rr := httptest.NewRecorder()
	handler.UpdateWebhook(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var response WebhookConfig
	err := json.NewDecoder(rr.Body).Decode(&response)
	if err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response.Name != "Updated Name" {
		t.Errorf("Expected name 'Updated Name', got %s", response.Name)
	}
}

func TestUpdateWebhookInvalidJSON(t *testing.T) {
	manager := newMockManager()
	handler := NewHandler(manager)

	req := httptest.NewRequest("PUT", "/webhooks/test-webhook", strings.NewReader("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	req = mux.SetURLVars(req, map[string]string{"id": "test-webhook"})

	rr := httptest.NewRecorder()
	handler.UpdateWebhook(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestHandlerDeleteWebhook(t *testing.T) {
	manager := newMockManager()
	handler := NewHandler(manager)

	config := WebhookConfig{
		ID:          "test-webhook",
		Name:        "Test Webhook",
		URL:         "https://example.com/webhook",
		EventMethod: "Page.navigate",
		Timing:      WebhookTimingBeforeEvent,
		Enabled:     true,
	}
	manager.webhooks["test-webhook"] = &config

	req := httptest.NewRequest("DELETE", "/webhooks/test-webhook", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "test-webhook"})

	rr := httptest.NewRecorder()
	handler.DeleteWebhook(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Errorf("Expected status %d, got %d", http.StatusNoContent, rr.Code)
	}

	if _, exists := manager.webhooks["test-webhook"]; exists {
		t.Error("Webhook should have been deleted")
	}
}

func TestHandlerListExecutions(t *testing.T) {
	manager := newMockManager()
	handler := NewHandler(manager)

	execution := WebhookExecution{
		ID:        "test-execution",
		WebhookID: "test-webhook",
		ClientID:  "client-123",
		Status:    WebhookStatusSuccess,
	}
	manager.executions["test-execution"] = &execution

	req := httptest.NewRequest("GET", "/webhook-executions", nil)
	rr := httptest.NewRecorder()
	handler.ListExecutions(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var response map[string]interface{}
	err := json.NewDecoder(rr.Body).Decode(&response)
	if err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	executions, ok := response["executions"].([]interface{})
	if !ok {
		t.Fatal("Expected executions field in response")
	}

	if len(executions) != 1 {
		t.Errorf("Expected 1 execution, got %d", len(executions))
	}
}

func TestListExecutionsWithQuery(t *testing.T) {
	manager := newMockManager()
	handler := NewHandler(manager)

	req := httptest.NewRequest("GET", "/webhook-executions?webhook_id=test&client_id=client123&status=success&limit=10", nil)
	rr := httptest.NewRecorder()
	handler.ListExecutions(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
}

func TestGetExecution(t *testing.T) {
	manager := newMockManager()
	handler := NewHandler(manager)

	execution := WebhookExecution{
		ID:        "test-execution",
		WebhookID: "test-webhook",
		ClientID:  "client-123",
		Status:    WebhookStatusSuccess,
	}
	manager.executions["test-execution"] = &execution

	req := httptest.NewRequest("GET", "/webhook-executions/test-execution", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "test-execution"})

	rr := httptest.NewRecorder()
	handler.GetExecution(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var response WebhookExecution
	err := json.NewDecoder(rr.Body).Decode(&response)
	if err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response.ID != "test-execution" {
		t.Errorf("Expected ID test-execution, got %s", response.ID)
	}
}

func TestHandlerGetExecutionNotFound(t *testing.T) {
	manager := newMockManager()
	handler := NewHandler(manager)

	req := httptest.NewRequest("GET", "/webhook-executions/nonexistent", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "nonexistent"})

	rr := httptest.NewRecorder()
	handler.GetExecution(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}

func TestHandlerTestWebhook(t *testing.T) {
	manager := newMockManager()
	handler := NewHandler(manager)

	config := WebhookConfig{
		ID:          "test-webhook",
		Name:        "Test Webhook",
		URL:         "https://example.com/webhook",
		EventMethod: "Page.navigate",
		Timing:      WebhookTimingBeforeEvent,
		Enabled:     true,
	}
	manager.webhooks["test-webhook"] = &config

	testRequest := map[string]interface{}{
		"webhook_id": "test-webhook",
		"client_id":  "client-123",
		"cdp_method": "Page.navigate",
		"cdp_params": map[string]interface{}{
			"url": "https://example.com",
		},
	}

	jsonData, _ := json.Marshal(testRequest)
	req := httptest.NewRequest("POST", "/webhooks/test", bytes.NewReader(jsonData))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.TestWebhook(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var response WebhookExecution
	err := json.NewDecoder(rr.Body).Decode(&response)
	if err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response.WebhookID != "test-webhook" {
		t.Errorf("Expected WebhookID test-webhook, got %s", response.WebhookID)
	}
}

func TestTestWebhookInvalidJSON(t *testing.T) {
	manager := newMockManager()
	handler := NewHandler(manager)

	req := httptest.NewRequest("POST", "/webhooks/test", strings.NewReader("invalid json"))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.TestWebhook(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestRegisterRoutes(t *testing.T) {
	manager := newMockManager()
	handler := NewHandler(manager)
	router := mux.NewRouter()

	handler.RegisterRoutes(router)

	var routeCount int
	router.Walk(func(route *mux.Route, router *mux.Router, ancestors []*mux.Route) error {
		routeCount++
		return nil
	})

	if routeCount == 0 {
		t.Error("Expected routes to be registered")
	}
}
