"""
Utility functions for database test fixtures.
"""
import pytest
from unittest.mock import MagicMock, AsyncMock
from pathlib import Path
import os
import tempfile


@pytest.fixture
def mock_base_metadata():
    """Create a mock Base with metadata for testing."""
    mock_base = MagicMock()
    mock_meta = MagicMock()
    mock_base.metadata = mock_meta
    return mock_base


@pytest.fixture
def mock_engine():
    """Create a mock SQLAlchemy engine for testing."""
    mock_engine = AsyncMock()
    # Set up URL property
    mock_engine.url = MagicMock()
    mock_engine.url.render_as_string.return_value = 'postgresql+asyncpg://test_url'
    
    # Set up connection context manager
    mock_connection = AsyncMock()
    cm = AsyncMock()
    cm.__aenter__.return_value = mock_connection
    mock_engine.connect.return_value = cm
    
    return mock_engine


@pytest.fixture
def alembic_config_dir():
    """Create a temporary directory for alembic config."""
    # Create a temporary directory
    temp_dir = tempfile.TemporaryDirectory()
    alembic_dir = Path(temp_dir.name) / 'alembic'
    os.makedirs(alembic_dir, exist_ok=True)
    
    # Yield the path
    yield alembic_dir
    
    # Clean up
    temp_dir.cleanup()


@pytest.fixture
def migration_test_env():
    """Create a test environment for alembic migration tests."""
    # Create mock alembic modules
    mock_alembic = MagicMock()
    mock_config = MagicMock()
    mock_command = MagicMock()
    
    # Set up the mock alembic module
    mock_alembic.config = mock_config
    mock_alembic.command = mock_command
    
    return {
        'alembic': mock_alembic,
        'config': mock_config,
        'command': mock_command
    }