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
    RUNNING = "running"
    COMPLETED = "completed"
    FAILED = "failed"
    EXPIRED = "expired"
