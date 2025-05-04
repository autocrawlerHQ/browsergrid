"""
Utility functions for sessions management
"""
from browsergrid.server.sessions.enums import SessionEventType, SessionStatus

def get_status_from_event(event_type: SessionEventType) -> SessionStatus:
    """
    Maps a session event type to a corresponding session status.
    
    This ensures that events and statuses stay in sync when events
    indicate a change in session lifecycle.
    
    Args:
        event_type: The SessionEventType to map
        
    Returns:
        The corresponding SessionStatus or None if no status change is needed
    """
    # Define the mapping between events and statuses
    mapping = {
        SessionEventType.SESSION_CREATED: SessionStatus.PENDING,
        SessionEventType.SESSION_ASSIGNED: SessionStatus.PENDING,
        SessionEventType.SESSION_STARTING: SessionStatus.STARTING,
        SessionEventType.BROWSER_STARTED: SessionStatus.RUNNING,
        SessionEventType.SESSION_COMPLETED: SessionStatus.COMPLETED,
        SessionEventType.SESSION_CRASHED: SessionStatus.CRASHED,
        SessionEventType.SESSION_TIMED_OUT: SessionStatus.TIMED_OUT,
        SessionEventType.SESSION_TERMINATED: SessionStatus.TERMINATED
    }
    
    # Return the mapped status or None if the event doesn't map to a status
    return mapping.get(event_type)

def should_update_status(current_status: SessionStatus, new_status: SessionStatus) -> bool:
    """
    Determines if a session status should be updated based on the lifecycle.
    
    This ensures that we don't move backwards in the session lifecycle
    (e.g., from COMPLETED back to RUNNING).
    
    Args:
        current_status: The current session status
        new_status: The proposed new status
        
    Returns:
        True if the status should be updated, False otherwise
    """
    # Define the status hierarchy
    hierarchy = {
        SessionStatus.PENDING: 0,
        SessionStatus.STARTING: 1,
        SessionStatus.RUNNING: 2,
        SessionStatus.COMPLETED: 3,
        SessionStatus.FAILED: 3,
        SessionStatus.EXPIRED: 3,
        SessionStatus.CRASHED: 3,
        SessionStatus.TIMED_OUT: 3,
        SessionStatus.TERMINATED: 3
    }
    
    # Only update if the new status is further along in the lifecycle
    return hierarchy.get(new_status, 0) > hierarchy.get(current_status, 0) 