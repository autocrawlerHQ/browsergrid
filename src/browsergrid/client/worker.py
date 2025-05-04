"""
Browsergrid Worker Client

This script runs on worker nodes to poll for sessions from a Browsergrid server.
Workers are infrastructure-agnostic and simply poll for pending sessions from
a work pool. The actual session deployment is handled by the orchestration layer
based on the work pool's provider configuration.
"""
import os
import time
import uuid
import argparse
import logging
import json
import requests
import platform
import psutil
import signal
import sys
from typing import Dict, Any, Optional, List, Union
from datetime import datetime, timedelta
from urllib.parse import urljoin
from enum import Enum
from browsergrid.server.sessions.enums import SessionEventType, SessionStatus


logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger("browsergrid-worker")



class BrowsergridWorker:
    """Client for Browsergrid worker node"""
    
    def __init__(
        self,
        api_url: str,
        api_key: str,
        worker_id: Optional[str] = None,
        work_pool_id: str = None,
        worker_name: Optional[str] = None,
        capacity: int = 5,
        poll_interval: int = 10,
    ):
        self.api_url = api_url.rstrip('/')
        self.api_key = api_key
        self.worker_id = worker_id
        self.work_pool_id = work_pool_id
        self.worker_name = worker_name or f"worker-{platform.node()}-{uuid.uuid4().hex[:8]}"
        self.capacity = capacity
        self.poll_interval = poll_interval
        self.active_sessions = {}  # Dict to track active sessions and their container stats
        self.metrics_interval = 60  # Collect metrics every 60 seconds by default
        self.last_metrics_time = {}  # Track when metrics were last sent for each session
        
        # Runtime state
        self.current_load = 0
        self.running = True
    
    def _make_api_request(self, method: str, endpoint: str, json_data: Optional[Dict[str, Any]] = None) -> Dict[str, Any]:
        """Make an API request to the Browsergrid server"""
        url = urljoin(self.api_url, endpoint)
        headers = {
            "X-API-Key": self.api_key,
            "Content-Type": "application/json"
        }
        
        try:
            response = requests.request(
                method=method,
                url=url,
                headers=headers,
                json=json_data,
                timeout=30
            )
            
            if response.status_code >= 400:
                logger.error(f"API request failed: {response.status_code} - {response.text}")
                return {"error": response.text}
            
            return response.json() if response.content else {}
            
        except requests.RequestException as e:
            logger.error(f"API request error: {str(e)}")
            return {"error": str(e)}
    
    def register_worker(self) -> bool:
        """Register this worker with the server"""
        if self.worker_id:
            # Check if worker exists
            response = self._make_api_request("GET", f"/v1/workerpool/workers/{self.worker_id}")
            if "error" not in response:
                logger.info(f"Using existing worker: {self.worker_id}")
                return True
        
        # Create new worker
        worker_data = {
            "name": self.worker_name,
            "work_pool_id": self.work_pool_id,
            "capacity": self.capacity,
            "status": "offline"
        }
        
        response = self._make_api_request("POST", "/v1/workerpool/workers", worker_data)
        
        if "error" in response:
            logger.error(f"Failed to register worker: {response['error']}")
            return False
        
        self.worker_id = response.get("id")
        logger.info(f"Worker registered with ID: {self.worker_id}")
        return True
    
    def send_heartbeat(self) -> bool:
        """Send heartbeat to server with status and metrics"""
        if not self.worker_id:
            logger.error("Cannot send heartbeat: Worker not registered")
            return False
        
        # Collect system metrics
        cpu_percent = psutil.cpu_percent()
        memory_info = psutil.virtual_memory()
        memory_mb = memory_info.used / (1024 * 1024)
        
        heartbeat_data = {
            "status": "busy" if self.current_load > 0 else "online",
            "current_load": self.current_load,
            "cpu_percent": cpu_percent,
            "memory_usage_mb": memory_mb,
            "ip_address": self._get_ip_address()
        }
        
        response = self._make_api_request(
            "PUT", 
            f"/v1/workerpool/workers/{self.worker_id}/heartbeat",
            heartbeat_data
        )
        
        if "error" in response:
            logger.error(f"Failed to send heartbeat: {response['error']}")
            return False
        
        logger.debug(f"Heartbeat sent: {heartbeat_data}")
        return True
    
    def claim_session(self) -> Optional[Dict[str, Any]]:
        """Poll for and claim a pending session"""
        if not self.worker_id:
            logger.error("Cannot claim session: Worker not registered")
            return None
        
        if self.current_load >= self.capacity:
            logger.debug("At capacity, not claiming new sessions")
            return None
        
        response = self._make_api_request("POST", f"/v1/workerpool/workers/{self.worker_id}/claim-session")
        
        if "error" in response:
            logger.error(f"Failed to claim session: {response['error']}")
            return None
        
        if not response.get("session_claimed", False):
            logger.debug(f"No session claimed: {response.get('reason', 'Unknown reason')}")
            return None
        
        session_id = response.get("session_id")
        session_details = response.get("session_details", {})
        
        logger.info(f"Claimed session: {session_id}")
        return {
            "session_id": session_id,
            "details": session_details
        }
    
    def send_event(self, session_id: str, event_type: Union[str, SessionEventType], data: Optional[Dict[str, Any]] = None) -> bool:
        """
        Send an event for a session to the server.
        
        Args:
            session_id: UUID of the session
            event_type: Type of event (either SessionEventType enum or string)
            data: Optional JSON-compatible data to include with the event
            
        Returns:
            bool: True if the event was successfully sent, False otherwise
        """
        # Convert enum to string if needed
        if isinstance(event_type, SessionEventType):
            event_type = event_type.value
        
        event_data = {
            "session_id": session_id,
            "event": event_type,
            "data": data or {}
        }
        
        try:
            response = self._make_api_request("POST", "/api/v1/events", event_data)
            if response:
                logger.info(f"Event {event_type} sent for session {session_id}")
                return True
            return False
        except Exception as e:
            logger.error(f"Failed to send event for session {session_id}: {str(e)}")
            return False
    
    def collect_container_metrics(self, session_id: str, container_id: str) -> Optional[Dict[str, Any]]:
        """
        Collect resource usage metrics for a container.
        
        Args:
            session_id: UUID of the session
            container_id: Docker container ID
            
        Returns:
            dict: Container metrics or None if collection failed
        """
        try:
            import docker
            client = docker.from_env()
            container = client.containers.get(container_id)
            
            # Get container stats
            stats = container.stats(stream=False)
            
            # Calculate CPU usage percentage
            cpu_delta = stats['cpu_stats']['cpu_usage']['total_usage'] - \
                        stats['precpu_stats']['cpu_usage']['total_usage']
            system_delta = stats['cpu_stats']['system_cpu_usage'] - \
                          stats['precpu_stats']['system_cpu_usage']
            cpu_percent = 0.0
            if system_delta > 0 and cpu_delta > 0:
                cpu_percent = (cpu_delta / system_delta) * len(stats['cpu_stats']['cpu_usage']['percpu_usage']) * 100.0
            
            # Calculate memory usage in MB
            memory_usage = stats['memory_stats']['usage'] / (1024 * 1024)  # Convert to MB
            
            # Network stats if available
            network_rx_bytes = 0
            network_tx_bytes = 0
            if 'networks' in stats:
                for interface, net_stats in stats['networks'].items():
                    network_rx_bytes += net_stats['rx_bytes']
                    network_tx_bytes += net_stats['tx_bytes']
            
            metrics = {
                "cpu_percent": cpu_percent,
                "memory_mb": memory_usage,
                "network_rx_bytes": network_rx_bytes,
                "network_tx_bytes": network_tx_bytes
            }
            
            return metrics
            
        except Exception as e:
            logger.error(f"Failed to collect metrics for container {container_id}: {str(e)}")
            return None
    
    def send_metrics(self, session_id: str, metrics: Dict[str, Any]) -> bool:
        """
        Send metrics for a session to the server.
        
        Args:
            session_id: UUID of the session
            metrics: Dictionary containing metrics data
            
        Returns:
            bool: True if metrics were successfully sent, False otherwise
        """
        metrics_data = {
            "session_id": session_id,
            **metrics
        }
        
        try:
            response = self._make_api_request("POST", "/api/v1/metrics", metrics_data)
            if response:
                logger.debug(f"Metrics sent for session {session_id}")
                return True
            return False
        except Exception as e:
            logger.error(f"Failed to send metrics for session {session_id}: {str(e)}")
            return False
    
    def check_and_send_metrics(self) -> None:
        """
        Check all active sessions and send metrics if it's time to do so.
        """
        current_time = time.time()
        
        for session_id, session_data in list(self.active_sessions.items()):
            # Check if we should send metrics for this session
            last_time = self.last_metrics_time.get(session_id, 0)
            if current_time - last_time >= self.metrics_interval:
                # Get container ID from session data
                container_id = session_data.get('container_id')
                if container_id:
                    # Collect metrics
                    metrics = self.collect_container_metrics(session_id, container_id)
                    if metrics:
                        # Send metrics to server
                        if self.send_metrics(session_id, metrics):
                            self.last_metrics_time[session_id] = current_time
    
    def handle_session(self, session: Dict[str, Any]) -> bool:
        """
        Handle a new browser session assignment.
        
        This method is responsible for provisioning the browser session
        and updating its status.
        """
        session_id = session['id']
        logger.info(f"Handling session {session_id}")
        
        # Send an event that we've received the session
        self.send_event(session_id, SessionEventType.SESSION_ASSIGNED, {"worker_id": self.worker_id})
        
        # Track this session in our active sessions
        self.active_sessions[session_id] = {
            "container_id": session.get('container_id'),
            "start_time": time.time()
        }
        
        # Set initial metrics time
        self.last_metrics_time[session_id] = time.time()
        
        # Update session status to STARTING
        self._update_session_status(session_id, SessionStatus.STARTING)
        
        # Send an event that the session is starting
        self.send_event(session_id, SessionEventType.SESSION_STARTING, {"worker_id": self.worker_id})
        
        # TODO: Actual session provisioning logic would go here
        # This would include launching a container, setting up the browser, etc.
        
        # Send an event that the browser started
        self.send_event(session_id, SessionEventType.BROWSER_STARTED, {"success": True})
        
        # Update session status to RUNNING
        return self._update_session_status(session_id, SessionStatus.RUNNING)
    
    def check_session_status(self, session_id: str) -> Dict[str, Any]:
        """Check the status of a session"""
        response = self._make_api_request("GET", f"/v1/sessions/{session_id}")
        if "error" in response:
            logger.error(f"Failed to get session status: {response['error']}")
            return {"status": "unknown", "error": response.get("error")}
        
        return response
    
    def check_sessions_health(self) -> None:
        """Check health of running sessions and update tracking if needed"""
        for session_id, session in list(self.active_sessions.items()):
            # Check session status
            session_status = self.check_session_status(session_id)
            status = session_status.get("status", "unknown")
            
            # If session is completed/failed/expired, remove from active sessions
            if status in ["completed", "failed", "expired"]:
                logger.info(f"Session {session_id} is no longer active (status: {status})")
                self.active_sessions.pop(session_id, None)
                self.current_load = max(0, self.current_load - 1)
                continue
            
            # Check for session expiration
            claimed_at = datetime.fromisoformat(session.get("claimed_at", datetime.now().isoformat()))
            resource_limits = session.get("details", {}).get("resource_limits", {})
            timeout_minutes = resource_limits.get("timeout_minutes", 30)
            
            if claimed_at + timedelta(minutes=timeout_minutes) < datetime.now():
                logger.info(f"Session {session_id} has expired (timeout: {timeout_minutes} minutes)")
                self._update_session_status(session_id, SessionStatus.EXPIRED)
                self.active_sessions.pop(session_id, None)
                self.current_load = max(0, self.current_load - 1)
    
    def _update_session_status(self, session_id: str, status: Union[str, SessionStatus]) -> bool:
        """Update the status of a session"""
        # Convert enum to string if needed
        if isinstance(status, SessionStatus):
            status = status.value
            
        url = f"/api/v1/sessions/{session_id}/status"
        data = {"status": status}
        
        try:
            response = self._make_api_request("PUT", url, data)
            logger.info(f"Updated session {session_id} status to {status}")
            return True
        except Exception as e:
            logger.error(f"Failed to update session status: {str(e)}")
            return False
    
    def _get_ip_address(self) -> Optional[str]:
        """Get the IP address of this worker"""
        try:
            import socket
            hostname = socket.gethostname()
            ip_address = socket.gethostbyname(hostname)
            return ip_address
        except Exception as e:
            logger.error(f"Error getting IP address: {str(e)}")
            return None
    
    def run(self) -> None:
        """
        Main worker loop.
        
        Continuously polls for new sessions, manages existing sessions,
        and sends heartbeats to the server.
        """
        # Set up signal handling for graceful shutdown
        signal.signal(signal.SIGINT, self._handle_signal)
        signal.signal(signal.SIGTERM, self._handle_signal)
        
        # Attempt to register the worker
        if not self.register_worker():
            logger.error("Failed to register worker. Exiting.")
            return
        
        logger.info(f"Worker registered successfully with ID: {self.worker_id}")
        
        try:
            while True:
                # Send heartbeat to server
                self.send_heartbeat()
                
                # Check health of existing sessions
                self.check_sessions_health()
                
                # Check and send metrics for active sessions
                self.check_and_send_metrics()
                
                # Only claim a new session if we have capacity
                if len(self.active_sessions) < self.capacity:
                    # Poll for a new session
                    session = self.claim_session()
                    if session:
                        self.handle_session(session)
                
                # Sleep for the configured interval
                time.sleep(self.poll_interval)
        except KeyboardInterrupt:
            logger.info("Keyboard interrupt received. Shutting down gracefully...")
        finally:
            self._shutdown()

    def cleanup_session(self, session_id: str, reason: str = "completed") -> None:
        """
        Clean up resources for a session and report its completion.
        
        Args:
            session_id: UUID of the session to clean up
            reason: Reason for cleanup (e.g., 'completed', 'crashed', 'timeout')
        """
        if session_id in self.active_sessions:
            # Send appropriate event based on reason
            event_type = None
            new_status = None
            
            if reason == "completed":
                event_type = SessionEventType.SESSION_COMPLETED
                new_status = SessionStatus.COMPLETED
            elif reason == "crashed":
                event_type = SessionEventType.SESSION_CRASHED
                new_status = SessionStatus.CRASHED
            elif reason == "timeout":
                event_type = SessionEventType.SESSION_TIMED_OUT
                new_status = SessionStatus.TIMED_OUT
            else:
                event_type = SessionEventType.SESSION_TERMINATED
                new_status = SessionStatus.TERMINATED
            
            # Send session end event
            self.send_event(session_id, event_type, {
                "duration": time.time() - self.active_sessions[session_id].get('start_time', time.time())
            })
            
            # Update session status
            self._update_session_status(session_id, new_status)
            
            # Remove session from tracking
            del self.active_sessions[session_id]
            if session_id in self.last_metrics_time:
                del self.last_metrics_time[session_id]
            
            logger.info(f"Session {session_id} cleaned up. Reason: {reason}")

    def _handle_signal(self, sig, frame) -> None:
        """Handle termination signals"""
        logger.info(f"Received signal {sig}. Shutting down...")
        self.running = False
        self._shutdown()
    
    def _shutdown(self) -> None:
        """Clean up resources and shutdown"""
        logger.info("Cleaning up active sessions...")
        
        # Mark worker as offline
        if self.worker_id:
            self._make_api_request(
                "PUT",
                f"/v1/workerpool/workers/{self.worker_id}/heartbeat",
                {"status": "offline", "current_load": 0}
            )
        
        logger.info("Worker shutdown complete")

def main():
    """Main entry point for worker client"""
    parser = argparse.ArgumentParser(description="Browsergrid Infrastructure-Agnostic Worker Client")
    parser.add_argument("--api-url", required=True, help="Browsergrid API URL")
    parser.add_argument("--api-key", required=True, help="Browsergrid API key")
    parser.add_argument("--worker-id", help="Existing worker ID (if reconnecting)")
    parser.add_argument("--work-pool-id", required=True, help="Work pool ID to join")
    parser.add_argument("--worker-name", help="Worker name")
    parser.add_argument("--capacity", type=int, default=5, help="Maximum number of concurrent sessions")
    parser.add_argument("--poll-interval", type=int, default=10, help="Interval in seconds to poll for sessions")
    parser.add_argument("--debug", action="store_true", help="Enable debug logging")
    
    args = parser.parse_args()
    
    if args.debug:
        logging.getLogger().setLevel(logging.DEBUG)
    
    worker = BrowsergridWorker(
        api_url=args.api_url,
        api_key=args.api_key,
        worker_id=args.worker_id,
        work_pool_id=args.work_pool_id,
        worker_name=args.worker_name,
        capacity=args.capacity,
        poll_interval=args.poll_interval
    )
    
    worker.run()

if __name__ == "__main__":
    main() 