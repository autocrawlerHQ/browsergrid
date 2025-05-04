"""
API routes for WorkPool and Worker management.
"""
from typing import List, Optional
from uuid import UUID

from fastapi import APIRouter, Depends, HTTPException, status, BackgroundTasks, Query
from sqlalchemy.orm import Session as SQLAlchemySession

from browsergrid.server.core.db.session import get_db
from browsergrid.server.workerpool.schemas import (
    WorkPool, WorkPoolCreate, WorkPoolWithRelations,
)
from browsergrid.server.workerpool.enums import WorkPoolStatus, ProviderType

# Create router
router = APIRouter()

# WorkPool Management Routes
@router.post("/pools", response_model=WorkPool, status_code=status.HTTP_201_CREATED)
async def create_work_pool(
    work_pool: WorkPoolCreate,
    db: SQLAlchemySession = Depends(get_db)
):
    """Create a new work pool"""
    from browsergrid.server.workerpool.models import WorkPool as WorkPoolModel
    
    # Check if a pool with the same name already exists
    existing_pool = db.query(WorkPoolModel).filter(WorkPoolModel.name == work_pool.name).first()
    if existing_pool:
        raise HTTPException(
            status_code=status.HTTP_409_CONFLICT,
            detail=f"Work pool with name '{work_pool.name}' already exists"
        )
    
    # Create new work pool
    db_work_pool = WorkPoolModel(
        id=work_pool.id,
        name=work_pool.name,
        provider_type=work_pool.provider_type,
        status=work_pool.status,
        default_browser=work_pool.default_browser,
        default_browser_version=work_pool.default_browser_version,
        default_headless=work_pool.default_headless,
        default_operating_system=work_pool.default_operating_system,
        default_screen=work_pool.default_screen.dict() if work_pool.default_screen else None,
        default_proxy=work_pool.default_proxy.dict() if work_pool.default_proxy else None,
        default_resource_limits=work_pool.default_resource_limits.dict() if work_pool.default_resource_limits else None,
        default_environment=work_pool.default_environment,
        min_workers=work_pool.min_workers,
        max_workers=work_pool.max_workers,
        max_sessions_per_worker=work_pool.max_sessions_per_worker,
        provider_config=work_pool.provider_config,
        description=work_pool.description
    )
    
    db.add(db_work_pool)
    db.commit()
    db.refresh(db_work_pool)
    
    return db_work_pool

@router.get("/pools", response_model=List[WorkPool])
async def list_work_pools(
    status: Optional[WorkPoolStatus] = None,
    provider_type: Optional[ProviderType] = None,
    offset: int = 0,
    limit: int = 100,
    db: SQLAlchemySession = Depends(get_db)
):
    """List work pools with optional filters"""
    from browsergrid.server.workerpool.models import WorkPool as WorkPoolModel
    
    query = db.query(WorkPoolModel)
    
    if status:
        query = query.filter(WorkPoolModel.status == status)
    
    if provider_type:
        query = query.filter(WorkPoolModel.provider_type == provider_type)
    
    return query.offset(offset).limit(limit).all()

@router.get("/pools/{pool_id}", response_model=WorkPoolWithRelations)
async def get_work_pool(
    pool_id: UUID,
    db: SQLAlchemySession = Depends(get_db)
):
    """Get a specific work pool with its workers"""
    from browsergrid.server.workerpool.models import WorkPool as WorkPoolModel
    
    db_work_pool = db.query(WorkPoolModel).filter(WorkPoolModel.id == pool_id).first()
    if not db_work_pool:
        raise HTTPException(
            status_code=status.HTTP_404_NOT_FOUND,
            detail=f"Work pool with ID {pool_id} not found"
        )
    
    return db_work_pool

@router.put("/pools/{pool_id}", response_model=WorkPool)
async def update_work_pool(
    pool_id: UUID,
    work_pool_update: WorkPoolCreate,
    db: SQLAlchemySession = Depends(get_db)
):
    """Update a work pool"""
    from browsergrid.server.workerpool.models import WorkPool as WorkPoolModel
    
    db_work_pool = db.query(WorkPoolModel).filter(WorkPoolModel.id == pool_id).first()
    if not db_work_pool:
        raise HTTPException(
            status_code=status.HTTP_404_NOT_FOUND,
            detail=f"Work pool with ID {pool_id} not found"
        )
    
    # Check if trying to change name to an existing name
    if work_pool_update.name != db_work_pool.name:
        existing = db.query(WorkPoolModel).filter(WorkPoolModel.name == work_pool_update.name).first()
        if existing:
            raise HTTPException(
                status_code=status.HTTP_409_CONFLICT,
                detail=f"Work pool with name '{work_pool_update.name}' already exists"
            )
    
    # Update fields
    update_data = work_pool_update.dict(exclude={"id"}, exclude_unset=True)
    
    # Handle nested objects
    if "default_screen" in update_data and update_data["default_screen"]:
        update_data["default_screen"] = update_data["default_screen"].dict()
    if "default_proxy" in update_data and update_data["default_proxy"]:
        update_data["default_proxy"] = update_data["default_proxy"].dict()
    if "default_resource_limits" in update_data and update_data["default_resource_limits"]:
        update_data["default_resource_limits"] = update_data["default_resource_limits"].dict()
    
    for key, value in update_data.items():
        setattr(db_work_pool, key, value)
    
    db.commit()
    db.refresh(db_work_pool)
    
    return db_work_pool

@router.delete("/pools/{pool_id}", status_code=status.HTTP_204_NO_CONTENT)
async def delete_work_pool(
    pool_id: UUID,
    force: bool = False,
    db: SQLAlchemySession = Depends(get_db)
):
    """Delete a work pool"""
    from browsergrid.server.workerpool.models import WorkPool as WorkPoolModel
    from browsergrid.server.sessions.models import Session as SessionModel
    
    db_work_pool = db.query(WorkPoolModel).filter(WorkPoolModel.id == pool_id).first()
    if not db_work_pool:
        raise HTTPException(
            status_code=status.HTTP_404_NOT_FOUND,
            detail=f"Work pool with ID {pool_id} not found"
        )
    
    # Check if there are active sessions using this pool
    if not force:
        active_sessions = db.query(SessionModel).filter(
            SessionModel.work_pool_id == pool_id,
            SessionModel.status.in_(["pending", "running"])
        ).count()
        
        if active_sessions > 0:
            raise HTTPException(
                status_code=status.HTTP_400_BAD_REQUEST,
                detail=f"Cannot delete work pool with {active_sessions} active sessions. Use force=true to force deletion."
            )
    
    db.delete(db_work_pool)
    db.commit()
    
    return None

