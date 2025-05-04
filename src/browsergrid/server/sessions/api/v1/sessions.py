"""
API endpoints for browser session management
"""
from typing import List, Optional
from uuid import UUID

from fastapi import APIRouter, Depends, HTTPException, Query, status, BackgroundTasks
from sqlalchemy.orm import Session as SQLAlchemySession

from browsergrid.server.core.db.session import get_db
from browsergrid.server.sessions.schema import (
    SessionCreate, Session, SessionDetails, SessionWithRelations,
    SessionStatus
)
from browsergrid.server.sessions.models import Session as SessionModel
from browsergrid.server.workerpool.manager import WorkPoolManager

router = APIRouter()

@router.post("/", response_model=SessionDetails, status_code=status.HTTP_201_CREATED)
async def create_session(
    session: SessionCreate,
    background_tasks: BackgroundTasks,
    db: SQLAlchemySession = Depends(get_db)
):
    """
    Create a new browser session.
    
    This endpoint launches a new remote Chrome instance with the specified configuration.
    The session is provisioned asynchronously, and the response includes connection details 
    once the browser is ready.
    
    If work_pool_id is specified, the session will be assigned to that work pool.
    Otherwise, a suitable work pool will be chosen automatically if available.
    """
    # Create DB model from schema
    db_session = SessionModel.create_from_schema(session)
    
    # Add session to database
    db.add(db_session)
    db.commit()
    db.refresh(db_session)
    
    # If work_pool_id is provided, assign session to that pool
    # otherwise, the session will be provisioned using the traditional approach
    if session.work_pool_id:
        WorkPoolManager.assign_session_to_work_pool(db, db_session.id, session.work_pool_id)
    else:
        # Try to find a suitable work pool automatically
        assigned = WorkPoolManager.assign_session_to_work_pool(db, db_session.id)
        
        # If no work pool was assigned, the session will be handled by the default provider
        if not assigned:
            # Here you'd implement the logic to provision the session directly
            # For now, we're just returning the created session
            pass
    
    return db_session

@router.get("/", response_model=List[Session])
async def list_sessions(
    status: Optional[SessionStatus] = None,
    offset: int = 0,
    limit: int = 100,
    db: SQLAlchemySession = Depends(get_db)
):
    """
    List all browser sessions.
    
    Returns a paginated list of browser sessions, optionally filtered by status.
    """
    # Implementation would query and return sessions
    return []

@router.get("/{session_id}", response_model=SessionWithRelations)
async def get_session(
    session_id: UUID,
    include_metrics: bool = False,
    include_events: bool = False,
    db: SQLAlchemySession = Depends(get_db)
):
    """
    Get details for a specific browser session.
    
    Returns comprehensive information about a session, optionally including
    related metrics and events.
    """
    # Implementation would retrieve the session with requested relations
    return {"message": "Session details retrieved"}


@router.delete("/{session_id}", status_code=status.HTTP_204_NO_CONTENT)
async def delete_session(
    session_id: UUID,
    force: bool = False,
    db: SQLAlchemySession = Depends(get_db)
):
    """
    Delete a browser session.
    
    Terminates the remote browser instance and removes all associated resources.
    The 'force' parameter can be used to force termination if graceful shutdown fails.
    """
    # Implementation would terminate and clean up the session
    return None

@router.post("/{session_id}/refresh", response_model=SessionDetails)
async def refresh_session(
    session_id: UUID,
    db: SQLAlchemySession = Depends(get_db)
):
    """
    Refresh a browser session.
    
    Extends the expiration time of the session to prevent automatic termination.
    """
    # Implementation would update the session's expiration time
    return {"message": "Session refreshed successfully"}

