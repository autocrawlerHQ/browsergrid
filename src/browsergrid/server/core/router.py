from fastapi import APIRouter, FastAPI

main_router = APIRouter()

def include_router(app: FastAPI):
    """
    Include all routes in the FastAPI app
    
    This function is responsible for including all routes from individual apps
    in the main FastAPI application.
    """
    from browsergrid.server.webhooks.routes import router as webhooks_router
    from browsergrid.server.sessions.routes import router as sessions_router 
    from browsergrid.server.workerpool.routes import router as workerpool_router
    
    # ensure all routes start with /api
    app.include_router(webhooks_router, prefix='/api')
    app.include_router(sessions_router, prefix='/api')
    app.include_router(workerpool_router, prefix='/api')
    
    
    return app 