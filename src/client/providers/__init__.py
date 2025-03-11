"""
Browsergrid provider registry for work pools

This module contains provider definitions for different infrastructure types
that the orchestration layer can use to deploy browser sessions.
"""
from abc import ABC, abstractmethod
from typing import Dict, Any, Type, Optional, List


class ProviderBase(ABC):
    """Base class for infrastructure providers"""
    
    provider_type: str = "base"
    display_name: str = "Base Provider"
    description: str = "Base provider class"
    
    @abstractmethod
    def get_pool_config(self) -> Dict[str, Any]:
        """Get the configuration for creating a work pool with this provider"""
        pass
    
    @abstractmethod
    def get_deploy_command(self, session_id: str, session_details: Dict[str, Any]) -> str:
        """
        Generate a command that would deploy a browser session with this provider
        
        This is used for debugging and demonstration purposes. The actual
        deployment is handled by the orchestration layer.
        """
        pass
    
    @classmethod
    def get_cli_options(cls) -> List[Dict[str, Any]]:
        """Get CLI options for this provider"""
        return []


# Provider registry
_PROVIDERS: Dict[str, Type[ProviderBase]] = {}


def register_provider(provider_class: Type[ProviderBase]):
    """Register a provider class"""
    _PROVIDERS[provider_class.provider_type] = provider_class
    return provider_class


def get_provider(provider_type: str) -> Optional[Type[ProviderBase]]:
    """Get a provider by type"""
    return _PROVIDERS.get(provider_type)


def get_all_providers() -> Dict[str, Type[ProviderBase]]:
    """Get all registered providers"""
    return _PROVIDERS.copy() 