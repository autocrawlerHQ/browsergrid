# ======================================================
# Unified Makefile for Server and Browser Services
# ======================================================
# This Makefile provides commands for building, starting,
# and managing Docker services for both server and browser
# environments.

# NOTE: This does not work in windows powershell, use WSL, git bash, msys2 bash,  if possible.

# ======================================================
# Default settings
# ======================================================
ENVIRONMENT ?= server
BROWSER ?= chrome
BROWSER_TYPES := chrome firefox chromium webkit # Add other supported browsers here

# ======================================================
# File paths
# ======================================================
SERVER_FILES := -f docker/docker-compose.server.yml
BROWSER_FILES := -f docker/docker-compose.browser.yml
DEV_FILES := -f docker/docker-compose.server.yml -f docker/docker-compose.dev.yml

# ======================================================
# Docker command
# ======================================================
DOCKER_COMPOSE_COMMAND = $(shell if docker compose version > /dev/null 2>&1; then echo "docker compose"; else echo "docker-compose"; fi)

# ======================================================
# Utility functions
# ======================================================
# Function to check if a browser type is valid
check_browser = $(filter $(1),$(BROWSER_TYPES))

# ======================================================
# Environment targets
# ======================================================
.PHONY: server browser session dev help status

help:
	@echo "Unified Makefile - Usage:"
	@echo "======================================================="
	@echo "Environment targets:"
	@echo "  make server               Set environment to server"
	@echo "  make dev                  Set environment to development (with Air hot reloading)"
	@echo "  make browser [BROWSER=chrome]  Set environment to browser"
	@echo "                            Supported browsers: $(BROWSER_TYPES)"
	@echo ""
	@echo "Operation targets:"
	@echo "  make build                Build containers for current environment"
	@echo "  make up                   Start containers for current environment"
	@echo "  make down                 Stop and remove containers"
	@echo "  make clean                Remove containers and images"
	@echo "  make logs                 View logs for current environment"
	@echo "  make status               Show status of current environment"
	@echo ""
	@echo "Server-specific commands:"
	@echo "  make restart              Restart server containers"
	@echo "  make config               Show server configuration"
	@echo "  make pull                 Pull server images"
	@echo "  make stop                 Stop services without removing"
	@echo ""
	@echo "Browser-specific commands:"
	@echo "  make shell                Open shell in browser container"
	@echo ""
	@echo "Atlas migration commands:"
	@echo "  make migrate-diff [NAME=<name>]  Generate a new migration (auto-named if NAME not provided)"
	@echo "  make migrate-apply        Apply migrations to database"
	@echo "  make migrate-status       Show migration status"
	@echo "  make migrate-validate     Validate migrations"
	@echo ""
	@echo "Documentation commands:"
	@echo "  make docs                 Generate Swagger/OpenAPI documentation"
	@echo ""
	@echo "Example:"
	@echo "  make browser"
	@echo "  make browser build"
	@echo "  make browser BROWSER=chromium up"
	@echo "  make server"
	@echo "  make server build"
	@echo "  make server up"
	@echo "  make dev build"
	@echo "  make dev up"
	@echo "  make migrate-diff NAME=add_new_field"
	@echo ""
	@echo "Current configuration:"
	@echo "  Environment: $(ENVIRONMENT)"
	@if [ "$(ENVIRONMENT)" = "browser" ]; then \
		echo "  Browser: $(BROWSER)"; \
	fi

# Show status of current environment
status:
	@echo "Current configuration:"
	@echo "  Environment: $(ENVIRONMENT)"
	@if [ "$(ENVIRONMENT)" = "browser" ]; then \
		echo "  Browser: $(BROWSER)"; \
	fi
	@echo ""
	@echo "Running containers:"
	@if [ "$(ENVIRONMENT)" = "server" ] || [ "$(ENVIRONMENT)" = "browser" ] || [ "$(ENVIRONMENT)" = "dev" ]; then \
		$(DOCKER_COMPOSE_COMMAND) $(FILES) ps; \
	else \
		echo "Error: Unknown environment '$(ENVIRONMENT)'"; \
		echo "Run 'make help' for usage information."; \
		exit 1; \
	fi

# Set environment to server
server:
	$(eval ENVIRONMENT := server)
	$(eval FILES := $(SERVER_FILES))
	@echo "Environment set to: server"

# Set environment to browser with specified browser type
browser:
	@if [ -z "$(call check_browser,$(BROWSER))" ]; then \
		echo "Error: Invalid browser type '$(BROWSER)'"; \
		echo "Supported browsers: $(BROWSER_TYPES)"; \
		exit 1; \
	fi
	$(eval ENVIRONMENT := browser)
	$(eval FILES := $(BROWSER_FILES))
	@echo "Environment set to: browser ($(BROWSER))"

# Set environment to development with Air hot reloading
dev:
	$(eval ENVIRONMENT := dev)
	$(eval FILES := $(DEV_FILES))
	@echo "Environment set to: development (with Air hot reloading)"

# Session environment placeholder
session:
	@echo "Session environment has been replaced with browser environment"
	@echo "Please use 'make browser' instead"

# ======================================================
# Operation targets
# ======================================================
.PHONY: build up down clean logs shell restart config pull stop

# Build containers for the current environment
build:
	@if [ "$(ENVIRONMENT)" = "server" ]; then \
		echo "Building server containers..."; \
		$(DOCKER_COMPOSE_COMMAND) $(FILES) build; \
	elif [ "$(ENVIRONMENT)" = "dev" ]; then \
		echo "Building development containers with Air..."; \
		$(DOCKER_COMPOSE_COMMAND) $(FILES) build; \
	elif [ "$(ENVIRONMENT)" = "browser" ]; then \
    	echo "Building browser base image..."; \
    	$(DOCKER_COMPOSE_COMMAND) $(FILES) build base; \
    	echo "Building $(BROWSER) image..."; \
    	$(DOCKER_COMPOSE_COMMAND) $(FILES) build browser; \
	else \
		echo "Error: Unknown environment '$(ENVIRONMENT)'"; \
		echo "Run 'make help' for usage information."; \
		exit 1; \
	fi

# Start containers for the current environment
up:
	@if [ "$(ENVIRONMENT)" = "server" ]; then \
		echo "Starting server containers..."; \
		$(DOCKER_COMPOSE_COMMAND) $(FILES) up -d; \
	elif [ "$(ENVIRONMENT)" = "dev" ]; then \
		echo "Starting development containers with Air hot reloading..."; \
		$(DOCKER_COMPOSE_COMMAND) $(FILES) up -d; \
	elif [ "$(ENVIRONMENT)" = "browser" ]; then \
		echo "Starting services with $(BROWSER) browser..."; \
		$(DOCKER_COMPOSE_COMMAND) $(FILES) up -d browser; \
	else \
		echo "Error: Unknown environment '$(ENVIRONMENT)'"; \
		echo "Run 'make help' for usage information."; \
		exit 1; \
	fi

# Stop and remove containers for the current environment
down:
	@if [ "$(ENVIRONMENT)" = "server" ] || [ "$(ENVIRONMENT)" = "browser" ] || [ "$(ENVIRONMENT)" = "dev" ]; then \
		echo "Stopping $(ENVIRONMENT) containers..."; \
		$(DOCKER_COMPOSE_COMMAND) $(FILES) down; \
	else \
		echo "Error: Unknown environment '$(ENVIRONMENT)'"; \
		echo "Run 'make help' for usage information."; \
		exit 1; \
	fi

# Remove containers and images for the current environment
clean:
	@if [ "$(ENVIRONMENT)" = "server" ] || [ "$(ENVIRONMENT)" = "browser" ] || [ "$(ENVIRONMENT)" = "dev" ]; then \
		echo "Cleaning $(ENVIRONMENT) containers and images..."; \
		$(DOCKER_COMPOSE_COMMAND) $(FILES) down --rmi all; \
	else \
		echo "Error: Unknown environment '$(ENVIRONMENT)'"; \
		echo "Run 'make help' for usage information."; \
		exit 1; \
	fi

# View logs for the current environment
logs:
	@if [ "$(ENVIRONMENT)" = "server" ]; then \
		echo "Viewing server logs..."; \
		$(DOCKER_COMPOSE_COMMAND) $(FILES) logs -f; \
	elif [ "$(ENVIRONMENT)" = "dev" ]; then \
		echo "Viewing development logs (Air hot reloading)..."; \
		$(DOCKER_COMPOSE_COMMAND) $(FILES) logs -f; \
	elif [ "$(ENVIRONMENT)" = "browser" ]; then \
		echo "Viewing $(BROWSER) browser logs..."; \
		$(DOCKER_COMPOSE_COMMAND) $(FILES) logs -f browser; \
	else \
		echo "Error: Unknown environment '$(ENVIRONMENT)'"; \
		echo "Run 'make help' for usage information."; \
		exit 1; \
	fi

# Open shell in browser container (browser environment only)
shell:
	@if [ "$(ENVIRONMENT)" = "browser" ]; then \
		echo "Opening shell in $(BROWSER) browser container..."; \
		$(DOCKER_COMPOSE_COMMAND) $(FILES) exec browser /bin/bash; \
	else \
		echo "Error: 'shell' command is only available in browser environment"; \
		echo "Run 'make browser' first, then 'make shell'"; \
		exit 1; \
	fi

# Restart containers (server environment only)
restart:
	@if [ "$(ENVIRONMENT)" = "server" ]; then \
		echo "Restarting server containers..."; \
		$(DOCKER_COMPOSE_COMMAND) $(FILES) restart; \
	else \
		echo "Error: 'restart' command is only available in server environment"; \
		echo "Run 'make server' first, then 'make restart'"; \
		exit 1; \
	fi

# Show configuration (server environment only)
config:
	@if [ "$(ENVIRONMENT)" = "server" ]; then \
		echo "Showing server configuration..."; \
		$(DOCKER_COMPOSE_COMMAND) $(FILES) config; \
	else \
		echo "Error: 'config' command is only available in server environment"; \
		echo "Run 'make server' first, then 'make config'"; \
		exit 1; \
	fi

# Pull images (server environment only)
pull:
	@if [ "$(ENVIRONMENT)" = "server" ]; then \
		echo "Pulling server images..."; \
		$(DOCKER_COMPOSE_COMMAND) $(FILES) pull; \
	else \
		echo "Error: 'pull' command is only available in server environment"; \
		echo "Run 'make server' first, then 'make pull'"; \
		exit 1; \
	fi

# Stop services without removing (server environment only)
stop:
	@if [ "$(ENVIRONMENT)" = "server" ]; then \
		echo "Stopping server containers (without removing)..."; \
		$(DOCKER_COMPOSE_COMMAND) $(FILES) stop; \
	else \
		echo "Error: 'stop' command is only available in server environment"; \
		echo "Run 'make server' first, then 'make stop'"; \
		exit 1; \
	fi

# ======================================================
# Atlas Migration targets
# ======================================================
.PHONY: migrate-diff migrate-apply migrate-status migrate-validate

# Generate a new migration
migration:
	@if [ -z "$(NAME)" ]; then \
		echo "Generating migration with auto-generated name..."; \
		atlas migrate diff --env gorm; \
	else \
		echo "Generating migration: $(NAME)"; \
		atlas migrate diff --env gorm --name $(NAME); \
	fi

# Apply migrations to database
migrate:
	@echo "Applying migrations..."
	@if [ -z "$(DATABASE_URL)" ]; then \
		echo "Error: DATABASE_URL environment variable is required"; \
		exit 1; \
	fi
	@atlas migrate apply --dir file://browsergrid/migrations --url "$(DATABASE_URL)" --verbose

# Show migration status
migrate-status:
	@echo "Checking migration status..."
	@if [ -z "$(DATABASE_URL)" ]; then \
		echo "Error: DATABASE_URL environment variable is required"; \
		exit 1; \
	fi
	@atlas migrate status --dir file://browsergrid/migrations --url "$(DATABASE_URL)"

# Validate migrations
migrate-validate:
	@echo "Validating migrations..."
	@atlas migrate validate --dir file://browsergrid/migrations

# ======================================================
# Documentation targets
# ======================================================
.PHONY: docs

# Generate Swagger/OpenAPI documentation
docs:
	@echo "Generating Swagger documentation..."
	@cd browsergrid && go install github.com/swaggo/swag/cmd/swag@latest
	@cd browsergrid && swag init -g cmd/api/main.go -o docs --parseDependency --parseInternal  
	@echo "Swagger documentation generated in browsergrid/docs/"
	@echo "API documentation will be available at http://localhost:8080/swagger/index.html when the server is running"

# Set default target to help
.DEFAULT_GOAL := help