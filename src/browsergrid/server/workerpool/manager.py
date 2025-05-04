"""
WorkPoolManager for managing work pools and workers
"""

from typing import Optional, List, Dict, Any
from uuid import UUID
import os
import json
import subprocess
from datetime import datetime, timedelta

from sqlalchemy.orm import Session as SQLAlchemySession

from browsergrid.server.sessions.enums import SessionStatus
from browsergrid.server.workerpool.enums import WorkPoolStatus, WorkerStatus, ProviderType
from browsergrid.server.sessions.models import Session as SessionModel
from browsergrid.server.workerpool.models import WorkPool as WorkPoolModel, Worker as WorkerModel

from loguru import logger


class WorkPoolManager:
    """Manages work pools and workers"""
    
    @staticmethod
    def assign_session_to_work_pool(db: SQLAlchemySession, session_id: UUID, work_pool_id: Optional[UUID] = None) -> bool:
        """
        Assign a session to a work pool.
        
        If work_pool_id is provided, it will try to assign to that specific pool.
        If not, it will choose the best available pool based on capacity and current load.
        
        Returns True if assigned successfully, False otherwise.
        """
        session = db.query(SessionModel).filter(SessionModel.id == session_id).first()
        if not session:
            logger.error(f"Session with ID {session_id} not found")
            return False
        
        if session.status != SessionStatus.PENDING:
            logger.error(f"Cannot assign session {session_id} with status {session.status}")
            return False
        
        # If work_pool_id is provided, try to assign to that specific pool
        if work_pool_id:
            work_pool = db.query(WorkPoolModel).filter(
                WorkPoolModel.id == work_pool_id,
                WorkPoolModel.status == WorkPoolStatus.ACTIVE
            ).first()
            
            if not work_pool:
                logger.error(f"Work pool with ID {work_pool_id} not found or not active")
                return False
            
            session.work_pool_id = work_pool_id
            db.commit()
            logger.info(f"Assigned session {session_id} to work pool {work_pool_id}")
            
            # Apply default configurations from work pool if not specified in session
            WorkPoolManager._apply_pool_defaults(db, session, work_pool)
            
            # Try provisioning the session via the orchestration layer
            WorkPoolManager._try_provision_session(db, session, work_pool)
            return True
        
        # Otherwise, find the best available work pool
        # Get active work pools
        active_pools = db.query(WorkPoolModel).filter(
            WorkPoolModel.status == WorkPoolStatus.ACTIVE
        ).all()
        
        if not active_pools:
            logger.error("No active work pools available")
            return False
        
        # Find the best pool based on:
        # 1. Compatible with session requirements
        # 2. Has capacity (workers online with available capacity)
        # 3. Current load
        
        best_pool = None
        best_pool_score = -1
        
        for pool in active_pools:
            # Check compatibility with browser requirements
            if (pool.default_browser and pool.default_browser != session.browser) or \
               (pool.default_operating_system and pool.default_operating_system != session.operating_system):
                continue
            
            # Get available workers for this pool
            available_workers = db.query(WorkerModel).filter(
                WorkerModel.work_pool_id == pool.id,
                WorkerModel.status.in_([WorkerStatus.ONLINE, WorkerStatus.BUSY]),
                WorkerModel.current_load < WorkerModel.capacity
            ).count()
            
            if available_workers == 0:
                continue
            
            # Calculate a score for this pool
            # This is a simple heuristic - can be improved based on performance metrics
            score = available_workers * 10  # Higher weight for available workers
            
            # Add current sessions as negative score (prefer pools with fewer sessions)
            current_sessions = db.query(SessionModel).filter(
                SessionModel.work_pool_id == pool.id,
                SessionModel.status.in_([SessionStatus.PENDING, SessionStatus.RUNNING])
            ).count()
            
            score -= current_sessions
            
            if score > best_pool_score:
                best_pool = pool
                best_pool_score = score
        
        if best_pool:
            session.work_pool_id = best_pool.id
            db.commit()
            logger.info(f"Assigned session {session_id} to best available work pool {best_pool.id}")
            
            # Apply default configurations from work pool if not specified in session
            WorkPoolManager._apply_pool_defaults(db, session, best_pool)
            
            # Try provisioning the session via the orchestration layer
            WorkPoolManager._try_provision_session(db, session, best_pool)
            return True
        
        logger.error(f"Could not find suitable work pool for session {session_id}")
        return False
    
    @staticmethod
    def _apply_pool_defaults(db: SQLAlchemySession, session: SessionModel, pool: WorkPoolModel) -> None:
        """Apply default configurations from the work pool to the session"""
        modified = False
        
        # Apply defaults if not set in session
        if pool.default_browser and not session.browser:
            session.browser = pool.default_browser
            modified = True
            
        if pool.default_browser_version and not session.version:
            session.version = pool.default_browser_version
            modified = True
            
        if pool.default_headless is not None and session.headless is None:
            session.headless = pool.default_headless
            modified = True
            
        if pool.default_operating_system and not session.operating_system:
            session.operating_system = pool.default_operating_system
            modified = True
            
        if pool.default_screen and not session.screen:
            session.screen = pool.default_screen
            modified = True
            
        if pool.default_proxy and not session.proxy:
            session.proxy = pool.default_proxy
            modified = True
            
        if pool.default_resource_limits and not session.resource_limits:
            session.resource_limits = pool.default_resource_limits
            modified = True
            
        if pool.default_environment and not session.environment:
            session.environment = pool.default_environment
            modified = True
        
        if modified:
            db.commit()
            logger.info(f"Applied work pool defaults to session {session.id}")
    
    @staticmethod
    def _try_provision_session(db: SQLAlchemySession, session: SessionModel, pool: WorkPoolModel) -> None:
        """
        Try to provision a session using the orchestration layer based on the work pool provider
        
        This function does not actually provision the session but prepares it for workers to
        claim and provision. The actual provisioning is handled by the workers.
        """
        # Depending on the provider type, we might want to set specific flags or
        # connect to specific orchestration APIs in the future (e.g., K8s APIs)
        if pool.provider_type == ProviderType.DOCKER:
            logger.info(f"Preparing session {session.id} for Docker provisioning")
        elif pool.provider_type == ProviderType.AZURE_CONTAINER_INSTANCE:
            logger.info(f"Preparing session {session.id} for Azure Container Instance provisioning")
        
        # The session is now ready to be claimed by a worker
    
    @staticmethod
    def find_available_workers(db: SQLAlchemySession, work_pool_id: UUID) -> List[Dict[str, Any]]:
        """Find available workers for a work pool"""
        workers = db.query(WorkerModel).filter(
            WorkerModel.work_pool_id == work_pool_id,
            WorkerModel.status.in_([WorkerStatus.ONLINE, WorkerStatus.BUSY]),
            WorkerModel.current_load < WorkerModel.capacity
        ).all()
        
        return [
            {
                "id": str(worker.id),
                "name": worker.name,
                "status": worker.status.value,
                "capacity": worker.capacity,
                "current_load": worker.current_load,
                "available_slots": worker.capacity - worker.current_load,
                "last_heartbeat": worker.last_heartbeat.isoformat() if worker.last_heartbeat else None
            }
            for worker in workers
        ]
    
    @staticmethod
    def mark_worker_offline(db: SQLAlchemySession, worker_id: UUID) -> bool:
        """Mark a worker as offline if it hasn't sent a heartbeat in too long"""
        from datetime import datetime, timedelta
        
        worker = db.query(WorkerModel).filter(WorkerModel.id == worker_id).first()
        if not worker:
            logger.error(f"Worker with ID {worker_id} not found")
            return False
        
        # If last heartbeat is more than 5 minutes ago, mark as offline
        if worker.last_heartbeat and worker.last_heartbeat < datetime.now() - timedelta(minutes=5):
            worker.status = WorkerStatus.OFFLINE
            db.commit()
            logger.info(f"Marked worker {worker_id} as offline due to missing heartbeats")
            return True
        
        return False
    
    @staticmethod
    def update_session_status(db: SQLAlchemySession, session_id: UUID, status: SessionStatus, details: Optional[Dict[str, Any]] = None) -> bool:
        """Update a session's status and details"""
        session = db.query(SessionModel).filter(SessionModel.id == session_id).first()
        if not session:
            logger.error(f"Session with ID {session_id} not found")
            return False
        
        session.status = status
        
        # Update additional details if provided
        if details:
            if "container_id" in details:
                session.container_id = details["container_id"]
            if "ws_endpoint" in details:
                session.ws_endpoint = details["ws_endpoint"]
            if "live_url" in details:
                session.live_url = details["live_url"]
        
        db.commit()
        
        # If session is completed/failed, update worker load
        if status in [SessionStatus.COMPLETED, SessionStatus.FAILED, SessionStatus.EXPIRED] and session.worker_id:
            worker = db.query(WorkerModel).filter(WorkerModel.id == session.worker_id).first()
            if worker and worker.current_load > 0:
                worker.current_load -= 1
                db.commit()
                logger.info(f"Decreased load for worker {worker.id} after session {session_id} {status}")
        
        logger.info(f"Updated session {session_id} status to {status}")
        return True
    
    @staticmethod
    def provision_session(db: SQLAlchemySession, session_id: UUID) -> bool:
        """
        Provision a session using the orchestration layer
        
        This method is used for direct provisioning without worker involvement.
        It's primarily for development/testing or for providers that can be
        directly managed by the orchestration layer.
        """
        session = db.query(SessionModel).filter(SessionModel.id == session_id).first()
        if not session:
            logger.error(f"Session with ID {session_id} not found")
            return False
        
        work_pool = None
        if session.work_pool_id:
            work_pool = db.query(WorkPoolModel).filter(WorkPoolModel.id == session.work_pool_id).first()
        
        if not work_pool:
            logger.error(f"Session {session_id} is not assigned to a work pool")
            return False
        
        # Extract session configuration
        session_config = {
            "id": str(session.id),
            "browser": session.browser.value,
            "version": session.version.value,
            "headless": session.headless,
            "operating_system": session.operating_system.value,
            "screen": session.screen,
            "proxy": session.proxy,
            "resource_limits": session.resource_limits,
            "environment": session.environment,
        }
        
        # Extract pool provider configuration
        provider_config = work_pool.provider_config or {}
        
        # This is an example of direct provisioning for development/testing
        if work_pool.provider_type == ProviderType.DOCKER:
            try:
                # Direct Docker provisioning example
                import docker
                client = docker.from_env()
                
                # Determine image
                browser = session.browser.value
                version = session.version.value
                registry = provider_config.get("registry", "")
                image_prefix = provider_config.get("image_prefix", "browserless")
                
                if registry:
                    registry = registry + "/"
                
                image = f"{registry}{image_prefix}/{browser}:{version}"
                
                # Environment variables
                env = {
                    "BROWSERLESS_HEADLESS": str(session.headless).lower(),
                    "BROWSERLESS_SESSION_ID": str(session.id),
                    **session.environment
                }
                
                # Resource limits
                resource_limits = session.resource_limits or {}
                cpu_limit = resource_limits.get("cpu")
                memory_limit = resource_limits.get("memory", "2G")
                
                # Network
                network = provider_config.get("network", "bridge")
                
                # Create and start container
                container = client.containers.run(
                    image=image,
                    detach=True,
                    environment=env,
                    name=f"browsergrid-{str(session.id)[:8]}",
                    ports={'3000/tcp': None},  # Auto-assign port
                    cpu_quota=int(float(cpu_limit) * 100000) if cpu_limit else None,
                    mem_limit=memory_limit,
                    network=network
                )
                
                # Get container info for connection details
                container.reload()
                container_id = container.id
                
                # Get the host port that was mapped to container port 3000
                port_bindings = container.attrs['NetworkSettings']['Ports']['3000/tcp']
                host_port = port_bindings[0]['HostPort'] if port_bindings else None
                
                if not host_port:
                    logger.error(f"Failed to get port mapping for container {container_id}")
                    return False
                
                # Update session with connection details
                session.container_id = container_id
                session.ws_endpoint = f"ws://localhost:{host_port}"
                session.live_url = f"http://localhost:{host_port}"
                session.status = SessionStatus.RUNNING
                db.commit()
                
                logger.info(f"Provisioned Docker container for session {session.id}: {container_id}")
                return True
            
            except Exception as e:
                logger.error(f"Failed to provision Docker container for session {session.id}: {str(e)}")
                session.status = SessionStatus.FAILED
                db.commit()
                return False
            
        elif work_pool.provider_type == ProviderType.AZURE_CONTAINER_INSTANCE:
            logger.warning(f"Direct Azure Container Instance provisioning not implemented, session {session.id}")
            return False
        
        else:
            logger.error(f"Unknown provider type for pool {work_pool.id}: {work_pool.provider_type}")
            return False
