# browsergrid/server/schemas/webhook.py
"""
Pydantic schemas for webhook-related API endpoints
"""
from __future__ import annotations
from typing import Dict, List, Optional, Any, Union
from datetime import datetime
from uuid import UUID, uuid4

from pydantic import BaseModel, Field, HttpUrl, validator
from browsergrid.server.webhooks.enums import WebhookTiming, WebhookStatus

class CDPEventPattern(BaseModel):
    """Pattern to match CDP events"""
    method: str  # CDP method name (e.g., "Page.navigate")
    param_filters: Optional[Dict[str, Any]] = None
    
    class Config:
        from_attributes = True
        
    @validator('method')
    def validate_method(cls, v):
        """Ensure method has domain and method name"""
        if '.' not in v:
            raise ValueError("CDP method must be in format 'Domain.method'")
        return v

class CDPWebhookBase(BaseModel):
    """Base webhook properties"""
    name: str
    description: Optional[str] = None
    event_pattern: CDPEventPattern
    timing: WebhookTiming
    webhook_url: HttpUrl
    webhook_headers: Dict[str, str] = Field(default_factory=dict)
    timeout_seconds: int = 10
    max_retries: int = 3
    active: bool = True
    
    class Config:
        from_attributes = True
        
    @validator('timeout_seconds')
    def validate_timeout(cls, v):
        """Validate timeout is within reasonable range"""
        if v < 1 or v > 60:
            raise ValueError("Timeout must be between 1 and 60 seconds")
        return v

class CDPWebhookCreate(CDPWebhookBase):
    """Schema for creating a new webhook"""
    session_id: UUID
    id: Optional[UUID] = Field(default_factory=uuid4)

class CDPWebhook(CDPWebhookBase):
    """Full webhook model"""
    id: UUID
    session_id: UUID
    created_at: datetime
    updated_at: Optional[datetime] = None
    
    class Config:
        from_attributes = True

class WebhookExecutionBase(BaseModel):
    """Base webhook execution properties"""
    cdp_event: str
    event_data: Dict[str, Any]
    timing: WebhookTiming
    
    class Config:
        from_attributes = True

class WebhookExecutionCreate(WebhookExecutionBase):
    """Schema for creating a webhook execution"""
    webhook_id: UUID
    session_id: UUID

class WebhookExecution(WebhookExecutionBase):
    """Full webhook execution model"""
    id: UUID
    webhook_id: UUID
    session_id: UUID
    status: WebhookStatus
    started_at: datetime
    completed_at: Optional[datetime] = None
    response_status_code: Optional[int] = None
    response_body: Optional[str] = None
    error_message: Optional[str] = None
    retry_count: int = 0
    execution_time_ms: Optional[float] = None
    
    class Config:
        from_attributes = True

class WebhookTemplate(BaseModel):
    """Template for common webhook use cases"""
    id: str
    name: str
    description: str
    event_pattern: CDPEventPattern
    timing: WebhookTiming
    default_url: HttpUrl
    example_config: Dict[str, Any]


WEBHOOK_TEMPLATES = {
    "captcha-solver": WebhookTemplate(
        id="captcha-solver",
        name="Captcha Solver",
        description="Automatically detects and solves captchas when they appear",
        event_pattern=CDPEventPattern(
            method="Page.frameNavigated",
        ),
        timing=WebhookTiming.AFTER_EVENT,
        default_url="https://your-captcha-service.example.com/solve",
        example_config={
            "webhook_headers": {"Authorization": "Bearer YOUR_API_KEY"}
        }
    ),
    "navigation-tracker": WebhookTemplate(
        id="navigation-tracker",
        name="Navigation Tracker",
        description="Logs all page navigations to an external service",
        event_pattern=CDPEventPattern(
            method="Page.frameNavigated",
        ),
        timing=WebhookTiming.AFTER_EVENT,
        default_url="https://your-analytics.example.com/page-visit",
        example_config={}
    ),
    "screenshot-capture": WebhookTemplate(
        id="screenshot-capture",
        name="Screenshot Capture",
        description="Captures screenshots when page loads complete",
        event_pattern=CDPEventPattern(
            method="Page.loadEventFired",
        ),
        timing=WebhookTiming.AFTER_EVENT,
        default_url="https://your-screenshot-service.example.com/capture",
        example_config={}
    ),
    "content-extraction": WebhookTemplate(
        id="content-extraction",
        name="Content Extraction",
        description="Extracts content from the page after DOM is fully loaded",
        event_pattern=CDPEventPattern(
            method="DOM.documentUpdated",
        ),
        timing=WebhookTiming.AFTER_EVENT,
        default_url="https://your-extraction-service.example.com/extract",
        example_config={}
    ),
    "network-interceptor": WebhookTemplate(
        id="network-interceptor", 
        name="Network Interceptor",
        description="Intercepts network requests and responses",
        event_pattern=CDPEventPattern(
            method="Network.responseReceived",
        ),
        timing=WebhookTiming.BEFORE_EVENT,
        default_url="https://your-proxy-service.example.com/intercept",
        example_config={
            "webhook_headers": {"Content-Type": "application/json"}
        }
    )
}