"""
Browser test helpers and utilities.
This module replaces the conftest.py approach with explicit utility functions.
"""

import os
import tempfile
import shutil
from playwright.sync_api import sync_playwright

# Configuration
def get_cdp_config():
    """Get configuration for CDP browser connection"""
    return {
        "host": os.environ.get("BROWSER_HOST", "localhost"),
        "port": int(os.environ.get("BROWSER_PORT", "9222"))
    }

# CDP Connection Helpers
def connect_to_cdp_browser(playwright, browser_type="chromium"):
    """Connect to a browser running in Docker via CDP"""
    config = get_cdp_config()
    
    # Get the appropriate browser instance based on type
    browser_instance = getattr(playwright, browser_type, playwright.chromium)
    
    # Connect to browser via CDP
    return browser_instance.connect_over_cdp(
        f"http://{config['host']}:{config['port']}"
    )

# Persistent Context Helpers
def create_temp_user_data_dir():
    """Create a temporary directory for browser user data"""
    return tempfile.mkdtemp(prefix="playwright_test_")

def remove_temp_dir(dir_path):
    """Remove a temporary directory"""
    shutil.rmtree(dir_path, ignore_errors=True)

def launch_persistent_browser(playwright, user_data_dir, browser_type="chromium", headless=True):
    """Launch a browser with persistent context"""
    # Get the appropriate browser instance based on type
    browser_instance = getattr(playwright, browser_type, playwright.chromium)
    
    # Handle browser-specific options
    browser_options = {}
    if browser_type == "chrome":
        browser_options["channel"] = "chrome"
    
    # Common launch options
    launch_options = {
        "user_data_dir": user_data_dir,
        "headless": headless,
        "no_viewport": True,
        **browser_options
    }
    
    return browser_instance.launch_persistent_context(**launch_options)

# Test browser types
CDP_BROWSER_TYPES = ["chromium", "chrome", "firefox", "webkit"]
PERSISTENT_BROWSER_TYPES = ["chromium", "firefox", "webkit"]  # Chrome handled separately 