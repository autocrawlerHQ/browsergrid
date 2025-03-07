# browserfleet/server/models/workpool.py
"""
Database models for work pools and workers
"""
import enum
import uuid
from datetime import datetime

from sqlalchemy import (
    Column, String, Boolean, Float, Integer, ForeignKey, Enum, 
    DateTime, Text, func, UniqueConstraint
)
from sqlalchemy.dialects.postgresql import UUID, JSONB
from sqlalchemy.orm import relationship

from browserfleet.server.database.base import Base
from browserfleet.server.workerpool.enums import WorkPoolStatus, WorkAllocationStrategy

class WorkPool(Base):
    """Work pool database model"""
    __tablename__ = "work_pools"
    
    # Primary key
    id = Column(UUID(as_uuid=True), primary_key=True, default=uuid.uuid4)
    
    # Basic information
    name = Column(String, nullable=False)
    description = Column(Text, nullable=True)
    
    # Pool configuration
    worker_type = Column(String, nullable=False)  # Stored as string from WorkerType enum
    provider_type = Column(String, nullable=False)  # Stored as string from ProviderType enum
    default_worker_config = Column(JSONB, nullable=False)
    concurrency_limit = Column(Integer, default=10, nullable=False)
    allocation_strategy = Column(String, nullable=False, default=WorkAllocationStrategy.LEAST_BUSY.value)
    
    # Auto-scaling configuration
    auto_scaling_enabled = Column(Boolean, default=False, nullable=False)
    min_workers = Column(Integer, default=1, nullable=False)
    max_workers = Column(Integer, default=5, nullable=False)
    scale_up_threshold = Column(Float, default=0.8, nullable=False)
    scale_down_threshold = Column(Float, default=0.3, nullable=False)
    scale_down_delay_minutes = Column(Integer, default=10, nullable=False)
    
    # Pool status and metadata
    status = Column(String, nullable=False, default=WorkPoolStatus.ACTIVE.value)
    labels = Column(JSONB, default={}, nullable=False)
    settings = Column(JSONB, default={}, nullable=False)
    created_at = Column(DateTime(timezone=True), server_default=func.now(), nullable=False)
    updated_at = Column(DateTime(timezone=True), server_default=func.now(), onupdate=func.now(), nullable=False)
    last_scale_time = Column(DateTime(timezone=True), server_default=func.now(), nullable=False)
    
    # Statistics
    worker_count = Column(Integer, default=0, nullable=False)
    online_workers = Column(Integer, default=0, nullable=False)
    busy_workers = Column(Integer, default=0, nullable=False)
    error_workers = Column(Integer, default=0, nullable=False)
    total_running_containers = Column(Integer, default=0, nullable=False)
    average_cpu_percent = Column(Float, nullable=True)
    average_memory_percent = Column(Float, nullable=True)
    utilization_percent = Column(Float, default=0.0, nullable=False)
    last_updated = Column(DateTime(timezone=True), nullable=True)
    
    # Relationships
    workers = relationship("Worker", back_populates="pool", cascade="all, delete-orphan")
    
    # Constraints
    __table_args__ = (
        UniqueConstraint('name', name='uq_work_pool_name'),
    )
