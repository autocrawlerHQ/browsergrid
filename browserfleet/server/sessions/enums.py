import enum

class OperatingSystem(enum.Enum):
    """Operating systems"""
    WINDOWS = "windows"
    MACOS = "macos"
    LINUX = "linux"

class Browser(enum.Enum):
    """Browser types"""
    CHROME = "chrome"
    FIREFOX = "firefox"
    EDGE = "edge"
    SAFARI = "safari"

class BrowserVersion(enum.Enum):
    """Browser versions"""
    LATEST = "latest"
    STABLE = "stable"
    CANARY = "canary"
    DEV = "dev"

class SessionStatus(enum.Enum):
    """Status for a browser session"""
    PENDING = "pending"
    STARTING = "starting"
    RUNNING = "running"
    COMPLETED = "completed"
    FAILED = "failed"
    EXPIRED = "expired"
    CRASHED = "crashed"
    TIMED_OUT = "timed_out"
    TERMINATED = "terminated"

class SessionEventType(enum.Enum):
    """Event types for browser session lifecycle"""
    # Session initialization events
    SESSION_CREATED = "session_created"       # Session record created in database
    SESSION_ASSIGNED = "session_assigned"     # Session assigned to a worker
    SESSION_STARTING = "session_starting"     # Browser process is starting
    BROWSER_STARTED = "browser_started"       # Browser successfully started
    
    # Session runtime events
    SESSION_IDLE = "session_idle"             # No activity detected for some time
    SESSION_ACTIVE = "session_active"         # Activity resumed after idle
    
    # Session termination events
    SESSION_COMPLETED = "session_completed"   # Session ended normally
    SESSION_CRASHED = "session_crashed"       # Browser crashed
    SESSION_TIMED_OUT = "session_timed_out"   # Session exceeded time limit
    SESSION_TERMINATED = "session_terminated" # Session forcibly terminated
