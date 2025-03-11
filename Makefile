# Browsergrid Makefile
# This Makefile provides commands for building, starting, and stopping Docker services

# Environment-specific settings
SERVER_FILES := -f docker/docker-compose.server.yml
SESSION_FILES := -f docker/docker-compose.session.yml
SESSION_PROXY_FILES := -f docker/docker-compose.session.yml --profile proxy
SESSION_CAPTCHA_FILES := -f docker/docker-compose.session.yml --profile captcha
SESSION_FULL_FILES := -f docker/docker-compose.session.yml --profile full

# Docker command
DOCKER_COMPOSE_COMMAND = docker compose

# Default environment
ENVIRONMENT = server

# Environment targets
.PHONY: server session session-proxy session-captcha session-full

server:
	$(eval FILES := $(SERVER_FILES))
	@echo "Environment: Server"

session:
	$(eval FILES := $(SESSION_FILES))
	@echo "Environment: Session"

session-proxy:
	$(eval FILES := $(SESSION_PROXY_FILES))
	@echo "Environment: Session with Proxy"

session-captcha:
	$(eval FILES := $(SESSION_CAPTCHA_FILES))
	@echo "Environment: Session with Captcha"

session-full:
	$(eval FILES := $(SESSION_FULL_FILES))
	@echo "Environment: Session Full"

# Operation targets
.PHONY: build up down logs restart config pull stop

build:
	$(DOCKER_COMPOSE_COMMAND) $(FILES) build

up:
	$(DOCKER_COMPOSE_COMMAND) $(FILES) up -d

down:
	$(DOCKER_COMPOSE_COMMAND) $(FILES) down

logs:
	$(DOCKER_COMPOSE_COMMAND) $(FILES) logs -f

restart:
	$(DOCKER_COMPOSE_COMMAND) $(FILES) restart

config:
	$(DOCKER_COMPOSE_COMMAND) $(FILES) config

pull:
	$(DOCKER_COMPOSE_COMMAND) $(FILES) pull

stop:
	$(DOCKER_COMPOSE_COMMAND) $(FILES) stop

# Default target
.PHONY: help
help:
	@echo "Browsergrid Makefile"
	@echo ""
	@echo "Environment targets:"
	@echo "  make server           - Set environment to Server"
	@echo "  make session          - Set environment to Session"
	@echo "  make session-proxy    - Set environment to Session with Proxy"
	@echo "  make session-captcha  - Set environment to Session with Captcha"
	@echo "  make session-full     - Set environment to Session Full"
	@echo ""
	@echo "Operation targets (run after setting environment):"
	@echo "  build            - Build Docker images"
	@echo "  up               - Start containers"
	@echo "  down             - Stop and remove containers"
	@echo "  logs             - View logs"
	@echo "  restart          - Restart containers"
	@echo "  config           - Show docker-compose configuration"
	@echo "  pull             - Pull Docker images"
	@echo "  stop             - Stop containers without removing them"
	@echo ""
	@echo "Examples:"
	@echo "  make server up        - Start server containers"
	@echo "  make session-full up  - Start session with all services"