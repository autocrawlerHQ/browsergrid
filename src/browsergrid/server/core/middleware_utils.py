import importlib
from typing import Dict, Any, List, Optional, Type, Union

from fastapi import FastAPI
from browsergrid.server.core.settings import MIDDLEWARE
from loguru import logger

def import_string(dotted_path: str) -> Any:
    """Import a dotted module path and return the attribute/class"""
    try:
        module_path, class_name = dotted_path.rsplit('.', 1)
        module = importlib.import_module(module_path)
        return getattr(module, class_name)
    except (ValueError, ImportError, AttributeError) as err:
        raise ImportError(f"Error importing {dotted_path}: {err}") from err


def register_middleware(
    class_path: str,
    kwargs: Dict[str, Any] = None,
    position: int = -1,  # -1 means append, 0 means insert at beginning
) -> bool:
    """Register a new middleware to the MIDDLEWARE list"""
    try:
        import_string(class_path)
        
        # Create the middleware config
        middleware_config = {
            "class": class_path,
            "kwargs": kwargs or {},
        }
        
        # Add the middleware to the list
        if position < 0:
            MIDDLEWARE.append(middleware_config)
        else:
            MIDDLEWARE.insert(position, middleware_config)
        
        return True
    except Exception as e:
        logger.error(f"Error registering middleware {class_path}: {e}")
        return False


def apply_middlewares(app: FastAPI, middleware_config: List[Dict[str, Any]], **global_kwargs: Any) -> FastAPI:
    """Apply middleware to a FastAPI application based on configuration"""
    for middleware in middleware_config:
        middleware_class_path = middleware.get('class')
        if not middleware_class_path:
            continue
        
        try:
            middleware_class = import_string(middleware_class_path)
        except ImportError:
            continue
        
        kwargs = middleware.get('kwargs', {}).copy()
        
        # Update kwargs with global kwargs if the key exists in both
        for key, value in global_kwargs.items():
            if key in kwargs:
                # Allow overriding None values with provided global values
                if kwargs[key] is None:
                    kwargs[key] = value
        
        try:
            app.add_middleware(middleware_class, **kwargs)
        except Exception as e:
            logger.error(f"Error applying middleware {middleware_class_path}: {e}")
            continue
    
    return app 