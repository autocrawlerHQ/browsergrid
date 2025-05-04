"""
Docker provider for browsergrid infrastructure
"""
from typing import Dict, Any, List
import os
import sys
import platform

from browsergrid.client.providers import ProviderBase, register_provider


@register_provider
class DockerProvider(ProviderBase):
    """Docker provider for provisioning browser sessions in Docker containers"""
    
    provider_type = "docker"
    display_name = "Docker"
    description = "Run browser sessions in Docker containers"
    
    def __init__(
        self,
        name: str = "docker-pool",
        browser: str = "chrome",
        browser_version: str = "latest",
        headless: bool = False,
        network: str = "bridge",
        timeout_minutes: int = 60,
        cpu_limit: float = 2.0,
        memory_limit: str = "4G",
        registry: str = None,
        image_prefix: str = "browserless",
    ):
        """Initialize Docker provider"""
        self.name = name
        self.browser = browser
        self.browser_version = browser_version
        self.headless = headless
        self.network = network
        self.timeout_minutes = timeout_minutes
        self.cpu_limit = cpu_limit
        self.memory_limit = memory_limit
        self.registry = registry
        self.image_prefix = image_prefix
    
    def get_pool_config(self) -> Dict[str, Any]:
        """Get Docker pool configuration"""
        return {
            "name": self.name,
            "provider_type": self.provider_type,
            "default_browser": self.browser,
            "default_browser_version": self.browser_version,
            "default_headless": self.headless,
            "default_operating_system": "linux",
            "default_resource_limits": {
                "cpu": self.cpu_limit,
                "memory": self.memory_limit,
                "timeout_minutes": self.timeout_minutes
            },
            "provider_config": {
                "network": self.network,
                "registry": self.registry,
                "image_prefix": self.image_prefix,
            },
            "description": f"Docker pool for {self.browser} sessions"
        }
    
    def get_deploy_command(self, session_id: str, session_details: Dict[str, Any]) -> str:
        """Generate Docker command to deploy a browser session"""
        # Determine browser and version
        browser = session_details.get("browser", self.browser)
        version = session_details.get("version", self.browser_version)
        headless = session_details.get("headless", self.headless)
        
        # Resource limits
        resource_limits = session_details.get("resource_limits", {})
        cpu = resource_limits.get("cpu", self.cpu_limit)
        memory = resource_limits.get("memory", self.memory_limit)
        
        # Determine image
        image_prefix = self.registry + "/" if self.registry else ""
        image_prefix += self.image_prefix + "/"
        
        image = f"{image_prefix}{browser}:{version}"
        
        # Environment variables
        env_vars = " ".join([
            f"-e BROWSERLESS_SESSION_ID={session_id}",
            f"-e BROWSERLESS_HEADLESS={'true' if headless else 'false'}"
        ])
        
        # Generate command
        cmd = [
            "docker run -d",
            "--name browsergrid-" + session_id[:8],
            f"--cpus={cpu}",
            f"--memory={memory}",
            f"--network={self.network}",
            "-p 3000:3000",
            env_vars,
            image
        ]
        
        return " \\\n  ".join(cmd)
    
    @classmethod
    def get_cli_options(cls) -> List[Dict[str, Any]]:
        """Get CLI options for Docker provider"""
        return [
            {
                "name": "--name",
                "default": "docker-pool",
                "help": "Name for the Docker work pool",
                "dest": "name",
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
                "help": "Run browsers in headless mode by default",
                "dest": "headless",
            },
            {
                "name": "--network",
                "default": "bridge",
                "help": "Docker network to use",
                "dest": "network",
            },
            {
                "name": "--timeout-minutes",
                "default": 60,
                "type": int,
                "help": "Default session timeout in minutes",
                "dest": "timeout_minutes",
            },
            {
                "name": "--cpu-limit",
                "default": 2.0,
                "type": float,
                "help": "CPU limit per container",
                "dest": "cpu_limit",
            },
            {
                "name": "--memory-limit",
                "default": "4G",
                "help": "Memory limit per container (e.g., '2G', '512M')",
                "dest": "memory_limit",
            },
            {
                "name": "--registry",
                "help": "Docker registry to use",
                "dest": "registry",
            },
            {
                "name": "--image-prefix",
                "default": "browserless",
                "help": "Image prefix for browser images",
                "dest": "image_prefix",
            }
        ] 