"""
Persistent work pool manager with database storage
"""
import asyncio
import uuid
import json
import logging
from abc import ABC, abstractmethod
from typing import Dict, List, Optional, Union, Any, Type, Tuple
from uuid import UUID
from datetime import datetime

from sqlalchemy.orm import Session as DbSession
from sqlalchemy import and_, or_, func
from pydantic import BaseModel
from browserfleet.server.database.db import get_db
from browserfleet.server.workers.enums import WorkerType, ProviderType, WorkerStatus
from browserfleet.server.workers.models import Worker
from browserfleet.server.workers.schema import WorkerConfig, WorkerStats


logger = logging.getLogger(__name__)


class Worker(ABC):
    """Abstract base class for all browserfleet workers"""
    
    def __init__(self, worker_id: UUID, config: WorkerConfig):
        self.worker_id = worker_id
        self.config = config
        self.status = WorkerStatus.OFFLINE
        self.stats = WorkerStats()
    
    @abstractmethod
    async def start(self) -> bool:
        """Start the worker"""
        pass
    
    @abstractmethod
    async def stop(self) -> bool:
        """Stop the worker"""
        pass
    
    @abstractmethod
    async def launch_container(self, session_id: UUID, image: str, environment: Dict[str, str], 
                        resources: Dict[str, Any]) -> Dict[str, Any]:
        """Launch a container for a browser session"""
        pass
    
    @abstractmethod
    async def terminate_container(self, container_id: str) -> bool:
        """Terminate a running container"""
        pass
    
    @abstractmethod
    async def get_container_status(self, container_id: str) -> Dict[str, Any]:
        """Get the status of a container"""
        pass
    
    @abstractmethod
    async def get_container_logs(self, container_id: str, lines: int = 100) -> str:
        """Get logs from a container"""
        pass
    
    @abstractmethod
    async def get_worker_stats(self) -> WorkerStats:
        """Get current worker statistics"""
        pass
    
    @abstractmethod
    async def health_check(self) -> bool:
        """Check if the worker is healthy"""
        pass

class WorkerManager(ABC):
    """Interface for worker management implementations"""
    
    @abstractmethod
    async def register_worker(self, config: WorkerConfig) -> UUID:
        """Register a new worker"""
        pass
    
    @abstractmethod
    async def unregister_worker(self, worker_id: UUID) -> bool:
        """Unregister a worker"""
        pass
    
    @abstractmethod
    async def get_worker(self, worker_id: UUID) -> Worker:
        """Get a worker by ID"""
        pass
    
    @abstractmethod
    async def list_workers(self, filters: Optional[Dict[str, Any]] = None) -> List[Worker]:
        """List all workers, optionally filtered"""
        pass
    
    @abstractmethod
    async def update_worker_config(self, worker_id: UUID, config: WorkerConfig) -> bool:
        """Update a worker's configuration"""
        pass
    
    @abstractmethod
    async def start_worker(self, worker_id: UUID) -> bool:
        """Start a worker"""
        pass
    
    @abstractmethod
    async def stop_worker(self, worker_id: UUID) -> bool:
        """Stop a worker"""
        pass

