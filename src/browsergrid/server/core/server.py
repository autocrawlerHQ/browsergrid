"""
Main application module for the browsergrid server

This module defines the FastAPI application instance that can be imported
directly by Uvicorn for serving, including when using auto-reload.
"""
import sys
from contextlib import asynccontextmanager
from fastapi import FastAPI, Request

from browsergrid.server.core.settings import (
    API_HOST,
    API_PORT,
    API_KEY,
    API_AUTH,
    SERVER_ID,
    DEBUG,
    UI_ENABLED,
    MIDDLEWARE,
    DATABASE_URL,
)
from browsergrid.server.core.apps import apps
from browsergrid.server.core.router import include_router
from browsergrid.server.core.middleware_utils import apply_middlewares
from browsergrid.server.core.db.session import sessionmanager, settings

from loguru import logger

if not apps.apps_ready:
    apps.populate()

# Create an application lifespan context manager to handle database connection
@asynccontextmanager
async def lifespan(app: FastAPI):
    # Initialize database on startup
    logger.info("Initializing database connection")
    sessionmanager.init(settings.database_url)
    
    # Yield control to FastAPI
    yield
    
    # Cleanup database on shutdown
    logger.info("Closing database connections")
    if sessionmanager._engine is not None:
        await sessionmanager.close()

# Initialize the FastAPI application with the lifespan manager
server = FastAPI(
    title="browsergrid API",
    description="API for managing browser sessions",
    version="0.1.0",
    debug=DEBUG,
    lifespan=lifespan,
)

# Apply middleware
apply_middlewares(
    app=server, 
    middleware_config=MIDDLEWARE,
    api_key=API_KEY,
    excluded_paths=API_AUTH["excluded_paths"],
    debug=DEBUG
)

# Include routers
include_router(server)

# Mark apps as ready
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