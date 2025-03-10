"""
Database migrations management for browserfleet server

This module provides functions to manage migrations across all app models.
"""

import importlib
import os
import sys
from pathlib import Path
import alembic
from alembic.config import Config
from alembic import command
from typing import List, Optional

from browserfleet.server.core.db.base import Base
from browserfleet.server.core.db.session import engine
from browserfleet.server.core.settings import INSTALLED_APPS
from browserfleet.server.core.apps import apps

# Apps with models to include in migrations
APP_MODULES = ["sessions", "webhooks", "workers", "workerpool"]


def get_alembic_config() -> Config:
    """Get Alembic configuration"""
    # Get the directory where the migrations are stored
    migrations_dir = Path(__file__).parent / "alembic"

    # Create an Alembic configuration object
    config = Config()
    config.set_main_option("script_location", str(migrations_dir))
    config.set_main_option(
        "sqlalchemy.url", engine.url.render_as_string(hide_password=False)
    )
    return config


def import_all_models():
    """Import all models to ensure they're registered with SQLAlchemy"""
    # Ensure app registry is populated
    if not apps.apps_ready:
        apps.populate()
    
    # Import models from all registered apps
    for app_config in apps.get_app_configs():
        if app_config.models_module:
            try:
                importlib.import_module(app_config.models_module)
            except ImportError as e:
                print(f"Warning: Could not import models from {app_config.name}: {e}")


def create_migration(message: str):
    """Create a new migration"""
    # Import all models to make sure they're available to Alembic
    import_all_models()

    # Get the Alembic configuration
    config = get_alembic_config()

    # Create a new migration
    command.revision(config, message=message, autogenerate=True)
    print(f"Created new migration: {message}")


def upgrade_database(revision: str = "head"):
    """Upgrade the database to the specified revision"""
    # Import all models to make sure they're available to Alembic
    import_all_models()

    # Get the Alembic configuration
    config = get_alembic_config()

    # Upgrade the database
    command.upgrade(config, revision)
    print(f"Database upgraded to: {revision}")


def downgrade_database(revision: str):
    """Downgrade the database to the specified revision"""
    # Import all models to make sure they're available to Alembic
    import_all_models()

    # Get the Alembic configuration
    config = get_alembic_config()

    # Downgrade the database
    command.downgrade(config, revision)
    print(f"Database downgraded to: {revision}")


def init_alembic():
    """Initialize Alembic for migrations"""
    # Get the directory where the migrations are stored
    migrations_dir = Path(__file__).parent / "alembic"

    # Create the migrations directory if it doesn't exist
    migrations_dir.mkdir(exist_ok=True)

    # Create an Alembic configuration object
    config = get_alembic_config()

    # Initialize Alembic
    command.init(config, str(migrations_dir))
    print(f"Initialized Alembic in: {migrations_dir}")


def list_migrations():
    """List all available migrations"""
    # Get the Alembic configuration
    config = get_alembic_config()

    # List the migrations
    command.history(config, verbose=True)


def init_db():
    """Initialize the database with tables"""
    # Import all models to make sure they're available to SQLAlchemy
    import_all_models()

    # Create all tables
    Base.metadata.create_all(bind=engine)
    print("Database tables created successfully.")
