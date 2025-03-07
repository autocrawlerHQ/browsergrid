"""
API v1 router configuration
"""
from fastapi import APIRouter

from browserfleet.server.sessions.api.v1 import sessions
from browserfleet.server.webhooks.api.v1 import webhooks
from browserfleet.server.metrics.api.v1 import metrics
from browserfleet.server.events.api.v1 import events
from browserfleet.server.workers.api.v1 import workers
from browserfleet.server.workpools.api.v1 import workpools

api_router = APIRouter()

api_router.include_router(
    sessions.router, 
    prefix="/v1/sessions", 
    tags=["Sessions"]
)
api_router.include_router(
    webhooks.router, 
    prefix="/v1/webhooks", 
    tags=["Webhooks"]
)
api_router.include_router(
    metrics.router, 
    prefix="/v1/metrics", 
    tags=["Metrics"]
)
api_router.include_router(
    events.router, 
    prefix="/v1/events", 
    tags=["Events"]
)
api_router.include_router(
    workers.router, 
    prefix="/v1/workers", 
    tags=["Workers"]
)
api_router.include_router(
    workpools.router, 
    prefix="/v1/workpools", 
    tags=["WorkPools"]
)
