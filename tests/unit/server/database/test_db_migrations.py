"""
Tests for database migrations module
"""
import pytest
from unittest.mock import patch, MagicMock, AsyncMock, call
import tempfile
from pathlib import Path
import sys
import importlib


@pytest.fixture
def mock_alembic_config():
    """Mock an Alembic config instance"""
    mock_config = MagicMock()
    return mock_config


@pytest.fixture
def mock_alembic_config_class(mock_alembic_config):
    """Mock the Alembic Config class"""
    mock_class = MagicMock(return_value=mock_alembic_config)
    return mock_class


@pytest.fixture
def mock_alembic_command():
    """Mock the alembic.command module"""
    mock_command = MagicMock()
    mock_command.revision = MagicMock()
    mock_command.upgrade = MagicMock()
    mock_command.downgrade = MagicMock()
    mock_command.init = MagicMock()
    mock_command.history = MagicMock()
    return mock_command


@pytest.fixture
def mock_alembic_modules(mock_alembic_config_class, mock_alembic_command):
    """Mock all alembic modules"""
    with patch.dict('sys.modules', {
        'alembic': MagicMock(),
        'alembic.config': MagicMock(Config=mock_alembic_config_class),
        'alembic.command': mock_alembic_command
    }):
        yield {
            'config_class': mock_alembic_config_class,
            'command': mock_alembic_command
        }


@pytest.fixture
def mock_session_manager():
    """Mock the sessionmanager for migrations"""
    mock_manager = MagicMock()
    mock_connection = AsyncMock()
    
    # Mock connect method to return async context manager
    async def mock_connect_cm():
        yield mock_connection
    
    mock_manager.connect = AsyncMock(__aenter__=mock_connect_cm.__aiter__().__anext__)
    
    # Mock init, create_all, drop_all methods
    mock_manager.init = MagicMock()
    mock_manager.create_all = AsyncMock()
    mock_manager.drop_all = AsyncMock()
    
    # Add URL property
    mock_settings = MagicMock()
    mock_settings.database_url = 'postgresql+asyncpg://test_url'
    
    return mock_manager, mock_connection, mock_settings


@pytest.fixture
def patch_migrations_path():
    """Create a consistent path for migrations directory"""
    # Create a real temporary directory
    with tempfile.TemporaryDirectory() as tmpdir:
        migration_dir = Path(tmpdir) / "alembic"
        migration_dir.mkdir(exist_ok=True)
        
        # Create a patcher function for Path
        def mock_path(filepath):
            mock = MagicMock(spec=Path)
            mock.parent = Path(tmpdir)
            mock.__truediv__.return_value = migration_dir
            mock.__str__.return_value = str(migration_dir)
            return mock
        
        # Apply patch
        with patch('pathlib.Path', mock_path):
            yield migration_dir


@pytest.fixture
def mock_app_module():
    """Mock app module for migrations"""
    mock_apps = MagicMock()
    mock_apps.apps_ready = False
    mock_apps.populate = MagicMock()
    
    app1 = MagicMock()
    app1.name = "app1"
    app1.models_module = "app1.models"
    
    mock_apps.get_app_configs.return_value = [app1]
    
    return mock_apps


class TestMigrations:
    """Tests for database migrations"""
    
    @pytest.fixture
    def migration_module(self, mock_alembic_modules, patch_migrations_path, mock_session_manager, mock_app_module):
        """Import migrations module with patched dependencies"""
        # Unpack fixtures
        manager, connection, settings = mock_session_manager
        
        # Create additional patches
        patches = {
            'browsergrid.server.core.db.migrations.sessionmanager': manager,
            'browsergrid.server.core.db.migrations.settings': settings,
            'browsergrid.server.core.db.migrations.apps': mock_app_module,
            'importlib.import_module': MagicMock()
        }
        
        # Apply all patches
        with patch.dict('sys.modules', patches):
            # Force reload the migrations module
            if 'browsergrid.server.core.db.migrations' in sys.modules:
                del sys.modules['browsergrid.server.core.db.migrations']
                
            # Import and return
            import browsergrid.server.core.db.migrations as migrations
            return migrations
    
    def test_get_alembic_config(self, migration_module, mock_alembic_modules, mock_alembic_config, patch_migrations_path):
        """Test get_alembic_config function"""
        # Mock the set_main_option method
        mock_alembic_config.set_main_option = MagicMock()
        
        # Call function
        config = migration_module.get_alembic_config()
        
        # Check result is the mocked config
        assert config is mock_alembic_config
        
        # Check configuration calls in any order
        mock_alembic_config.set_main_option.assert_any_call('script_location', str(patch_migrations_path))
        mock_alembic_config.set_main_option.assert_any_call('sqlalchemy.url', 'postgresql+asyncpg://test_url')
    
    def test_create_migration(self, migration_module, mock_alembic_modules, mock_alembic_config):
        """Test create_migration function"""
        # Create a mock for import_all_models
        mock_import = MagicMock()
        
        # Replace the import_all_models function
        with patch.object(migration_module, 'import_all_models', mock_import):
            # Call function
            migration_module.create_migration("test migration")
            
            # Verify imports and command call
            mock_import.assert_called_once()
            mock_alembic_modules['command'].revision.assert_called_once_with(
                mock_alembic_config,
                message="test migration", 
                autogenerate=True
            )
    
    def test_upgrade_database(self, migration_module, mock_alembic_modules, mock_alembic_config):
        """Test upgrade_database function"""
        # Create a mock for import_all_models
        mock_import = MagicMock()
        
        # Replace the import_all_models function
        with patch.object(migration_module, 'import_all_models', mock_import):
            # Call function with default revision
            migration_module.upgrade_database()
            
            # Verify imports and command call
            mock_import.assert_called_once()
            mock_alembic_modules['command'].upgrade.assert_called_once_with(
                mock_alembic_config,
                "head"
            )
    
    def test_downgrade_database(self, migration_module, mock_alembic_modules, mock_alembic_config):
        """Test downgrade_database function"""
        # Create a mock for import_all_models
        mock_import = MagicMock()
        
        # Replace the import_all_models function
        with patch.object(migration_module, 'import_all_models', mock_import):
            # Call function
            migration_module.downgrade_database("abc123")
            
            # Verify imports and command call
            mock_import.assert_called_once()
            mock_alembic_modules['command'].downgrade.assert_called_once_with(
                mock_alembic_config,
                "abc123"
            )
    
    def test_init_alembic(self, migration_module, mock_alembic_modules, mock_alembic_config, patch_migrations_path):
        """Test init_alembic function"""
        # Call function
        migration_module.init_alembic()
        
        # Verify command call
        mock_alembic_modules['command'].init.assert_called_once_with(
            mock_alembic_config,
            str(patch_migrations_path)
        )
    
    def test_list_migrations(self, migration_module, mock_alembic_modules, mock_alembic_config):
        """Test list_migrations function"""
        # Call function
        migration_module.list_migrations()
        
        # Verify command call
        mock_alembic_modules['command'].history.assert_called_once_with(
            mock_alembic_config,
            verbose=True
        )
    
    @pytest.mark.asyncio
    async def test_init_db(self, migration_module, mock_session_manager):
        """Test init_db async function"""
        # Unpack fixtures
        manager, connection, settings = mock_session_manager
        
        # Create a mock for import_all_models
        mock_import = MagicMock()
        
        # Replace the import_all_models function
        with patch.object(migration_module, 'import_all_models', mock_import):
            # Call function
            await migration_module.init_db()
            
            # Verify imports and calls - directly compare objects
            assert mock_import.call_count == 1
            assert manager.init.call_count == 1
            assert manager.create_all.await_count == 1
            assert manager.create_all.await_args[0][0] is connection