# BrowserGrid

BrowserGrid is a distributed browser automation platform that provides scalable browser instances for testing, web scraping, and automation tasks.


- **Distributed Architecture**: Scale browser instances across multiple workers
- **Multiple Browser Support**: Chrome, Chromium, Firefox, and WebKit
- **Provider Flexibility**: Docker, Azure Container Instances, and local providers
- **Session Management**: Full lifecycle management of browser sessions
- **Auto-scaling**: Automatic scaling based on demand
- **WebSocket & HTTP APIs**: Multiple ways to interact with browser instances
- **Computer Using Agent (CUA)**: AI-powered browser automation

## Quick Start

### Default Workpools (Recommended)

The easiest way to get started is to let BrowserGrid automatically create default workpools:

1. **Start the server and worker without specifying a pool ID:**
   ```bash
   # build the browser images
   make browser build # chrome default
   make browser BROWSER=firefox build
   ```
   ```bash
   # Start the server
   make server up
   ```

3. **The worker will automatically:**
   - Create a default workpool named `default-{provider}` (e.g., `default-docker`)
   - Register itself to that pool
   - Start processing browser sessions

4. **Multiple workers with the same provider will share the same default pool**, enabling proper load balancing and capacity management.

### Manual Workpool Configuration

If you need custom workpool settings:

1. **Create a workpool via the API:**
   ```bash
   curl -X POST http://localhost:8765/workpools \
        -H 'Content-Type: application/json' \
        -d '{
          "name": "my-custom-pool",
          "provider": "docker",
          "max_concurrency": 20,
          "auto_scale": true,
          "min_size": 2
        }'
   ```

2. **Set the pool ID in your environment:**
   ```bash
   export BROWSERGRID_WORKER_POOL_ID=<pool-id-from-response>
   ```

3. **Start the worker:**
   ```bash
   docker compose -f docker/docker-compose.server.yml up -d
   ```

## Environment Variables

### Worker Configuration

- `BROWSERGRID_WORKER_POOL_ID` (optional): Specific workpool ID. If not set, will create/use `default-{provider}`
- `BROWSERGRID_WORKER_NAME` (optional): Worker name, defaults to `worker-{hostname}`
- `BROWSERGRID_WORKER_PROVIDER` (default: `docker`): Provider type (`docker`, `azure_aci`, `local`)
- `BROWSERGRID_WORKER_CONCURRENCY` (default: `2`): Maximum concurrent sessions per worker
- `BROWSERGRID_WORKER_POLL_INTERVAL` (default: `10s`): How often to poll for new work

### Database Configuration

- `BROWSERGRID_POSTGRES_HOST` (default: `db`): PostgreSQL hostname
- `BROWSERGRID_POSTGRES_PORT` (default: `5432`): PostgreSQL port
- `BROWSERGRID_POSTGRES_USER` (default: `browsergrid`): PostgreSQL username
- `BROWSERGRID_POSTGRES_PASSWORD`: PostgreSQL password
- `BROWSERGRID_POSTGRES_DB` (default: `browsergrid`): PostgreSQL database name

## Why Browsergrid?

BrowserGrid uses a workpool-based architecture similar to Prefect:

- **Workpools**: Logical groupings of workers with shared configuration and capacity management
- **Workers**: Runtime processes that poll workpools for browser sessions to execute
- **Sessions**: Individual browser instances with their lifecycle managed by workers
- **Reconciler**: Background service that handles auto-scaling and session cleanup

## API Endpoints

### Workpools
- `POST /workpools` - Create a new workpool
- `GET /workpools` - List all workpools
- `GET /workpools/:id` - Get workpool details
- `PATCH /workpools/:id` - Update workpool settings
- `DELETE /workpools/:id` - Delete a workpool

### Workers
- `GET /workers` - List all workers
- `GET /workers/:id` - Get worker details
- `POST /workers/:id/pause` - Pause/resume a worker
- `DELETE /workers/:id` - Remove a worker

### Sessions
- `POST /sessions` - Create a new browser session
- `GET /sessions` - List sessions
- `GET /sessions/:id` - Get session details
- `DELETE /sessions/:id` - Terminate a session

## Development

### Running Tests

```bash
cd browsergrid
go test ./...
```

### Building

```bash
cd browsergrid
make build
```

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
