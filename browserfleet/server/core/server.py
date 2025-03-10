"""
Server initialization and management for browserfleet

This module provides a unified interface for creating, starting, and managing
the browserfleet server. It handles all the necessary setup for running the API server.
"""
import sys
import signal
import uvicorn
import time
from typing import Optional
from fastapi import FastAPI, Request

from browserfleet.server.core.settings import (
    API_HOST,
    API_PORT,
    API_KEY,
    API_AUTH,
    SERVER_ID,
    DEBUG,
    UI_ENABLED,
    MIDDLEWARE,
)
from browserfleet.server.core.apps import apps
from browserfleet.server.core.router import include_router
from browserfleet.server.core.middleware_utils import apply_middlewares

class ServerInstance:
    """
    Server instance class that manages the lifecycle of the browserfleet server.
    """
    def __init__(
        self,
        host: str = API_HOST,
        port: int = API_PORT,
        api_key: Optional[str] = API_KEY,
        server_id: str = SERVER_ID,
        debug: bool = DEBUG,
        ui_enabled: bool = UI_ENABLED,
    ):
        self.host = host
        self.port = port
        self.server_id = server_id
        self.debug = debug
        self.ui_enabled = ui_enabled
        
        # API authentication configuration
        self.api_key = api_key
        self.auth_config = API_AUTH.copy()
        if api_key is not None:
            self.auth_config.update({
                "key": api_key
            })
        
        self.app = None
        self._server = None
        self._is_running = False
        self._running_app = None
    
    @property
    def server_url(self) -> str:
        """Get the server URL"""
        protocol = "http"  # TODO: Add HTTPS support ?
        return f"{protocol}://{self.host}:{self.port}"
    
    @property
    def is_running(self) -> bool:
        """Check if the server is running"""
        return self._is_running
    
    def create_app(self) -> FastAPI:
        """
        Create and configure the FastAPI application

        This method is similar to the create_app function in app.py, but uses
        the settings and app registry for configuration.
        """
        # Ensure app registry is populated
        if not apps.apps_ready:
            apps.populate()
        
        # Create FastAPI app
        app = FastAPI(
            title="browserfleet API",
            description="API for managing browser sessions",
            version="0.1.0",
            debug=self.debug,
        )
        
        # Apply middlewares using the unified approach
        apply_middlewares(
            app=app, 
            middleware_config=MIDDLEWARE,
            # Provide instance-specific values that can override middleware kwargs
            api_key=self.auth_config["key"],
            excluded_paths=self.auth_config["excluded_paths"],
            debug=self.debug
        )
        
        # Include all routes using the centralized router
        include_router(app)
        
        # Trigger the ready method on all app configs
        apps.ready()
        
        # Add global exception handler
        @app.exception_handler(Exception)
        async def global_exception_handler(request: Request, exc: Exception):
            # Log the exception
            request_id = getattr(request.state, "request_id", "unknown")
            print(f"[{request_id}] Global exception: {exc}", file=sys.stderr)
            
            # Return a 500 error
            return {
                "error": "Internal server error",
                "detail": str(exc) if self.debug else "An internal server error occurred",
                "request_id": request_id
            }
        
        self.app = app
        return app
    
    def start(self, block: bool = True) -> bool:
        """
        Start the server
        
        Args:
            block: Whether to block the current thread while the server runs
            
        Returns:
            bool: Whether the server was started successfully
        """
        if self.is_running:
            print(f"Server is already running at {self.server_url}")
            return True
        
        if self.app is None:
            self.create_app()
        
        print(f"Starting browserfleet server at {self.server_url}")
        
        # Set up signal handling
        def handle_signal(signum, frame):
            print("Shutting down browserfleet server...")
            self.stop()
            sys.exit(0)
        
        signal.signal(signal.SIGINT, handle_signal)
        signal.signal(signal.SIGTERM, handle_signal)
        
        if block:
            # Run the server in the current thread
            self._is_running = True
            uvicorn.run(
                self.app,
                host=self.host,
                port=self.port,
                log_level="debug" if self.debug else "info",
            )
            return True
        else:
            # Run the server in a separate thread
            config = uvicorn.Config(
                self.app,
                host=self.host,
                port=self.port,
                log_level="debug" if self.debug else "info",
            )
            self._server = uvicorn.Server(config)
            
            # Start the server in a new thread
            import threading
            
            def run_server():
                self._is_running = True
                self._server.run()
                self._is_running = False
            
            thread = threading.Thread(target=run_server)
            thread.daemon = True
            thread.start()
            
            # Wait for the server to start
            for _ in range(10):
                if self._is_running:
                    break
                time.sleep(0.5)
            
            return self._is_running
    
    def stop(self) -> bool:
        """
        Stop the server
        
        Returns:
            bool: Whether the server was stopped successfully
        """
        if not self.is_running:
            print("Server is not running")
            return True
        
        if self._server:
            self._server.should_exit = True
            self._is_running = False
            return True
        
        return False

# Global server instance
_SERVER_INSTANCE = None

def get_server(
    host: str = API_HOST,
    port: int = API_PORT,
    api_key: Optional[str] = API_KEY,
    server_id: str = SERVER_ID,
    debug: bool = DEBUG,
    ui_enabled: bool = UI_ENABLED,
) -> ServerInstance:
    """
    Get or create a server instance
    
    Args:
        host: Server host
        port: Server port
        api_key: API key for authentication
        server_id: Server ID
        debug: Whether to enable debug mode
        ui_enabled: Whether to enable the UI
        
    Returns:
        ServerInstance: Server instance
    """
    global _SERVER_INSTANCE
    
    if _SERVER_INSTANCE is None:
        _SERVER_INSTANCE = ServerInstance(
            host=host,
            port=port,
            api_key=api_key,
            server_id=server_id,
            debug=debug,
            ui_enabled=ui_enabled,
        )
    
    return _SERVER_INSTANCE 