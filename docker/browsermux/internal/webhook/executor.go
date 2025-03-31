package webhook

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"

	"browsermux/internal/events"
)

// Executor handles webhook execution
type Executor interface {
	// ExecuteWebhook executes a webhook
	ExecuteWebhook(webhook *WebhookConfig, clientID, cdpEvent string, eventData map[string]interface{}) (*WebhookExecution, error)

	// TestWebhook executes a webhook for testing
	TestWebhook(webhook *WebhookConfig, clientID, cdpEvent string, eventData map[string]interface{}) (*WebhookExecution, error)

	// GetExecution gets a webhook execution by ID
	GetExecution(id string) (*WebhookExecution, error)

	// ListExecutions lists webhook executions
	ListExecutions(webhookID, clientID, status string, limit int) []*WebhookExecution
}

// defaultExecutor is the default implementation of Executor
type defaultExecutor struct {
	executions      map[string]*WebhookExecution
	client          *http.Client
	eventDispatcher events.Dispatcher
	mu              sync.RWMutex
}

// NewExecutor creates a new webhook executor
func NewExecutor(dispatcher events.Dispatcher) Executor {
	return &defaultExecutor{
		executions:      make(map[string]*WebhookExecution),
		client:          &http.Client{},
		eventDispatcher: dispatcher,
	}
}

// ExecuteWebhook implements Executor.ExecuteWebhook
func (e *defaultExecutor) ExecuteWebhook(webhook *WebhookConfig, clientID, cdpEvent string, eventData map[string]interface{}) (*WebhookExecution, error) {
	// Create execution record
	execution := &WebhookExecution{
		ID:         uuid.New().String(),
		WebhookID:  webhook.ID,
		ClientID:   clientID,
		CDPEvent:   cdpEvent,
		EventData:  eventData,
		Timing:     webhook.Timing,
		Status:     WebhookStatusPending,
		StartedAt:  time.Now(),
		RetryCount: 0,
	}

	// Store execution
	e.mu.Lock()
	e.executions[execution.ID] = execution
	e.mu.Unlock()

	// Execute the webhook in a separate goroutine
	go e.executeWithRetries(webhook, execution)

	return execution, nil
}

// GetExecution implements Executor.GetExecution
func (e *defaultExecutor) GetExecution(id string) (*WebhookExecution, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	execution, ok := e.executions[id]
	if !ok {
		return nil, fmt.Errorf("execution not found: %s", id)
	}

	return execution, nil
}

// ListExecutions implements Executor.ListExecutions
func (e *defaultExecutor) ListExecutions(webhookID, clientID, status string, limit int) []*WebhookExecution {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var results []*WebhookExecution
	for _, execution := range e.executions {
		// Apply filters
		if webhookID != "" && execution.WebhookID != webhookID {
			continue
		}
		if clientID != "" && execution.ClientID != clientID {
			continue
		}
		if status != "" && string(execution.Status) != status {
			continue
		}

		results = append(results, execution)
		if limit > 0 && len(results) >= limit {
			break
		}
	}

	return results
}

// TestWebhook implements Executor.TestWebhook
func (e *defaultExecutor) TestWebhook(webhook *WebhookConfig, clientID, cdpEvent string, eventData map[string]interface{}) (*WebhookExecution, error) {
	// Similar to ExecuteWebhook but marked as a test
	execution := &WebhookExecution{
		ID:         uuid.New().String(),
		WebhookID:  webhook.ID,
		ClientID:   clientID,
		CDPEvent:   cdpEvent,
		EventData:  eventData,
		Timing:     webhook.Timing,
		Status:     WebhookStatusPending,
		StartedAt:  time.Now(),
		RetryCount: 0,
	}

	// Execute synchronously for tests
	e.executeWebhook(webhook, execution)

	return execution, nil
}

// executeWithRetries executes a webhook with retries
func (e *defaultExecutor) executeWithRetries(webhook *WebhookConfig, execution *WebhookExecution) {
	// Implementation of retry logic
	var err error
	for i := 0; i <= webhook.MaxRetries; i++ {
		if i > 0 {
			execution.RetryCount = i
		}

		err = e.executeWebhook(webhook, execution)
		if err == nil {
			// Success, no need to retry
			break
		}

		// Sleep before retry (could implement exponential backoff)
		if i < webhook.MaxRetries {
			time.Sleep(time.Duration(1+i) * time.Second)
		}
	}
}

// executeWebhook executes a single webhook call
func (e *defaultExecutor) executeWebhook(webhook *WebhookConfig, execution *WebhookExecution) error {
	// Prepare webhook payload
	payload, err := json.Marshal(map[string]interface{}{
		"event":     execution.CDPEvent,
		"client_id": execution.ClientID,
		"data":      execution.EventData,
	})
	if err != nil {
		execution.Status = WebhookStatusFailed
		execution.Error = "Failed to marshal payload: " + err.Error()
		return err
	}

	// Create HTTP request
	req, err := http.NewRequest("POST", webhook.URL, bytes.NewBuffer(payload))
	if err != nil {
		execution.Status = WebhookStatusFailed
		execution.Error = "Failed to create request: " + err.Error()
		return err
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	for key, value := range webhook.Headers {
		req.Header.Set(key, value)
	}

	// Execute request with timeout
	client := &http.Client{
		Timeout: time.Duration(webhook.Timeout) * time.Second,
	}

	// Record result
	start := time.Now()
	resp, err := client.Do(req)
	duration := time.Since(start)

	execution.DurationMS = int(duration.Milliseconds())
	now := time.Now()
	execution.CompletedAt = &now

	if err != nil {
		execution.Status = WebhookStatusFailed
		execution.Error = "Request failed: " + err.Error()
		return err
	}
	defer resp.Body.Close()

	// Record response
	execution.ResponseStatus = resp.StatusCode

	// Simple response body capture (limited)
	var respBody bytes.Buffer
	respBody.ReadFrom(resp.Body)
	execution.ResponseBody = respBody.String()

	// Check for success status code
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		execution.Status = WebhookStatusSuccess
		return nil
	}

	// Consider non-2xx as failure
	execution.Status = WebhookStatusFailed
	execution.Error = fmt.Sprintf("HTTP error: %d", resp.StatusCode)
	return fmt.Errorf("HTTP error: %d", resp.StatusCode)
}
