from fastapi import APIRouter

from browsergrid.server.workerpool.api.v1.pools import router as pools_router
from browsergrid.server.workerpool.api.v1.workers import router as workers_router
from browsergrid.server.workerpool.api.v1.metrics import router as metrics_router

# Create router for workerpool
router = APIRouter(
    prefix="/v1/workerpools",
    tags=["Workerpools V2"]
)

# Include the workerpool routes
router.include_router(pools_router)
router.include_router(workers_router)
router.include_router(metrics_router, prefix="/metrics", tags=["Metrics"])


