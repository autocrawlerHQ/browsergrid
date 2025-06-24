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

type Executor interface {
	ExecuteWebhook(webhook *WebhookConfig, clientID, cdpEvent string, eventData map[string]interface{}) (*WebhookExecution, error)

	TestWebhook(webhook *WebhookConfig, clientID, cdpEvent string, eventData map[string]interface{}) (*WebhookExecution, error)

	GetExecution(id string) (*WebhookExecution, error)

	ListExecutions(webhookID, clientID, status string, limit int) []*WebhookExecution
}

type defaultExecutor struct {
	executions      map[string]*WebhookExecution
	client          *http.Client
	eventDispatcher events.Dispatcher
	mu              sync.RWMutex
}

func NewExecutor(dispatcher events.Dispatcher) Executor {
	return &defaultExecutor{
		executions:      make(map[string]*WebhookExecution),
		client:          &http.Client{},
		eventDispatcher: dispatcher,
	}
}

func (e *defaultExecutor) ExecuteWebhook(webhook *WebhookConfig, clientID, cdpEvent string, eventData map[string]interface{}) (*WebhookExecution, error) {
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

	e.mu.Lock()
	e.executions[execution.ID] = execution
	e.mu.Unlock()

	go e.executeWithRetries(webhook, execution)

	return execution, nil
}

func (e *defaultExecutor) GetExecution(id string) (*WebhookExecution, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	execution, ok := e.executions[id]
	if !ok {
		return nil, fmt.Errorf("execution not found: %s", id)
	}

	return execution.GetCopy(), nil
}

func (e *defaultExecutor) ListExecutions(webhookID, clientID, status string, limit int) []*WebhookExecution {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var results []*WebhookExecution
	for _, execution := range e.executions {
		if webhookID != "" && execution.WebhookID != webhookID {
			continue
		}
		if clientID != "" && execution.ClientID != clientID {
			continue
		}
		if status != "" && string(execution.GetStatus()) != status {
			continue
		}

		results = append(results, execution.GetCopy())
		if limit > 0 && len(results) >= limit {
			break
		}
	}

	return results
}

func (e *defaultExecutor) TestWebhook(webhook *WebhookConfig, clientID, cdpEvent string, eventData map[string]interface{}) (*WebhookExecution, error) {
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

	e.executeWebhook(webhook, execution)

	return execution, nil
}

func (e *defaultExecutor) executeWithRetries(webhook *WebhookConfig, execution *WebhookExecution) {
	var err error
	for i := 0; i <= webhook.MaxRetries; i++ {
		if i > 0 {
			execution.SetRetryCount(i)
		}

		err = e.executeWebhook(webhook, execution)
		if err == nil {
			break
		}

		if i < webhook.MaxRetries {
			time.Sleep(time.Duration(1+i) * time.Second)
		}
	}
}

func (e *defaultExecutor) executeWebhook(webhook *WebhookConfig, execution *WebhookExecution) error {
	payload, err := json.Marshal(map[string]interface{}{
		"event":     execution.CDPEvent,
		"client_id": execution.ClientID,
		"data":      execution.EventData,
	})
	if err != nil {
		execution.SetStatus(WebhookStatusFailed)
		execution.SetError("Failed to marshal payload: " + err.Error())
		return err
	}

	req, err := http.NewRequest("POST", webhook.URL, bytes.NewBuffer(payload))
	if err != nil {
		execution.SetStatus(WebhookStatusFailed)
		execution.SetError("Failed to create request: " + err.Error())
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	for key, value := range webhook.Headers {
		req.Header.Set(key, value)
	}

	client := &http.Client{
		Timeout: time.Duration(webhook.Timeout) * time.Second,
	}

	start := time.Now()
	resp, err := client.Do(req)
	duration := time.Since(start)

	if err != nil {
		execution.SetStatus(WebhookStatusFailed)
		execution.SetError("Request failed: " + err.Error())
		execution.SetResponse(0, "", int(duration.Milliseconds()))
		return err
	}
	defer resp.Body.Close()

	var respBody bytes.Buffer
	respBody.ReadFrom(resp.Body)
	responseBody := respBody.String()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		execution.SetStatus(WebhookStatusSuccess)
		execution.SetResponse(resp.StatusCode, responseBody, int(duration.Milliseconds()))
		return nil
	}

	execution.SetStatus(WebhookStatusFailed)
	execution.SetError(fmt.Sprintf("HTTP error: %d", resp.StatusCode))
	execution.SetResponse(resp.StatusCode, responseBody, int(duration.Milliseconds()))
	return fmt.Errorf("HTTP error: %d", resp.StatusCode)
}
