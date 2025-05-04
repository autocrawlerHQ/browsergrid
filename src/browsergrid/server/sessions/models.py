"""
Session database models for browsergrid API
"""
import uuid
from datetime import datetime, timedelta

from sqlalchemy import (
    Column, String, Boolean, Float, Integer, ForeignKey, Enum, 
    DateTime, Text, func
)
from sqlalchemy.dialects.postgresql import UUID, JSONB
from sqlalchemy.orm import relationship

from browsergrid.server.core.db.base import Base
from browsergrid.server.sessions.enums import OperatingSystem, Browser, BrowserVersion, SessionStatus
from browsergrid.server.core.db.crud import CRUDMixin

class Session(Base, CRUDMixin):
    """Browser session database model"""
    __tablename__ = "sessions"
    
    # Primary identifiers
    id = Column(UUID(as_uuid=True), primary_key=True, default=uuid.uuid4)
    
    # Browser configuration
    browser = Column(Enum(Browser), nullable=False)
    version = Column(Enum(BrowserVersion), nullable=False)
    headless = Column(Boolean, default=False, nullable=False)
    operating_system = Column(Enum(OperatingSystem), nullable=False, default=OperatingSystem.LINUX)
    
    # JSON stored configurations
    screen = Column(JSONB, nullable=False)
    proxy = Column(JSONB, nullable=True)
    resource_limits = Column(JSONB, nullable=True)
    environment = Column(JSONB, default={}, nullable=False)
    
    # State and lifecycle 
    status = Column(Enum(SessionStatus), default=SessionStatus.PENDING, nullable=False)
    created_at = Column(DateTime(timezone=True), server_default=func.now(), nullable=False)
    updated_at = Column(DateTime(timezone=True), server_default=func.now(), onupdate=func.now(), nullable=False)
    expires_at = Column(DateTime(timezone=True), nullable=True)
    
    # Runtime details
    container_id = Column(String, nullable=True)
    provider = Column(String, default="local", nullable=False)
    webhooks_enabled = Column(Boolean, default=True, nullable=False)
    
    # Connection details
    ws_endpoint = Column(String, nullable=True)
    live_url = Column(String, nullable=True)

    # WorkPool and Worker references
    worker_id = Column(UUID(as_uuid=True), ForeignKey("workers.id", ondelete="SET NULL"), nullable=True)
    work_pool_id = Column(UUID(as_uuid=True), ForeignKey("work_pools.id", ondelete="SET NULL"), nullable=True)
    
    # Relationships
    events = relationship("SessionEvent", back_populates="session", cascade="all, delete-orphan")
    metrics = relationship("SessionMetrics", back_populates="session", cascade="all, delete-orphan")
    webhooks = relationship("CDPWebhook", back_populates="session", cascade="all, delete-orphan")
    worker = relationship("Worker", back_populates="sessions")
    work_pool = relationship("WorkPool", back_populates="sessions")

    @classmethod
    def create_from_schema(cls, session_create):
        """Create a database model from a SessionCreate schema"""
        # Default expires_at to 30 minutes from now if timeout_minutes is set
        expires_at = None
        if session_create.resource_limits and session_create.resource_limits.timeout_minutes:
            minutes = session_create.resource_limits.timeout_minutes
            expires_at = datetime.now() + timedelta(minutes=minutes)
        
        return cls(
            id=session_create.id,
            browser=session_create.browser.value,
            version=session_create.version.value,
            headless=session_create.headless,
            operating_system=session_create.operating_system.value,
            screen=session_create.screen.dict(),
            proxy=session_create.proxy.dict() if session_create.proxy else None,
            resource_limits=session_create.resource_limits.dict() if session_create.resource_limits else None,
            environment=session_create.environment,
            provider=session_create.provider,
            webhooks_enabled=session_create.webhooks_enabled,
            expires_at=expires_at
        )

class SessionEvent(Base, CRUDMixin):
    """Event database model for a browser session"""
    __tablename__ = "session_events"
    
    id = Column(UUID(as_uuid=True), primary_key=True, default=uuid.uuid4)
    session_id = Column(UUID(as_uuid=True), ForeignKey("sessions.id", ondelete="CASCADE"), nullable=False)
    event = Column(String, nullable=False)
    timestamp = Column(DateTime(timezone=True), server_default=func.now(), nullable=False)
    data = Column(JSONB, nullable=True)
    
    # Relationship
    session = relationship("Session", back_populates="events")

class SessionMetrics(Base, CRUDMixin):
    """Metrics database model for a browser session"""
    __tablename__ = "session_metrics"
    
    id = Column(UUID(as_uuid=True), primary_key=True, default=uuid.uuid4)
    session_id = Column(UUID(as_uuid=True), ForeignKey("sessions.id", ondelete="CASCADE"), nullable=False)
    cpu_percent = Column(Float, nullable=True)
    memory_mb = Column(Float, nullable=True)
    network_rx_bytes = Column(Integer, nullable=True)
    network_tx_bytes = Column(Integer, nullable=True)
    timestamp = Column(DateTime(timezone=True), server_default=func.now(), nullable=False)
    
    # Relationship
    session = relationship("Session", back_populates="metrics")