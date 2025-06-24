# BrowserGrid UI

A modern React dashboard for managing browser sessions, work pools, and workers in the BrowserGrid automation platform.

## Features

### 🎯 **Overview Dashboard**
- Real-time statistics for active sessions, work pools, and workers
- System health monitoring
- Resource utilization metrics

### 🌐 **Browser Sessions**
- **Create Sessions**: Launch browser sessions with customizable configurations
  - Browser type (Chrome, Firefox, Edge, Safari)
  - Browser versions (Latest, Stable, Canary, Dev)
  - Operating system selection (Linux, Windows, macOS)
  - Screen resolution settings
  - Headless/UI mode toggle
  - Resource limits and timeouts
- **Session Management**: View, monitor, and control active sessions
- **Live Access**: Direct links to live browser instances
- **Status Tracking**: Real-time session status updates

### 🏊 **Work Pools**
- **Pool Creation**: Create and configure worker pools
  - Provider selection (Docker, Azure ACI, Local)
  - Auto-scaling configuration
  - Concurrency limits
  - Resource allocation
- **Pool Management**: Monitor pool health and performance
- **Scaling Operations**: Manual and automatic scaling controls
- **Pool Statistics**: Detailed metrics and analytics

### ⚒️ **Workers**
- **Worker Monitoring**: Real-time worker status and health
- **Capacity Management**: Track active vs. maximum capacity
- **Worker Controls**: Pause, resume, and remove workers
- **Heartbeat Tracking**: Monitor worker connectivity

### 🪝 **Webhooks** *(Coming Soon)*
- Event-driven notifications
- Custom webhook endpoints
- Event filtering and routing

## Technology Stack

- **Frontend**: React 18 + TypeScript
- **Styling**: Tailwind CSS + shadcn/ui
- **State Management**: TanStack Query (React Query)
- **API Integration**: Auto-generated TypeScript client from OpenAPI spec
- **Icons**: Lucide React
- **Notifications**: Sonner

## Getting Started

1. **Install Dependencies**
   ```bash
   pnpm install
   ```

2. **Configure API Endpoint**
   - Update the API base URL in your mutator configuration
   - Default: `http://localhost:8080/api/v1`

3. **Start Development Server**
   ```bash
   pnpm run dev
   ```

4. **Generate API Types** (if schema changes)
   ```bash
   pnpm run generate-api
   ```

## API Integration

The UI uses auto-generated React Query hooks from your OpenAPI specification:

- **Sessions**: `useGetApiV1Sessions`, `usePostApiV1Sessions`
- **Work Pools**: `useGetApiV1Workpools`, `usePostApiV1Workpools`
- **Workers**: `useGetApiV1Workers`, `useDeleteApiV1WorkersId`

All API interactions are fully type-safe with automatic TypeScript generation.

## Environment Configuration

Configure your API connection in the mutator file:

```typescript
// src/lib/api/mutator.ts
export const customInstance = axios.create({
  baseURL: process.env.VITE_API_URL || 'http://localhost:8080'
});
```

## Features in Detail

### Session Creation
- **Browser Configuration**: Choose from Chrome, Firefox, Edge, or Safari
- **Version Selection**: Pick specific browser versions or use latest/stable
- **Environment Setup**: Configure OS, screen resolution, and resource limits
- **Advanced Options**: Proxy settings, webhooks, and custom environments

### Work Pool Management
- **Provider Support**: Docker containers, Azure ACI, or local processes
- **Auto-scaling**: Intelligent scaling based on demand
- **Resource Management**: CPU, memory, and concurrency limits
- **Health Monitoring**: Real-time pool status and performance metrics

### Worker Operations
- **Real-time Status**: Live updates on worker health and activity
- **Capacity Tracking**: Visual indicators for worker utilization
- **Management Actions**: Pause, resume, or remove workers
- **Performance Metrics**: Detailed worker performance data

## Contributing

1. Follow the existing code structure and patterns
2. Use TypeScript for all new components
3. Implement proper error handling and loading states
4. Add appropriate documentation for new features
5. Test your changes with the actual BrowserGrid API

## API Documentation

The complete API documentation is available at:
- Swagger UI: `http://localhost:8080/swagger/`
- OpenAPI Spec: `http://localhost:8080/swagger/doc.json`

## Support

For issues and questions:
1. Check the browser developer console for errors
2. Verify API connectivity and authentication
3. Review the network requests in browser dev tools
4. Check the BrowserGrid server logs for backend issues
