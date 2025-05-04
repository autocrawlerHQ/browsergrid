"""
Azure Container Instances provider for browsergrid infrastructure
"""
from typing import Dict, Any, List
import os
import sys
import platform

from browsergrid.client.providers import ProviderBase, register_provider


@register_provider
class AzureContainerInstanceProvider(ProviderBase):
    """Azure Container Instances provider for provisioning browser sessions"""
    
    provider_type = "azure_container_instance"
    display_name = "Azure Container Instances"
    description = "Run browser sessions in Azure Container Instances"
    
    def __init__(
        self,
        name: str = "azure-pool",
        browser: str = "chrome",
        browser_version: str = "latest",
        headless: bool = True,
        resource_group: str = None,
        subscription_id: str = None,
        location: str = "eastus",
        cpu: int = 2,
        memory_gb: int = 4,
        timeout_minutes: int = 60,
        registry: str = None,
        image_prefix: str = "browserless",
    ):
        """Initialize Azure Container Instances provider"""
        self.name = name
        self.browser = browser
        self.browser_version = browser_version
        self.headless = headless
        self.resource_group = resource_group
        self.subscription_id = subscription_id
        self.location = location
        self.cpu = cpu
        self.memory_gb = memory_gb
        self.timeout_minutes = timeout_minutes
        self.registry = registry
        self.image_prefix = image_prefix
    
    def get_pool_config(self) -> Dict[str, Any]:
        """Get Azure Container Instances pool configuration"""
        return {
            "name": self.name,
            "provider_type": self.provider_type,
            "default_browser": self.browser,
            "default_browser_version": self.browser_version,
            "default_headless": self.headless,
            "default_operating_system": "linux",
            "default_resource_limits": {
                "cpu": self.cpu,
                "memory": f"{self.memory_gb}G",
                "timeout_minutes": self.timeout_minutes
            },
            "provider_config": {
                "resource_group": self.resource_group,
                "subscription_id": self.subscription_id,
                "location": self.location,
                "registry": self.registry,
                "image_prefix": self.image_prefix
            },
            "description": f"Azure Container Instances pool in {self.location}"
        }
    
    def get_deploy_command(self, session_id: str, session_details: Dict[str, Any]) -> str:
        """Generate Azure CLI command to deploy a browser session"""
        # Determine browser and version
        browser = session_details.get("browser", self.browser)
        version = session_details.get("version", self.browser_version)
        headless = session_details.get("headless", self.headless)
        
        # Resource limits
        resource_limits = session_details.get("resource_limits", {})
        cpu = resource_limits.get("cpu", self.cpu)
        memory_gb = int(resource_limits.get("memory", f"{self.memory_gb}G").rstrip("G"))
        
        # Determine image
        image_prefix = self.registry + "/" if self.registry else ""
        image_prefix += self.image_prefix + "/"
        
        image = f"{image_prefix}{browser}:{version}"
        
        # Subscription parameter
        subscription_param = f"--subscription {self.subscription_id}" if self.subscription_id else ""
        
        # Generate command
        container_name = f"browsergrid-{session_id[:8]}"
        
        cmd = [
            "az container create",
            f"--resource-group {self.resource_group}",
            subscription_param,
            f"--location {self.location}",
            f"--name {container_name}",
            f"--image {image}",
            f"--cpu {cpu}",
            f"--memory {memory_gb}",
            "--ports 3000",
            "--ip-address public",
            "--dns-name-label " + container_name,
            f"--environment-variables BROWSERLESS_HEADLESS={'true' if headless else 'false'} BROWSERLESS_SESSION_ID={session_id}"
        ]
        
        # Filter out empty elements
        cmd = [c for c in cmd if c]
        
        return " \\\n  ".join(cmd)
    
    @classmethod
    def get_cli_options(cls) -> List[Dict[str, Any]]:
        """Get CLI options for Azure Container Instances provider"""
        return [
            {
                "name": "--name",
                "default": "azure-pool",
                "help": "Name for the Azure work pool",
                "dest": "name",
            },
            {
                "name": "--resource-group",
                "required": True,
                "help": "Azure resource group name",
                "dest": "resource_group",
            },
            {
                "name": "--subscription-id",
                "help": "Azure subscription ID (uses default from az CLI if not provided)",
                "dest": "subscription_id",
            },
            {
                "name": "--location",
                "default": "eastus",
                "help": "Azure region to deploy container instances",
                "dest": "location",
            },
            {
                "name": "--browser",
                "default": "chrome",
                "choices": ["chrome", "firefox"],
                "help": "Default browser for the pool",
                "dest": "browser",
            },
            {
                "name": "--browser-version",
                "default": "latest",
                "choices": ["latest", "stable", "dev", "canary"],
                "help": "Default browser version",
                "dest": "browser_version",
            },
            {
                "name": "--headless",
                "action": "store_true",
                "default": True,
                "help": "Run browsers in headless mode by default",
                "dest": "headless",
            },
            {
                "name": "--cpu",
                "default": 2,
                "type": int, 
                "help": "CPU cores per container",
                "dest": "cpu",
            },
            {
                "name": "--memory-gb",
                "default": 4,
                "type": int,
                "help": "Memory in GB per container",
                "dest": "memory_gb",
            },
            {
                "name": "--timeout-minutes",
                "default": 60,
                "type": int,
                "help": "Default session timeout in minutes",
                "dest": "timeout_minutes",
            },
            {
                "name": "--registry",
                "help": "Container registry to use",
                "dest": "registry",
            },
            {
                "name": "--image-prefix",
                "default": "browserless",
                "help": "Image prefix for browser images",
                "dest": "image_prefix",
            }
        ] 