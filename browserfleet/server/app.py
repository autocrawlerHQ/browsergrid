#!/usr/bin/env python3
"""
browserfleet API Server

This module implements the API server for browserfleet using FastAPI.
"""
import os
import sys
import argparse
import uuid
import json
from pathlib import Path
from typing import Dict, List, Optional, Any, Union

import uvicorn
from fastapi import FastAPI, Depends, HTTPException, status, Request, Response
from fastapi.middleware.cors import CORSMiddleware
from fastapi.responses import JSONResponse
from fastapi.staticfiles import StaticFiles
from fastapi.security import APIKeyHeader
from pydantic import BaseModel, Field

from browserfleet.server.services.session import SessionService
from browserfleet.server.services.config import ConfigService
from browserfleet.server.services.auth import AuthService
from browserfleet.server.models.session import Session, SessionCreate, SessionUpdate
from browserfleet.server.models.config import ConfigItem, ConfigUpdate

# API key header
API_KEY_HEADER = APIKeyHeader(name="X-API-Key")

def create_app(config: Dict[str, Any] = None) -> FastAPI:
    """
    Create and configure the FastAPI application
    
    Args:
        config: Configuration options
    
    Returns:
        FastAPI: Configured FastAPI application
    """
    # Set up configuration
    if not config:
        config = {}
    
    config_dir = config.get("config_dir", os.path.expanduser("~/.browserfleet"))
    host = config.get("host", "127.0.0.1")
    port = config.get("port", 8765)
    debug = config.get("debug", False)
    ui_enabled = config.get("ui_enabled", True)
    api_key = config.get("api_key")
    server_id = config.get("server_id", str(uuid.uuid4()))
    
    # Create services
    session_service = SessionService(config_dir=config_dir)
    config_service = ConfigService(config_dir=config_dir)
    auth_service = AuthService(config_dir=config_dir)
    
    # Initialize services
    session_service.initialize()
    config_service.initialize()
    auth_service.initialize()
    
    # Set up API key if provided
    if api_key:
        auth_service.set_api_key(api_key)
    
    # Create FastAPI app
    app = FastAPI(
        title="browserfleet API",
        description="API for managing browser sessions",
        version="0.1.0",
        debug=debug
    )
    
    # Add CORS middleware
    app.add_middleware(
        CORSMiddleware,
        allow_origins=["*"],
        allow_credentials=True,
        allow_methods=["*"],
        allow_headers=["*"],
    )
    
    # Function to validate API key
    async def validate_api_key(api_key: str = Depends(API_KEY_HEADER)) -> str:
        if auth_service.validate_api_key(api_key):
            return api_key
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail="Invalid API Key",
        )
    
    # Optional security dependency
    async def optional_api_key(api_key: Optional[str] = Depends(API_KEY_HEADER)) -> Optional[str]:
        if not api_key:
            return None
        
        if auth_service.validate_api_key(api_key):
            return api_key
        
        return None
    
    # Register API routes
    
    @app.get("/api/v1/health", tags=["Health"])
    async def health_check():
        """Check if the API server is healthy"""
        return {"status": "ok"}
    
    @app.get("/api/v1/info", tags=["Info"])
    async def server_info(api_key: Optional[str] = Depends(optional_api_key)):
        """Get server information"""
        server_info = {
            "id": server_id,
            "version": "0.1.0",
            "config_dir": config_dir,
            "host": host,
            "port": port,
            "ui_enabled": ui_enabled,
            "ephemeral": config.get("ephemeral", True),
            "auth_required": auth_service.auth_required(),
            "sessions": len(session_service.list_sessions()),
        }
        
        # Add more detailed info if authenticated
        if api_key:
            server_info["sessions_details"] = [
                {
                    "id": session.id,
                    "browser": session.browser,
                    "status": session.status
                }
                for session in session_service.list_sessions()
            ]
        
        return server_info
    
    # Session endpoints
    
    @app.post("/api/v1/sessions", response_model=Session, tags=["Sessions"])
    async def create_session(
        session_create: SessionCreate,
        api_key: Optional[str] = Depends(optional_api_key)
    ):
        """Create a new browser session"""
        if auth_service.auth_required() and not api_key:
            raise HTTPException(
                status_code=status.HTTP_401_UNAUTHORIZED,
                detail="Authentication required",
            )
        
        try:
            session = await session_service.create_session(session_create)
            return session
        except Exception as e:
            raise HTTPException(
                status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
                detail=str(e),
            )
    
    @app.get("/api/v1/sessions", response_model=List[Session], tags=["Sessions"])
    async def list_sessions(api_key: Optional[str] = Depends(optional_api_key)):
        """List all browser sessions"""
        if auth_service.auth_required() and not api_key:
            raise HTTPException(
                status_code=status.HTTP_401_UNAUTHORIZED,
                detail="Authentication required",
            )
        
        return session_service.list_sessions()
    
    @app.get("/api/v1/sessions/{session_id}", response_model=Session, tags=["Sessions"])
    async def get_session(
        session_id: str,
        api_key: Optional[str] = Depends(optional_api_key)
    ):
        """Get information about a specific session"""
        if auth_service.auth_required() and not api_key:
            raise HTTPException(
                status_code=status.HTTP_401_UNAUTHORIZED,
                detail="Authentication required",
            )
        
        session = session_service.get_session(session_id)
        if not session:
            raise HTTPException(
                status_code=status.HTTP_404_NOT_FOUND,
                detail=f"Session {session_id} not found",
            )
        
        return session
    
    @app.put("/api/v1/sessions/{session_id}", response_model=Session, tags=["Sessions"])
    async def update_session(
        session_id: str,
        session_update: SessionUpdate,
        api_key: str = Depends(validate_api_key)
    ):
        """Update a browser session"""
        session = session_service.get_session(session_id)
        if not session:
            raise HTTPException(
                status_code=status.HTTP_404_NOT_FOUND,
                detail=f"Session {session_id} not found",
            )
        
        try:
            updated_session = await session_service.update_session(session_id, session_update)
            return updated_session
        except Exception as e:
            raise HTTPException(
                status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
                detail=str(e),
            )
    
    @app.delete("/api/v1/sessions/{session_id}", tags=["Sessions"])
    async def delete_session(
        session_id: str,
        api_key: Optional[str] = Depends(optional_api_key)
    ):
        """Close and delete a browser session"""
        if auth_service.auth_required() and not api_key:
            raise HTTPException(
                status_code=status.HTTP_401_UNAUTHORIZED,
                detail="Authentication required",
            )
        
        session = session_service.get_session(session_id)
        if not session:
            raise HTTPException(
                status_code=status.HTTP_404_NOT_FOUND,
                detail=f"Session {session_id} not found",
            )
        
        try:
            success = await session_service.delete_session(session_id)
            return {"success": success}
        except Exception as e:
            raise HTTPException(
                status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
                detail=str(e),
            )
    
    @app.get("/api/v1/sessions/{session_id}/tabs", tags=["Sessions"])
    async def get_session_tabs(
        session_id: str,
        api_key: Optional[str] = Depends(optional_api_key)
    ):
        """Get tabs/pages for a browser session"""
        if auth_service.auth_required() and not api_key:
            raise HTTPException(
                status_code=status.HTTP_401_UNAUTHORIZED,
                detail="Authentication required",
            )
        
        session = session_service.get_session(session_id)
        if not session:
            raise HTTPException(
                status_code=status.HTTP_404_NOT_FOUND,
                detail=f"Session {session_id} not found",
            )
        
        try:
            tabs = await session_service.get_session_tabs(session_id)
            return tabs
        except Exception as e:
            raise HTTPException(
                status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
                detail=str(e),
            )
    
    # Configuration endpoints
    
    @app.get("/api/v1/config", response_model=Dict[str, Any], tags=["Configuration"])
    async def get_config(api_key: str = Depends(validate_api_key)):
        """Get configuration settings"""
        return config_service.get_all_config()
    
    @app.get("/api/v1/config/{key}", response_model=ConfigItem, tags=["Configuration"])
    async def get_config_item(key: str, api_key: str = Depends(validate_api_key)):
        """Get a specific configuration setting"""
        value = config_service.get_config(key)
        if value is None:
            raise HTTPException(
                status_code=status.HTTP_404_NOT_FOUND,
                detail=f"Configuration key {key} not found",
            )
        
        return {"key": key, "value": value}
    
    @app.put("/api/v1/config/{key}", response_model=ConfigItem, tags=["Configuration"])
    async def update_config_item(
        key: str,
        config_update: ConfigUpdate,
        api_key: str = Depends(validate_api_key)
    ):
        """Update a configuration setting"""
        try:
            value = config_service.set_config(key, config_update.value)
            return {"key": key, "value": value}
        except Exception as e:
            raise HTTPException(
                status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
                detail=str(e),
            )
    
    # Mount UI if enabled
    if ui_enabled:
        # Check if UI files exist
        ui_dir = Path(__file__).parent / "ui" / "build"
        if ui_dir.exists():
            app.mount("/ui", StaticFiles(directory=str(ui_dir), html=True), name="ui")
            
            @app.get("/", include_in_schema=False)
            async def redirect_to_ui():
                return JSONResponse(
                    status_code=status.HTTP_302_FOUND,
                    content={"url": "/ui"},
                    headers={"Location": "/ui"}
                )
    
    # Exception handlers
    @app.exception_handler(Exception)
    async def global_exception_handler(request: Request, exc: Exception):
        if debug:
            return JSONResponse(
                status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
                content={"detail": str(exc), "type": type(exc).__name__}
            )
        else:
            return JSONResponse(
                status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
                content={"detail": "Internal server error"}
            )
    
    return app

def main():
    """Run the browserfleet API server"""
    parser = argparse.ArgumentParser(description="browserfleet API Server")
    parser.add_argument("--host", default="127.0.0.1", help="Server host")
    parser.add_argument("--port", type=int, default=8765, help="Server port")
    parser.add_argument("--config-dir", help="Configuration directory")
    parser.add_argument("--debug", action="store_true", help="Enable debug mode")
    parser.add_argument("--ui", action="store_true", help="Enable UI")
    parser.add_argument("--api-key", help="API key for authentication")
    
    args = parser.parse_args()
    
    config = {
        "config_dir": args.config_dir or os.path.expanduser("~/.browserfleet"),
        "host": args.host,
        "port": args.port,
        "debug": args.debug,
        "ui_enabled": args.ui,
        "api_key": args.api_key,
        "ephemeral": False
    }
    
    app = create_app(config)
    
    uvicorn.run(
        app,
        host=args.host,
        port=args.port,
        log_level="debug" if args.debug else "info"
    )

if __name__ == "__main__":
    main()