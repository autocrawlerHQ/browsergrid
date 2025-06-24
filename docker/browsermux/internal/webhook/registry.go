package webhook

import (
	"sync"
	"time"

	"github.com/google/uuid"
)

type Registry interface {
	RegisterWebhook(config WebhookConfig) (string, error)

	GetWebhook(id string) (*WebhookConfig, error)

	UpdateWebhook(id string, updates map[string]interface{}) (*WebhookConfig, error)

	DeleteWebhook(id string) error

	ListWebhooks() []*WebhookConfig

	FindMatchingWebhooks(method string, params map[string]interface{}, timing WebhookTiming) []*WebhookConfig
}

type defaultRegistry struct {
	webhooks map[string]*WebhookConfig
	mu       sync.RWMutex
}

func NewRegistry() Registry {
	return &defaultRegistry{
		webhooks: make(map[string]*WebhookConfig),
	}
}

func (r *defaultRegistry) RegisterWebhook(config WebhookConfig) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if config.ID == "" {
		config.ID = uuid.New().String()
	}

	now := time.Now()
	config.CreatedAt = now
	config.UpdatedAt = now

	r.webhooks[config.ID] = &config

	return config.ID, nil
}

func (r *defaultRegistry) GetWebhook(id string) (*WebhookConfig, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	webhook, ok := r.webhooks[id]
	if !ok {
		return nil, ErrWebhookNotFound
	}

	return webhook, nil
}

func (r *defaultRegistry) UpdateWebhook(id string, updates map[string]interface{}) (*WebhookConfig, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	webhook, ok := r.webhooks[id]
	if !ok {
		return nil, ErrWebhookNotFound
	}

	if name, ok := updates["name"].(string); ok {
		webhook.Name = name
	}
	if url, ok := updates["url"].(string); ok {
		webhook.URL = url
	}

	webhook.UpdatedAt = time.Now()
	return webhook, nil
}

func (r *defaultRegistry) DeleteWebhook(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.webhooks[id]; !ok {
		return ErrWebhookNotFound
	}

	delete(r.webhooks, id)
	return nil
}

func (r *defaultRegistry) ListWebhooks() []*WebhookConfig {
	r.mu.RLock()
	defer r.mu.RUnlock()

	webhooks := make([]*WebhookConfig, 0, len(r.webhooks))
	for _, webhook := range r.webhooks {
		webhooks = append(webhooks, webhook)
	}

	return webhooks
}

func (r *defaultRegistry) FindMatchingWebhooks(method string, params map[string]interface{}, timing WebhookTiming) []*WebhookConfig {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var matches []*WebhookConfig
	for _, webhook := range r.webhooks {
		if !webhook.Enabled {
			continue
		}

		if webhook.Timing != timing {
			continue
		}

		if webhook.EventMethod != method && webhook.EventMethod != "*" {
			continue
		}

		matches = append(matches, webhook)
	}

	return matches
}

var (
	ErrWebhookNotFound = NewError("webhook not found")
)

type WebhookError struct {
	message string
}

func NewError(msg string) error {
	return &WebhookError{message: msg}
}

func (e *WebhookError) Error() string {
	return e.message
}
