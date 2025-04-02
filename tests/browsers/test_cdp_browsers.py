"""
Tests for browsers running in Docker containers via CDP.
These tests use explicit setup and teardown rather than pytest fixtures.
"""

import pytest
from playwright.sync_api import sync_playwright
from tests.browsers.browser_helpers import connect_to_cdp_browser, CDP_BROWSER_TYPES

def test_cdp_connection_all_browsers():
    """Test connecting to different browsers running in Docker containers via CDP"""
    with sync_playwright() as playwright:
        for browser_type in CDP_BROWSER_TYPES:
            # Connect to browser via CDP
            browser = connect_to_cdp_browser(playwright, browser_type)
            try:
                # Create a page
                page = browser.new_page()
                try:
                    # Navigate to a test page
                    page.goto("https://example.com")
                    
                    # Verify page loaded successfully
                    assert "Example Domain" in page.title()
                    
                    # Test basic interaction
                    assert page.query_selector("h1")
                    
                    # Test JavaScript execution
                    result = page.evaluate("1 + 1")
                    assert result == 2
                    
                    # Test taking a screenshot
                    screenshot = page.screenshot()
                    assert screenshot is not None and len(screenshot) > 0
                    
                    print(f"✅ Browser {browser_type} passed basic functionality test")
                finally:
                    page.close()
            finally:
                browser.close()

def test_cdp_cookie_persistence_all_browsers():
    """Test that cookies persist within the same CDP browser session"""
    with sync_playwright() as playwright:
        for browser_type in CDP_BROWSER_TYPES:
            # Connect to browser via CDP
            browser = connect_to_cdp_browser(playwright, browser_type)
            try:
                # Create a page
                page = browser.new_page()
                try:
                    # Set a cookie
                    page.goto("https://httpbin.org/cookies/set?test=1")
                    
                    # Navigate to another page
                    page.goto("https://example.com")
                    
                    # Check if cookie persists
                    page.goto("https://httpbin.org/cookies")
                    cookies_json = page.evaluate("() => JSON.parse(document.body.textContent)")
                    assert "cookies" in cookies_json
                    assert "test" in cookies_json["cookies"]
                    assert cookies_json["cookies"]["test"] == "1"
                    
                    print(f"✅ Browser {browser_type} passed cookie persistence test")
                finally:
                    page.close()
            finally:
                browser.close()

# For individual browser testing
@pytest.mark.parametrize("browser_type", CDP_BROWSER_TYPES)
def test_cdp_individual_browser(browser_type):
    """Test a specific browser type via CDP - useful for focused testing"""
    with sync_playwright() as playwright:
        # Connect to browser via CDP
        browser = connect_to_cdp_browser(playwright, browser_type)
        try:
            # Create a page
            page = browser.new_page()
            try:
                # Navigate to a test page
                page.goto("https://example.com")
                
                # Simple verification
                assert "Example Domain" in page.title()
                assert page.query_selector("h1")
                
                print(f"✅ Individual browser test for {browser_type} passed")
            finally:
                page.close()
        finally:
            browser.close() 