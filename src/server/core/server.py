"""
Main application module for the browsergrid server

This module defines the FastAPI application instance that can be imported
directly by Uvicorn for serving, including when using auto-reload.
"""
import sys
from fastapi import FastAPI, Request

from src.server.core.settings import (
    API_HOST,
    API_PORT,
    API_KEY,
    API_AUTH,
    SERVER_ID,
    DEBUG,
    UI_ENABLED,
    MIDDLEWARE,
)
from src.server.core.apps import apps
from src.server.core.router import include_router
from src.server.core.middleware_utils import apply_middlewares

from loguru import logger

if not apps.apps_ready:
    apps.populate()

server = FastAPI(
    title="browsergrid API",
    description="API for managing browser sessions",
    version="0.1.0",
    debug=DEBUG,
)

apply_middlewares(
    app=server, 
    middleware_config=MIDDLEWARE,
    api_key=API_KEY,
    excluded_paths=API_AUTH["excluded_paths"],
    debug=DEBUG
)

include_router(server)

apps.ready()

@server.exception_handler(Exception)
async def global_exception_handler(request: Request, exc: Exception):
    request_id = getattr(request.state, "request_id", "unknown")
    logger.error(f"[{request_id}] Global exception: {exc}")
    
    return {
        "error": "Internal server error",
        "detail": str(exc) if DEBUG else "An internal server error occurred",
        "request_id": request_id
    }

@server.get("/health", include_in_schema=False)
async def health_check():
    return {"status": "healthy", "server_id": SERVER_ID}