import enum

class WebhookTiming(enum.Enum):
    """When to trigger the webhook, before or after the event is sent to connected clients
    """
    BEFORE_EVENT = "before_event"
    AFTER_EVENT = "after_event"

class WebhookStatus(enum.Enum):
    """Status of a webhook execution
    """
    PENDING = "pending"
    SUCCESS = "success"
    FAILED = "failed"
    TIMEOUT = "timeout"
