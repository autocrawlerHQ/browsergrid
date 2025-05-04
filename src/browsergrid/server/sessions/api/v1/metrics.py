"""
API endpoints for session metrics
"""
from typing import List, Optional
from uuid import UUID
from datetime import datetime, timedelta

from fastapi import APIRouter, Depends, HTTPException, Query, status
from sqlalchemy.orm import Session as SQLAlchemySession
from sqlalchemy import desc, func, and_

from browsergrid.server.core.db.session import get_db
from browsergrid.server.sessions.schema import (
    SessionMetrics, SessionMetricsCreate, SessionMetricsBase
)
from browsergrid.server.sessions.models import SessionMetrics as SessionMetricsModel
from browsergrid.server.sessions.models import Session as SessionModel

router = APIRouter()

@router.post("/", response_model=SessionMetrics, status_code=status.HTTP_201_CREATED)
async def create_metrics(
    metrics: SessionMetricsCreate,
    db: SQLAlchemySession = Depends(get_db)
):
    """
    Record metrics for a session.
    
    This endpoint allows recording resource usage metrics for a browser session,
    such as CPU, memory and network usage.
    """
    # Verify session exists
    session = db.query(SessionModel).filter(SessionModel.id == metrics.session_id).first()
    if not session:
        raise HTTPException(
            status_code=status.HTTP_404_NOT_FOUND,
            detail=f"Session with id {metrics.session_id} not found"
        )
    
    # Create DB model from schema
    db_metrics = SessionMetricsModel(
        session_id=metrics.session_id,
        cpu_percent=metrics.cpu_percent,
        memory_mb=metrics.memory_mb,
        network_rx_bytes=metrics.network_rx_bytes,
        network_tx_bytes=metrics.network_tx_bytes
    )
    
    # Add metrics to database
    db.add(db_metrics)
    db.commit()
    db.refresh(db_metrics)
    
    return db_metrics

@router.get("/", response_model=List[SessionMetrics])
async def list_metrics(
    session_id: Optional[UUID] = None,
    start_time: Optional[datetime] = None,
    end_time: Optional[datetime] = None,
    offset: int = 0,
    limit: int = 100,
    db: SQLAlchemySession = Depends(get_db)
):
    """
    List session metrics with optional filtering.
    
    Returns a list of metrics, optionally filtered by session ID and time range.
    """
    query = db.query(SessionMetricsModel)
    
    # Apply filters
    if session_id:
        query = query.filter(SessionMetricsModel.session_id == session_id)
    
    if start_time:
        query = query.filter(SessionMetricsModel.timestamp >= start_time)
    
    if end_time:
        query = query.filter(SessionMetricsModel.timestamp <= end_time)
    
    # Order by timestamp descending (newest first)
    query = query.order_by(desc(SessionMetricsModel.timestamp))
    
    # Apply pagination
    metrics = query.offset(offset).limit(limit).all()
    
    return metrics

@router.get("/{metrics_id}", response_model=SessionMetrics)
async def get_metrics(
    metrics_id: UUID,
    db: SQLAlchemySession = Depends(get_db)
):
    """
    Get details of specific metrics entry.
    """
    metrics = db.query(SessionMetricsModel).filter(SessionMetricsModel.id == metrics_id).first()
    if not metrics:
        raise HTTPException(
            status_code=status.HTTP_404_NOT_FOUND,
            detail=f"Metrics with id {metrics_id} not found"
        )
    
    return metrics

@router.delete("/{metrics_id}", status_code=status.HTTP_204_NO_CONTENT)
async def delete_metrics(
    metrics_id: UUID,
    db: SQLAlchemySession = Depends(get_db)
):
    """
    Delete a specific metrics entry.
    """
    metrics = db.query(SessionMetricsModel).filter(SessionMetricsModel.id == metrics_id).first()
    if not metrics:
        raise HTTPException(
            status_code=status.HTTP_404_NOT_FOUND,
            detail=f"Metrics with id {metrics_id} not found"
        )
    
    db.delete(metrics)
    db.commit()
    
    return None

@router.get("/session/{session_id}", response_model=List[SessionMetrics])
async def get_session_metrics(
    session_id: UUID,
    start_time: Optional[datetime] = None,
    end_time: Optional[datetime] = None,
    interval: Optional[str] = None,  # e.g., "1min", "5min", "1h"
    offset: int = 0,
    limit: int = 100,
    db: SQLAlchemySession = Depends(get_db)
):
    """
    Get all metrics for a specific session, with optional time-based aggregation.
    
    If interval is specified, metrics will be aggregated over the specified time interval.
    """
    # Verify session exists
    session = db.query(SessionModel).filter(SessionModel.id == session_id).first()
    if not session:
        raise HTTPException(
            status_code=status.HTTP_404_NOT_FOUND,
            detail=f"Session with id {session_id} not found"
        )
    
    # If interval is specified, use time-based aggregation
    if interval:
        # PostgreSQL specific time bucketing
        time_bucket = func.date_trunc(interval, SessionMetricsModel.timestamp)
        
        query = db.query(
            time_bucket.label('timestamp'),
            func.avg(SessionMetricsModel.cpu_percent).label('cpu_percent'),
            func.avg(SessionMetricsModel.memory_mb).label('memory_mb'),
            func.max(SessionMetricsModel.network_rx_bytes).label('network_rx_bytes'),
            func.max(SessionMetricsModel.network_tx_bytes).label('network_tx_bytes'),
            SessionMetricsModel.session_id
        ).filter(SessionMetricsModel.session_id == session_id)
        
        if start_time:
            query = query.filter(SessionMetricsModel.timestamp >= start_time)
        
        if end_time:
            query = query.filter(SessionMetricsModel.timestamp <= end_time)
        
        query = query.group_by(time_bucket, SessionMetricsModel.session_id)
        query = query.order_by(time_bucket)
        
        # Apply pagination
        metrics = query.offset(offset).limit(limit).all()
        
        # Convert to model-like objects
        result = []
        for m in metrics:
            result.append({
                "id": None,  # Aggregates don't have a specific ID
                "session_id": m.session_id,
                "timestamp": m.timestamp,
                "cpu_percent": m.cpu_percent,
                "memory_mb": m.memory_mb,
                "network_rx_bytes": m.network_rx_bytes,
                "network_tx_bytes": m.network_tx_bytes
            })
        
        return result
    else:
        # Standard query without aggregation
        query = db.query(SessionMetricsModel).filter(SessionMetricsModel.session_id == session_id)
        
        if start_time:
            query = query.filter(SessionMetricsModel.timestamp >= start_time)
        
        if end_time:
            query = query.filter(SessionMetricsModel.timestamp <= end_time)
        
        # Order by timestamp ascending (oldest first)
        query = query.order_by(SessionMetricsModel.timestamp)
        
        # Apply pagination
        metrics = query.offset(offset).limit(limit).all()
        
        return metrics

@router.get("/aggregate/sessions", response_model=List[dict])
async def get_aggregate_metrics(
    session_ids: List[UUID] = Query(...),
    metric_type: str = Query(..., description="Type of metric to aggregate (cpu_percent, memory_mb, network_rx_bytes, network_tx_bytes)"),
    start_time: Optional[datetime] = None,
    end_time: Optional[datetime] = None,
    db: SQLAlchemySession = Depends(get_db)
):
    """
    Get aggregated metrics for multiple sessions.
    
    Compare metrics across different sessions.
    """
    valid_metrics = ['cpu_percent', 'memory_mb', 'network_rx_bytes', 'network_tx_bytes']
    if metric_type not in valid_metrics:
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST,
            detail=f"Invalid metric_type. Must be one of: {valid_metrics}"
        )
    
    # Build query filters
    filters = [SessionMetricsModel.session_id.in_(session_ids)]
    
    if start_time:
        filters.append(SessionMetricsModel.timestamp >= start_time)
    
    if end_time:
        filters.append(SessionMetricsModel.timestamp <= end_time)
    
    # Query for average metrics per session
    query = db.query(
        SessionMetricsModel.session_id,
        func.avg(getattr(SessionMetricsModel, metric_type)).label('avg_value'),
        func.min(getattr(SessionMetricsModel, metric_type)).label('min_value'),
        func.max(getattr(SessionMetricsModel, metric_type)).label('max_value')
    ).filter(and_(*filters))
    
    query = query.group_by(SessionMetricsModel.session_id)
    result = query.all()
    
    # Format the response
    formatted_result = []
    for row in result:
        formatted_result.append({
            "session_id": row.session_id,
            "metric_type": metric_type,
            "avg_value": row.avg_value,
            "min_value": row.min_value,
            "max_value": row.max_value
        })
    
    return formatted_result
