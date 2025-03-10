"""
Application configuration for the webhooks app
"""
from browserfleet.server.core.apps import AppConfig

class WebhooksConfig(AppConfig):
    """Configuration for the webhooks app"""
    name = "webhooks"
    verbose_name = "CDP Webhooks"
    models_module = "browserfleet.server.webhooks.models"
    
    def ready(self):
        """
        Initialization code for the webhooks app
        
        This method is called when the app is ready to be used.
        """
        print(f"Initializing {self.verbose_name} app")
        
        # Any custom initialization for webhooks could go here
        # For example, registering signal handlers, starting background tasks, etc. 