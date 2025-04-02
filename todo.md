# BrowserGrid Development Tasks

## Testing
- Write system tests for core server framework (DB, migrations, app provisioning)
- Write API tests
- Write browser tests
- Add automated readme badges based on test results

## Client Development
- Refactor client by removing CLI functionality
- Create specialized clients:
  - Worker client
  - Pool client
  - Session instance client (for browser session communication)
  - Webhook client
- Build browser group functionality
- Create CLI and SDK from client code
- Develop SDKs in:
  - JavaScript/TypeScript
  - Python
  - Go
  - Java
  - C#

## UI Development
- Add UI using components from AutoCrawler console repo
  - Session management (list, details with chat interface)
  - Profile management (list, details)
  - Work pool management (list, details)
  - Worker management (list, details)
  - Webhook management (list, details, CRUD operations)
  - Deployment management (list, details, CRUD operations)

## API Integration
- Link server API and browser instance API
- Integrate webhook API between server and browser instance

## Deployment System
- Add deployments to package code that runs with browser instances ( or separately, not sure yet )

## Version Management
- Implement tagging, semantic versioning, and releases
- Support multiple clients across different languages

## Profile Management
- Implement persistent user data directories
- Design volume management across different platforms:
  - Docker, Kubernetes, cloud providers (Azure, AWS, GCP)
  - Support for multiple browser types and versions
- Enable multiple profiles per instance
- Allow profile sharing between instances
- Address concurrency issues with shared profiles