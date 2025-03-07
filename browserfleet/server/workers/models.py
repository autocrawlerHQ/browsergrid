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
from browserfleet.server.workers.enums import WorkerStatus, WorkerType, ProviderType


class Worker(Base):
    """Worker database model"""
    __tablename__ = "workers"
    
    # Primary key
    id = Column(UUID(as_uuid=True), primary_key=True, default=uuid.uuid4)
    
    # Foreign key to work pool
    pool_id = Column(UUID(as_uuid=True), ForeignKey("work_pools.id", ondelete="CASCADE"), nullable=False)
    
    # Worker configuration
    name = Column(String, nullable=False)
    worker_type = Column(String, default=WorkerType.DOCKER.value, nullable=False)  # Stored as string from WorkerType enum
    provider_type = Column(String, default=ProviderType.DOCKER.value, nullable=False)  # Stored as string from ProviderType enum
    config = Column(JSONB, nullable=False)
    concurrency_limit = Column(Integer, default=5, nullable=False)
    labels = Column(JSONB, default={}, nullable=False)
    
    # Status and metadata
    status = Column(String, nullable=False, default=WorkerStatus.OFFLINE.value)
    created_at = Column(DateTime(timezone=True), server_default=func.now(), nullable=False)
    updated_at = Column(DateTime(timezone=True), server_default=func.now(), onupdate=func.now(), nullable=False)
    last_heartbeat = Column(DateTime(timezone=True), nullable=True)
    
    # Runtime statistics
    running_containers = Column(Integer, default=0, nullable=False)
    cpu_percent = Column(Float, nullable=True)
    memory_percent = Column(Float, nullable=True)
    disk_percent = Column(Float, nullable=True)
    network_rx_bytes = Column(Integer, nullable=True)
    network_tx_bytes = Column(Integer, nullable=True)
    
    # Provider-specific details
    host_address = Column(String, nullable=True)  # IP or hostname where the worker is running
    api_endpoint = Column(String, nullable=True)  # Endpoint for communicating with the worker
    provider_resource_id = Column(String, nullable=True)  # ID in the provider's system (e.g., container ID, instance ID)
    
    # Relationships
    pool = relationship("WorkPool", back_populates="workers")
    sessions = relationship("Session", back_populates="worker")
    
    # Constraints
    __table_args__ = (
        UniqueConstraint('pool_id', 'name', name='uq_worker_pool_name'),
    )

