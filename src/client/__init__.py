"""
Browsergrid client module

This module provides client functionality for interacting with Browsergrid.
"""
# Import provider modules to register them
from src.client.providers import get_provider, get_all_providers

# Import provider implementations
try:
    # These imports register the providers
    from src.client.providers.docker import DockerProvider
    from src.client.providers.azure import AzureContainerInstanceProvider
except ImportError:
    # Handle import errors for optional dependencies
    pass 