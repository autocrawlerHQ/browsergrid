"""
Base test fixtures for all tests
"""
import asyncio
import pytest
from unittest.mock import MagicMock, AsyncMock
from sqlalchemy.ext.asyncio import AsyncEngine, AsyncSession, AsyncConnection


@pytest.fixture(scope="session")
def event_loop():
    """
    Create an instance of the default event loop for the entire test session.
    This overrides pytest-asyncio's function-scoped event_loop fixture.
    """
    policy = asyncio.get_event_loop_policy()
    loop = policy.new_event_loop()
    yield loop
    loop.close()


class AsyncContextManagerMock(AsyncMock):
    """Custom AsyncMock that properly mocks async context managers"""
    
    async def __aenter__(self):
        return self.aenter
    
    async def __aexit__(self, *args):
        pass


@pytest.fixture
def mock_async_engine():
    """Create a mock AsyncEngine with proper async context manager support"""
    engine = AsyncMock(spec=AsyncEngine)
    connection = AsyncMock(spec=AsyncConnection)
    
    # Set up begin method to return an async context manager
    begin_ctx = AsyncContextManagerMock()
    begin_ctx.aenter = connection
    engine.begin.return_value = begin_ctx
    
    # Set up connect method to return an async context manager
    connect_ctx = AsyncContextManagerMock()
    connect_ctx.aenter = connection
    engine.connect.return_value = connect_ctx
    
    # Set up URL property
    engine.url = MagicMock()
    engine.url.render_as_string.return_value = "postgresql+asyncpg://test_url"
    
    return engine, connection


@pytest.fixture
def mock_async_session():
    """Create a mock AsyncSession with proper async context manager support"""
    session = AsyncMock(spec=AsyncSession)
    return session


@pytest.fixture
def patch_settings():
    """Global fixture to patch settings"""
    # Import inside the fixture to avoid circular imports
    from unittest.mock import patch
    
    # Define a unified patching function for consistent patching
    def patch_env(monkeypatch):
        with patch('browsergrid.server.core.settings.POSTGRES_USER', 'test_user'), \
             patch('browsergrid.server.core.settings.POSTGRES_PASSWORD', 'test_password'), \
             patch('browsergrid.server.core.settings.POSTGRES_HOST', 'test_host'), \
             patch('browsergrid.server.core.settings.POSTGRES_PORT', 5432), \
             patch('browsergrid.server.core.settings.POSTGRES_DB', 'test_db'), \
             patch('browsergrid.server.core.settings.DATABASE_URL', 'postgresql://test_url'):
            yield
    
    return patch_env