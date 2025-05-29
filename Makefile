# Viewra Development Makefile

# Variables
BACKEND_DIR = backend
PLUGINS_DIR = $(BACKEND_DIR)/data/plugins
BUILD_SCRIPT = $(BACKEND_DIR)/scripts/build-plugin.sh
DOCKER_COMPOSE = docker-compose

# Colors for output
GREEN = \033[0;32m
YELLOW = \033[1;33m
NC = \033[0m # No Color

.PHONY: help build-plugin build-plugins clean-binaries clean-plugins migrate-db check-db restart-backend logs check-env dev-setup rebuild-troublesome db-web db-web-stop db-web-restart db-web-logs

help: ## Show this help message
	@echo "Viewra Development Commands:"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  $(GREEN)%-20s$(NC) %s\n", $$1, $$2}'
	@echo ""
	@echo "Plugin Build Examples:"
	@echo "  make build-plugin p=audiodb_enricher              # Auto-build (detects CGO)"
	@echo "  make build-plugin p=audiodb_enricher mode=container  # Force container build"
	@echo "  make build-plugin p=musicbrainz_enricher mode=host   # Force host build"
	@echo ""
	@echo "📖 For comprehensive documentation see: DEVELOPMENT.md"
	@echo ""

build-plugin: ## Build a specific plugin (usage: make build-plugin p=PLUGIN_NAME [mode=auto|host|container] [arch=amd64|arm64])
	@if [ -z "$(p)" ]; then \
		echo "$(YELLOW)Error: Plugin name required. Usage: make build-plugin p=PLUGIN_NAME$(NC)"; \
		echo "Available plugins:"; \
		find $(PLUGINS_DIR) -maxdepth 1 -type d -name "*_*" -printf "  %f\n" 2>/dev/null | sort; \
		exit 1; \
	fi
	@echo "$(GREEN)Building plugin: $(p)$(NC)"
	@chmod +x $(BUILD_SCRIPT)
	@$(BUILD_SCRIPT) $(p) $(or $(mode),auto) $(arch)

build-plugins: ## Build all plugins using auto-detection
	@echo "$(GREEN)Building all plugins...$(NC)"
	@for plugin in $$(find $(PLUGINS_DIR) -maxdepth 1 -type d -name "*_*" -printf "%f\n" 2>/dev/null); do \
		echo "$(GREEN)Building $$plugin...$(NC)"; \
		$(MAKE) build-plugin p=$$plugin mode=auto || echo "$(YELLOW)Failed to build $$plugin$(NC)"; \
	done

build-plugins-container: ## Build all plugins using container builds for maximum compatibility
	@echo "$(GREEN)Building all plugins in container for maximum compatibility...$(NC)"
	@for plugin in $$(find $(PLUGINS_DIR) -maxdepth 1 -type d -name "*_*" -printf "%f\n" 2>/dev/null); do \
		echo "$(GREEN)Building $$plugin in container...$(NC)"; \
		$(MAKE) build-plugin p=$$plugin mode=container || echo "$(YELLOW)Failed to build $$plugin$(NC)"; \
	done

build-plugins-host: ## Build all plugins on host (fastest, may have compatibility issues)
	@echo "$(GREEN)Building all plugins on host...$(NC)"
	@for plugin in $$(find $(PLUGINS_DIR) -maxdepth 1 -type d -name "*_*" -printf "%f\n" 2>/dev/null); do \
		echo "$(GREEN)Building $$plugin on host...$(NC)"; \
		$(MAKE) build-plugin p=$$plugin mode=host || echo "$(YELLOW)Failed to build $$plugin$(NC)"; \
	done

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

migrate-db: ## Move database to proper location (viewra-data/database.db)
	@echo "$(GREEN)Migrating database to viewra-data/database.db...$(NC)"
	@if [ -f "$(BACKEND_DIR)/data/viewra.db" ] && [ ! -f "viewra-data/database.db" ]; then \
		mkdir -p viewra-data; \
		mv "$(BACKEND_DIR)/data/viewra.db" "viewra-data/database.db"; \
		echo "Database migrated successfully"; \
	elif [ -f "viewra-data/database.db" ]; then \
		echo "Database already at correct location"; \
	else \
		echo "No database to migrate"; \
	fi

check-db: ## Check database status and size
	@echo "$(GREEN)Checking database status...$(NC)"
	@if [ -f "viewra-data/database.db" ]; then \
		echo "Database location: viewra-data/database.db"; \
		echo "Database size: $$(du -h viewra-data/database.db | cut -f1)"; \
		if command -v docker >/dev/null 2>&1; then \
			container_id=$$(docker ps --filter "expose=8080" --format "{{.ID}}" | head -1); \
			if [ -n "$$container_id" ]; then \
				echo "Container database path: /app/viewra-data/database.db"; \
				docker exec "$$container_id" test -f "/app/viewra-data/database.db" && echo "✅ Database accessible in container" || echo "❌ Database not accessible in container"; \
			fi; \
		fi; \
	else \
		echo "❌ Database not found at viewra-data/database.db"; \
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
