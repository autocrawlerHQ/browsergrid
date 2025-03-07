import uuid
import logging
from typing import Dict, Any, Optional, Tuple, Type
from uuid import UUID
from datetime import datetime

from browserfleet.server.workerpool.enums import WorkAllocationStrategy, WorkPoolStatus
from browserfleet.server.workerpool.schemas import WorkPoolConfig, WorkPoolStats


from browserfleet.server.workers.enums import WorkerStatus
from browserfleet.server.workers.schema import WorkerConfig
from browserfleet.server.workers.manager import Worker

# todo: integrate with db
logger = logging.getLogger(__name__)

class WorkPool:
    """
    Represents a pool of workers that can execute browser sessions
    """
    def __init__(self, pool_id: UUID, config: WorkPoolConfig):
        self.pool_id = pool_id
        self.config = config
        self.status = WorkPoolStatus.ACTIVE
        self.workers: Dict[UUID, Worker] = {}
        self.stats = WorkPoolStats()
        self.last_scale_time = datetime.utcnow()
        
        # Worker type implementation mapping
        # In a real system, this would use a plugin system or factory pattern
        self._worker_types: Dict[str, Type[Worker]] = {}
    
    def register_worker_type(self, worker_type: str, worker_class: Type[Worker]):
        """Register a worker type implementation"""
        self._worker_types[worker_type] = worker_class
    
    async def create_worker(self, worker_config: Optional[WorkerConfig] = None) -> Tuple[UUID, Worker]:
        """
        Create and register a new worker in this pool
        
        Args:
            worker_config: Optional custom config for this worker.
                           If not provided, use the pool default.
        
        Returns:
            Tuple of (worker_id, worker_instance)
        """
        # Use provided config or pool default
        config = worker_config or self.config.default_worker_config
        
        # Generate a worker ID
        worker_id = uuid.uuid4()
        
        # Get the appropriate worker class
        worker_class = self._worker_types.get(
            str(config.worker_type),
            None
        )
        
        if not worker_class:
            logger.error(f"No implementation found for worker type: {config.worker_type}")
            raise ValueError(f"Unsupported worker type: {config.worker_type}")
        
        # Create the worker
        worker = worker_class(worker_id=worker_id, config=config)
        
        # Register it in our pool
        self.workers[worker_id] = worker
        
        # Start the worker
        success = await worker.start()
        if not success:
            logger.error(f"Failed to start worker {worker_id}")
            self.workers.pop(worker_id, None)
            raise RuntimeError(f"Failed to start worker {worker_id}")
        
        # Update pool stats
        await self.update_stats()
        
        return worker_id, worker
    
    async def remove_worker(self, worker_id: UUID) -> bool:
        """
        Remove a worker from the pool
        
        Args:
            worker_id: ID of the worker to remove
        
        Returns:
            True if the worker was removed, False otherwise
        """
        worker = self.workers.get(worker_id)
        if not worker:
            logger.warning(f"Worker {worker_id} not found in pool {self.pool_id}")
            return False
        
        # Try to stop the worker gracefully
        try:
            await worker.stop()
        except Exception as e:
            logger.error(f"Error stopping worker {worker_id}: {e}")
        
        # Remove from our registry
        self.workers.pop(worker_id, None)
        
        # Update pool stats
        await self.update_stats()
        
        return True
    
    async def select_worker_for_session(self, session_requirements: Optional[Dict[str, Any]] = None) -> Optional[Worker]:
        """
        Select a worker for a new session based on the allocation strategy
        
        Args:
            session_requirements: Optional requirements for the session
                                 that may influence worker selection
        
        Returns:
            Selected worker or None if no suitable worker is available
        """
        # Filter out workers that are not online
        available_workers = [
            w for w in self.workers.values() 
            if w.status == WorkerStatus.ONLINE
        ]
        
        if not available_workers:
            logger.warning(f"No available workers in pool {self.pool_id}")
            return None
        
        # Apply allocation strategy
        strategy = self.config.allocation_strategy
        
        if strategy == WorkAllocationStrategy.ROUND_ROBIN:
            # Simple round-robin
            return available_workers[0]  # In a real impl, we'd track the last used index
        
        elif strategy == WorkAllocationStrategy.LEAST_BUSY:
            # Sort workers by number of running containers
            return sorted(
                available_workers, 
                key=lambda w: w.stats.running_containers
            )[0]
        
        elif strategy == WorkAllocationStrategy.RANDOM:
            # Random worker
            import random
            return random.choice(available_workers)
        
        elif strategy == WorkAllocationStrategy.LEAST_RECENTLY_USED:
            # Would need to track last used time for each worker
            # For this mock implementation, just use the first one
            return available_workers[0]
        
        # Default fallback
        return available_workers[0]
    
    async def scale_workers(self) -> Tuple[int, int]:
        """
        Scale the number of workers based on current utilization
        
        Returns:
            Tuple of (added_workers, removed_workers)
        """
        if not self.config.auto_scaling_enabled:
            return 0, 0
        
        # Check if we need to cool down after scaling
        now = datetime.utcnow()
        since_last_scale = (now - self.last_scale_time).total_seconds() / 60.0
        
        if since_last_scale < self.config.scale_down_delay_minutes:
            logger.info(f"Cooling down after recent scaling, {since_last_scale:.1f} minutes elapsed")
            return 0, 0
        
        # Get current worker counts
        total_workers = len(self.workers)
        online_workers = sum(1 for w in self.workers.values() if w.status == WorkerStatus.ONLINE)
        
        if online_workers == 0:
            # No workers online, add one
            if total_workers < self.config.max_workers:
                try:
                    await self.create_worker()
                    self.last_scale_time = now
                    return 1, 0
                except Exception as e:
                    logger.error(f"Failed to scale up: {e}")
                    return 0, 0
            return 0, 0
        
        # Calculate pool utilization
        total_capacity = sum(w.config.concurrency_limit for w in self.workers.values() 
                          if w.status == WorkerStatus.ONLINE)
        
        total_running = sum(w.stats.running_containers for w in self.workers.values())
        
        if total_capacity > 0:
            utilization = total_running / total_capacity
        else:
            utilization = 1.0  # Force scale up if we somehow have no capacity
        
        self.stats.utilization_percent = utilization * 100.0
        
        added, removed = 0, 0
        
        # Scale up logic
        if utilization >= self.config.scale_up_threshold:
            if total_workers < self.config.max_workers:
                try:
                    worker_id, _ = await self.create_worker()
                    logger.info(f"Scaled up by adding worker {worker_id}, utilization: {utilization:.1%}")
                    added = 1
                    self.last_scale_time = now
                except Exception as e:
                    logger.error(f"Failed to scale up: {e}")
        
        # Scale down logic
        elif (utilization <= self.config.scale_down_threshold and 
              total_workers > self.config.min_workers and
              since_last_scale >= self.config.scale_down_delay_minutes):
            
            # Find least busy worker to remove
            workers_by_load = sorted(
                [w for w in self.workers.values() if w.status == WorkerStatus.ONLINE],
                key=lambda w: w.stats.running_containers
            )
            
            if workers_by_load and workers_by_load[0].stats.running_containers == 0:
                worker_to_remove = workers_by_load[0]
                try:
                    await self.remove_worker(worker_to_remove.worker_id)
                    logger.info(f"Scaled down by removing worker {worker_to_remove.worker_id}, utilization: {utilization:.1%}")
                    removed = 1
                    self.last_scale_time = now
                except Exception as e:
                    logger.error(f"Failed to scale down: {e}")
        
        return added, removed
    
    async def update_stats(self) -> WorkPoolStats:
        """
        Update and return the pool statistics
        
        Returns:
            Updated pool statistics
        """
        worker_count = len(self.workers)
        online_workers = sum(1 for w in self.workers.values() if w.status == WorkerStatus.ONLINE)
        busy_workers = sum(1 for w in self.workers.values() 
                        if w.status == WorkerStatus.BUSY)
        error_workers = sum(1 for w in self.workers.values() 
                         if w.status == WorkerStatus.ERROR)
        
        # Get stats from all workers
        worker_stats = []
        running_containers = 0
        
        for worker in self.workers.values():
            try:
                stats = await worker.get_worker_stats()
                worker_stats.append(stats)
                running_containers += stats.running_containers
            except Exception as e:
                logger.error(f"Failed to get stats from worker {worker.worker_id}: {e}")
        
        # Calculate averages
        if worker_stats:
            cpu_values = [s.cpu_percent for s in worker_stats if s.cpu_percent is not None]
            memory_values = [s.memory_percent for s in worker_stats if s.memory_percent is not None]
            
            avg_cpu = sum(cpu_values) / len(cpu_values) if cpu_values else None
            avg_memory = sum(memory_values) / len(memory_values) if memory_values else None
        else:
            avg_cpu = None
            avg_memory = None
        
        # Calculate utilization
        total_capacity = sum(w.config.concurrency_limit for w in self.workers.values() 
                          if w.status == WorkerStatus.ONLINE)
        
        if total_capacity > 0:
            utilization = running_containers / total_capacity
        else:
            utilization = 0.0
        
        # Update stats
        self.stats = WorkPoolStats(
            worker_count=worker_count,
            online_workers=online_workers,
            busy_workers=busy_workers,
            error_workers=error_workers,
            total_running_containers=running_containers,
            average_cpu_percent=avg_cpu,
            average_memory_percent=avg_memory,
            utilization_percent=utilization * 100.0,
            last_updated=datetime.utcnow().isoformat() + "Z"
        )
        
        return self.stats
    
    async def health_check(self) -> bool:
        """
        Check the health of the pool and its workers
        
        Returns:
            True if the pool is healthy, False otherwise
        """
        # Check all workers
        for worker_id, worker in list(self.workers.items()):
            try:
                is_healthy = await worker.health_check()
                if not is_healthy and worker.status == WorkerStatus.ERROR:
                    logger.warning(f"Worker {worker_id} is unhealthy")
            except Exception as e:
                logger.error(f"Health check failed for worker {worker_id}: {e}")
                worker.status = WorkerStatus.ERROR
        
        # Update pool health status
        error_count = sum(1 for w in self.workers.values() if w.status == WorkerStatus.ERROR)
        total_workers = len(self.workers)
        
        if total_workers == 0:
            # No workers, but not necessarily an error
            self.status = WorkPoolStatus.ACTIVE
        elif error_count == total_workers:
            # All workers are in error state
            self.status = WorkPoolStatus.ERROR
        else:
            # Some workers are healthy
            self.status = WorkPoolStatus.ACTIVE
        
        # Update pool stats
        await self.update_stats()
        
        return self.status != WorkPoolStatus.ERROR