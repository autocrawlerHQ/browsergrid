# BrowserGrid Development Tasks

## Critical Issues & Bug Fixes
- [ ] Test warm pools functionality
- [ ] Fix worker cleanup - resolve duplicate worker entries in database when users close down workers
- [ ] Fix active sessions not updating in UI
- [x] Investigate session random shutdown and failed state - check timeout handling
- [ ] Add duration counter to UI for sessions
- [x] Browser container should stream visual output
- [ ] Migrate auth from hard coded to a server action where hash is saved in the database

- [ ] Add CUA to browser instances
- [ ] Add chat functionality to browsergrid (chat with sessions, chat with workers, chat with deployments, chat with workpools, chat with profiles) using MCP

- [ ] Add storage system for browser profiles, deployment artifacts, deployment logs, etc.

- [x] Add deployment system
- [x] Add deployment run system
- [ ] Add scheduled tasks for deployments

- [ ] improve task ui to see what tasks are scheduled and individual stats on each task

- [ ] Add bring your own proxy system
- [ ] Network interceptor for browser sessions, should be reusable stored in server applied to browser instances when requested


- [ ] Explore pluggable side car services that connect to browser instances and provide additional functionality
- [ ] Built in custom defined CDP events  like browserless and brightdata ex. `BrowserGrid.DetectCaptcha` 
- [ ] Add a way to add custom CDP events to browser instances

 ## Testing
- [x] Write system tests for core server framework (DB, migrations, app provisioning)
- [x] Write API tests
- [x] Write browser tests
- [ ] Add automated readme badges based on test results

## Client Development & Connection System


- [ ] Create CLI and SDK from client code
- [ ] Develop SDKs in:
  - [ ] JavaScript/TypeScript
  - [ ] Python
 
- [ ] Implement quick connect endpoint (e.g., `playwright.connect_over_cdp("connect.browsergrid.io")`)
  - [ ] Support ephemeral sessions that live only as long as the connection
  - [ ] Handle connection timeouts and reconnection logic
- [ ] Develop SDK-based connection system
  - [ ] Allow users to create and acquire persistent sessions
  - [ ] Support session management (pause, resume, transfer)
  - [ ] Implement session state persistence

## API Integration
- [x] Link server API and browser instance API

## Worker Integrations
- [x] local worker
- [x] docker worker
- [ ] kubernetes worker ( support both spot and on-demand )
- [x] azure container instance worker
- [ ] gcp cloud function worker
- [ ] aws ecs worker

## UI Development
- [ ] Add UI using components from AutoCrawler console repo
  - [x] Session management (list, details with chat interface)
  - [x] Profile management (list, details)
  - [x] Work pool management (list, details)
  - [x] Worker management (list, details)
  - [x] Deployment management (list, details, CRUD operations (deployment runs with browser instance)   )
  - [ ] Observability for sessions, deployments, workpools, workers  (metrics, logs, traces)

## Documentation
- [x] Create comprehensive API documentation
- [ ] Write user guides for each client SDK
- [ ] Develop installation and deployment guides
- [ ] Create browser compatibility matrix documentation
- [ ] Document version upgrade paths
- [ ] Create architecture diagrams and system design documentation
- [ ] Document browser version tracking and management process
- [ ] Create troubleshooting guides and FAQs
- [ ] Add Issue templates for bugs, feature requests, and documentation updates 

## Version Management
- [x] Implement tagging, semantic versioning, and releases

## Profile Management
- [x] Implement persistent user data directories
- [x] Design volume management across different platforms:
  - [x] Docker, Kubernetes, cloud providers (Azure, AWS, GCP)
  - [x] Support for multiple browser types and versions
- [x] Enable multiple profiles per instance
- [x] Allow profile sharing between instances (NOTE: this works but its last to write wins after containers terminate)

## Deployment System
- [x] Add deployments to package code that runs with browser instances ( or separately, not sure yet )

## Security
- [ ] Implement secure authentication and authorization mechanisms
- [ ] Add rate limiting and throttling
- [ ] Implement logging and monitoring
- [ ] Create security best practices documentation  
- [ ] allow the users to self-host user authentication and authorization, static api keys, etc.

## Maintenance
- [ ] Add scheduled maintenance mode
- [ ] Session migration between browser instances (if needed. see apify for inspiration - live migration of sessions for running scrapers)
- [ ] Implement health checks and monitoring
- [ ] Create backup and restore procedures
- [ ] Add logging and monitoring (metrics, logs, traces)
- [ ] Add alerting and notifications
- [ ] observability for the server, browser instances, deployments, workpools, workers  