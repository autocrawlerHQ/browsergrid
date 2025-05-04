"""
API endpoints for workerpool metrics
"""
from typing import List, Optional, Dict, Any
from uuid import UUID
from datetime import datetime, timedelta

from fastapi import APIRouter, Depends, HTTPException, Query, status
from sqlalchemy.orm import Session as SQLAlchemySession
from sqlalchemy import desc, func, and_

from browsergrid.server.core.db.session import get_db
from browsergrid.server.workerpool.models import Worker, WorkPool
from browsergrid.server.sessions.models import SessionMetrics

router = APIRouter()

@router.get("/workers/{worker_id}")
async def get_worker_metrics(
    worker_id: UUID,
    start_time: Optional[datetime] = None,
    end_time: Optional[datetime] = None,
    interval: Optional[str] = None,  # e.g., "1min", "5min", "1h"
    db: SQLAlchemySession = Depends(get_db)
):
    """
    Get system resource metrics for a specific worker over time.
    
    Returns CPU, memory, and disk usage metrics for the worker.
    If interval is specified, metrics will be aggregated over the interval.
    """
    # Verify worker exists
    worker = db.query(Worker).filter(Worker.id == worker_id).first()
    if not worker:
        raise HTTPException(
            status_code=status.HTTP_404_NOT_FOUND,
            detail=f"Worker with id {worker_id} not found"
        )
    
    # Get sessions associated with this worker
    session_query = db.query(SessionMetrics).join(
        Worker, Worker.id == worker_id
    )
    
    if start_time:
        session_query = session_query.filter(SessionMetrics.timestamp >= start_time)
    
    if end_time:
        session_query = session_query.filter(SessionMetrics.timestamp <= end_time)
    
    if interval:
        # Aggregate data by time interval using PostgreSQL's date_trunc
        time_bucket = func.date_trunc(interval, SessionMetrics.timestamp)
        
        metrics_query = session_query.with_entities(
            time_bucket.label('timestamp'),
            func.avg(SessionMetrics.cpu_percent).label('avg_cpu_percent'),
            func.avg(SessionMetrics.memory_mb).label('avg_memory_mb'),
            func.sum(SessionMetrics.memory_mb).label('total_memory_mb'),
            func.sum(SessionMetrics.network_rx_bytes).label('total_network_rx_bytes'),
            func.sum(SessionMetrics.network_tx_bytes).label('total_network_tx_bytes'),
            func.count(SessionMetrics.id).label('sample_count')
        ).group_by(time_bucket).order_by(time_bucket)
    else:
        # Return raw metrics data
        metrics_query = session_query.order_by(SessionMetrics.timestamp)
    
    metrics_data = metrics_query.all()
    
    # Format the response
    result = []
    for m in metrics_data:
        if interval:
            result.append({
                "timestamp": m.timestamp,
                "avg_cpu_percent": m.avg_cpu_percent,
                "avg_memory_mb": m.avg_memory_mb,
                "total_memory_mb": m.total_memory_mb,
                "total_network_rx_bytes": m.total_network_rx_bytes,
                "total_network_tx_bytes": m.total_network_tx_bytes,
                "sample_count": m.sample_count
            })
        else:
            result.append({
                "id": m.id,
                "session_id": m.session_id,
                "timestamp": m.timestamp,
                "cpu_percent": m.cpu_percent,
                "memory_mb": m.memory_mb,
                "network_rx_bytes": m.network_rx_bytes,
                "network_tx_bytes": m.network_tx_bytes
            })
    
    return {
        "worker_id": worker_id,
        "worker_name": worker.name,
        "metrics": result
    }

@router.get("/workpool/{work_pool_id}")
async def get_workpool_metrics(
    work_pool_id: UUID,
    start_time: Optional[datetime] = None,
    end_time: Optional[datetime] = None,
    interval: str = "1h",  # Default to hourly aggregation for work pools
    include_worker_breakdown: bool = False,
    db: SQLAlchemySession = Depends(get_db)
):
    """
    Get system resource metrics aggregated for an entire work pool.
    
    Returns CPU, memory, and network usage metrics for all workers in the pool.
    """
    # Verify work pool exists
    work_pool = db.query(WorkPool).filter(WorkPool.id == work_pool_id).first()
    if not work_pool:
        raise HTTPException(
            status_code=status.HTTP_404_NOT_FOUND,
            detail=f"Work pool with id {work_pool_id} not found"
        )
    
    # Get all workers in this pool
    worker_ids = [worker.id for worker in work_pool.workers]
    
    if not worker_ids:
        return {
            "work_pool_id": work_pool_id,
            "work_pool_name": work_pool.name,
            "metrics": []
        }
    
    # Base query to get session metrics for sessions assigned to workers in this pool
    base_query = db.query(SessionMetrics).join(
        Worker, Worker.id == SessionMetrics.session_id
    ).filter(Worker.work_pool_id == work_pool_id)
    
    if start_time:
        base_query = base_query.filter(SessionMetrics.timestamp >= start_time)
    
    if end_time:
        base_query = base_query.filter(SessionMetrics.timestamp <= end_time)
    
    # Aggregate data by time interval using PostgreSQL's date_trunc
    time_bucket = func.date_trunc(interval, SessionMetrics.timestamp)
    
    if include_worker_breakdown:
        # Include worker-specific breakdown
        metrics_query = base_query.with_entities(
            time_bucket.label('timestamp'),
            Worker.id.label('worker_id'),
            Worker.name.label('worker_name'),
            func.avg(SessionMetrics.cpu_percent).label('avg_cpu_percent'),
            func.avg(SessionMetrics.memory_mb).label('avg_memory_mb'),
            func.sum(SessionMetrics.memory_mb).label('total_memory_mb'),
            func.sum(SessionMetrics.network_rx_bytes).label('total_network_rx_bytes'),
            func.sum(SessionMetrics.network_tx_bytes).label('total_network_tx_bytes'),
            func.count(SessionMetrics.id).label('sample_count')
        ).group_by(time_bucket, Worker.id, Worker.name).order_by(time_bucket)
    else:
        # Just pool-wide aggregates
        metrics_query = base_query.with_entities(
            time_bucket.label('timestamp'),
            func.avg(SessionMetrics.cpu_percent).label('avg_cpu_percent'),
            func.avg(SessionMetrics.memory_mb).label('avg_memory_mb'),
            func.sum(SessionMetrics.memory_mb).label('total_memory_mb'),
            func.sum(SessionMetrics.network_rx_bytes).label('total_network_rx_bytes'),
            func.sum(SessionMetrics.network_tx_bytes).label('total_network_tx_bytes'),
            func.count(SessionMetrics.id).label('sample_count'),
            func.count(func.distinct(SessionMetrics.session_id)).label('active_sessions')
        ).group_by(time_bucket).order_by(time_bucket)
    
    metrics_data = metrics_query.all()
    
    # Format the response
    result = []
    for m in metrics_data:
        if include_worker_breakdown:
            result.append({
                "timestamp": m.timestamp,
                "worker_id": m.worker_id,
                "worker_name": m.worker_name,
                "avg_cpu_percent": m.avg_cpu_percent,
                "avg_memory_mb": m.avg_memory_mb,
                "total_memory_mb": m.total_memory_mb,
                "total_network_rx_bytes": m.total_network_rx_bytes,
                "total_network_tx_bytes": m.total_network_tx_bytes,
                "sample_count": m.sample_count
            })
        else:
            result.append({
                "timestamp": m.timestamp,
                "avg_cpu_percent": m.avg_cpu_percent,
                "avg_memory_mb": m.avg_memory_mb,
                "total_memory_mb": m.total_memory_mb,
                "total_network_rx_bytes": m.total_network_rx_bytes,
                "total_network_tx_bytes": m.total_network_tx_bytes,
                "sample_count": m.sample_count,
                "active_sessions": m.active_sessions
            })
    
    return {
        "work_pool_id": work_pool_id,
        "work_pool_name": work_pool.name,
        "metrics": result
    }

@router.get("/system/overview")
async def get_system_metrics_overview(
    start_time: Optional[datetime] = Query(default=None),
    end_time: Optional[datetime] = Query(default=None),
    interval: str = Query(default="1h"),
    db: SQLAlchemySession = Depends(get_db)
):
    """
    Get system-wide metrics overview across all work pools.
    
    Returns aggregated statistics on resource usage, session counts, and worker status.
    """
    # Set default time range if not provided (last 24 hours)
    if not end_time:
        end_time = datetime.now()
    
    if not start_time:
        start_time = end_time - timedelta(hours=24)
    
    # Get active worker count
    active_workers = db.query(func.count(Worker.id)).filter(
        Worker.status == 'ONLINE'
    ).scalar()
    
    # Get total worker count
    total_workers = db.query(func.count(Worker.id)).scalar()
    
    # Get active session count
    active_sessions = db.query(func.count('*')).filter(
        Worker.status.in_(['RUNNING', 'STARTING'])
    ).scalar()
    
    # Get aggregated metrics over time
    time_bucket = func.date_trunc(interval, SessionMetrics.timestamp)
    
    metrics_query = db.query(
        time_bucket.label('timestamp'),
        func.avg(SessionMetrics.cpu_percent).label('avg_cpu_percent'),
        func.avg(SessionMetrics.memory_mb).label('avg_memory_mb'),
        func.sum(SessionMetrics.memory_mb).label('total_memory_mb'),
        func.sum(SessionMetrics.network_rx_bytes).label('total_network_rx_bytes'),
        func.sum(SessionMetrics.network_tx_bytes).label('total_network_tx_bytes'),
        func.count(SessionMetrics.id).label('sample_count'),
        func.count(func.distinct(SessionMetrics.session_id)).label('active_sessions')
    ).filter(
        SessionMetrics.timestamp.between(start_time, end_time)
    ).group_by(time_bucket).order_by(time_bucket)
    
    metrics_data = metrics_query.all()
    
    # Format the metrics data
    timeline_metrics = []
    for m in metrics_data:
        timeline_metrics.append({
            "timestamp": m.timestamp,
            "avg_cpu_percent": m.avg_cpu_percent,
            "avg_memory_mb": m.avg_memory_mb,
            "total_memory_mb": m.total_memory_mb,
            "total_network_rx_bytes": m.total_network_rx_bytes,
            "total_network_tx_bytes": m.total_network_tx_bytes,
            "sample_count": m.sample_count,
            "active_sessions": m.active_sessions
        })
    
    # Get worker pool statistics
    work_pools = db.query(
        WorkPool.id,
        WorkPool.name,
        func.count(Worker.id).label('worker_count'),
        func.sum(Worker.current_load).label('active_sessions')
    ).outerjoin(
        Worker, Worker.work_pool_id == WorkPool.id
    ).group_by(
        WorkPool.id, WorkPool.name
    ).all()
    
    work_pool_stats = []
    for wp in work_pools:
        work_pool_stats.append({
            "id": wp.id,
            "name": wp.name,
            "worker_count": wp.worker_count,
            "active_sessions": wp.active_sessions or 0
        })
    
    return {
        "overview": {
            "active_workers": active_workers,
            "total_workers": total_workers,
            "active_sessions": active_sessions,
            "time_range": {
                "start": start_time,
                "end": end_time,
                "interval": interval
            }
        },
        "timeline_metrics": timeline_metrics,
        "work_pools": work_pool_stats
    } 