from fastapi import APIRouter


router = APIRouter()


from browsergrid.server.sessions.api.v1.sessions import router as sessions_v1_router
from browsergrid.server.sessions.api.v1.events import router as events_v1_router
from browsergrid.server.sessions.api.v1.metrics import router as metrics_v1_router


router.include_router(
    sessions_v1_router, 
    prefix="/v1/sessions",
    tags=["Sessions"]
)

router.include_router(
    metrics_v1_router, 
    prefix="/v1/metrics",
    tags=["Metrics"]
)

router.include_router(
    events_v1_router, 
    prefix="/v1/events",
    tags=["Events"]
) 