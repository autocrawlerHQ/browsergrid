"""
Tests for database settings and session manager
"""
import pytest
from unittest.mock import patch, MagicMock, AsyncMock
import sys
import importlib


class TestDatabaseSettings:
    """Tests for the DatabaseSettings class"""
    
    def test_database_settings(self):
        """Test database settings properties with proper patching"""
        # Create a mock settings module
        mock_settings = MagicMock()
        mock_settings.POSTGRES_USER = 'test_user'
        mock_settings.POSTGRES_PASSWORD = 'test_password'
        mock_settings.POSTGRES_HOST = 'test_host'
        mock_settings.POSTGRES_PORT = 5432
        mock_settings.POSTGRES_DB = 'test_db'
        mock_settings.DATABASE_URL = 'postgresql://test_url'
        
        # Patch sys.modules to replace the settings module
        with patch.dict('sys.modules', {'browsergrid.server.core.settings': mock_settings}):
            # Force reload to pick up the mock
            if 'browsergrid.server.core.db.session' in sys.modules:
                del sys.modules['browsergrid.server.core.db.session']
            
            # Import after patching
            from browsergrid.server.core.db.session import DatabaseSettings
            
            # Create a new settings instance
            settings = DatabaseSettings()
            
            # Check properties
            assert settings.POSTGRES_USER == 'test_user'
            assert settings.POSTGRES_PASSWORD == 'test_password'
            assert settings.POSTGRES_HOST == 'test_host'
            assert settings.POSTGRES_PORT == 5432
            assert settings.POSTGRES_DB == 'test_db'
            
            # Check URL conversion
            assert settings.database_url == 'postgresql+asyncpg://test_url'


class TestDatabaseManager:
    """Tests for the DatabaseSessionManager class"""
    
    def test_init_method(self):
        """Test initialization with database URL"""
        # Create mocks
        mock_engine = AsyncMock()
        mock_create_engine = MagicMock(return_value=mock_engine)
        
        mock_factory = MagicMock()
        mock_sessionmaker = MagicMock(return_value=mock_factory)
        
        # Import with patched functions
        with patch('browsergrid.server.core.db.session.create_async_engine', mock_create_engine), \
             patch('browsergrid.server.core.db.session.async_sessionmaker', mock_sessionmaker):
            
            from browsergrid.server.core.db.session import DatabaseSessionManager
            
            # Create fresh instance
            manager = DatabaseSessionManager()
            manager.init('postgresql+asyncpg://test_url')
            
            # Check engine creation
            mock_create_engine.assert_called_once_with('postgresql+asyncpg://test_url')
            mock_sessionmaker.assert_called_once_with(autocommit=False, bind=mock_engine)
            
            # Check properties set
            assert manager._engine is mock_engine
            assert manager._sessionmaker is mock_factory
    
    @pytest.mark.asyncio
    async def test_close_method(self):
        """Test that close disposes of engine and resets properties"""
        # Create mocks
        mock_engine = AsyncMock()
        
        # Import the manager class
        from browsergrid.server.core.db.session import DatabaseSessionManager
        
        # Create manager with mocked engine
        manager = DatabaseSessionManager()
        manager._engine = mock_engine  # Set directly
        manager._sessionmaker = MagicMock()
        
        # Call close
        await manager.close()
        
        # Check engine disposal
        mock_engine.dispose.assert_awaited_once()
        
        # Check properties reset
        assert manager._engine is None
        assert manager._sessionmaker is None
    
    @pytest.mark.asyncio
    async def test_connect_context_manager(self):
        """Test connect context manager"""
        # Create mocks
        mock_engine = AsyncMock()
        mock_connection = AsyncMock()
        
        # Create a proper async context manager mock
        class AsyncCtxMock(AsyncMock):
            async def __aenter__(self):
                return mock_connection
            
            async def __aexit__(self, *args):
                pass
        
        # Set the begin method to return our context manager
        mock_engine.begin = MagicMock(return_value=AsyncCtxMock())
        
        # Import the manager class
        from browsergrid.server.core.db.session import DatabaseSessionManager
        
        # Create manager with mocked engine
        manager = DatabaseSessionManager()
        manager._engine = mock_engine
        
        # Use connect context manager
        async with manager.connect() as conn:
            assert conn is mock_connection
            
            # Test some operation
            await conn.execute(MagicMock())
        
        # Check that begin was called
        mock_engine.begin.assert_called_once()

    @pytest.mark.asyncio
    async def test_session_context_manager(self):
        """Test session context manager"""
        # Create mocks
        mock_session = AsyncMock()
        mock_factory = MagicMock(return_value=mock_session)
        
        # Import the manager class
        from browsergrid.server.core.db.session import DatabaseSessionManager
        
        # Create manager with mocked sessionmaker
        manager = DatabaseSessionManager()
        manager._sessionmaker = mock_factory
        
        # Use session context manager
        async with manager.session() as sess:
            assert sess is mock_session
            
            # Test some operation
            await sess.execute(MagicMock())
        
        # Check that close was called
        mock_session.close.assert_awaited_once()
    
    @pytest.mark.asyncio
    async def test_get_db_dependency(self):
        """Test get_db FastAPI dependency"""
        # Create mocks
        mock_session = AsyncMock()
        
        # Create a proper async context manager mock for session()
        async def mock_session_cm():
            yield mock_session
        
        # Import with patched session manager
        with patch('browsergrid.server.core.db.session.sessionmanager.session', mock_session_cm):
            from browsergrid.server.core.db.session import get_db
            
            # Use get_db as an async generator
            db_gen = get_db()
            db = await db_gen.__anext__()
            
            # Check that it yields the session
            assert db is mock_session