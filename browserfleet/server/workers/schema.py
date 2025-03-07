"""
Base worker classes and interfaces for browserfleet
"""
import uuid
from abc import ABC, abstractmethod
from enum import Enum
from typing import Dict, List, Optional, Union, Any, Tuple, Type
from uuid import UUID
from datetime import datetime
from pydantic import BaseModel, Field

from browserfleet.server.workers.enums import WorkerType, ProviderType,
from browserfleet.server.workers.manager import Worker


class WorkerConfig(BaseModel):
    """Base configuration for workers"""
    name: str
    worker_type: WorkerType
    provider_type: ProviderType
    concurrency_limit: int = 5
    labels: Dict[str, str] = Field(default_factory=dict)
    
    # Provider-specific configuration that varies by provider type
    provider_config: Dict[str, Any] = Field(default_factory=dict)
    
    class Config:
        extra = "allow"  # Allow extra fields for provider-specific configs

class WorkerStats(BaseModel):
    """Runtime statistics for workers"""
    running_containers: int = 0
    cpu_percent: Optional[float] = None
    memory_percent: Optional[float] = None
    disk_percent: Optional[float] = None
    network_rx_bytes: Optional[int] = None
    network_tx_bytes: Optional[int] = None
    last_updated: Optional[str] = None  # ISO timestamp


