package webhook

import (
	"sync"
	"time"
)

type WebhookTiming string

const (
	WebhookTimingBeforeEvent WebhookTiming = "before_event"
	WebhookTimingAfterEvent  WebhookTiming = "after_event"
)

type WebhookStatus string

const (
	WebhookStatusPending WebhookStatus = "pending"
	WebhookStatusSuccess WebhookStatus = "success"
	WebhookStatusFailed  WebhookStatus = "failed"
	WebhookStatusTimeout WebhookStatus = "timeout"
)

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

type WebhookExecution struct {
	mu             sync.RWMutex           `json:"-"`
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

func (w *WebhookExecution) SetStatus(status WebhookStatus) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.Status = status
}

func (w *WebhookExecution) GetStatus() WebhookStatus {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.Status
}

func (w *WebhookExecution) SetError(err string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.Error = err
}

func (w *WebhookExecution) GetError() string {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.Error
}

func (w *WebhookExecution) SetRetryCount(count int) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.RetryCount = count
}

func (w *WebhookExecution) GetRetryCount() int {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.RetryCount
}

func (w *WebhookExecution) SetResponse(status int, body string, duration int) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.ResponseStatus = status
	w.ResponseBody = body
	w.DurationMS = duration
	now := time.Now()
	w.CompletedAt = &now
}

func (w *WebhookExecution) GetCopy() *WebhookExecution {
	w.mu.RLock()
	defer w.mu.RUnlock()

	var completedAt *time.Time
	if w.CompletedAt != nil {
		t := *w.CompletedAt
		completedAt = &t
	}

	return &WebhookExecution{
		ID:             w.ID,
		WebhookID:      w.WebhookID,
		ClientID:       w.ClientID,
		CDPEvent:       w.CDPEvent,
		EventData:      w.EventData,
		Timing:         w.Timing,
		Status:         w.Status,
		StartedAt:      w.StartedAt,
		CompletedAt:    completedAt,
		DurationMS:     w.DurationMS,
		ResponseStatus: w.ResponseStatus,
		ResponseBody:   w.ResponseBody,
		RetryCount:     w.RetryCount,
		Error:          w.Error,
	}
}
