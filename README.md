# Browsergrid

> Easy browser infrastructure for automation, testing, and development

Browsergrid is a powerful browser infrastructure platform that provides managed browser instances for automation, testing, and development. It offers a scalable, cloud-native solution for browser orchestration with flexible deployment options.

## Features

- **Multi-Browser Support**: Run Chrome, Firefox, Chromium, and WebKit sessions
- **Browser Multiplexing**: Connect multiple clients to a single browser instance
- **Dynamic Infrastructure**: Scale browser sessions on demand with work pools
- **Webhook Integration**: Automate workflows based on browser events
- **Containerized Deployment**: Run browser instances in Docker or cloud environments
- **CDP Support**: Full Chrome DevTools Protocol compatibility
- **API-First Design**: RESTful APIs for all functionality

## Architecture

Browsergrid follows a modular architecture:

```
┌─────────────┐     ┌────────────────────┐     ┌──────────────────┐
│  Client API  │────▶│  Browsergrid Server │────▶│  Browser Workers │
└─────────────┘     └────────────────────┘     └──────────────────┘
                             │                          │
                             ▼                          ▼
                     ┌──────────────┐          ┌──────────────────┐
                     │   Database   │          │ Browser Instances │
                     └──────────────┘          └──────────────────┘
```

## Key Components

### Browsergrid Server

The core API service providing endpoints for:
- Session management
- Work pool configuration
- Worker registration and monitoring
- Webhook management

### BrowserMux

A Chrome DevTools Protocol (CDP) proxy that allows multiple clients to connect to a single browser instance, featuring:
- CDP protocol compatibility
- Real-time event streaming
- Client connection management
- WebSocket multiplexing

### Sessions

Managed browser instances with configurable options:
- Browser type and version selection
- Headless/headful mode
- Resource limitations (CPU, memory, timeout)
- Operating system selection
- Network proxy configuration
- Screen resolution settings

### Work Pools and Workers

A dynamic infrastructure layer that enables:
- On-demand browser session provisioning
- Load balancing across workers
- Pool-specific browser configurations
- Workers that poll for and manage sessions

### Browser Support

The following browsers are fully supported:
- **Chrome**: Latest stable release
- **Chromium**: Latest stable release
- **Firefox**: Latest stable release
- **WebKit**: Latest stable release

## Getting Started

### Prerequisites

- Docker and Docker Compose
- Python 3.8+
- Make (optional, for using the Makefile)

### Quick Start

1. Clone the repository:
   ```bash
   git clone https://github.com/galad-ca/browsergrid.git
   cd browsergrid
   ```

2. Start the server:
   ```bash
   make server up
   ```

3. Start a browser instance:
   ```bash
   make browser BROWSER=chrome up
   ```

## Usage Examples

### Launch a Browser Session

```bash
curl -X POST "http://localhost:8765/v1/sessions" \
  -H "Content-Type: application/json" \
  -H "X-API-Key: your_api_key" \
  -d '{
    "browser": "chrome",
    "version": "latest",
    "headless": false,
    "resource_limits": {"cpu": 1, "memory": "2G"}
  }'
```

### Create a Work Pool

```bash
python -m browsergrid.client.workerpool_cli create docker \
  --name "my-docker-pool" \
  --browser chrome \
  --browser-version latest
```

### Starting a Worker

```bash
python -m browsergrid.client.worker \
  --api-url http://localhost:8765 \
  --api-key your_api_key \
  --work-pool-id your_work_pool_id
```

## Deployment Options

Browsergrid supports multiple deployment models:

### Local Development
- Run the server and browsers on your local machine
- Great for development and testing

### Docker Containers
- Run each component in separate containers
- Ideal for CI/CD pipelines and testing environments

### Cloud Deployment
- Deploy to Azure Container Instances
- Scale based on demand
- Easily extensible to other cloud providers

## Webhook System

Configurable webhooks that trigger on browser events:
- Monitor page navigation events
- React to network requests/responses
- Capture console logs and errors
- Process DOM mutations
- Integrate with external systems and workflows

## Development

### Running Tests

```bash
pytest

# Run browser-specific tests
pytest tests/browsers/
```

### Building from Source

1. Install development dependencies:
   ```bash
   pip install -e ".[dev]"
   ```

2. Build the project:
   ```bash
   make build
   ```

## Contributing

We welcome contributions! Please refer to our [Contributing Guide](./.github/CONTRIBUTING.md) for more information.

## License

This project is licensed under the MIT License - see the [LICENSE](./LICENSE) file for details.
