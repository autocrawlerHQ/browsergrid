from fastapi import APIRouter, FastAPI

# Create the main API router
main_router = APIRouter()

def include_router(app: FastAPI):
    """
    Include all routes in the FastAPI app
    
    This function is responsible for including all routes from individual apps
    in the main FastAPI application.
    """
    # Import all app routers
    from browserfleet.server.webhooks.routes import router as webhooks_router
    from browserfleet.server.sessions.routes import router as sessions_router 
    from browserfleet.server.workerpool.routes import router as workerpool_router
    
    # Include all routers in the main API
    app.include_router(webhooks_router)
    app.include_router(sessions_router)
    app.include_router(workerpool_router)
    
    
    return app 