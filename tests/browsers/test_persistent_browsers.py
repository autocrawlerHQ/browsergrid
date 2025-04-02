"""
Tests for browser persistence using Playwright's persistent context.
These tests use explicit setup and teardown rather than pytest fixtures.
"""

import pytest
import os
from playwright.sync_api import sync_playwright
from .browser_helpers import (
    create_temp_user_data_dir, 
    remove_temp_dir,
    launch_persistent_browser,
    PERSISTENT_BROWSER_TYPES
)

def test_persistent_context_session_all_browsers():
    """Test using a persistent context browser session"""
    with sync_playwright() as playwright:
        for browser_type in PERSISTENT_BROWSER_TYPES:
            # Create temp directory
            user_data_dir = create_temp_user_data_dir()
            try:
                # Launch browser with persistent context
                context = launch_persistent_browser(playwright, user_data_dir, browser_type)
                try:
                    # Create a page in the persistent context
                    page = context.new_page()
                    try:
                        # Set a cookie
                        page.goto("https://httpbin.org/cookies/set?persistent_context_test=1")
                        
                        # Verify the cookie exists
                        page.goto("https://httpbin.org/cookies")
                        cookies_json = page.evaluate("() => JSON.parse(document.body.textContent)")
                        assert "cookies" in cookies_json
                        assert "persistent_context_test" in cookies_json["cookies"]
                        
                        # Store something in localStorage
                        page.goto("about:blank")
                        page.evaluate("() => localStorage.setItem('persistent_test', 'persistent_value')")
                    finally:
                        page.close()
                    
                    # Create a new page in the same context
                    new_page = context.new_page()
                    try:
                        # Check if cookie persists in the new page
                        new_page.goto("https://httpbin.org/cookies")
                        cookies_json = new_page.evaluate("() => JSON.parse(document.body.textContent)")
                        assert "cookies" in cookies_json
                        assert "persistent_context_test" in cookies_json["cookies"]
                        
                        # Check if localStorage persists
                        new_page.goto("about:blank")
                        storage_value = new_page.evaluate("() => localStorage.getItem('persistent_test')")
                        assert storage_value == "persistent_value"
                        
                        print(f"✅ Browser {browser_type} passed persistent context test")
                    finally:
                        new_page.close()
                finally:
                    context.close()
            finally:
                remove_temp_dir(user_data_dir)

def test_persistent_context_across_sessions_all_browsers():
    """Test persistence across multiple browser sessions using the same user_data_dir"""
    with sync_playwright() as playwright:
        for browser_type in PERSISTENT_BROWSER_TYPES:
            # Create temp directory
            user_data_dir = create_temp_user_data_dir()
            try:
                # FIRST SESSION
                # Create first browser instance with persistent context
                context1 = launch_persistent_browser(playwright, user_data_dir, browser_type)
                try:
                    # Use the first browser to set data
                    page1 = context1.new_page()
                    try:
                        page1.goto("https://httpbin.org/cookies/set?cross_session=1")
                        page1.goto("about:blank")
                        page1.evaluate("() => localStorage.setItem('cross_session', 'remembered')")
                        
                        # Create a file in the user_data_dir to verify it persists
                        test_file_path = os.path.join(user_data_dir, "test_file.txt")
                        with open(test_file_path, "w") as f:
                            f.write("Test file content")
                    finally:
                        page1.close()
                finally:
                    # Close first browser completely
                    context1.close()
                
                # SECOND SESSION  
                # Create a second browser instance with the same user_data_dir
                context2 = launch_persistent_browser(playwright, user_data_dir, browser_type)
                try:
                    # Check if data persisted to the second browser
                    page2 = context2.new_page()
                    try:
                        # Check cookies
                        page2.goto("https://httpbin.org/cookies")
                        cookies_json = page2.evaluate("() => JSON.parse(document.body.textContent)")
                        assert "cookies" in cookies_json
                        assert "cross_session" in cookies_json["cookies"]
                        
                        # Check localStorage (should persist)
                        page2.goto("about:blank")
                        storage_value = page2.evaluate("() => localStorage.getItem('cross_session')")
                        assert storage_value == "remembered"
                        
                        # Verify the file exists in user_data_dir
                        assert os.path.exists(test_file_path)
                        with open(test_file_path, "r") as f:
                            content = f.read()
                        assert content == "Test file content"
                        
                        print(f"✅ Browser {browser_type} passed cross-session persistence test")
                    finally:
                        page2.close()
                finally:
                    context2.close()
            finally:
                remove_temp_dir(user_data_dir)

def test_chrome_specific_persistent_context():
    """Test Chrome-specific persistent context with explicit Chrome channel"""
    with sync_playwright() as playwright:
        # Create temp directory for Chrome
        user_data_dir = create_temp_user_data_dir()
        try:
            # Launch Chrome with persistent context as specified in requirements
            context = launch_persistent_browser(
                playwright,
                user_data_dir,
                browser_type="chrome",
                headless=False
            )
            try:
                # Create a page and navigate
                page = context.new_page()
                try:
                    page.goto("https://example.com")
                    
                    # Verify browser identity
                    browser_info = page.evaluate("""() => {
                        return {
                            userAgent: navigator.userAgent,
                            vendor: navigator.vendor
                        }
                    }""")
                    
                    assert "Chrome" in browser_info["userAgent"]
                    assert "Google" in browser_info["vendor"]
                    
                    # Set up data to be persisted
                    page.goto("https://httpbin.org/cookies/set?chrome_persistence=1")
                    page.goto("about:blank")
                    page.evaluate("() => localStorage.setItem('chrome_test', 'chrome_data')")
                finally:
                    page.close()
            finally:
                context.close()
            
            # Re-launch with same user_data_dir
            new_context = launch_persistent_browser(
                playwright,
                user_data_dir,
                browser_type="chrome",
                headless=False
            )
            try:
                # Create a new page and verify persistence
                new_page = new_context.new_page()
                try:
                    # Check cookies
                    new_page.goto("https://httpbin.org/cookies")
                    cookies_json = new_page.evaluate("() => JSON.parse(document.body.textContent)")
                    assert "cookies" in cookies_json
                    assert "chrome_persistence" in cookies_json["cookies"]
                    
                    # Check localStorage
                    new_page.goto("about:blank")
                    storage_value = new_page.evaluate("() => localStorage.getItem('chrome_test')")
                    assert storage_value == "chrome_data"
                    
                    print("✅ Chrome specific persistent context test passed")
                finally:
                    new_page.close()
            finally:
                new_context.close()
        finally:
            remove_temp_dir(user_data_dir) 