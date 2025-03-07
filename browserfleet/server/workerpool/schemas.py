from pydantic import BaseModel, Field
from typing import Dict, Any, Optional


from browserfleet.server.workerpool.enums import WorkAllocationStrategy, WorkPoolStatus

from browserfleet.server.workers.enums import WorkerType, ProviderType
from browserfleet.server.workers.schema import WorkerConfig

class WorkPoolConfig(BaseModel):
    """Configuration for a work pool"""
    name: str
    description: Optional[str] = None
    worker_type: WorkerType
    provider_type: ProviderType
    default_worker_config: WorkerConfig
    concurrency_limit: int = 10
    allocation_strategy: WorkAllocationStrategy = WorkAllocationStrategy.LEAST_BUSY
    auto_scaling_enabled: bool = False
    min_workers: int = 1
    max_workers: int = 5
    scale_up_threshold: float = 0.8  # Scale up when pool utilization exceeds 80%
    scale_down_threshold: float = 0.3  # Scale down when pool utilization is below 30%
    scale_down_delay_minutes: int = 10  # Wait before scaling down to avoid oscillation
    labels: Dict[str, str] = Field(default_factory=dict)
    
    # Custom settings specific to the worker type and provider
    settings: Dict[str, Any] = Field(default_factory=dict)
    
    class Config:
        extra = "allow"  # Allow extra fields for provider-specific configs

class WorkPoolStats(BaseModel):
    """Runtime statistics for a work pool"""
    worker_count: int = 0
    online_workers: int = 0
    busy_workers: int = 0
    error_workers: int = 0
    total_running_containers: int = 0
    average_cpu_percent: Optional[float] = None
    average_memory_percent: Optional[float] = None
    utilization_percent: float = 0.0
    last_updated: Optional[str] = None  # ISO timestamp

