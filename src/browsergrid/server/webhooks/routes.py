from fastapi import APIRouter

router = APIRouter()

from browsergrid.server.webhooks.api.v1.webhooks import router as webhooks_v1_router

router.include_router(
    webhooks_v1_router, 
    prefix="/v1/webhooks",
    tags=["Webhooks"]
) 