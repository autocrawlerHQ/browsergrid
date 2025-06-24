package webhook

import (
	"browsermux/internal/events"
)

type Manager interface {
	RegisterWebhook(config WebhookConfig) (string, error)
	GetWebhook(id string) (*WebhookConfig, error)
	UpdateWebhook(id string, updates map[string]interface{}) (*WebhookConfig, error)
	DeleteWebhook(id string) error
	ListWebhooks() []*WebhookConfig
	TriggerWebhooks(method string, params map[string]interface{}, clientID string, timing WebhookTiming) []*WebhookExecution
	GetExecution(id string) (*WebhookExecution, error)
	ListExecutions(webhookID, clientID, status string, limit int) []*WebhookExecution
	TestWebhook(webhookID, clientID, method string, params map[string]interface{}) (*WebhookExecution, error)
}

type defaultManager struct {
	registry   Registry
	executor   Executor
	dispatcher events.Dispatcher
}

func NewManager(dispatcher events.Dispatcher) Manager {
	registry := NewRegistry()
	executor := NewExecutor(dispatcher)

	return &defaultManager{
		registry:   registry,
		executor:   executor,
		dispatcher: dispatcher,
	}
}

func (m *defaultManager) RegisterWebhook(config WebhookConfig) (string, error) {
	id, err := m.registry.RegisterWebhook(config)
	if err != nil {
		return "", err
	}

	return id, nil
}

func (m *defaultManager) GetWebhook(id string) (*WebhookConfig, error) {
	return m.registry.GetWebhook(id)
}

func (m *defaultManager) UpdateWebhook(id string, updates map[string]interface{}) (*WebhookConfig, error) {
	return m.registry.UpdateWebhook(id, updates)
}

func (m *defaultManager) DeleteWebhook(id string) error {
	return m.registry.DeleteWebhook(id)
}

func (m *defaultManager) ListWebhooks() []*WebhookConfig {
	return m.registry.ListWebhooks()
}

func (m *defaultManager) TriggerWebhooks(method string, params map[string]interface{}, clientID string, timing WebhookTiming) []*WebhookExecution {
	webhooks := m.registry.FindMatchingWebhooks(method, params, timing)

	if len(webhooks) == 0 {
		return []*WebhookExecution{}
	}

	executions := make([]*WebhookExecution, 0, len(webhooks))

	for _, webhook := range webhooks {
		execution, err := m.executor.ExecuteWebhook(webhook, clientID, method, params)
		if err != nil {
			continue
		}

		executions = append(executions, execution)
	}

	return executions
}

func (m *defaultManager) GetExecution(id string) (*WebhookExecution, error) {
	return m.executor.GetExecution(id)
}

func (m *defaultManager) ListExecutions(webhookID, clientID, status string, limit int) []*WebhookExecution {
	return m.executor.ListExecutions(webhookID, clientID, status, limit)
}

func (m *defaultManager) TestWebhook(webhookID, clientID, method string, params map[string]interface{}) (*WebhookExecution, error) {
	webhook, err := m.registry.GetWebhook(webhookID)
	if err != nil {
		return nil, err
	}

	return m.executor.TestWebhook(webhook, clientID, method, params)
}
