# Browser Tests with Playwright

This directory contains tests for various browser instances, both running in Docker containers (CDP tests) and launched directly by Playwright (persistent context tests).

## Test Structure

The test structure uses explicit setup and teardown rather than hidden fixtures:

- `browser_helpers.py` - Explicit utility functions for browser testing
- `test_cdp_browsers.py` - Tests for browsers running in Docker accessed via CDP
- `test_persistent_browsers.py` - Tests for browsers launched directly with persistent user data directories
- `conftest.py` - Minimal file with only essential session-level fixtures

This approach makes test flow and setup/teardown explicit and visible, avoiding "magic" that's hidden in pytest fixtures.

## Prerequisites

- For CDP tests: Docker containers running with the browser instances (Chrome, Chromium, Firefox, WebKit) with remote debugging enabled on port 9222
- For persistent context tests: locally installed browsers (Chrome, Firefox, WebKit)
- Python with pytest and pytest-playwright installed

## Running the Tests

To run all browser tests:

```bash
pytest tests/browsers/
```

To run only CDP tests (for Docker containers):

```bash
pytest tests/browsers/test_cdp_browsers.py
```

To run only persistent context tests (for locally installed browsers):

```bash
pytest tests/browsers/test_persistent_browsers.py
```

To run a specific test function:

```bash
# Run a specific test function
pytest tests/browsers/test_cdp_browsers.py::test_cdp_connection_all_browsers

# Run a specific test with parameter
pytest tests/browsers/test_cdp_browsers.py::test_cdp_individual_browser[firefox]
```

## Configuring Browser Connection

### CDP Tests
By default, CDP tests connect to browsers on `localhost:9222`. You can override these settings with environment variables:

```bash
BROWSER_HOST=my-docker-host BROWSER_PORT=9333 pytest tests/browsers/test_cdp_browsers.py
```

## Test Details

### CDP Tests (Docker Containers)
These tests connect to running browser instances via the Chrome DevTools Protocol:
- Verify basic browser functionality (page navigation, DOM access)
- Test JavaScript execution
- Verify data persistence within a single browser session

### Persistent Context Tests
These tests launch browser instances with a persistent user data directory:
- Test data persistence between pages in the same browser context
- Test data persistence across multiple browser launches with the same profile
- Special tests for Chrome with the `channel="chrome"`, `headless=False`, `no_viewport=True` options 