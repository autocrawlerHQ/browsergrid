"""
Application registry for browsergrid server

This module provides a simple application registry system. Each app in the
INSTALLED_APPS setting can define an AppConfig class to customize its behavior.
"""
import importlib
from typing import Dict, List, Optional, Type, Any

from browsergrid.server.core.settings import INSTALLED_APPS

class AppConfig:
    """Base class for application configuration"""
    
    name: str = None
    verbose_name: str = None
    models_module: Optional[str] = None
    
    def __init__(self, app_name: str):
        if not self.name:
            self.name = app_name.split('.')[-1]
        
        if not self.verbose_name:
            self.verbose_name = self.name.title()
            
        if not self.models_module:
            try:
                self.models_module = f"{app_name}.models"
                importlib.import_module(self.models_module)
            except ImportError:
                self.models_module = None
    
    def ready(self):
        """
        Called when the app is ready. Override this method to perform
        initialization tasks when the app is ready.
        """
        pass

class AppRegistry:
    """Registry for application configurations"""
    
    def __init__(self):
        self.app_configs: Dict[str, AppConfig] = {}
        self.apps_ready = False
        self.ready_called = False
        
    def populate(self, installed_apps: Optional[List[str]] = None):
        """
        Load application configurations for all installed apps
        """
        if installed_apps is None:
            installed_apps = INSTALLED_APPS
            
        self.app_configs = {}
        
        for app_name in installed_apps:
            self.load_app_config(app_name)
            
        self.apps_ready = True
        
    def load_app_config(self, app_name: str) -> AppConfig:
        """
        Load the AppConfig for a single app
        """
        if app_name in self.app_configs:
            return self.app_configs[app_name]
        
        app_config = None
        try:
            module = importlib.import_module(f"{app_name}.apps")
            for item_name in dir(module):
                item = getattr(module, item_name)
                if isinstance(item, type) and issubclass(item, AppConfig) and item is not AppConfig:
                    app_config = item(app_name)
                    break
        except ImportError:
            pass
            
        if app_config is None:
            app_config = AppConfig(app_name)
            
        self.app_configs[app_name] = app_config
        return app_config
    
    def get_app_configs(self) -> List[AppConfig]:
        """
        Return a list of all application configurations
        """
        return list(self.app_configs.values())
    
    def ready(self):
        """
        Trigger the ready() method on all app configs
        """
        if self.ready_called:
            return
            
        for app_config in self.get_app_configs():
            app_config.ready()
            
        self.ready_called = True

apps = AppRegistry()

def autodiscover_modules(module_name: str):
    """
    Auto-discover modules with the given name in all installed apps,
    """
    for app_config in apps.get_app_configs():
        app_name = app_config.name
        try:
            importlib.import_module(f"{app_name}.{module_name}")
        except ImportError:
            pass 