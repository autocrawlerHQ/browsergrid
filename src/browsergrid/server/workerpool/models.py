"""
WorkPool and Worker database models
"""
import uuid
from datetime import datetime

from sqlalchemy import (
    Column, String, Integer, Boolean, ForeignKey, Enum, 
    DateTime, Float, func, UniqueConstraint
)
from sqlalchemy.dialects.postgresql import UUID, JSONB
from sqlalchemy.orm import relationship

from browsergrid.server.core.db.base import Base
from browsergrid.server.workerpool.enums import WorkerStatus, WorkPoolStatus, ProviderType
from browsergrid.server.sessions.enums import OperatingSystem, Browser, BrowserVersion
from browsergrid.server.core.db.crud import CRUDMixin

class WorkPool(Base, CRUDMixin):
    """WorkPool database model"""
    __tablename__ = "work_pools"
    
    # Primary identifiers
    id = Column(UUID(as_uuid=True), primary_key=True, default=uuid.uuid4)
    name = Column(String, unique=True, nullable=False)
    
    # Pool configuration
    provider_type = Column(Enum(ProviderType), nullable=False, default=ProviderType.DOCKER)
    status = Column(Enum(WorkPoolStatus), default=WorkPoolStatus.ACTIVE, nullable=False)
    
    # Default browser session configuration (all optional)
    default_browser = Column(Enum(Browser), nullable=True)
    default_browser_version = Column(Enum(BrowserVersion), nullable=True)
    default_headless = Column(Boolean, nullable=True)
    default_operating_system = Column(Enum(OperatingSystem), nullable=True)
    
    # JSON stored default configurations
    default_screen = Column(JSONB, nullable=True)
    default_proxy = Column(JSONB, nullable=True)
    default_resource_limits = Column(JSONB, nullable=True)
    default_environment = Column(JSONB, nullable=True)
    
    # Pool limits and scaling configuration
    min_workers = Column(Integer, default=0, nullable=False)
    max_workers = Column(Integer, default=10, nullable=False)
    max_sessions_per_worker = Column(Integer, default=5, nullable=False)
    
    # Provider-specific configuration
    provider_config = Column(JSONB, default={}, nullable=False)
    
    # Metadata
    description = Column(String, nullable=True)
    created_at = Column(DateTime(timezone=True), server_default=func.now(), nullable=False)
    updated_at = Column(DateTime(timezone=True), server_default=func.now(), onupdate=func.now(), nullable=False)
    
    # Relationships
    workers = relationship("Worker", back_populates="work_pool", cascade="all, delete-orphan")
    sessions = relationship("Session", back_populates="work_pool")


class Worker(Base, CRUDMixin):
    """Worker database model"""
    __tablename__ = "workers"
    
    # Primary identifiers
    id = Column(UUID(as_uuid=True), primary_key=True, default=uuid.uuid4)
    name = Column(String, nullable=False)
    
    # Worker configuration
    work_pool_id = Column(UUID(as_uuid=True), ForeignKey("work_pools.id", ondelete="CASCADE"), nullable=False)
    status = Column(Enum(WorkerStatus), default=WorkerStatus.OFFLINE, nullable=False)
    
    # Worker capacity and utilization
    capacity = Column(Integer, default=5, nullable=False)  # How many sessions this worker can handle
    current_load = Column(Integer, default=0, nullable=False)  # Current number of active sessions
    
    # System resources metrics
    cpu_percent = Column(Float, nullable=True)
    memory_usage_mb = Column(Float, nullable=True)
    disk_usage_mb = Column(Float, nullable=True)
    
    # Worker connectivity
    ip_address = Column(String, nullable=True)
    last_heartbeat = Column(DateTime(timezone=True), nullable=True)
    
    # Provider-specific details
    provider_type = Column(Enum(ProviderType), nullable=False)
    provider_id = Column(String, nullable=True)  # ID within the provider (e.g., container ID, instance ID)
    provider_details = Column(JSONB, default={}, nullable=False)
    
    # Security
    api_key = Column(String, nullable=True)  # For worker authentication
    
    # Metadata
    created_at = Column(DateTime(timezone=True), server_default=func.now(), nullable=False)
    updated_at = Column(DateTime(timezone=True), server_default=func.now(), onupdate=func.now(), nullable=False)
    
    # Relationships
    work_pool = relationship("WorkPool", back_populates="workers")
    sessions = relationship("Session", back_populates="worker")
    
    # Constraints
    __table_args__ = (
        UniqueConstraint('name', 'work_pool_id', name='unique_worker_name_per_pool'),
    )
