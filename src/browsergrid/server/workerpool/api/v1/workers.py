"""
API endpoints for worker management
"""
from typing import List, Optional
from uuid import UUID

from fastapi import APIRouter, Depends, HTTPException, status, BackgroundTasks, Query
from sqlalchemy.orm import Session as SQLAlchemySession

from browsergrid.server.core.db.session import get_db
from browsergrid.server.workerpool.schemas import (
    Worker, WorkerCreate, WorkerHeartbeat, WorkerStatus, WorkerWithSessions
)


router = APIRouter()

# Worker Management Routes
@router.post("/workers", response_model=Worker, status_code=status.HTTP_201_CREATED)
async def create_worker(
    worker: WorkerCreate,
    db: SQLAlchemySession = Depends(get_db)
):
    """Create a new worker"""
    from browsergrid.server.workerpool.models import Worker as WorkerModel
    from browsergrid.server.workerpool.models import WorkPool as WorkPoolModel
    import secrets
    
    # Check if work pool exists
    work_pool = db.query(WorkPoolModel).filter(WorkPoolModel.id == worker.work_pool_id).first()
    if not work_pool:
        raise HTTPException(
            status_code=status.HTTP_404_NOT_FOUND,
            detail=f"Work pool with ID {worker.work_pool_id} not found"
        )
    
    # Check for worker name uniqueness within the work pool
    existing_worker = db.query(WorkerModel).filter(
        WorkerModel.name == worker.name,
        WorkerModel.work_pool_id == worker.work_pool_id
    ).first()
    
    if existing_worker:
        raise HTTPException(
            status_code=status.HTTP_409_CONFLICT,
            detail=f"Worker with name '{worker.name}' already exists in this work pool"
        )
    
    # Generate API key if not provided
    api_key = worker.api_key or secrets.token_urlsafe(32)
    
    # Create new worker
    db_worker = WorkerModel(
        id=worker.id,
        name=worker.name,
        work_pool_id=worker.work_pool_id,
        status=worker.status,
        capacity=worker.capacity,
        provider_type=worker.provider_type,
        provider_details=worker.provider_details,
        api_key=api_key,
        current_load=0
    )
    
    db.add(db_worker)
    db.commit()
    db.refresh(db_worker)
    
    return db_worker

@router.get("/workers", response_model=List[Worker])
async def list_workers(
    work_pool_id: Optional[UUID] = None,
    status: Optional[WorkerStatus] = None,
    offset: int = 0,
    limit: int = 100,
    db: SQLAlchemySession = Depends(get_db)
):
    """List workers with optional filters"""
    from browsergrid.server.workerpool.models import Worker as WorkerModel
    
    query = db.query(WorkerModel)
    
    if work_pool_id:
        query = query.filter(WorkerModel.work_pool_id == work_pool_id)
    
    if status:
        query = query.filter(WorkerModel.status == status)
    
    return query.offset(offset).limit(limit).all()

@router.get("/workers/{worker_id}", response_model=WorkerWithSessions)
async def get_worker(
    worker_id: UUID,
    db: SQLAlchemySession = Depends(get_db)
):
    """Get a specific worker"""
    from browsergrid.server.workerpool.models import Worker as WorkerModel
    from browsergrid.server.sessions.models import Session as SessionModel
    
    db_worker = db.query(WorkerModel).filter(WorkerModel.id == worker_id).first()
    if not db_worker:
        raise HTTPException(
            status_code=status.HTTP_404_NOT_FOUND,
            detail=f"Worker with ID {worker_id} not found"
        )
    
    # Get session count
    session_count = db.query(SessionModel).filter(SessionModel.worker_id == worker_id).count()
    
    result = Worker.from_orm(db_worker)
    result_dict = result.dict()
    result_dict["session_count"] = session_count
    
    return WorkerWithSessions(**result_dict)

@router.put("/workers/{worker_id}/heartbeat", response_model=Worker)
async def update_worker_heartbeat(
    worker_id: UUID,
    heartbeat: WorkerHeartbeat,
    db: SQLAlchemySession = Depends(get_db)
):
    """Update worker heartbeat and status"""
    from browsergrid.server.workerpool.models import Worker as WorkerModel
    from datetime import datetime
    
    db_worker = db.query(WorkerModel).filter(WorkerModel.id == worker_id).first()
    if not db_worker:
        raise HTTPException(
            status_code=status.HTTP_404_NOT_FOUND,
            detail=f"Worker with ID {worker_id} not found"
        )
    
    # Update worker fields
    db_worker.status = heartbeat.status
    db_worker.current_load = heartbeat.current_load
    db_worker.cpu_percent = heartbeat.cpu_percent
    db_worker.memory_usage_mb = heartbeat.memory_usage_mb
    db_worker.disk_usage_mb = heartbeat.disk_usage_mb
    db_worker.ip_address = heartbeat.ip_address
    db_worker.last_heartbeat = datetime.now()
    
    db.commit()
    db.refresh(db_worker)
    
    return db_worker

@router.delete("/workers/{worker_id}", status_code=status.HTTP_204_NO_CONTENT)
async def delete_worker(
    worker_id: UUID,
    force: bool = False,
    db: SQLAlchemySession = Depends(get_db)
):
    """Delete a worker"""
    from browsergrid.server.workerpool.models import Worker as WorkerModel
    from browsergrid.server.sessions.models import Session as SessionModel
    
    db_worker = db.query(WorkerModel).filter(WorkerModel.id == worker_id).first()
    if not db_worker:
        raise HTTPException(
            status_code=status.HTTP_404_NOT_FOUND,
            detail=f"Worker with ID {worker_id} not found"
        )
    
    # Check if there are active sessions using this worker
    if not force:
        active_sessions = db.query(SessionModel).filter(
            SessionModel.worker_id == worker_id,
            SessionModel.status.in_(["pending", "running"])
        ).count()
        
        if active_sessions > 0:
            raise HTTPException(
                status_code=status.HTTP_400_BAD_REQUEST,
                detail=f"Cannot delete worker with {active_sessions} active sessions. Use force=true to force deletion."
            )
    
    db.delete(db_worker)
    db.commit()
    
    return None

# Worker API for claiming sessions and reporting status
@router.post("/workers/{worker_id}/claim-session", response_model=dict)
async def claim_pending_session(
    worker_id: UUID,
    db: SQLAlchemySession = Depends(get_db)
):
    """Claim a pending session for a worker"""
    from browsergrid.server.workerpool.models import Worker as WorkerModel
    from browsergrid.server.sessions.models import Session as SessionModel
    from browsergrid.server.sessions.enums import SessionStatus
    
    # Verify worker exists and is online
    db_worker = db.query(WorkerModel).filter(
        WorkerModel.id == worker_id,
        WorkerModel.status.in_([WorkerStatus.ONLINE, WorkerStatus.BUSY])
    ).first()
    
    if not db_worker:
        raise HTTPException(
            status_code=status.HTTP_404_NOT_FOUND,
            detail=f"Worker with ID {worker_id} not found or not in active state"
        )
    
    # Check if worker has capacity
    if db_worker.current_load >= db_worker.capacity:
        return {"session_claimed": False, "reason": "Worker at capacity"}
    
    # Get work pool
    work_pool_id = db_worker.work_pool_id
    
    # Find a pending session assigned to this pool
    pending_session = db.query(SessionModel).filter(
        SessionModel.work_pool_id == work_pool_id,
        SessionModel.status == SessionStatus.PENDING,
        SessionModel.worker_id == None
    ).first()
    
    if not pending_session:
        return {"session_claimed": False, "reason": "No pending sessions"}
    
    # Claim the session
    pending_session.worker_id = worker_id
    db_worker.current_load += 1
    
    db.commit()
    
    return {
        "session_claimed": True,
        "session_id": str(pending_session.id),
        "session_details": {
            "browser": pending_session.browser.value,
            "version": pending_session.version.value,
            "headless": pending_session.headless,
            "operating_system": pending_session.operating_system.value,
            "screen": pending_session.screen,
            "proxy": pending_session.proxy,
            "resource_limits": pending_session.resource_limits,
            "environment": pending_session.environment
        }
    }
