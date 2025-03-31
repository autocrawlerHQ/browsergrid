package webhook

import (
	"browsermux/internal/events"
)

// Manager manages webhook configurations and executions
type Manager interface {
	// RegisterWebhook registers a new webhook
	RegisterWebhook(config WebhookConfig) (string, error)

	// GetWebhook gets a webhook by ID
	GetWebhook(id string) (*WebhookConfig, error)

	// UpdateWebhook updates an existing webhook
	UpdateWebhook(id string, updates map[string]interface{}) (*WebhookConfig, error)

	// DeleteWebhook deletes a webhook
	DeleteWebhook(id string) error

	// ListWebhooks lists all webhooks
	ListWebhooks() []*WebhookConfig

	// TriggerWebhooks triggers matching webhooks for an event
	TriggerWebhooks(method string, params map[string]interface{}, clientID string, timing WebhookTiming) []*WebhookExecution

	// GetExecution gets a webhook execution by ID
	GetExecution(id string) (*WebhookExecution, error)

	// ListExecutions lists webhook executions
	ListExecutions(webhookID, clientID, status string, limit int) []*WebhookExecution

	// TestWebhook tests a webhook with sample data
	TestWebhook(webhookID, clientID, method string, params map[string]interface{}) (*WebhookExecution, error)
}

// defaultManager is the default implementation of Manager
type defaultManager struct {
	registry   Registry
	executor   Executor
	dispatcher events.Dispatcher
}

// NewManager creates a new webhook manager
func NewManager(dispatcher events.Dispatcher) Manager {
	registry := NewRegistry()
	executor := NewExecutor(dispatcher)

	return &defaultManager{
		registry:   registry,
		executor:   executor,
		dispatcher: dispatcher,
	}
}

// RegisterWebhook implements Manager.RegisterWebhook
func (m *defaultManager) RegisterWebhook(config WebhookConfig) (string, error) {
	id, err := m.registry.RegisterWebhook(config)
	if err != nil {
		return "", err
	}

	// Emit event about webhook registration
	// m.dispatcher.Emit("webhook.registered", map[string]interface{}{"webhook_id": id})

	return id, nil
}

// GetWebhook implements Manager.GetWebhook
func (m *defaultManager) GetWebhook(id string) (*WebhookConfig, error) {
	return m.registry.GetWebhook(id)
}

// UpdateWebhook implements Manager.UpdateWebhook
func (m *defaultManager) UpdateWebhook(id string, updates map[string]interface{}) (*WebhookConfig, error) {
	return m.registry.UpdateWebhook(id, updates)
}

// DeleteWebhook implements Manager.DeleteWebhook
func (m *defaultManager) DeleteWebhook(id string) error {
	return m.registry.DeleteWebhook(id)
}

// ListWebhooks implements Manager.ListWebhooks
func (m *defaultManager) ListWebhooks() []*WebhookConfig {
	return m.registry.ListWebhooks()
}

// TriggerWebhooks implements Manager.TriggerWebhooks
func (m *defaultManager) TriggerWebhooks(method string, params map[string]interface{}, clientID string, timing WebhookTiming) []*WebhookExecution {
	// Find matching webhooks
	webhooks := m.registry.FindMatchingWebhooks(method, params, timing)

	// No matches, return empty slice
	if len(webhooks) == 0 {
		return []*WebhookExecution{}
	}

	// Execute each webhook
	executions := make([]*WebhookExecution, 0, len(webhooks))

	for _, webhook := range webhooks {
		// Execute webhook
		execution, err := m.executor.ExecuteWebhook(webhook, clientID, method, params)
		if err != nil {
			// Log error but continue with other webhooks
			// log.Printf("Error executing webhook %s: %v", webhook.ID, err)
			continue
		}

		// Add to results
		executions = append(executions, execution)
	}

	return executions
}

// GetExecution implements Manager.GetExecution
func (m *defaultManager) GetExecution(id string) (*WebhookExecution, error) {
	return m.executor.GetExecution(id)
}

// ListExecutions implements Manager.ListExecutions
func (m *defaultManager) ListExecutions(webhookID, clientID, status string, limit int) []*WebhookExecution {
	return m.executor.ListExecutions(webhookID, clientID, status, limit)
}

// TestWebhook implements Manager.TestWebhook
func (m *defaultManager) TestWebhook(webhookID, clientID, method string, params map[string]interface{}) (*WebhookExecution, error) {
	webhook, err := m.registry.GetWebhook(webhookID)
	if err != nil {
		return nil, err
	}

	return m.executor.TestWebhook(webhook, clientID, method, params)
}
