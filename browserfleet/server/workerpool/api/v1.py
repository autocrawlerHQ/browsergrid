# browserfleet/server/api/v1/endpoints/workpools.py
"""
API endpoints for work pool management
"""
from typing import List, Dict, Optional, Any
from uuid import UUID

from fastapi import APIRouter, Depends, HTTPException, BackgroundTasks, status
from sqlalchemy.orm import Session

from browserfleet.server.db import get_db
from browserfleet.server.workers.enums import WorkerType, ProviderType
from browserfleet.server.workerpool.models import WorkPoolConfig, WorkAllocationStrategy
from browserfleet.server.workerpool.manager import WorkPoolManager

# Manager singleton
work_pool_manager = WorkPoolManager()

# Start the manager's background tasks
work_pool_manager.start()

router = APIRouter()

@router.get("/", response_model=List[Dict[str, Any]])
async def list_work_pools():
    """
    List all work pools.
    
    Returns information about all configured work pools including their status,
    worker count, and utilization.
    """
    return await work_pool_manager.list_pools()

@router.post("/", response_model=Dict[str, Any], status_code=status.HTTP_201_CREATED)
async def create_work_pool(config: WorkPoolConfig):
    """
    Create a new work pool.
    
    Creates a new work pool with the specified configuration and provisions
    the initial set of workers.
    """
    try:
        pool_id = await work_pool_manager.create_pool(config)
        return {"id": str(pool_id), "message": f"Work pool '{config.name}' created successfully"}
    except Exception as e:
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST,
            detail=f"Failed to create work pool: {str(e)}"
        )

@router.get("/{pool_id}", response_model=Dict[str, Any])
async def get_work_pool(pool_id: UUID):
    """
    Get details for a specific work pool.
    
    Returns comprehensive information about a work pool including its configuration,
    status, and all workers.
    """
    pool = await work_pool_manager.get_pool(pool_id)
    if not pool:
        raise HTTPException(
            status_code=status.HTTP_404_NOT_FOUND,
            detail=f"Work pool {pool_id} not found"
        )
    
    # Update stats before returning
    await pool.update_stats()
    
    # Prepare worker details
    workers = []
    for worker_id, worker in pool.workers.items():
        workers.append({
            "id": str(worker_id),
            "status": str(worker.status),
            "running_containers": worker.stats.running_containers,
            "cpu_percent": worker.stats.cpu_percent,
            "memory_percent": worker.stats.memory_percent
        })
    
    return {
        "id": str(pool_id),
        "name": pool.config.name,
        "description": pool.config.description,
        "worker_type": str(pool.config.worker_type),
        "provider_type": str(pool.config.provider_type),
        "status": str(pool.status),
        "concurrency_limit": pool.config.concurrency_limit,
        "allocation_strategy": str(pool.config.allocation_strategy),
        "auto_scaling_enabled": pool.config.auto_scaling_enabled,
        "min_workers": pool.config.min_workers,
        "max_workers": pool.config.max_workers,
        "worker_count": pool.stats.worker_count,
        "online_workers": pool.stats.online_workers,
        "busy_workers": pool.stats.busy_workers,
        "error_workers": pool.stats.error_workers,
        "total_running_containers": pool.stats.total_running_containers,
        "utilization_percent": pool.stats.utilization_percent,
        "workers": workers
    }

@router.patch("/{pool_id}", response_model=Dict[str, Any])
async def update_work_pool(pool_id: UUID, config: WorkPoolConfig):
    """
    Update a work pool's configuration.
    
    Modifies the configuration of an existing work pool. Some changes may require
    workers to be restarted or may trigger scaling operations.
    """
    success = await work_pool_manager.update_pool(pool_id, config)
    if not success:
        raise HTTPException(
            status_code=status.HTTP_404_NOT_FOUND,
            detail=f"Work pool {pool_id} not found"
        )
    
    return {"message": f"Work pool {pool_id} updated successfully"}

@router.delete("/{pool_id}", status_code=status.HTTP_204_NO_CONTENT)
async def delete_work_pool(pool_id: UUID):
    """
    Delete a work pool.
    
    Terminates all workers in the pool and removes the pool configuration.
    """
    success = await work_pool_manager.delete_pool(pool_id)
    if not success:
        raise HTTPException(
            status_code=status.HTTP_404_NOT_FOUND,
            detail=f"Work pool {pool_id} not found"
        )
    
    return None

@router.post("/{pool_id}/workers", response_model=Dict[str, Any])
async def add_worker_to_pool(pool_id: UUID):
    """
    Add a new worker to a work pool.
    
    Creates and starts a new worker in the specified pool using the
    pool's default worker configuration.
    """
    pool = await work_pool_manager.get_pool(pool_id)
    if not pool:
        raise HTTPException(
            status_code=status.HTTP_404_NOT_FOUND,
            detail=f"Work pool {pool_id} not found"
        )
    
    try:
        worker_id, _ = await pool.create_worker()
        return {"worker_id": str(worker_id), "message": "Worker added successfully"}
    except Exception as e:
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST,
            detail=f"Failed to add worker: {str(e)}"
        )

@router.delete("/{pool_id}/workers/{worker_id}", status_code=status.HTTP_204_NO_CONTENT)
async def remove_worker_from_pool(pool_id: UUID, worker_id: UUID):
    """
    Remove a worker from a work pool.
    
    Stops and removes the specified worker from the pool. Any running
    containers will be terminated.
    """
    pool = await work_pool_manager.get_pool(pool_id)
    if not pool:
        raise HTTPException(
            status_code=status.HTTP_404_NOT_FOUND,
            detail=f"Work pool {pool_id} not found"
        )
    
    success = await pool.remove_worker(worker_id)
    if not success:
        raise HTTPException(
            status_code=status.HTTP_404_NOT_FOUND,
            detail=f"Worker {worker_id} not found in pool {pool_id}"
        )
    
    return None

@router.get("/worker-types", response_model=Dict[str, List[str]])
async def get_worker_types():
    """
    Get available worker types and their supported providers.
    
    Returns a mapping of worker types to the infrastructure providers
    that support them.
    """
    return await work_pool_manager.get_worker_types()

@router.post("/{pool_id}/pause", response_model=Dict[str, Any])
async def pause_work_pool(pool_id: UUID):
    """
    Pause a work pool.
    
    Prevents the pool from accepting new work but allows current
    tasks to complete.
    """
    pool = await work_pool_manager.get_pool(pool_id)
    if not pool:
        raise HTTPException(
            status_code=status.HTTP_404_NOT_FOUND,
            detail=f"Work pool {pool_id} not found"
        )
    
    pool.status = "paused"
    return {"message": f"Work pool {pool_id} paused"}

@router.post("/{pool_id}/resume", response_model=Dict[str, Any])
async def resume_work_pool(pool_id: UUID):
    """
    Resume a paused work pool.
    
    Allows the pool to accept new work again.
    """
    pool = await work_pool_manager.get_pool(pool_id)
    if not pool:
        raise HTTPException(
            status_code=status.HTTP_404_NOT_FOUND,
            detail=f"Work pool {pool_id} not found"
        )
    
    pool.status = "active"
    return {"message": f"Work pool {pool_id} resumed"}

@router.post("/{pool_id}/scale", response_model=Dict[str, Any])
async def scale_work_pool(pool_id: UUID, worker_count: int):
    """
    Scale a work pool to a specific number of workers.
    
    Adds or removes workers to reach the specified count.
    """
    pool = await work_pool_manager.get_pool(pool_id)
    if not pool:
        raise HTTPException(
            status_code=status.HTTP_404_NOT_FOUND,
            detail=f"Work pool {pool_id} not found"
        )
    
    current_count = len(pool.workers)
    
    if worker_count < 1:
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST,
            detail="Worker count must be at least 1"
        )
    
    if worker_count > pool.config.max_workers:
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST,
            detail=f"Worker count exceeds maximum of {pool.config.max_workers}"
        )
    
    # Scale up or down as needed
    if worker_count > current_count:
        # Add workers
        for _ in range(worker_count - current_count):
            try:
                await pool.create_worker()
            except Exception as e:
                return {
                    "message": f"Partially scaled to {len(pool.workers)} workers",
                    "error": str(e)
                }
    elif worker_count < current_count:
        # Remove workers
        workers_to_remove = current_count - worker_count
        
        # Remove the least busy workers
        workers_by_load = sorted(
            [(worker_id, worker.stats.running_containers) for worker_id, worker in pool.workers.items()],
            key=lambda x: x[1]
        )
        
        for worker_id, _ in workers_by_load[:workers_to_remove]:
            try:
                await pool.remove_worker(worker_id)
            except Exception as e:
                return {
                    "message": f"Partially scaled to {len(pool.workers)} workers",
                    "error": str(e)
                }
    
    return {
        "message": f"Work pool scaled to {len(pool.workers)} workers",
        "previous_count": current_count,
        "current_count": len(pool.workers)
    }