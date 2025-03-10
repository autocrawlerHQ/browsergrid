#!/usr/bin/env python
"""
Management script for browserfleet server

This script provides a simple management interface for the browserfleet server.
It includes commands for running the server, managing the database, and more.
"""
import argparse

def main():
    """Main entry point for the management script"""
    parser = argparse.ArgumentParser(description="Browserfleet server management")
    subparsers = parser.add_subparsers(dest="command", help="Command to run")
    
    # runserver command
    runserver_parser = subparsers.add_parser("runserver", help="Run the browserfleet server")
    runserver_parser.add_argument("--host", help="Server host")
    runserver_parser.add_argument("--port", type=int, help="Server port")
    runserver_parser.add_argument("--debug", action="store_true", help="Enable debug mode")
    runserver_parser.add_argument("--no-ui", dest="ui_enabled", action="store_false", help="Disable UI")
    
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
        from browserfleet.server.core.server import get_server
        
        server_config = {}
        if args.host:
            server_config["host"] = args.host
        if args.port:
            server_config["port"] = args.port
        if args.debug:
            server_config["debug"] = args.debug
        if args.ui_enabled is not None:
            server_config["ui_enabled"] = args.ui_enabled

        
        server = get_server(**server_config)
        server.start()
    
    elif args.command == "migrate":
        from browserfleet.server.core.db.migrations import upgrade_database
        
        upgrade_database(args.revision)
    
    elif args.command == "makemigrations":
        from browserfleet.server.core.db.migrations import create_migration
        
        create_migration(args.message)
    
    else:
        parser.print_help()

if __name__ == "__main__":
    main() 