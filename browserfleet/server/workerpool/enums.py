import enum
class WorkPoolStatus(enum.Enum):
    """Status of a work pool"""
    ACTIVE = "active"
    PAUSED = "paused"
    ERROR = "error"
    MAINTENANCE = "maintenance"

class WorkAllocationStrategy(enum.Enum):
    """Strategy for allocating work to workers"""
    ROUND_ROBIN = "round_robin"
    LEAST_BUSY = "least_busy"
    RANDOM = "random"
    LEAST_RECENTLY_USED = "least_recently_used"


