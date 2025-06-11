# Viewra Development Makefile

# Variables
BACKEND_DIR = backend
PLUGINS_DIR = $(BACKEND_DIR)/data/plugins
BUILD_SCRIPT = $(BACKEND_DIR)/scripts/build-plugin.sh
DOCKER_COMPOSE = docker-compose

# Colors for output
GREEN = \033[0;32m
YELLOW = \033[1;33m
RED = \033[0;31m
NC = \033[0m # No Color

.PHONY: help build-plugin build-plugins clean-binaries clean-plugins migrate-db check-db restart-backend logs check-env dev-setup rebuild-troublesome db-web db-web-stop db-web-restart db-web-logs enforce-docker-builds

help: ## Show this help message
	@echo "Viewra Development Commands:"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  $(GREEN)%-20s$(NC) %s\n", $$1, $$2}'
	@echo ""
	@echo "Plugin Build Examples:"
	@echo "  make build-plugin p=audiodb_enricher              # Docker build (enforced)"
	@echo "  make build-plugin p=musicbrainz_enricher          # Docker build (enforced)"
	@echo "  make build-plugin p=tmdb_enricher_v2              # Docker build (enforced)"
	@echo ""
	@echo "$(RED)⚠️  All plugin builds now use Docker containers for consistency$(NC)"
	@echo "📖 For comprehensive documentation see: DEVELOPMENT.md"
	@echo ""

# Enforce Docker builds for all plugins - no more host builds
enforce-docker-builds: ## Check that Docker is available for plugin builds
	@echo "$(GREEN)Checking Docker environment for plugin builds...$(NC)"
	@if ! command -v docker >/dev/null 2>&1; then \
		echo "$(RED)❌ Docker not found. Plugin builds require Docker for consistency.$(NC)"; \
		echo "$(YELLOW)Please install Docker to build plugins.$(NC)"; \
		exit 1; \
	fi
	@if ! docker ps >/dev/null 2>&1; then \
		echo "$(RED)❌ Docker daemon not running. Please start Docker.$(NC)"; \
		exit 1; \
	fi
	@container_id=$$(docker ps --filter "expose=8080" --format "{{.ID}}" | head -1); \
	if [ -z "$$container_id" ]; then \
		echo "$(RED)❌ Backend container not running. Starting services...$(NC)"; \
		$(DOCKER_COMPOSE) up -d backend; \
		sleep 5; \
		container_id=$$(docker ps --filter "expose=8080" --format "{{.ID}}" | head -1); \
		if [ -z "$$container_id" ]; then \
			echo "$(RED)❌ Failed to start backend container$(NC)"; \
			exit 1; \
		fi; \
	fi
	@echo "$(GREEN)✅ Docker environment ready for plugin builds$(NC)"

build-plugin: enforce-docker-builds ## Build a specific plugin using Docker (usage: make build-plugin p=PLUGIN_NAME)
	@if [ -z "$(p)" ]; then \
		echo "$(YELLOW)Error: Plugin name required. Usage: make build-plugin p=PLUGIN_NAME$(NC)"; \
		echo "Available plugins:"; \
		find $(PLUGINS_DIR) -maxdepth 1 -type d -name "*_*" -printf "  %f\n" 2>/dev/null | sort; \
		exit 1; \
	fi
	@echo "$(GREEN)Building plugin: $(p) (Docker-only mode)$(NC)"
	@chmod +x $(BUILD_SCRIPT)
	@$(BUILD_SCRIPT) $(p) container

build-plugins: enforce-docker-builds ## Build all plugins using Docker containers
	@echo "$(GREEN)Building all plugins using Docker containers...$(NC)"
	@for plugin in $$(find $(PLUGINS_DIR) -maxdepth 1 -type d -name "*_*" -printf "%f\n" 2>/dev/null); do \
		echo "$(GREEN)Building $$plugin in Docker container...$(NC)"; \
		$(MAKE) build-plugin p=$$plugin || echo "$(YELLOW)Failed to build $$plugin$(NC)"; \
	done

# Remove the old host/container mode options - everything is Docker now
build-plugins-container: build-plugins ## Alias for build-plugins (all builds are now containerized)

# Remove host build option entirely
build-plugins-host: ## DEPRECATED: Host builds are no longer supported
	@echo "$(RED)❌ Host builds are no longer supported for consistency and reliability.$(NC)"
	@echo "$(YELLOW)All plugin builds now use Docker containers.$(NC)"
	@echo "$(GREEN)Use 'make build-plugins' instead.$(NC)"
	@exit 1

clean-binaries: ## Remove all plugin binaries
	@echo "$(GREEN)Cleaning plugin binaries...$(NC)"
	@find $(PLUGINS_DIR) -name "*_*" -type f -executable -delete 2>/dev/null || true
	@find $(PLUGINS_DIR) -name "*.exe" -type f -delete 2>/dev/null || true
	@echo "Plugin binaries cleaned"

clean-plugins: ## Clean plugin build artifacts and caches
	@echo "$(GREEN)Cleaning plugin build artifacts...$(NC)"
	@for plugin in $$(find $(PLUGINS_DIR) -maxdepth 1 -type d -name "*_*" -printf "%f\n" 2>/dev/null); do \
		echo "Cleaning $$plugin..."; \
		(cd $(PLUGINS_DIR)/$$plugin && go clean -cache -modcache -testcache 2>/dev/null || true); \
	done
	@$(MAKE) clean-binaries
	@echo "Plugin cleanup completed"

test-plugin: ## Test a specific plugin build (usage: make test-plugin p=PLUGIN_NAME)
	@if [ -z "$(p)" ]; then \
		echo "$(YELLOW)Error: Plugin name required. Usage: make test-plugin p=PLUGIN_NAME$(NC)"; \
		exit 1; \
	fi
	@echo "$(GREEN)Testing plugin: $(p)$(NC)"
	@if [ -f "$(PLUGINS_DIR)/$(p)/$(p)" ]; then \
		echo "Binary exists: $(PLUGINS_DIR)/$(p)/$(p)"; \
		file "$(PLUGINS_DIR)/$(p)/$(p)"; \
		ls -lh "$(PLUGINS_DIR)/$(p)/$(p)"; \
		if command -v docker >/dev/null 2>&1; then \
			container_id=$$(docker ps --filter "expose=8080" --format "{{.ID}}" | head -1); \
			if [ -n "$$container_id" ]; then \
				echo "Testing binary in container..."; \
				docker exec "$$container_id" test -x "/app/data/plugins/$(p)/$(p)" && echo "✅ Binary is executable in container" || echo "❌ Binary is not executable in container"; \
			fi; \
		fi; \
	else \
		echo "❌ Binary not found: $(PLUGINS_DIR)/$(p)/$(p)"; \
		exit 1; \
	fi

migrate-db: ## Move database to proper location (viewra-data/viewra.db)
	@echo "$(GREEN)Migrating database to viewra-data/viewra.db...$(NC)"
	@if [ -f "$(BACKEND_DIR)/data/viewra.db" ] && [ ! -f "viewra-data/viewra.db" ]; then \
		echo "Moving database from backend/data/ to viewra-data/"; \
		mv "$(BACKEND_DIR)/data/viewra.db" "viewra-data/viewra.db"; \
		echo "✅ Database migrated successfully"; \
	elif [ -f "viewra-data/viewra.db" ]; then \
		echo "✅ Database already in correct location"; \
	else \
		echo "ℹ️ No existing database to migrate"; \
	fi

check-db: ## Check database status and show information
	@echo "$(CYAN)Database Status:$(NC)"
	@if [ -f "viewra-data/viewra.db" ]; then \
		echo "Database location: viewra-data/viewra.db"; \
		echo "Database size: $$(du -h viewra-data/viewra.db | cut -f1)"; \
		echo "Tables: $$(sqlite3 viewra-data/viewra.db '.tables' 2>/dev/null | wc -w || echo 'Error reading database')"; \
		if [ -n "$$(docker ps -q -f name=viewra-backend)" ]; then \
			container_id=$$(docker ps -q -f name=viewra-backend); \
			echo "Container database path: /app/viewra-data/viewra.db"; \
			docker exec "$$container_id" test -f "/app/viewra-data/viewra.db" && echo "✅ Database accessible in container" || echo "❌ Database not accessible in container"; \
		else \
			echo "ℹ️ Backend container not running"; \
		fi; \
	else \
		echo "❌ Database not found at viewra-data/viewra.db"; \
	fi

restart-backend: ## Restart the backend container
	@echo "$(GREEN)Restarting backend container...$(NC)"
	@$(DOCKER_COMPOSE) restart backend
	@echo "Backend restarted. Waiting for startup..."
	@sleep 3
	@echo "Backend should be available at http://localhost:8080"

logs: ## Show backend container logs
	@$(DOCKER_COMPOSE) logs -f backend

check-env: ## Check development environment
	@echo "$(GREEN)Checking development environment...$(NC)"
	@echo "Docker: $$(command -v docker >/dev/null && echo "✅ Available" || echo "❌ Not found")"
	@echo "Docker Compose: $$(command -v docker-compose >/dev/null && echo "✅ Available" || echo "❌ Not found")"
	@echo "Go: $$(command -v go >/dev/null && echo "✅ Available ($$(go version))" || echo "❌ Not found")"
	@echo "Architecture: $$(uname -m)"
	@if command -v docker >/dev/null 2>&1; then \
		container_id=$$(docker ps --filter "expose=8080" --format "{{.ID}}" | head -1); \
		if [ -n "$$container_id" ]; then \
			echo "Backend container: ✅ Running ($$container_id)"; \
			echo "Container architecture: $$(docker exec "$$container_id" uname -m)"; \
		else \
			echo "Backend container: ❌ Not running"; \
		fi; \
	fi
	@echo "Available plugins: $$(find $(PLUGINS_DIR) -maxdepth 1 -type d -name "*_*" | wc -l)"
	@for plugin in $$(find $(PLUGINS_DIR) -maxdepth 1 -type d -name "*_*" -printf "%f\n" 2>/dev/null); do \
		if [ -f "$(PLUGINS_DIR)/$$plugin/$$plugin" ]; then \
			echo "  $$plugin: ✅ Binary exists"; \
		else \
			echo "  $$plugin: ❌ No binary"; \
		fi; \
	done

dev-setup: ## Initial development environment setup
	@echo "$(GREEN)Setting up development environment...$(NC)"
	@$(MAKE) migrate-db
	@$(MAKE) build-plugins
	@$(MAKE) restart-backend
	@echo "$(GREEN)Development setup completed!$(NC)"

rebuild-troublesome: ## Rebuild plugins that commonly have issues (CGO-dependent ones)
	@echo "$(GREEN)Rebuilding CGO-dependent plugins with container builds...$(NC)"
	@$(MAKE) build-plugin p=audiodb_enricher mode=container
	@echo "$(GREEN)Troublesome plugins rebuilt$(NC)"

db-web: ## Start SQLite Web for database visualization
	@echo "$(GREEN)Starting SQLite Web...$(NC)"
	@$(DOCKER_COMPOSE) --profile dev up -d sqliteweb
	@echo "SQLite Web available at http://localhost:8081"

db-web-stop: ## Stop SQLite Web
	@echo "$(GREEN)Stopping SQLite Web...$(NC)"
	@$(DOCKER_COMPOSE) stop sqliteweb

db-web-restart: ## Restart SQLite Web
	@echo "$(GREEN)Restarting SQLite Web...$(NC)"
	@$(DOCKER_COMPOSE) restart sqliteweb
	@echo "SQLite Web available at http://localhost:8081"

db-web-logs: ## Show SQLite Web logs
