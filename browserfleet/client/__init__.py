"""
BrowserFleet client module

This module provides client functionality for interacting with BrowserFleet.
"""
# Import provider modules to register them
from browserfleet.client.providers import get_provider, get_all_providers

# Import provider implementations
try:
    # These imports register the providers
    from browserfleet.client.providers.docker import DockerProvider
    from browserfleet.client.providers.azure import AzureContainerInstanceProvider
except ImportError:
    # Handle import errors for optional dependencies
    pass 