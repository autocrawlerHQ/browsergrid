import enum
import uuid

from sqlalchemy import (
    Column, String, Boolean, Float, Integer, ForeignKey, Enum, 
    DateTime, Text, func
)
from sqlalchemy.dialects.postgresql import UUID, JSONB
from sqlalchemy.orm import relationship

from browsergrid.server.core.db.base import Base
from browsergrid.server.webhooks.enums import WebhookTiming, WebhookStatus
from browsergrid.server.core.db.crud import CRUDMixin


class CDPWebhook(Base, CRUDMixin):
    """CDP webhook database model"""
    __tablename__ = "cdp_webhooks"
    
    id = Column(UUID(as_uuid=True), primary_key=True, default=uuid.uuid4)
    session_id = Column(UUID(as_uuid=True), ForeignKey("sessions.id", ondelete="CASCADE"), nullable=False)
    name = Column(String, nullable=False)
    description = Column(Text, nullable=True)
    
    # Event pattern stored as JSON
    event_pattern = Column(JSONB, nullable=False)
    
    timing = Column(Enum(WebhookTiming), nullable=False)
    webhook_url = Column(String, nullable=False)
    webhook_headers = Column(JSONB, default={}, nullable=False)
    timeout_seconds = Column(Integer, default=10, nullable=False)
    max_retries = Column(Integer, default=3, nullable=False)
    active = Column(Boolean, default=True, nullable=False)
    created_at = Column(DateTime(timezone=True), server_default=func.now(), nullable=False)
    updated_at = Column(DateTime(timezone=True), onupdate=func.now(), nullable=True)
    
    # Relationships
    session = relationship("Session", back_populates="webhooks")
    executions = relationship("WebhookExecution", back_populates="webhook", cascade="all, delete-orphan")

class WebhookExecution(Base, CRUDMixin):
    """Webhook execution database model"""
    __tablename__ = "webhook_executions"
    
    id = Column(UUID(as_uuid=True), primary_key=True, default=uuid.uuid4)
    webhook_id = Column(UUID(as_uuid=True), ForeignKey("cdp_webhooks.id", ondelete="CASCADE"), nullable=False)
    session_id = Column(UUID(as_uuid=True), ForeignKey("sessions.id", ondelete="CASCADE"), nullable=False)
    cdp_event = Column(String, nullable=False)
    event_data = Column(JSONB, nullable=False)
    timing = Column(Enum(WebhookTiming), nullable=False)
    status = Column(Enum(WebhookStatus), default=WebhookStatus.PENDING, nullable=False)
    started_at = Column(DateTime(timezone=True), server_default=func.now(), nullable=False)
    completed_at = Column(DateTime(timezone=True), nullable=True)
    response_status_code = Column(Integer, nullable=True)
    response_body = Column(Text, nullable=True)
    error_message = Column(Text, nullable=True)
    retry_count = Column(Integer, default=0, nullable=False)
    execution_time_ms = Column(Float, nullable=True)
    
    # Relationships
    webhook = relationship("CDPWebhook", back_populates="executions")
    session = relationship("Session")