"""
Database migrations management for browsergrid server with async support

This module provides functions to manage migrations across all app models.
"""

import importlib
import os
import sys
from pathlib import Path
import asyncio
import alembic
from alembic.config import Config
from alembic import command
from typing import List, Optional

from browsergrid.server.core.db.base import Base
from browsergrid.server.core.db.session import sessionmanager, settings
from browsergrid.server.core.settings import INSTALLED_APPS
from browsergrid.server.core.apps import apps

from loguru import logger

APP_MODULES = ["sessions", "webhooks", "workers", "workerpool"]


def get_alembic_config() -> Config:
    """Get Alembic configuration"""
    migrations_dir = Path(__file__).parent / "alembic"

    config = Config()
    config.set_main_option("script_location", str(migrations_dir))
    config.set_main_option("sqlalchemy.url", settings.database_url)
    return config


def import_all_models():
    """Import all models to ensure they're registered with SQLAlchemy"""
    if not apps.apps_ready:
        apps.populate()
    
    for app_config in apps.get_app_configs():
        if app_config.models_module:
            try:
                importlib.import_module(app_config.models_module)
            except ImportError as e:
                logger.warning(f"Warning: Could not import models from {app_config.name}: {e}")


def create_migration(message: str):
    """Create a new migration"""
    import_all_models()

    config = get_alembic_config()

    command.revision(config, message=message, autogenerate=True)
    logger.info(f"Created new migration: {message}")


def upgrade_database(revision: str = "head"):
    """Upgrade the database to the specified revision"""
    import_all_models()

    config = get_alembic_config()

    command.upgrade(config, revision)
    logger.info(f"Database upgraded to: {revision}")


def downgrade_database(revision: str):
    """Downgrade the database to the specified revision"""
    import_all_models()

    config = get_alembic_config()

    command.downgrade(config, revision)
    logger.info(f"Database downgraded to: {revision}")


def init_alembic():
    """Initialize Alembic for migrations"""
    migrations_dir = Path(__file__).parent / "alembic"

    migrations_dir.mkdir(exist_ok=True)

    config = get_alembic_config()

    command.init(config, str(migrations_dir))
    logger.info(f"Initialized Alembic in: {migrations_dir}")


def list_migrations():
    """List all available migrations"""
    config = get_alembic_config()

    command.history(config, verbose=True)


async def init_db():
    """Initialize the database with tables using async engine"""
    import_all_models()
    
    # Initialize the session manager with the database URL
    sessionmanager.init(settings.database_url)
    
    async with sessionmanager.connect() as connection:
        await sessionmanager.create_all(connection)
        logger.info("Database tables created successfully.")
    
    await sessionmanager.close()

# Helper to run async init_db from synchronous code if needed
def sync_init_db():
    """Synchronous wrapper for init_db"""
    asyncio.run(init_db())