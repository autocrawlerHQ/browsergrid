import enum

class WorkerStatus(enum.Enum):
    """Status for a worker"""
    OFFLINE = "offline"
    ONLINE = "online"
    BUSY = "busy"
    DRAINING = "draining"  # No new sessions, but finishing current ones
    FAILED = "failed"

class WorkPoolStatus(enum.Enum):
    """Status for a work pool"""
    ACTIVE = "active"
    PAUSED = "paused"  # Not accepting new sessions
    DRAINING = "draining"  # Not accepting new sessions, but processing current ones
    INACTIVE = "inactive"

class ProviderType(enum.Enum):
    """Type of provider for worker"""
    DOCKER = "docker"
    AZURE_CONTAINER_INSTANCE = "azure_container_instance"
