#!/usr/bin/env python
"""
Management script for browsergrid server

This script provides a simple management interface for the browsergrid server.
It includes commands for running the server, managing the database, and more.
"""
import argparse
import os
import uvicorn
from browsergrid.server.core.settings import API_HOST, API_PORT, DEBUG
from loguru import logger

def main():
    """Main entry point for the management script"""
    parser = argparse.ArgumentParser(description="Browsergrid server management")
    subparsers = parser.add_subparsers(dest="command", help="Command to run")
    
    # runserver command
    runserver_parser = subparsers.add_parser("runserver", help="Run the browsergrid server")
    runserver_parser.add_argument("--host", help="Server host", default=API_HOST)
    runserver_parser.add_argument("--port", type=int, help="Server port", default=API_PORT)
    runserver_parser.add_argument("--debug", action="store_true", help="Enable debug mode", default=DEBUG)
    runserver_parser.add_argument("--reload", action="store_true", help="Enable auto-reload on file changes")
    runserver_parser.add_argument("--reload-dir", action="append", help="Directories to watch for changes (can be used multiple times)")
    runserver_parser.add_argument("--exclude", action="append", help="Patterns to exclude from reload watching (can be used multiple times)")
    
    # migrate command
    migrate_parser = subparsers.add_parser("migrate", help="Run database migrations")
    migrate_parser.add_argument("revision", help="Revision to migrate to", nargs="?", default="head")
    
    # makemigrations command
    makemigrations_parser = subparsers.add_parser("makemigrations", help="Create new database migrations")
    makemigrations_parser.add_argument("message", help="Migration message", nargs="?", default="auto")
       
    # Parse arguments
    args = parser.parse_args()
    
    if not args.command:
        parser.print_help()
        return
    
    if args.command == "runserver":
        logger.info(f"Starting browsergrid server at http://{args.host}:{args.port}")
        
        # Configure Uvicorn
        config_kwargs = {
            "app": "browsergrid.server.core.server:server",
            "host": args.host,
            "port": args.port,
            "log_level": "debug" if args.debug else "info",
        }
        
        if args.reload:
            logger.info("Auto-reload enabled: Server will restart when files change")
            config_kwargs["reload"] = True
            
            # Use watchfiles for faster reloading
            try:
                import watchfiles
                logger.info("Using watchfiles for faster reloading")
                config_kwargs["reload_delay"] = 0.25
                config_kwargs["reload_includes"] = ["*.py"]
                
                if args.reload_dir:
                    config_kwargs["reload_dirs"] = args.reload_dir
                else:
                    # Find the project root - this is a heuristic
                    project_root = os.path.abspath(os.path.join(os.path.dirname(__file__), "../../"))
                    config_kwargs["reload_dirs"] = [project_root]
                    
                if args.exclude:
                    config_kwargs["reload_excludes"] = args.exclude
                else:
                    config_kwargs["reload_excludes"] = [
                        "*/.git/*", "*/\\.git/*",
                        "*/node_modules/*",
                        "*/venv/*", "*/.venv/*",
                        "*/__pycache__/*", "*/.pytest_cache/*"
                    ]
            except ImportError:
                logger.warning("Warning: watchfiles not installed. Falling back to default reloader.")
                logger.warning("Install watchfiles for faster reloading: pip install watchfiles")
        
        uvicorn.run(**config_kwargs)
    
    elif args.command == "migrate":
        from browsergrid.server.core.db.migrations import upgrade_database
        upgrade_database(args.revision)
    
    elif args.command == "makemigrations":
        from browsergrid.server.core.db.migrations import create_migration
        create_migration(args.message)
    
    else:
        parser.print_help()

if __name__ == "__main__":
    main()