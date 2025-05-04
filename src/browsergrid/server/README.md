# Browsergrid Server

This directory contains the browsergrid server, which provides an API for managing browser automation sessions.

## Server Architecture


```
browsergrid/server/
├── core/                         # Core functionality
│   ├── db/                       # Database functionality
│   │   ├── alembic/              # Migration files
│   │   ├── base.py               # Base model configuration
│   │   ├── migrations.py         # Migration management
│   │   └── session.py            # Database session
│   ├── apps.py                   # App registry
│   ├── router.py                 # Central router configuration
│   ├── server.py                 # Server initialization and management
│   └── settings.py               # Centralized settings
├── sessions/                     # Sessions app
│   ├── api/                      # API endpoints (versioned)
│   │   └── v1/                   # API version 1
│   │       └── sessions.py       # Session endpoints
│   ├── models.py                 # Session models
│   ├── routes.py                 # Routes registration
│   └── apps.py                   # App configuration
├── webhooks/                     # Webhooks app
│   ├── api/                      # API endpoints (versioned)
│   │   └── v1/                   # API version 1
│   │       └── webhooks.py       # Webhook endpoints
│   ├── models.py                 # Webhook models
│   ├── routes.py                 # Routes registration
│   └── apps.py                   # App configuration
├── workers/                      # Workers app
│   ├── api/                      # API endpoints (versioned)
│   │   └── v1/                   # API version 1
│   ├── models.py                 # Worker models
│   ├── routes.py                 # Routes registration
│   └── apps.py                   # App configuration
├── workerpool/                   # Workerpool app
│   ├── api/                      # API endpoints (versioned)
│   │   └── v1/                   # API version 1
│   │       ├── pools.py          # Work Pool endpoints
│   │       └── workers.py        # Worker endpoints
│   ├── models.py                 # Workerpool and Worker models
│   ├── manager.py                # Workerpool manager for session assignments
│   ├── schemas.py                # Pydantic schemas for API
│   ├── enums.py                  # Enums for status and provider types
│   ├── routes.py                 # Routes registration
│   └── apps.py                   # App configuration
├── manage.py                     # Management script
└── manage_db.py                  # Database management script
```

## Getting Started

### Running the Server

You can run the server using the `manage.py` script:

```bash
# Run the server with default settings
python -m browsergrid.server.manage runserver

# Run the server with custom settings
python -m browsergrid.server.manage runserver --host 0.0.0.0 --port 8000 --debug
```

### Managing the Database

You can manage the database using the `manage.py` script:

```bash
# Initialize the database
python -m browsergrid.server.manage createdb

# Create migrations
python -m browsergrid.server.manage makemigrations "migration message"

# Run migrations
python -m browsergrid.server.manage migrate

# Open a database shell
python -m browsergrid.server.manage dbshell
```

### Using the Shell

You can open a Python shell with the browsergrid environment:

```bash
python -m browsergrid.server.manage shell
```

### Checking for Issues

You can check the project for issues:

```bash
python -m browsergrid.server.manage check
```

## Work Pools and Workers

Browsergrid supports dynamic infrastructure provisioning for browser sessions through Work Pools and Workers.

### Architecture Overview

Browsergrid uses a clean separation of responsibilities:

1. **Work Pools**: Define infrastructure configurations for browser session deployment 
2. **Workers**: Poll for pending sessions and track them (infrastructure-agnostic)
3. **Orchestration Layer**: Handles the actual deployment of sessions using provider-specific logic

This architecture allows workers to be deployed anywhere (on-prem, cloud, etc.) while the orchestration layer handles the infrastructure-specific details.

### What are Work Pools?

Work pools are a bridge between the Browsergrid orchestration layer and infrastructure for browser sessions that can be dynamically provisioned. They enable:

- Dynamic scaling of browser sessions based on demand
- Default browser configurations that all sessions inherit
- Resource allocation and prioritization of session requests
- Platform team control over browser infrastructure

### Why Infrastructure-Agnostic Workers?

Workers are client-side agents that poll for pending sessions from a work pool. Benefits include:

- Workers can be deployed anywhere regardless of the infrastructure they use
- A worker connected to an Azure pool can run locally or in any environment
- Workers simply poll for sessions and track their status
- The orchestration layer handles deploying the actual browser sessions

### Supported Providers

Currently supported providers:
- Docker: Run browser containers directly on Docker hosts
- Azure Container Instances: Run browser containers in Azure (coming soon)

### Easy Command-Line Management

Browsergrid provides a dedicated CLI tool for managing work pools and workers easily:

```bash
# Set environment variables for convenience (optional)
export BROWSERGRID_API_URL="http://localhost:8765"
export BROWSERGRID_API_KEY="your-api-key"

# Create a Docker work pool
python -m browsergrid.client.workerpool_cli create docker \
  --name "my-docker-pool" \
  --browser chrome \
  --browser-version latest

# Create an Azure Container Instances work pool
python -m browsergrid.client.workerpool_cli create azure_container_instance \
  --name "my-azure-pool" \
  --resource-group "my-resource-group" \
  --location "eastus" \
  --subscription-id "12345678-1234-1234-1234-123456789012" \
  --cpu 2 \
  --memory-gb 4

# List all work pools
python -m browsergrid.client.workerpool_cli list

# Get details for a work pool
python -m browsergrid.client.workerpool_cli get <pool_id>

# Start a worker for a pool (infrastructure-agnostic)
python -m browsergrid.client.workerpool_cli start-worker <pool_id>
```

### Architecture Flow

The architecture follows this flow:
1. Session Request → assigned to Work Pool by orchestration layer
2. Worker → polls Work Pool for pending sessions 
3. Orchestration Layer → deploys browser session based on provider configuration
4. Worker → tracks session status and updates orchestration layer

### Using Work Pools

#### Creating a Session with a Work Pool

```bash
curl -X POST "http://localhost:8765/v1/sessions" \
  -H "Content-Type: application/json" \
  -H "X-API-Key: your_api_key" \
  -d '{
    "browser": "chrome",
    "version": "latest",
    "headless": false,
    "resource_limits": {"cpu": 1, "memory": "2G", "timeout_minutes": 30},
    "work_pool_id": "your_work_pool_id"
  }'
```

### Running a Worker

Workers are infrastructure-agnostic and can be run anywhere:

```bash
python -m browsergrid.client.worker \
  --api-url http://localhost:8765 \
  --api-key your_api_key \
  --work-pool-id your_work_pool_id
```

Additional options:
- `--worker-id`: Connect as an existing worker (for reconnection)
- `--worker-name`: Custom name for the worker
- `--capacity`: Maximum number of concurrent sessions (default: 5)
- `--poll-interval`: Seconds between polling for new sessions (default: 10)
- `--debug`: Enable verbose logging

### Work Pool Management API

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/v1/workerpool/pools` | GET | List all work pools |
| `/v1/workerpool/pools` | POST | Create a new work pool |
| `/v1/workerpool/pools/{pool_id}` | GET | Get work pool details |
| `/v1/workerpool/pools/{pool_id}` | PUT | Update a work pool |
| `/v1/workerpool/pools/{pool_id}` | DELETE | Delete a work pool |

### Worker Management API

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/v1/workerpool/workers` | GET | List all workers |
| `/v1/workerpool/workers` | POST | Create a new worker |
| `/v1/workerpool/workers/{worker_id}` | GET | Get worker details |
| `/v1/workerpool/workers/{worker_id}` | DELETE | Delete a worker |
| `/v1/workerpool/workers/{worker_id}/heartbeat` | PUT | Update worker heartbeat |
| `/v1/workerpool/workers/{worker_id}/claim-session` | POST | Claim a pending session |

## Configuration

The server configuration is centralized in `core/settings.py`. You can override settings by setting environment variables (prefixed with `BROWSERGRID_`)

## Adding New Work Pool Providers

To add a new work pool provider:

1. Create a new file in `browsergrid/client/providers/` (e.g., `kubernetes.py`)
2. Implement a provider class that extends `ProviderBase`
3. Register the provider with the `@register_provider` decorator

Example:

```python
# browsergrid/client/providers/kubernetes.py
from browsergrid.client.providers import ProviderBase, register_provider

@register_provider
class KubernetesProvider(ProviderBase):
    provider_type = "kubernetes"
    display_name = "Kubernetes"
    description = "Run browser sessions in Kubernetes pods"
    
    def __init__(self, name="k8s-pool", namespace="browsergrid", ...):
        self.name = name
        self.namespace = namespace
        # ...
    
    def get_pool_config(self) -> Dict[str, Any]:
        return {
            "name": self.name,
            "provider_type": self.provider_type,
            # ... rest of configuration
        }
    
    def get_deploy_command(self, session_id, session_details) -> str:
        # Generate command to deploy a K8s pod with browser
        # ...
```

## Adding New Apps

To add a new app to the server:

1. Create a new directory in `browsergrid/server/`
2. Add the app to `INSTALLED_APPS` in `core/settings.py`
3. Create the needed structure:
   - `models.py` - Database models
   - `apps.py` - App configuration (optional)
   - `api/v1/` - API endpoints (versioned)
   - `routes.py` - Routes registration

### Example App Structure

#### App Configuration

```python
# browsergrid/server/myapp/apps.py
from browsergrid.server.core.apps import AppConfig

class MyAppConfig(AppConfig):
    name = "myapp"
    verbose_name = "My Custom App"
    
    def ready(self):
        # Custom initialization code
        pass
```

#### API Endpoints

```python
# browsergrid/server/myapp/api/v1/resources.py
from fastapi import APIRouter, Depends
from sqlalchemy.orm import Session
from typing import List, Dict

from browsergrid.server.core.db.session import get_db
from browsergrid.server.myapp.models import MyModel

router = APIRouter()

@router.get("/")
async def list_resources(db: Session = Depends(get_db)):
    resources = db.query(MyModel).all()
    return resources
```

#### Routes Registration

```python
# browsergrid/server/myapp/routes.py
from fastapi import APIRouter

# Create the main router for the app
router = APIRouter()

# Import versioned API modules
from browsergrid.server.myapp.api.v1.resources import router as resources_v1_router

# Include v1 routes with the appropriate prefix
router.include_router(
    resources_v1_router, 
    prefix="/api/v1/myapp",
    tags=["My App"]
)
```

#### Central Router Registration

After creating a new app, you need to register its router in the central router:

```python
# browsergrid/server/core/router.py
from fastapi import APIRouter, FastAPI

def include_router(app: FastAPI):
    # ... existing imports ...
    from browsergrid.server.myapp.routes import router as myapp_router
    
    # ... existing registrations ...
    app.include_router(myapp_router)
    
    return app
```

## API Versioning

The API is versioned using directory structure:
- `api/v1/` - API version 1
- `api/v2/` - API version 2 (when needed)

Each app's `routes.py` file combines routes from different versions with appropriate prefixes, making it easy to maintain backward compatibility when adding new features.

## Middleware System

Browsergrid uses a lightweight middleware system to process requests and responses.

### Middleware Configuration

Middlewares are configured in `core/settings.py` using a simple list format:

```python
MIDDLEWARE = [
    {
        "class": "fastapi.middleware.cors.CORSMiddleware",
        "kwargs": {
            "allow_origins": CORS_ALLOW_ORIGINS,
        },
        "enabled": True,
    },
    # More middleware entries...
]
```

### Built-in Middlewares

The server comes with several built-in middlewares:

1. **CORS**: Handles Cross-Origin Resource Sharing
2. **GZip**: Compresses responses
3. **RequestLog**: Adds request IDs and logs requests/responses
4. **ApiKey**: Handles API key authentication
5. **RateLimit**: Limits request rates by client IP

### Creating Custom Middleware

To create a custom middleware, extend the `BrowsergridMiddleware` class:

```python
from browsergrid.server.core.middleware import BrowsergridMiddleware

class CustomMiddleware(BrowsergridMiddleware):
    async def process_request(self, request):
        # Pre-process the request
        return request
    
    async def process_response(self, request, response):
        # Post-process the response
        return response
```

### Registering Middleware

You can register middleware programmatically:

```python
from browsergrid.server.core.middleware_utils import register_middleware

register_middleware(
    class_path="myapp.middleware.CustomMiddleware",
    kwargs={"option": "value"},
)
```

### Middleware Order

- Middleware is applied in the order listed
- Request processing flows top-to-bottom
- Response processing flows bottom-to-top
- Authentication should come early in the chain 