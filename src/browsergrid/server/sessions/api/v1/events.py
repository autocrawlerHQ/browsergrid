"""
API endpoints for session events
"""
from typing import List, Optional
from uuid import UUID
from datetime import datetime, timedelta

from fastapi import APIRouter, Depends, HTTPException, Query, status
from sqlalchemy.orm import Session as SQLAlchemySession
from sqlalchemy import desc

from browsergrid.server.core.db.session import get_db
from browsergrid.server.sessions.schema import (
    SessionEvent, SessionEventCreate, SessionEventBase
)
from browsergrid.server.sessions.models import SessionEvent as SessionEventModel
from browsergrid.server.sessions.models import Session as SessionModel
from browsergrid.server.sessions.enums import SessionEventType, SessionStatus
from browsergrid.server.sessions.utils import get_status_from_event, should_update_status

router = APIRouter()

@router.post("/", response_model=SessionEvent, status_code=status.HTTP_201_CREATED)
async def create_event(
    event: SessionEventCreate,
    db: SQLAlchemySession = Depends(get_db)
):
    """
    Record a new session event.
    
    This endpoint allows recording various events that occur during a browser session lifecycle,
    using standardized event types defined in the SessionEventType enum.
    
    The session status will be automatically updated when appropriate based on the event type.
    """
    # Verify session exists
    session = db.query(SessionModel).filter(SessionModel.id == event.session_id).first()
    if not session:
        raise HTTPException(
            status_code=status.HTTP_404_NOT_FOUND,
            detail=f"Session with id {event.session_id} not found"
        )
    
    # Get event type as enum if it's a string
    event_type = event.event
    if isinstance(event_type, str):
        try:
            event_type = SessionEventType(event_type)
        except ValueError:
            # If it's not a valid enum value, keep it as a string
            pass
    
    # Get string value of event for database
    event_str = event_type.value if isinstance(event_type, SessionEventType) else event_type
    
    # Create DB model from schema
    db_event = SessionEventModel(
        session_id=event.session_id,
        event=event_str,
        data=event.data
    )
    
    # Add event to database
    db.add(db_event)
    
    # Check if this event should update the session status
    if isinstance(event_type, SessionEventType):
        new_status = get_status_from_event(event_type)
        if new_status and should_update_status(session.status, new_status):
            # Update the session status
            session.status = new_status
            db.add(session)
    
    db.commit()
    db.refresh(db_event)
    
    return db_event

@router.get("/", response_model=List[SessionEvent])
async def list_events(
    session_id: Optional[UUID] = None,
    event_type: Optional[SessionEventType] = None,
    start_time: Optional[datetime] = None,
    end_time: Optional[datetime] = None,
    offset: int = 0,
    limit: int = 100,
    db: SQLAlchemySession = Depends(get_db)
):
    """
    List session events with optional filtering.
    
    Returns a list of events, optionally filtered by session ID, event type,
    and time range.
    """
    query = db.query(SessionEventModel)
    
    # Apply filters
    if session_id:
        query = query.filter(SessionEventModel.session_id == session_id)
    
    if event_type:
        # Handle enum value
        if isinstance(event_type, SessionEventType):
            event_type = event_type.value
        query = query.filter(SessionEventModel.event == event_type)
    
    if start_time:
        query = query.filter(SessionEventModel.timestamp >= start_time)
    
    if end_time:
        query = query.filter(SessionEventModel.timestamp <= end_time)
    
    # Order by timestamp descending (newest first)
    query = query.order_by(desc(SessionEventModel.timestamp))
    
    # Apply pagination
    events = query.offset(offset).limit(limit).all()
    
    return events

@router.get("/{event_id}", response_model=SessionEvent)
async def get_event(
    event_id: UUID,
    db: SQLAlchemySession = Depends(get_db)
):
    """
    Get details of a specific event.
    """
    event = db.query(SessionEventModel).filter(SessionEventModel.id == event_id).first()
    if not event:
        raise HTTPException(
            status_code=status.HTTP_404_NOT_FOUND,
            detail=f"Event with id {event_id} not found"
        )
    
    return event

@router.delete("/{event_id}", status_code=status.HTTP_204_NO_CONTENT)
async def delete_event(
    event_id: UUID,
    db: SQLAlchemySession = Depends(get_db)
):
    """
    Delete a specific event.
    """
    event = db.query(SessionEventModel).filter(SessionEventModel.id == event_id).first()
    if not event:
        raise HTTPException(
            status_code=status.HTTP_404_NOT_FOUND,
            detail=f"Event with id {event_id} not found"
        )
    
    db.delete(event)
    db.commit()
    
    return None

@router.get("/session/{session_id}", response_model=List[SessionEvent])
async def get_session_events(
    session_id: UUID,
    event_type: Optional[str] = None,
    offset: int = 0,
    limit: int = 100,
    db: SQLAlchemySession = Depends(get_db)
):
    """
    Get all events for a specific session.
    """
    # Verify session exists
    session = db.query(SessionModel).filter(SessionModel.id == session_id).first()
    if not session:
        raise HTTPException(
            status_code=status.HTTP_404_NOT_FOUND,
            detail=f"Session with id {session_id} not found"
        )
    
    # Build query
    query = db.query(SessionEventModel).filter(SessionEventModel.session_id == session_id)
    
    if event_type:
        query = query.filter(SessionEventModel.event == event_type)
    
    # Order by timestamp ascending (oldest first, for chronological order)
    query = query.order_by(SessionEventModel.timestamp)
    
    # Apply pagination
    events = query.offset(offset).limit(limit).all()
    
    return events
