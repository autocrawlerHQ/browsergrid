package webhook

import (
	"time"
)

// WebhookTiming defines when a webhook should be triggered
type WebhookTiming string

const (
	WebhookTimingBeforeEvent WebhookTiming = "before_event"
	WebhookTimingAfterEvent  WebhookTiming = "after_event"
)

// WebhookStatus represents the execution status of a webhook
type WebhookStatus string

const (
	WebhookStatusPending WebhookStatus = "pending"
	WebhookStatusSuccess WebhookStatus = "success"
	WebhookStatusFailed  WebhookStatus = "failed"
	WebhookStatusTimeout WebhookStatus = "timeout"
)

// WebhookConfig defines the configuration for a webhook
type WebhookConfig struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	URL         string                 `json:"url"`
	EventMethod string                 `json:"event_method"`
	EventParams map[string]interface{} `json:"event_params,omitempty"`
	Timing      WebhookTiming          `json:"timing"`
	Headers     map[string]string      `json:"headers,omitempty"`
	Timeout     int                    `json:"timeout"`
	MaxRetries  int                    `json:"max_retries"`
	Enabled     bool                   `json:"enabled"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
}

// WebhookExecution tracks a single execution of a webhook
type WebhookExecution struct {
	ID             string                 `json:"id"`
	WebhookID      string                 `json:"webhook_id"`
	ClientID       string                 `json:"client_id,omitempty"`
	CDPEvent       string                 `json:"cdp_event"`
	EventData      map[string]interface{} `json:"event_data"`
	Timing         WebhookTiming          `json:"timing"`
	Status         WebhookStatus          `json:"status"`
	StartedAt      time.Time              `json:"started_at"`
	CompletedAt    *time.Time             `json:"completed_at,omitempty"`
	DurationMS     int                    `json:"duration_ms,omitempty"`
	ResponseStatus int                    `json:"response_status,omitempty"`
	ResponseBody   string                 `json:"response_body,omitempty"`
	RetryCount     int                    `json:"retry_count"`
	Error          string                 `json:"error,omitempty"`
}
