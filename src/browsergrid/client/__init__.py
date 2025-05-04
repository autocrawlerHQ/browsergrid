"""
Browsergrid client module

This module provides client functionality for interacting with Browsergrid.
"""
# Import provider modules to register them
from browsergrid.client.providers import get_provider, get_all_providers

# Import provider implementations
try:
    # These imports register the providers
    from browsergrid.client.providers.docker import DockerProvider
    from browsergrid.client.providers.azure import AzureContainerInstanceProvider
except ImportError:
    # Handle import errors for optional dependencies
    pass 