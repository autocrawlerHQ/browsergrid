import enum
class WorkerStatus(enum.Enum):
    """Status of a worker"""
    OFFLINE = "offline"
    ONLINE = "online"
    BUSY = "busy"
    ERROR = "error"
    STARTING = "starting"
    STOPPING = "stopping"

class WorkerType(enum.Enum):
    """Type of worker infrastructure"""
    DOCKER = "docker"
    AZURE_CONTAINER_INSTANCE = "azure_container_instance"
    AWS_ECS = "aws_ecs"
    GCP_CLOUD_RUN = "gcp_cloud_run"
    KUBERNETES = "kubernetes"

class ProviderType(enum.Enum):
    """Type of infrastructure provider"""
    LOCAL = "local"
    AZURE = "azure"
    AWS = "aws"
    GCP = "gcp"
    CUSTOM = "custom"