#!/usr/bin/env python
"""
Browsergrid Work Pool CLI

Command-line tool for creating and managing Browsergrid work pools.
"""
import os
import sys
import argparse
import json
import requests
from typing import Dict, Any, Optional
from urllib.parse import urljoin

# Import provider registry
from browsergrid.client.providers import get_provider, get_all_providers


class WorkPoolCLI:
    """Command-line interface for managing work pools"""
    
    def __init__(self, api_url: str, api_key: str):
        """Initialize the CLI with API credentials"""
        self.api_url = api_url.rstrip('/')
        self.api_key = api_key
    
    def _make_api_request(
        self, 
        method: str, 
        endpoint: str, 
        json_data: Optional[Dict[str, Any]] = None
    ) -> Dict[str, Any]:
        """Make an API request to the Browsergrid server"""
        url = urljoin(self.api_url, endpoint)
        headers = {
            "X-API-Key": self.api_key,
            "Content-Type": "application/json"
        }
        
        try:
            response = requests.request(
                method=method,
                url=url,
                headers=headers,
                json=json_data,
                timeout=30
            )
            
            if response.status_code >= 400:
                print(f"API request failed: {response.status_code} - {response.text}")
                return {"error": response.text}
            
            return response.json() if response.content else {}
            
        except requests.RequestException as e:
            print(f"API request error: {str(e)}")
            return {"error": str(e)}
    
    def create_pool(self, config: Dict[str, Any]) -> Dict[str, Any]:
        """Create a new work pool"""
        return self._make_api_request("POST", "/v1/workerpool/pools", config)
    
    def list_pools(self) -> Dict[str, Any]:
        """List all work pools"""
        return self._make_api_request("GET", "/v1/workerpool/pools")
    
    def get_pool(self, pool_id: str) -> Dict[str, Any]:
        """Get details for a specific work pool"""
        return self._make_api_request("GET", f"/v1/workerpool/pools/{pool_id}")
    
    def delete_pool(self, pool_id: str, force: bool = False) -> Dict[str, Any]:
        """Delete a work pool"""
        force_param = "?force=true" if force else ""
        return self._make_api_request("DELETE", f"/v1/workerpool/pools/{pool_id}{force_param}")
    
    def list_workers(self, pool_id: Optional[str] = None) -> Dict[str, Any]:
        """List workers, optionally filtered by pool ID"""
        endpoint = "/v1/workerpool/workers"
        if pool_id:
            endpoint += f"?work_pool_id={pool_id}"
        return self._make_api_request("GET", endpoint)
    
    def deploy_session_cmd(self, pool_id: str, session_id: str) -> None:
        """Get an example command to deploy a session directly with a specific provider"""
        # Get pool info
        pool = self.get_pool(pool_id)
        if "error" in pool:
            print(f"Failed to get pool: {pool['error']}")
            return
        
        # Get session info
        session = self._make_api_request("GET", f"/v1/sessions/{session_id}")
        if "error" in session:
            print(f"Failed to get session: {session['error']}")
            return
        
        # Get provider
        provider_type = pool.get("provider_type")
        provider_class = get_provider(provider_type)
        if not provider_class:
            print(f"Unknown provider type: {provider_type}")
            return
        
        # Create provider instance with pool configuration
        provider_config = {}
        for key, value in pool.items():
            if key.startswith("default_") and key[8:] in provider_class.__init__.__code__.co_varnames:
                provider_config[key[8:]] = value
        
        provider_config["name"] = pool.get("name", "example-pool")
        if "provider_config" in pool and isinstance(pool["provider_config"], dict):
            provider_config.update(pool["provider_config"])
        
        provider = provider_class(**provider_config)
        
        # Get session details
        session_details = {
            "browser": session.get("browser"),
            "version": session.get("version"),
            "headless": session.get("headless"),
            "resource_limits": session.get("resource_limits"),
            "environment": session.get("environment", {})
        }
        
        # Get deploy command
        command = provider.get_deploy_command(session_id, session_details)
        
        print("\n==================================================")
        print(f"Example command to deploy session with {provider.display_name}")
        print("==================================================")
        print("\nThis is for demonstration purposes only. In a real deployment,")
        print("the orchestration layer would handle this automatically.\n")
        print(command)
        print("\n==================================================")
    
    def start_worker(self, pool_id: str, worker_name: Optional[str] = None) -> None:
        """Start a worker for a specific pool"""
        # Generate worker command
        python_executable = sys.executable
        
        # Determine worker name based on hostname
        import platform
        default_name = f"worker-{platform.node()}"
        worker_name = worker_name or default_name
        
        command = [
            f"{python_executable} -m browsergrid.client.worker",
            f"--api-url {self.api_url}",
            f"--api-key {self.api_key}",
            f"--work-pool-id {pool_id}",
            f"--worker-name {worker_name}"
        ]
        
        print("\n==================================================")
        print(f"Start infrastructure-agnostic worker for pool {pool_id}")
        print("==================================================")
        print("\nRun the following command in a terminal:")
        print(f"\n{' \\\n  '.join(command)}\n")
        print("==================================================")
        print("\nThis worker will poll for sessions from the work pool and")
        print("track their status. The actual deployment of browser sessions")
        print("is handled by the orchestration layer based on the work pool")
        print("provider configuration.")
        print("==================================================")


def main():
    """Main entry point for the CLI"""
    # Create the parser
    parser = argparse.ArgumentParser(
        description="Browsergrid Work Pool Management CLI"
    )
    
    # Add common arguments
    parser.add_argument(
        "--api-url",
        required=True,
        help="Browsergrid API URL",
        default=os.environ.get("BROWSERGRID_API_URL")
    )
    parser.add_argument(
        "--api-key",
        required=True,
        help="Browsergrid API key",
        default=os.environ.get("BROWSERGRID_API_KEY")
    )
    
    # Create subparsers for commands
    subparsers = parser.add_subparsers(dest="command", help="Command to execute")
    
    # Create pool command
    create_parser = subparsers.add_parser("create", help="Create a new work pool")
    
    # Add provider type argument to create command
    providers = get_all_providers()
    provider_types = list(providers.keys())
    
    create_parser.add_argument(
        "provider",
        choices=provider_types,
        help="Provider type for the work pool"
    )
    
    # Add dynamic provider-specific arguments
    for provider_type, provider_class in providers.items():
        for option in provider_class.get_cli_options():
            option_name = option.pop("name")
            create_parser.add_argument(
                option_name, 
                **option,
                help=f"[{provider_type}] {option.get('help', '')}"
            )
    
    # List pools command
    list_parser = subparsers.add_parser("list", help="List all work pools")
    
    # Get pool command
    get_parser = subparsers.add_parser("get", help="Get details for a work pool")
    get_parser.add_argument("pool_id", help="Work pool ID")
    
    # Delete pool command
    delete_parser = subparsers.add_parser("delete", help="Delete a work pool")
    delete_parser.add_argument("pool_id", help="Work pool ID")
    delete_parser.add_argument("--force", action="store_true", help="Force deletion")
    
    # List workers command
    list_workers_parser = subparsers.add_parser("list-workers", help="List workers")
    list_workers_parser.add_argument("--pool-id", help="Filter by work pool ID")
    
    # Start worker command
    start_worker_parser = subparsers.add_parser("start-worker", help="Start a worker for a work pool")
    start_worker_parser.add_argument("pool_id", help="Work pool ID")
    start_worker_parser.add_argument("--name", help="Worker name")
    
    # Deploy session command for development/testing
    deploy_parser = subparsers.add_parser("deploy-session", help="Generate a command to deploy a session directly (dev/test only)")
    deploy_parser.add_argument("pool_id", help="Work pool ID")
    deploy_parser.add_argument("session_id", help="Session ID")
    
    # Parse arguments
    args = parser.parse_args()
    
    # Create CLI instance
    cli = WorkPoolCLI(args.api_url, args.api_key)
    
    # Execute command
    if args.command == "create":
        # Get provider class
        provider_class = get_provider(args.provider)
        if not provider_class:
            print(f"Unknown provider type: {args.provider}")
            return 1
        
        # Create provider instance with args
        provider_args = {
            key: getattr(args, key) 
            for key in [opt["dest"] for opt in provider_class.get_cli_options() if "dest" in opt]
            if hasattr(args, key) and getattr(args, key) is not None
        }
        
        provider = provider_class(**provider_args)
        
        # Get pool config
        pool_config = provider.get_pool_config()
        
        # Create pool
        print(f"Creating {provider.display_name} work pool '{pool_config['name']}'...")
        result = cli.create_pool(pool_config)
        
        if "error" in result:
            print(f"Failed to create work pool: {result['error']}")
            return 1
        
        print(f"Work pool created: {result['id']}")
        print("\nTo start a worker for this pool, run:")
        print(f"python -m browsergrid.client.workerpool_cli start-worker {result['id']}")
        
    elif args.command == "list":
        # List pools
        result = cli.list_pools()
        if "error" in result:
            print(f"Failed to list work pools: {result['error']}")
            return 1
        
        print("\nWork Pools:")
        print("===========")
        for pool in result:
            print(f"{pool['name']} ({pool['id']}) - {pool['provider_type']} - Status: {pool['status']}")
        
    elif args.command == "get":
        # Get pool details
        result = cli.get_pool(args.pool_id)
        if "error" in result:
            print(f"Failed to get work pool: {result['error']}")
            return 1
        
        print(json.dumps(result, indent=2))
        
    elif args.command == "delete":
        # Delete pool
        result = cli.delete_pool(args.pool_id, args.force)
        if "error" in result:
            print(f"Failed to delete work pool: {result['error']}")
            return 1
        
        print(f"Work pool {args.pool_id} deleted successfully")
        
    elif args.command == "list-workers":
        # List workers
        result = cli.list_workers(args.pool_id)
        if "error" in result:
            print(f"Failed to list workers: {result['error']}")
            return 1
        
        print("\nWorkers:")
        print("========")
        for worker in result:
            print(f"{worker['name']} ({worker['id']}) - Status: {worker['status']} - Load: {worker['current_load']}/{worker['capacity']}")
        
    elif args.command == "start-worker":
        # Start worker
        cli.start_worker(args.pool_id, args.name)
    
    elif args.command == "deploy-session":
        # Deploy session command (for development/testing)
        cli.deploy_session_cmd(args.pool_id, args.session_id)
        
    else:
        parser.print_help()
        return 1
    
    return 0


if __name__ == "__main__":
    sys.exit(main()) 