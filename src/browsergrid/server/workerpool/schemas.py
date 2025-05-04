"""
Pydantic schemas for workerpool API endpoints
"""
from typing import Optional, Dict, List, Any, Union
from datetime import datetime
from uuid import UUID, uuid4

from pydantic import BaseModel, Field, validator, root_validator

from browsergrid.server.workerpool.enums import WorkerStatus, WorkPoolStatus, ProviderType
from browsergrid.server.sessions.enums import OperatingSystem, Browser, BrowserVersion
from browsergrid.server.sessions.schema import ScreenConfig, ProxyConfig, ResourceLimits

# WorkPool Schemas
class WorkPoolBase(BaseModel):
    """Base schema for WorkPool"""
    name: str
    provider_type: ProviderType = ProviderType.DOCKER
    status: WorkPoolStatus = WorkPoolStatus.ACTIVE
    
    # Default browser session configuration (all optional)
    default_browser: Optional[Browser] = None
    default_browser_version: Optional[BrowserVersion] = None
    default_headless: Optional[bool] = None
    default_operating_system: Optional[OperatingSystem] = None
    
    # JSON stored default configurations
    default_screen: Optional[ScreenConfig] = None
    default_proxy: Optional[ProxyConfig] = None
    default_resource_limits: Optional[ResourceLimits] = None
    default_environment: Optional[Dict[str, str]] = None
    
    # Pool limits and scaling configuration
    min_workers: int = 0
    max_workers: int = 10
    max_sessions_per_worker: int = 5
    
    # Provider-specific configuration
    provider_config: Dict[str, Any] = Field(default_factory=dict)
    
    # Metadata
    description: Optional[str] = None
    
    class Config:
        from_attributes = True
        
    @validator('name')
    def validate_name(cls, v):
        if not v or not v.strip():
            raise ValueError("Name cannot be empty")
        return v

class WorkPoolCreate(WorkPoolBase):
    """Schema for creating a new WorkPool"""
    id: Optional[UUID] = Field(default_factory=uuid4)
    
    class Config:
        json_schema_extra = {
            "example": {
                "name": "docker-pool-1",
                "provider_type": "docker",
                "default_browser": "chrome",
                "default_browser_version": "latest",
                "default_headless": False,
                "default_operating_system": "linux",
                "default_resource_limits": {"cpu": 2, "memory": "4G", "timeout_minutes": 60},
                "min_workers": 1,
                "max_workers": 5,
                "description": "Docker pool for Chrome sessions"
            }
        }

class WorkPool(WorkPoolBase):
    """Full WorkPool model"""
    id: UUID
    created_at: datetime
    updated_at: datetime
    
    class Config:
        from_attributes = True

class WorkPoolWithRelations(WorkPool):
    """WorkPool with related workers"""
    workers: List["Worker"] = []
    
    class Config:
        from_attributes = True

# Worker Schemas
class WorkerBase(BaseModel):
    """Base schema for Worker"""
    name: str
    work_pool_id: UUID
    status: WorkerStatus = WorkerStatus.OFFLINE
    capacity: int = 5
    provider_type: ProviderType
    provider_details: Dict[str, Any] = Field(default_factory=dict)
    
    class Config:
        from_attributes = True
        
    @validator('name')
    def validate_name(cls, v):
        if not v or not v.strip():
            raise ValueError("Name cannot be empty")
        return v

class WorkerCreate(WorkerBase):
    """Schema for creating a new Worker"""
    id: Optional[UUID] = Field(default_factory=uuid4)
    api_key: Optional[str] = None
    
    class Config:
        json_schema_extra = {
            "example": {
                "name": "worker-1",
                "work_pool_id": "550e8400-e29b-41d4-a716-446655440000",
                "provider_type": "docker",
                "capacity": 5
            }
        }

class Worker(WorkerBase):
    """Full Worker model"""
    id: UUID
    current_load: int
    cpu_percent: Optional[float] = None
    memory_usage_mb: Optional[float] = None
    disk_usage_mb: Optional[float] = None
    ip_address: Optional[str] = None
    last_heartbeat: Optional[datetime] = None
    provider_id: Optional[str] = None
    created_at: datetime
    updated_at: datetime
    
    class Config:
        from_attributes = True

class WorkerHeartbeat(BaseModel):
    """Schema for worker heartbeat updates"""
    status: WorkerStatus
    current_load: int
    cpu_percent: Optional[float] = None
    memory_usage_mb: Optional[float] = None
    disk_usage_mb: Optional[float] = None
    ip_address: Optional[str] = None

class WorkerWithSessions(Worker):
    """Worker with session information"""
    session_count: int
    
    class Config:
        from_attributes = True

# Update for circular reference
WorkPoolWithRelations.update_forward_refs()
