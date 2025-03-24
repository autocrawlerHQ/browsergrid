package webhook

import (
	"sync"
	"time"

	"github.com/google/uuid"
)

// Registry manages webhook configurations
type Registry interface {
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

	// FindMatchingWebhooks finds webhooks matching method/params/timing criteria
	FindMatchingWebhooks(method string, params map[string]interface{}, timing WebhookTiming) []*WebhookConfig
}

// defaultRegistry is the default implementation of Registry
type defaultRegistry struct {
	webhooks map[string]*WebhookConfig
	mu       sync.RWMutex
}

// NewRegistry creates a new webhook registry
func NewRegistry() Registry {
	return &defaultRegistry{
		webhooks: make(map[string]*WebhookConfig),
	}
}

// RegisterWebhook implements Registry.RegisterWebhook
func (r *defaultRegistry) RegisterWebhook(config WebhookConfig) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Generate ID if not provided
	if config.ID == "" {
		config.ID = uuid.New().String()
	}

	// Set timestamps
	now := time.Now()
	config.CreatedAt = now
	config.UpdatedAt = now

	// Store webhook
	r.webhooks[config.ID] = &config

	return config.ID, nil
}

// GetWebhook implements Registry.GetWebhook
func (r *defaultRegistry) GetWebhook(id string) (*WebhookConfig, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	webhook, ok := r.webhooks[id]
	if !ok {
		return nil, ErrWebhookNotFound
	}

	return webhook, nil
}

// UpdateWebhook implements Registry.UpdateWebhook
func (r *defaultRegistry) UpdateWebhook(id string, updates map[string]interface{}) (*WebhookConfig, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	webhook, ok := r.webhooks[id]
	if !ok {
		return nil, ErrWebhookNotFound
	}

	// Apply updates
	// Note: This is a simplified implementation
	if name, ok := updates["name"].(string); ok {
		webhook.Name = name
	}
	if url, ok := updates["url"].(string); ok {
		webhook.URL = url
	}
	// Update other fields similarly...

	webhook.UpdatedAt = time.Now()
	return webhook, nil
}

// DeleteWebhook implements Registry.DeleteWebhook
func (r *defaultRegistry) DeleteWebhook(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.webhooks[id]; !ok {
		return ErrWebhookNotFound
	}

	delete(r.webhooks, id)
	return nil
}

// ListWebhooks implements Registry.ListWebhooks
func (r *defaultRegistry) ListWebhooks() []*WebhookConfig {
	r.mu.RLock()
	defer r.mu.RUnlock()

	webhooks := make([]*WebhookConfig, 0, len(r.webhooks))
	for _, webhook := range r.webhooks {
		webhooks = append(webhooks, webhook)
	}

	return webhooks
}

// FindMatchingWebhooks implements Registry.FindMatchingWebhooks
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

		// Simplified match, can be expanded with more sophisticated matching
		matches = append(matches, webhook)
	}

	return matches
}

// Common errors
var (
	ErrWebhookNotFound = NewError("webhook not found")
)

// WebhookError represents a webhook-specific error
type WebhookError struct {
	message string
}

// NewError creates a new webhook error
func NewError(msg string) error {
	return &WebhookError{message: msg}
}

// Error implements error.Error
func (e *WebhookError) Error() string {
	return e.message
}
