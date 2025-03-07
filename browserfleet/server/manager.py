#!/usr/bin/env python3
"""
browserfleet Server Manager

This module handles the lifecycle of the browserfleet API server,
including detecting running servers, starting ephemeral servers,
and shutting them down when no longer needed.
"""
import os
import sys
import signal
import socket
import time
import json
import subprocess
import threading
import uuid
import requests
from pathlib import Path
from typing import Optional, Dict, Any, List, Tuple

# Default server port
DEFAULT_PORT = 8000

# TODO: ephemral service should use an open port ex. IF port is already in use, try another port + 1
class ServerManager:
    """
    Manages the browserfleet API server lifecycle
    """
    
    def __init__(
        self,
        config_dir: Optional[str] = None,
        host: str = "127.0.0.1",
        port: int = DEFAULT_PORT,
        api_key: Optional[str] = None,
        ephemeral: bool = True,
        start_ui: bool = True,
        debug: bool = False
    ):
        """
        Initialize the server manager
        
        Args:
            config_dir: Configuration directory
            host: Server host
            port: Server port
            api_key: API key for authentication
            ephemeral: Whether to use ephemeral mode
            start_ui: Whether to start the UI
            debug: Whether to enable debug mode
        """
        # Set up configuration directory
        if config_dir:
            self.config_dir = Path(config_dir)
        else:
            self.config_dir = Path.home() / ".browserfleet"
        
        # Create config directory if it doesn't exist
        self.config_dir.mkdir(parents=True, exist_ok=True)
        
        # Server info
        self.host = host
        self.port = port
        self.api_key = api_key or os.environ.get("browserfleet_API_KEY")
        self.ephemeral = ephemeral
        self.start_ui = start_ui
        self.debug = debug
        
        # Server process
        self.server_process = None
        self.server_thread = None
        self.server_pid_file = self.config_dir / "server.pid"
        self.server_lock_file = self.config_dir / "server.lock"
        self.server_url = f"http://{self.host}:{self.port}"
        
        # Generate server ID if needed
        self.server_id = str(uuid.uuid4())
    
    def is_server_running(self) -> bool:
        """
        Check if a browserfleet server is already running
        
        Returns:
            bool: True if running, False otherwise
        """
        # First check environment variable for remote API
        api_url = os.environ.get("browserfleet_API_URL")
        if api_url:
            try:
                response = requests.get(f"{api_url}/api/v1/health")
                if response.status_code == 200:
                    self.server_url = api_url
                    return True
            except Exception:
                pass
        
        # Check if port is in use
        try:
            with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as s:
                s.settimeout(1)
                if s.connect_ex((self.host, self.port)) == 0:
                    # Port is in use, check if it's a browserfleet server
                    try:
                        response = requests.get(f"http://{self.host}:{self.port}/api/v1/health")
                        if response.status_code == 200:
                            return True
                    except Exception:
                        # Port is in use but not a browserfleet server
                        pass
        except Exception:
            pass
        
        # Check PID file
        if self.server_pid_file.exists():
            try:
                pid = int(self.server_pid_file.read_text().strip())
                
                # Check if process is running
                try:
                    os.kill(pid, 0)  # Signal 0 doesn't kill the process, just checks if it exists
                    return True
                except OSError:
                    # Process not running, clean up PID file
                    self.server_pid_file.unlink(missing_ok=True)
            except Exception:
                # Invalid PID file
                self.server_pid_file.unlink(missing_ok=True)
        
        return False
    
    def start_server(self) -> bool:
        """
        Start the browserfleet API server
        
        Returns:
            bool: True if server was started or is already running, False otherwise
        """
        # Check if server is already running
        if self.is_server_running():
            print(f"browserfleet server is already running at {self.server_url}")
            return True
        
        # Check if we can create a lock file
        if self.server_lock_file.exists():
            try:
                # Check if lock is stale (older than 5 minutes)
                if time.time() - self.server_lock_file.stat().st_mtime > 300:
                    print("Removing stale server lock file")
                    self.server_lock_file.unlink()
                else:
                    print("Another process is starting the server")
                    # Wait for up to 30 seconds for server to start
                    for _ in range(30):
                        if self.is_server_running():
                            return True
                        time.sleep(1)
                    return False
            except Exception as e:
                print(f"Error checking lock file: {e}")
                return False
        
        # Create lock file
        try:
            self.server_lock_file.write_text(f"{self.server_id}")
        except Exception as e:
            print(f"Error creating lock file: {e}")
            return False
        
        try:
            # Start API server
            if self.ephemeral:
                return self._start_embedded_server()
            else:
                return self._start_standalone_server()
        except Exception as e:
            print(f"Error starting server: {e}")
            self.server_lock_file.unlink(missing_ok=True)
            return False
    
    def _start_embedded_server(self) -> bool:
        """
        Start the server in embedded mode
        
        Returns:
            bool: True if server was started, False otherwise
        """
        from browserfleet.server.app import create_app
        import uvicorn
        
        print(f"Starting browserfleet server on {self.host}:{self.port}")
        
        # Create app configuration
        config = {
            "config_dir": str(self.config_dir),
            "host": self.host,
            "port": self.port,
            "api_key": self.api_key,
            "debug": self.debug,
            "server_id": self.server_id,
            "ephemeral": True,
            "ui_enabled": self.start_ui
        }
        
        # Create and run app in a separate thread
        def run_server():
            # Set up signal handling
            def handle_signal(signum, frame):
                # Clean up and exit
                if self.server_lock_file.exists():
                    self.server_lock_file.unlink()
                if self.server_pid_file.exists():
                    self.server_pid_file.unlink()
                sys.exit(0)
            
            signal.signal(signal.SIGINT, handle_signal)
            signal.signal(signal.SIGTERM, handle_signal)
            
            # Create and run app
            app = create_app(config)
            
            # Write PID file
            try:
                self.server_pid_file.write_text(str(os.getpid()))
            except Exception as e:
                print(f"Error writing PID file: {e}")
            
            # Remove lock file
            self.server_lock_file.unlink(missing_ok=True)
            
            uvicorn.run(
                app,
                host=self.host,
                port=self.port,
                log_level="debug" if self.debug else "info"
            )
        
        # Start server in a separate thread
        self.server_thread = threading.Thread(target=run_server, daemon=True)
        self.server_thread.start()
        
        # Wait for server to start
        for _ in range(30):
            if self.is_server_running():
                print(f"browserfleet server started at {self.server_url}")
                return True
            time.sleep(0.5)
        
        print("Server failed to start in time")
        return False
    
    def _start_standalone_server(self) -> bool:
        """
        Start the server as a separate process
        
        Returns:
            bool: True if server was started, False otherwise
        """
        print(f"Starting browserfleet server on {self.host}:{self.port}")
        
        # Define command to start server
        cmd = [
            sys.executable,
            "-m", "browserfleet.server.app",
            "--host", self.host,
            "--port", str(self.port),
            "--config-dir", str(self.config_dir)
        ]
        
        if self.debug:
            cmd.append("--debug")
        
        if self.start_ui:
            cmd.append("--ui")
        
        if self.api_key:
            cmd.extend(["--api-key", self.api_key])
        
        # Start server process
        try:
            self.server_process = subprocess.Popen(
                cmd,
                stdout=subprocess.PIPE if not self.debug else None,
                stderr=subprocess.PIPE if not self.debug else None,
                universal_newlines=True
            )
            
            # Wait for server to start
            for _ in range(30):
                if self.is_server_running():
                    print(f"browserfleet server started at {self.server_url}")
                    # Remove lock file
                    self.server_lock_file.unlink(missing_ok=True)
                    return True
                time.sleep(0.5)
            
            print("Server failed to start in time")
            return False
        except Exception as e:
            print(f"Error starting server process: {e}")
            return False
    
    def stop_server(self) -> bool:
        """
        Stop the browserfleet API server
        
        Returns:
            bool: True if server was stopped, False otherwise
        """
        # If server was started by someone else, don't stop it
        if not self.server_process and not self.server_thread:
            return False
        
        # Stop server
        if self.server_process:
            # Stop standalone process
            try:
                self.server_process.terminate()
                self.server_process.wait(timeout=5)
                self.server_process = None
                
                # Clean up PID file
                self.server_pid_file.unlink(missing_ok=True)
                
                return True
            except Exception as e:
                print(f"Error stopping server process: {e}")
                return False
        elif self.server_thread:
            # Let the thread handle cleanup via signals
            os.kill(os.getpid(), signal.SIGTERM)
            return True
        
        return False
    
    def get_server_info(self) -> Dict[str, Any]:
        """
        Get information about the server
        
        Returns:
            Dict[str, Any]: Server information
        """
        if not self.is_server_running():
            return {
                "status": "stopped",
                "url": None
            }
        
        try:
            response = requests.get(f"{self.server_url}/api/v1/info")
            if response.status_code == 200:
                info = response.json()
                info["status"] = "running"
                info["url"] = self.server_url
                return info
            else:
                return {
                    "status": "running",
                    "url": self.server_url,
                    "error": f"Failed to get server info: {response.status_code}"
                }
        except Exception as e:
            return {
                "status": "running",
                "url": self.server_url,
                "error": f"Failed to get server info: {e}"
            }

def get_or_create_server(
    config_dir: Optional[str] = None,
    host: str = "127.0.0.1",
    port: int = DEFAULT_PORT,
    api_key: Optional[str] = None,
    ephemeral: bool = True,
    start_ui: bool = True,
    debug: bool = False
) -> Tuple[ServerManager, str]:
    """
    Get an existing browserfleet server or create a new one
    
    Args:
        config_dir: Configuration directory
        host: Server host
        port: Server port
        api_key: API key for authentication
        ephemeral: Whether to use ephemeral mode
        start_ui: Whether to start the UI
        debug: Whether to enable debug mode
    
    Returns:
        Tuple[ServerManager, str]: Server manager and server URL
    """
    manager = ServerManager(
        config_dir=config_dir,
        host=host,
        port=port,
        api_key=api_key,
        ephemeral=ephemeral,
        start_ui=start_ui,
        debug=debug
    )
    
    # Check if server is running
    if manager.is_server_running():
        return manager, manager.server_url
    
    # Start server
    success = manager.start_server()
    
    if success:
        return manager, manager.server_url
    else:
        raise Exception("Failed to start browserfleet server")

if __name__ == "__main__":
    # Simple CLI for testing
    import argparse
    
    parser = argparse.ArgumentParser(description="browserfleet Server Manager")
    parser.add_argument("--start", action="store_true", help="Start the server")
    parser.add_argument("--stop", action="store_true", help="Stop the server")
    parser.add_argument("--info", action="store_true", help="Get server info")
    parser.add_argument("--host", default="127.0.0.1", help="Server host")
    parser.add_argument("--port", type=int, default=DEFAULT_PORT, help="Server port")
    parser.add_argument("--config-dir", help="Configuration directory")
    parser.add_argument("--debug", action="store_true", help="Enable debug mode")
    parser.add_argument("--no-ui", action="store_true", help="Don't start the UI")
    
    args = parser.parse_args()
    
    manager = ServerManager(
        config_dir=args.config_dir,
        host=args.host,
        port=args.port,
        debug=args.debug,
        start_ui=not args.no_ui
    )
    
    if args.start:
        success = manager.start_server()
        print(f"Server {'started' if success else 'failed to start'}")
    elif args.stop:
        success = manager.stop_server()
        print(f"Server {'stopped' if success else 'failed to stop'}")
    elif args.info:
        info = manager.get_server_info()
        print(json.dumps(info, indent=2))
    else:
        parser.print_help()