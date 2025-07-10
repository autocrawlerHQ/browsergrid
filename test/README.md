# BrowserMux Test Scripts

This directory contains test scripts to verify BrowserMux connectivity and functionality.

## Prerequisites

- Node.js 18+ 
- A running BrowserMux instance with Chrome

## Setup

1. Install dependencies:
```bash
npm install
```

## Usage

### Test with default URL (localhost:32771)
```bash
npm test
```

### Test with custom URL
```bash
node connect.js http://localhost:32772
```

### Test with different port
```bash
node connect.js http://localhost:8080
```

## What the test does

1. **Checks `/json` endpoint** - Verifies BrowserMux is accessible and returns target list
2. **Validates URL rewriting** - Ensures WebSocket URLs use the correct external port
3. **Connects via Puppeteer** - Tests actual browser connection using CDP
4. **Takes screenshot** - Verifies full functionality works
5. **Checks version info** - Validates browser information endpoint

## Expected Output

```
🔍 Testing BrowserMux connection to: http://localhost:32771

📋 Test 1: Checking /json endpoint...
✅ Found 2 targets
  1. Google Hangouts (background_page)
     ID: E3A16CEA6D514252398E77E24DF1ACB7
     WebSocket URL: ws://localhost:32771/devtools/page/E3A16CEA6D514252398E77E24DF1ACB7
     ✅ Port correct: 32771

🌐 Test 2: Connecting to page "Google Hangouts"...
✅ Connected successfully! Found 1 pages
  Page 1: Google Hangouts (https://hangouts.google.com/)
📸 Screenshot taken (45678 bytes)
✅ Disconnected successfully

🔧 Test 3: Checking browser version...
✅ Browser: Chrome/120.0.6099.109
✅ Protocol: 1.3
✅ User-Agent: Mozilla/5.0...

🎉 All tests passed! BrowserMux is working correctly.
```

## Troubleshooting

If tests fail, check:

1. **BrowserMux is running** - Verify the container is up and healthy
2. **Port is accessible** - Ensure the port is exposed and not blocked
3. **Chrome is running** - Check that Chrome started with remote debugging enabled
4. **URL is correct** - Verify the URL matches your BrowserMux instance

## Port Mapping Verification

The test specifically verifies that URL rewriting works correctly:

- **Before fix**: `ws://localhost:61000/devtools/page/...` ❌
- **After fix**: `ws://localhost:32771/devtools/page/...` ✅

This ensures that Puppeteer and other CDP clients can connect properly through the external port. 