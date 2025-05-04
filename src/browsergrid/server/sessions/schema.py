"""
Pydantic schemas for session-related API endpoints
"""
from __future__ import annotations
from typing import Dict, List, Optional, Any, Union
from datetime import datetime
from uuid import UUID, uuid4

from pydantic import BaseModel, Field, validator, root_validator

from browsergrid.server.sessions.enums import OperatingSystem, Browser, BrowserVersion, SessionStatus, SessionEventType

class ProxyConfig(BaseModel):
    """Configuration for HTTP proxy"""
    proxy_url: str
    proxy_username: Optional[str] = None
    proxy_password: Optional[str] = None
    
    class Config:
        from_attributes = True

class ResourceLimits(BaseModel):
    """Resource limits for browser sessions"""
    cpu: Optional[float] = None
    memory: Optional[str] = None
    timeout_minutes: Optional[int] = 30
    
    class Config:
        from_attributes = True
        
    @validator('memory')
    def validate_memory(cls, v):
        """Validate memory format (e.g., '2G', '512M')"""
        if v is None:
            return v
        
        import re
        if not re.match(r'^\d+[MG]$', v):
            raise ValueError("Memory must be in format like '512M' or '2G'")
        return v

class ScreenConfig(BaseModel):
    """Screen configuration"""
    width: int = 1280
    height: int = 720
    dpi: int = 96
    scale: float = 1.0
    
    class Config:
        from_attributes = True
        
    @validator('width', 'height')
    def validate_dimensions(cls, v):
        """Validate screen dimensions"""
        if v <= 0:
            raise ValueError("Dimensions must be positive")
        return v

# Session event schemas
class SessionEventBase(BaseModel):
    """Base model for session events"""
    event: SessionEventType
    data: Optional[Dict[str, Any]] = None
    
    class Config:
        from_attributes = True
        
    @validator('event')
    def validate_event_type(cls, v):
        """Convert string to enum value if needed"""
        if isinstance(v, str):
            try:
                return SessionEventType(v)
            except ValueError:
                raise ValueError(f"Invalid event type: {v}")
        return v

class SessionEventCreate(SessionEventBase):
    """Schema for creating session events"""
    session_id: UUID

class SessionEvent(SessionEventBase):
    """Full session event model"""
    id: UUID
    session_id: UUID
    timestamp: datetime
    
    class Config:
        from_attributes = True

# Session metrics schemas
class SessionMetricsBase(BaseModel):
    """Base model for session metrics"""
    cpu_percent: Optional[float] = None
    memory_mb: Optional[float] = None
    network_rx_bytes: Optional[int] = None
    network_tx_bytes: Optional[int] = None
    
    class Config:
        from_attributes = True

class SessionMetricsCreate(SessionMetricsBase):
    """Schema for creating session metrics"""
    session_id: UUID

class SessionMetrics(SessionMetricsBase):
    """Full session metrics model"""
    id: UUID
    session_id: UUID
    timestamp: datetime
    
    class Config:
        from_attributes = True

# Session schemas
class SessionBase(BaseModel):
    """Base session properties"""
    browser: Browser = Browser.CHROME
    version: BrowserVersion = BrowserVersion.LATEST
    headless: bool = False
    operating_system: OperatingSystem = OperatingSystem.LINUX
    screen: ScreenConfig = Field(default_factory=ScreenConfig)
    proxy: Optional[ProxyConfig] = None
    resource_limits: ResourceLimits = Field(default_factory=ResourceLimits)
    environment: Dict[str, str] = Field(default_factory=dict)
    provider: str = "local"
    webhooks_enabled: bool = True
    
    class Config:
        from_attributes = True

class SessionCreate(SessionBase):
    """Schema for creating a new session"""
    id: Optional[UUID] = Field(default_factory=uuid4)
    work_pool_id: Optional[UUID] = None
    
    class Config:
        json_schema_extra = {
            "example": {
                "browser": "chrome",
                "version": "latest",
                "headless": False,
                "resource_limits": {"cpu": 2, "memory": "4G", "timeout_minutes": 60},
                "operating_system": "linux",
                "screen": {"width": 1280, "height": 1024, "dpi": 96, "scale": 1.0},
                "proxy": {"proxy_url": "http://127.0.0.1:9090"},
                "webhooks_enabled": True,
                "work_pool_id": "docker-pool-1"
            }
        }

class Session(SessionBase):
    """Full session model"""
    id: UUID
    status: SessionStatus
    created_at: datetime
    updated_at: datetime
    expires_at: Optional[datetime] = None
    container_id: Optional[str] = None
    work_pool_id: Optional[UUID] = None
    worker_id: Optional[UUID] = None
    
    class Config:
        from_attributes = True

class SessionDetails(Session):
    """Session with connection details"""
    ws_endpoint: Optional[str] = None
    live_url: Optional[str] = None
    
    class Config:
        from_attributes = True

class SessionWithRelations(SessionDetails):
    """Session with its related data"""
    events: List[SessionEvent] = []
    metrics: List[SessionMetrics] = []
    
    class Config:
        from_attributes = True