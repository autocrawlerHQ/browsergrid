# BrowserMux API Documentation

BrowserMux is a Chrome DevTools Protocol (CDP) proxy that allows multiple clients to connect to a single browser instance. It provides an API for managing webhooks that can be triggered on browser events.

## Overview

The BrowserMux API is divided into several areas:

1. **CDP Proxy** - Standard Chrome DevTools Protocol endpoints for browser interaction
2. **Browser API** - Endpoints for browser information and client management
3. **Webhook API** - CRUD operations for managing webhooks
4. **Health Check** - Simple endpoint to check service health

## Base URL

All API endpoints are served from the root URL of the BrowserMux service.

## CDP Proxy Endpoints

These endpoints implement the standard Chrome DevTools Protocol HTTP interface:

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/json/version` | GET, POST | Get browser version information |
| `/json` or `/json/list` | GET, POST | List available targets/pages |
| `/json/new` | GET, POST | Create a new target/page |
| `/json/activate/{targetId}` | GET, POST | Activate a specific target |
| `/json/close/{targetId}` | GET, POST | Close a specific target |
| `/json/protocol` | GET, POST | Get the DevTools Protocol definition |
| `/devtools/{path}` | WebSocket | WebSocket endpoint for CDP connections |

## Browser API Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/browser` | GET | Get information about the connected browser |
| `/api/clients` | GET | List all connected clients |

### Response Examples

#### GET `/api/browser`

```json
{
  "browser": {
    "version": "Chrome/91.0.4472.124",
    "protocol": "1.3"
  },
  "clients": 2,
  "status": true
}
```

#### GET `/api/clients`

```json
{
  "clients": [
    {
      "id": "client-123",
      "connected_at": "2023-03-17T10:20:30Z"
    },
    {
      "id": "client-456",
      "connected_at": "2023-03-17T11:25:15Z"
    }
  ],
  "count": 2
}
```

## Webhook API Endpoints

Webhooks allow you to receive HTTP callbacks when certain browser events occur.

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/webhooks` | POST | Create a new webhook |
| `/api/webhooks` | GET | List all registered webhooks |
| `/api/webhooks/{id}` | GET | Get a specific webhook |
| `/api/webhooks/{id}` | PUT | Update a webhook |
| `/api/webhooks/{id}` | DELETE | Delete a webhook |
| `/api/webhook-executions` | GET | List webhook execution history |
| `/api/webhook-executions/{id}` | GET | Get details of a specific execution |
| `/api/webhooks/test` | POST | Test a webhook |

### Webhook Configuration

Webhooks can be configured to trigger either before or after CDP events.

```json
{
  "name": "Page Load Webhook",
  "url": "https://example.com/webhook",
  "event_method": "Page.loadEventFired",
  "timing": "after_event",
  "headers": {
    "Authorization": "Bearer token123"
  },
  "timeout": 5000,
  "max_retries": 3,
  "enabled": true
}
```

### Webhook Timing Options

- `before_event`: Triggered before a CDP command is sent to the browser
- `after_event`: Triggered after a CDP event is received from the browser

### Testing a Webhook

```json
{
  "webhook_id": "webhook-123",
  "client_id": "client-456",
  "cdp_method": "Page.loadEventFired",
  "cdp_params": {}
}
```

## Health Check

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/health` | GET | Returns 200 OK if the service is healthy |

## Configuration

BrowserMux can be configured using environment variables:

- `BROWSER_URL`: WebSocket URL of the browser (default: `ws://localhost:9222/devtools/browser`)
- `PORT`: Port to listen on (default: `8080`)
- `MAX_MESSAGE_SIZE`: Maximum size of WebSocket messages in bytes (default: `1048576`)
- `CONNECTION_TIMEOUT`: Timeout for browser connections in seconds (default: `10`)

## WebSocket Connections

Clients can connect to the `/devtools/{path}` WebSocket endpoint to communicate with the browser using the Chrome DevTools Protocol. All messages are forwarded to the browser, and responses are sent back to the client.

## Event System

BrowserMux includes an event system that dispatches events when:

1. CDP commands are sent to the browser
2. CDP events are received from the browser
3. Clients connect or disconnect

These events can trigger webhooks based on the configured event methods and timing. 