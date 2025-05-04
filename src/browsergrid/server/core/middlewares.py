"""
Middleware components for browsergrid

This module contains middleware components for the FastAPI application.
"""
import time
import uuid
from typing import Optional, Dict, Any

from fastapi import Request, Response, status
from starlette.middleware.base import BaseHTTPMiddleware, RequestResponseEndpoint
from starlette.types import ASGIApp

from loguru import logger


class Middleware(BaseHTTPMiddleware):
    """Base middleware class for request/response hooks"""
    
    def __init__(self, app: ASGIApp, **options: Any):
        super().__init__(app)
        self.options = options
    
    async def dispatch(self, request: Request, call_next: RequestResponseEndpoint) -> Response:
        # Process the request
        modified_request = await self.process_request(request)
        if isinstance(modified_request, Response):
            return modified_request
        
        # Call the next middleware
        response = await call_next(request)
        
        # Process the response
        return await self.process_response(request, response)
    
    async def process_request(self, request: Request) -> Request:
        """Process the request before it's handled by the view"""
        return request
    
    async def process_response(self, request: Request, response: Response) -> Response:
        """Process the response after it's been handled by the view"""
        return response


class ApiKeyMiddleware(Middleware):
    """
    Middleware to handle API key authentication
    
    This middleware checks for a valid API key in incoming requests using the X-API-Key header.
    If no API key is configured or the request is an OPTIONS request, the check is skipped.
    """
    
    async def process_request(self, request: Request) -> Request:
        # Get API key from options
        api_key = self.options.get('api_key')
        
        # Skip API key check in these cases:
        # 1. No API key is configured
        # 2. It's an OPTIONS request (for CORS preflight)
        # 3. If the request is to a path that doesn't require authentication (if configured)
        if (not api_key or 
            request.method == "OPTIONS" or 
            self._is_excluded_path(request.url.path)):
            return request
        
        # Extract the API key from the header
        request_api_key = request.headers.get("X-API-Key")
        
        # Check if the API key is valid
        if not self._is_valid_api_key(request_api_key, api_key):
            return Response(
                status_code=status.HTTP_401_UNAUTHORIZED,
                content='{"error": "Invalid or missing API key"}',
                media_type="application/json"
            )
        
        # Add a flag to request state to indicate it's been authenticated
        request.state.api_key_authenticated = True
        return request
    
    def _is_excluded_path(self, path: str) -> bool:
        """Check if the path is excluded from API key authentication"""
        excluded_paths = self.options.get('excluded_paths', [])
        return any(path.startswith(excluded) for excluded in excluded_paths)
    
    def _is_valid_api_key(self, request_key: Optional[str], config_key: str) -> bool:
        """Validate the provided API key against the configured key"""
        if not request_key:
            return False
            
        # Simple equality check - could be extended for more complex validation
        return request_key == config_key


class RequestLogMiddleware(Middleware):
    """Middleware to log requests and add request IDs"""
    
    async def process_request(self, request: Request) -> Request:
        # Add timing info and request ID
        request.state.start_time = time.time()
        request.state.request_id = str(uuid.uuid4())
        
        # Log request if needed
        log_level = self.options.get('log_level', 'info')
        if log_level != 'none':
            logger.info(f"[{request.state.request_id}] {request.method} {request.url.path}")
        
        return request
    
    async def process_response(self, request: Request, response: Response) -> Response:
        # Add request ID to response headers
        request_id = getattr(request.state, 'request_id', 'unknown')
        response.headers["X-Request-ID"] = request_id
        
        # Log response if needed
        if hasattr(request.state, 'start_time'):
            process_time = time.time() - request.state.start_time
            log_level = self.options.get('log_level', 'info')
            if log_level != 'none':
                logger.info(f"[{request_id}] {response.status_code} ({process_time:.3f}s)")
        
        return response


class RateLimitMiddleware(Middleware):
    """Basic rate limiting by IP address"""
    
    def __init__(self, app: ASGIApp, **options: Any):
        super().__init__(app, **options)
        self.requests = {}  # IP -> [(timestamp, count)]
        self.blocked = {}   # IP -> unblock_time
    
    async def process_request(self, request: Request) -> Request:
        client_ip = request.client.host
        requests_per_minute = self.options.get('requests_per_minute', 60)
        block_time = self.options.get('block_time', 60)
        current_time = time.time()
        
        # Check if IP is blocked
        if client_ip in self.blocked and current_time < self.blocked[client_ip]:
            return Response(
                status_code=status.HTTP_429_TOO_MANY_REQUESTS,
                content='{"error": "Too many requests"}',
                media_type="application/json"
            )
        elif client_ip in self.blocked:
            del self.blocked[client_ip]
        
        # Initialize and clean up request tracking
        if client_ip not in self.requests:
            self.requests[client_ip] = []
        
        self.requests[client_ip] = [
            req for req in self.requests[client_ip] 
            if current_time - req[0] < 60
        ]
        
        # Check rate limit
        request_count = sum(req[1] for req in self.requests[client_ip])
        if request_count >= requests_per_minute:
            self.blocked[client_ip] = current_time + block_time
            return Response(
                status_code=status.HTTP_429_TOO_MANY_REQUESTS,
                content='{"error": "Too many requests"}',
                media_type="application/json"
            )
        
        # Track this request
        self.requests[client_ip].append((current_time, 1))
        return request

