"""
Minimal conftest.py file with only a session-level Playwright fixture.
Most functionality has been moved to explicit browser_helpers.py module.
"""
import pytest
from playwright.sync_api import sync_playwright

# Keep only the most basic fixture that's useful across all tests
@pytest.fixture(scope="session")
def playwright_fixture():
    """Session-level Playwright instance"""
    with sync_playwright() as pw:
        yield pw 