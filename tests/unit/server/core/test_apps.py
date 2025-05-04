"""
Tests for the application registry module.
"""
import pytest
from unittest.mock import patch, MagicMock, call
import importlib
import sys
from typing import List, Dict, Type

# Import the modules to be tested
from browsergrid.server.core.apps import AppConfig, AppRegistry, autodiscover_modules, apps


# ===== APP CONFIG TESTS =====

def test_app_config_init_with_simple_app_name():
    """Test initialization with a simple app name"""
    app_config = AppConfig("myapp")
    assert app_config.name == "myapp"
    assert app_config.verbose_name == "Myapp"
    assert app_config.models_module is None


def test_app_config_init_with_dotted_app_name():
    """Test initialization with a dotted app name"""
    app_config = AppConfig("browsergrid.server.apps.myapp")
    assert app_config.name == "myapp"
    assert app_config.verbose_name == "Myapp"
    assert app_config.models_module is None


def test_app_config_explicit_name_overrides_automatic():
    """Test that explicit name overrides automatic name"""
    class CustomAppConfig(AppConfig):
        name = "custom"
        
    app_config = CustomAppConfig("browsergrid.server.apps.myapp")
    assert app_config.name == "custom"


def test_app_config_explicit_verbose_name_overrides_automatic():
    """Test that explicit verbose_name overrides automatic name"""
    class CustomAppConfig(AppConfig):
        verbose_name = "My Custom App"
        
    app_config = CustomAppConfig("myapp")
    assert app_config.verbose_name == "My Custom App"


def test_app_config_models_module_found(monkeypatch):
    """Test that models module is found when it exists"""
    mock_import = MagicMock()
    monkeypatch.setattr(importlib, 'import_module', mock_import)
    
    app_config = AppConfig("myapp")
    
    assert app_config.models_module == "myapp.models"
    mock_import.assert_called_once_with("myapp.models")


def test_app_config_models_module_not_found(monkeypatch):
    """Test that models_module is None when module doesn't exist"""
    def mock_import_module(name):
        raise ImportError(f"No module named '{name}'")
    
    monkeypatch.setattr(importlib, 'import_module', mock_import_module)
    
    app_config = AppConfig("myapp")
    assert app_config.models_module is None


def test_app_config_ready_method():
    """Test that ready method exists and doesn't fail"""
    app_config = AppConfig("myapp")
    # Should not raise an exception
    app_config.ready()


def test_app_config_custom_ready_method():
    """Test that custom ready method is called"""
    class CustomAppConfig(AppConfig):
        def ready(self):
            self.ready_called = True
            
    app_config = CustomAppConfig("myapp")
    app_config.ready()
    assert hasattr(app_config, 'ready_called')
    assert app_config.ready_called


# ===== APP REGISTRY TESTS =====

@pytest.fixture
def app_registry():
    """Fixture that returns a fresh AppRegistry for each test"""
    return AppRegistry()


def test_app_registry_init(app_registry):
    """Test initialization of AppRegistry"""
    assert app_registry.app_configs == {}
    assert app_registry.apps_ready is False
    assert app_registry.ready_called is False


def test_app_registry_populate_uses_installed_apps_by_default(app_registry, monkeypatch):
    """Test that populate uses INSTALLED_APPS by default"""
    # Mock INSTALLED_APPS
    monkeypatch.setattr('browsergrid.server.core.apps.INSTALLED_APPS', ['app1', 'app2'])
    
    # Mock load_app_config
    mock_load_app_config = MagicMock()
    monkeypatch.setattr(app_registry, 'load_app_config', mock_load_app_config)
    
    app_registry.populate()
    
    assert mock_load_app_config.call_args_list == [call('app1'), call('app2')]
    assert app_registry.apps_ready is True


def test_app_registry_populate_with_custom_apps(app_registry, monkeypatch):
    """Test populate with custom list of apps"""
    # Mock load_app_config
    mock_load_app_config = MagicMock()
    monkeypatch.setattr(app_registry, 'load_app_config', mock_load_app_config)
    
    app_registry.populate(['custom1', 'custom2'])
    
    assert mock_load_app_config.call_args_list == [call('custom1'), call('custom2')]
    assert app_registry.apps_ready is True


def test_app_registry_load_app_config_returns_existing(app_registry):
    """Test that load_app_config returns existing config if already loaded"""
    mock_config = MagicMock()
    app_registry.app_configs['myapp'] = mock_config
    
    result = app_registry.load_app_config('myapp')
    
    assert result is mock_config


def test_app_registry_load_app_config_creates_default_when_no_module(app_registry, monkeypatch):
    """Test that a default AppConfig is created when module doesn't exist"""
    def mock_import_module(name):
        raise ImportError(f"No module named '{name}'")
    
    monkeypatch.setattr(importlib, 'import_module', mock_import_module)
    
    result = app_registry.load_app_config('myapp')
    
    assert isinstance(result, AppConfig)
    assert result.name == 'myapp'
    assert app_registry.app_configs['myapp'] == result


def test_app_registry_load_app_config_finds_custom_config(app_registry, monkeypatch):
    """Test that load_app_config finds a custom AppConfig"""
    # Create a mock module with a custom AppConfig
    class CustomAppConfig(AppConfig):
        name = "custom"
    
    mock_module = MagicMock()
    mock_module.__name__ = "myapp.apps"
    
    # Setup dir to return the class name
    mock_module.__dir__ = lambda self: ['CustomAppConfig']

    
    # Setup getattr to return our custom class
    def mock_getattr(self, name):
        if name == 'CustomAppConfig':
            return CustomAppConfig
        return MagicMock()
    
    type(mock_module).__getattr__ = mock_getattr
    
    # Make importlib return our mock module
    def mock_import_module(name):
        if name == 'myapp.apps':
            return mock_module
        raise ImportError(f"No module named '{name}'")
    
    monkeypatch.setattr(importlib, 'import_module', mock_import_module)
    monkeypatch.setattr(mock_module, 'CustomAppConfig', CustomAppConfig)
    
    # Call the method
    result = app_registry.load_app_config('myapp')
    
    # Verify results
    assert isinstance(result, CustomAppConfig)
    assert result.name == 'custom'
    assert app_registry.app_configs['myapp'] == result


def test_app_registry_get_app_configs(app_registry):
    """Test get_app_configs returns all configs"""
    config1 = AppConfig('app1')
    config2 = AppConfig('app2')
    
    app_registry.app_configs = {'app1': config1, 'app2': config2}
    
    result = app_registry.get_app_configs()
    
    assert isinstance(result, list)
    assert len(result) == 2
    assert config1 in result
    assert config2 in result


def test_app_registry_ready_calls_all_app_configs(app_registry):
    """Test that ready calls all app configs"""
    config1 = MagicMock()
    config2 = MagicMock()
    
    app_registry.app_configs = {'app1': config1, 'app2': config2}
    
    app_registry.ready()
    
    config1.ready.assert_called_once()
    config2.ready.assert_called_once()
    assert app_registry.ready_called is True


def test_app_registry_ready_only_calls_once(app_registry):
    """Test that ready only calls app configs once"""
    config = MagicMock()
    
    app_registry.app_configs = {'app': config}
    app_registry.ready_called = True
    
    app_registry.ready()
    
    config.ready.assert_not_called()


# ===== AUTODISCOVER MODULES TESTS =====

def test_autodiscover_imports_modules(monkeypatch):
    """Test autodiscover_modules imports the correct modules"""
    # Setup app configs
    config1 = MagicMock()
    config1.name = 'app1'
    config2 = MagicMock()
    config2.name = 'app2'
    
    # Mock apps registry
    mock_apps = MagicMock()
    mock_apps.get_app_configs.return_value = [config1, config2]
    monkeypatch.setattr('browsergrid.server.core.apps.apps', mock_apps)
    
    # Mock importlib
    mock_import = MagicMock()
    monkeypatch.setattr(importlib, 'import_module', mock_import)
    
    # Call the function
    autodiscover_modules('admin')
    
    # Check imports
    assert mock_import.call_args_list == [
        call('app1.admin'),
        call('app2.admin')
    ]


def test_autodiscover_handles_import_errors(monkeypatch):
    """Test autodiscover_modules handles ImportError gracefully"""
    # Setup app configs
    config1 = MagicMock()
    config1.name = 'app1'
    config2 = MagicMock()
    config2.name = 'app2'
    
    # Mock apps registry
    mock_apps = MagicMock()
    mock_apps.get_app_configs.return_value = [config1, config2]
    monkeypatch.setattr('browsergrid.server.core.apps.apps', mock_apps)
    
    # Mock importlib to raise error on second import
    call_count = 0
    def mock_import_module(name):
        nonlocal call_count
        call_count += 1
        if call_count == 2:  # Second call fails
            raise ImportError(f"No module named '{name}'")
        return MagicMock()
    
    monkeypatch.setattr(importlib, 'import_module', mock_import_module)
    
    # Should not raise an exception
    autodiscover_modules('admin')
    
    # Check that it was called twice (once for each app)
    assert call_count == 2


# ===== APPS INSTANCE TEST =====

def test_apps_is_instance_of_app_registry():
    """Test that apps is an instance of AppRegistry"""
    assert isinstance(apps, AppRegistry)


# ===== ASYNC SUPPORT EXAMPLES =====

# Example of asynchronous test
@pytest.mark.asyncio
async def test_async_app_config():
    """Example of asynchronous test for future async functionality"""
    # This is a placeholder for when you need to test async functionality
    class AsyncAppConfig(AppConfig):
        async def async_ready(self):
            self.async_ready_called = True
            return True
    
    app_config = AsyncAppConfig("async_app")
    
    # Demonstrate that we can await async methods
    result = await app_config.async_ready()
    
    assert result is True
    assert app_config.async_ready_called is True