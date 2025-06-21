# Viewra Development Makefile

# Variables
PLUGINS_DIR = plugins
DOCKER_COMPOSE = docker-compose
GO = go

# Colors for output
GREEN = \033[0;32m
YELLOW = \033[1;33m
RED = \033[0;31m
NC = \033[0m # No Color

.PHONY: help build-plugin build-plugins clean-binaries clean-plugins migrate-db check-db restart-backend logs check-env dev-setup rebuild-troublesome db-web db-web-stop db-web-restart db-web-logs enforce-docker-builds plugins build-plugins-docker build-plugins-host build-plugin-% setup-plugins dev-plugins logs-plugins plugin-dev plugin-setup plugin-build plugin-reload plugin-test

help: ## Show this help message
	@echo "Viewra Development Commands:"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  $(GREEN)%-20s$(NC) %s\n", $$1, $$2}'
	@echo ""
	@echo "Plugin Build Examples:"
	@echo "  make build-plugin p=audiodb_enricher              # Build specific plugin"
	@echo "  make build-plugin p=musicbrainz_enricher          # Build specific plugin"
	@echo "  make build-plugin p=tmdb_enricher_v2              # Build specific plugin"
	@echo "  make build-plugins                                # Build all plugins"
	@echo ""
	@echo "$(GREEN)✅ Fast local builds for rapid development$(NC)"
	@echo "📖 For comprehensive documentation see: DEVELOPMENT.md"
	@echo ""

# Simplified build system - support both local and container builds
build-plugin: ## Build a specific plugin (usage: make build-plugin p=PLUGIN_NAME)
	@if [ -z "$(p)" ]; then \
		echo "$(YELLOW)Error: Plugin name required. Usage: make build-plugin p=PLUGIN_NAME$(NC)"; \
		echo "Available plugins:"; \
		find plugins/ -maxdepth 1 -type d -name "*_*" -printf "  %f\n" 2>/dev/null | sort; \
		exit 1; \
	fi
	@echo "$(GREEN)Building plugin for container architecture: $(p)$(NC)"
	@mkdir -p viewra-data/plugins/$(p)
	@# Use the same base image as backend to ensure compatibility
	@docker run --rm \
		-v $(shell pwd):/workspace \
		-w /workspace \
		--platform linux/amd64 \
		golang:1.24-alpine \
		sh -c " \
			apk add --no-cache gcc musl-dev git && \
			cd plugins/$(p) && \
			CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -buildvcs=false -o /workspace/viewra-data/plugins/$(p)/$(p) . && \
			chmod +x /workspace/viewra-data/plugins/$(p)/$(p) \
		"
	@if [ -f "$(PLUGINS_DIR)/$(p)/plugin.cue" ]; then \
		cp $(PLUGINS_DIR)/$(p)/plugin.cue viewra-data/plugins/$(p)/; \
	fi
	@# Verify the binary is executable and correct architecture
	@echo "$(YELLOW)Verifying plugin binary...$(NC)"
	@ls -la viewra-data/plugins/$(p)/$(p) || echo "$(RED)❌ Binary not found!$(NC)"
	@echo "$(GREEN)✅ Plugin $(p) built for container architecture$(NC)"

build-plugins: ## Build all plugins locally
	@echo "$(GREEN)Building all plugins locally...$(NC)"
	@for plugin in $$(find $(PLUGINS_DIR) -maxdepth 1 -type d -name "*_*" -printf "%f\n" 2>/dev/null); do \
		echo "$(GREEN)Building $$plugin...$(NC)"; \
		$(MAKE) build-plugin p=$$plugin || echo "$(YELLOW)Failed to build $$plugin$(NC)"; \
	done
	@echo "$(GREEN)✅ All plugins built$(NC)"

# Remove the old host/container mode options - everything is Docker now
build-plugins-container: build-plugins ## Alias for build-plugins (all builds are now containerized)

clean-binaries: ## Remove all plugin binaries
	@echo "$(GREEN)Cleaning plugin binaries...$(NC)"
	@find $(PLUGINS_DIR) -name "*_*" -type f -executable -delete 2>/dev/null || true
	@find $(PLUGINS_DIR) -name "*.exe" -type f -delete 2>/dev/null || true
	@echo "Plugin binaries cleaned"

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

# Hot-reload development mode
dev-hot: check-env ## Start development with hot-reload (no caching issues!)
	@echo "$(GREEN)Starting development with hot-reload...$(NC)"
	docker-compose -f docker-compose.yml -f docker-compose.dev.yml up

# Production build
prod-build: ## Build for production
	@echo "$(GREEN)Building for production...$(NC)"
	docker-compose -f docker-compose.yml -f docker-compose.prod.yml build

# Production deployment
prod-deploy: prod-build ## Deploy production environment
	@echo "$(GREEN)Deploying production...$(NC)"
	docker-compose -f docker-compose.yml -f docker-compose.prod.yml up -d

# Clean rebuild (no cache)
rebuild-clean: ## Force clean rebuild without any cache
	@echo "$(YELLOW)Performing clean rebuild...$(NC)"
	docker-compose down -v
	docker-compose build --no-cache
	docker-compose up -d

# Debug playback issues
debug-playback: ## Debug playback/transcoding issues
	@echo "$(GREEN)Running playback debug script...$(NC)"
	@bash scripts/debug-playback.sh

# Debug FFmpeg specifically
debug-ffmpeg: ## Debug FFmpeg processes and issues
	@echo "$(GREEN)Running FFmpeg debug script...$(NC)"
	@bash scripts/ffmpeg-debug.sh all

# Development with Air hot-reload (recommended)
dev: check-env ## Start development with Air hot-reload
	@echo "$(GREEN)Starting development with Air hot-reload...$(NC)"
	@echo "$(YELLOW)Changes to Go files will automatically rebuild and restart$(NC)"
	@echo "$(YELLOW)FFmpeg debug mode enabled - check /app/viewra-data/transcoding/debug/$(NC)"
	docker-compose -f docker-compose.yml -f docker-compose.dev.yml up

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

# Plugin management targets
.PHONY: plugins build-plugins setup-plugins clean-plugins verify-transcoding

# Build all plugins with auto-detection
plugins:
	@echo "🔨 Building plugins..."
	@./scripts/build-plugins.sh

# Build plugins in Docker (force mode)
build-plugins-docker:
	@echo "🐳 Building plugins in Docker..."
	@./scripts/build-plugins.sh docker

# Build plugins on host (force mode)
build-plugins-host:
	@echo "🏠 Building plugins on host..."
	@./scripts/build-plugins.sh host

# Build specific plugin
build-plugin-%:
	@echo "🎯 Building plugin: $*"
	@./scripts/build-plugins.sh auto $*

# Complete plugin setup (build + enable + restart)
setup-plugins:
	@echo "⚙️ Setting up plugins..."
	@./scripts/setup-plugins.sh

# Clean all plugin binaries
clean-plugins:
	@echo "🧹 Cleaning plugin binaries..."
	@find backend/data/plugins -name "*_transcoder" -o -name "*_enricher" -o -name "*_scanner" | xargs -I {} find {} -type f -executable -name "*_*" -delete 2>/dev/null || true
	@echo "✅ Plugin binaries cleaned"

# Quick development workflow
dev-plugins: clean-plugins build-plugins-docker
	@echo "🚀 Development plugin build complete"
	@docker-compose logs backend --tail=5 | grep -i plugin || echo "Check logs with: make logs-plugins"

# Show plugin-related logs
logs-plugins:
	@echo "📋 Recent plugin logs:"
	@docker-compose logs backend --tail=20 | grep -i plugin || echo "No recent plugin logs found"

# Verify transcoding setup
verify-transcoding: ## Verify FFmpeg transcoder is properly configured
	@echo "$(GREEN)Verifying transcoding setup...$(NC)"
	@bash scripts/verify-transcoding-setup.sh

# =====================================
# Enhanced Plugin Development Workflow
# =====================================

plugin-setup: ## Setup complete plugin development environment
	@echo "$(GREEN)Setting up plugin development environment...$(NC)"
	@./scripts/plugin-dev.sh setup

plugin-build: ## Build specific plugin (usage: make plugin-build p=PLUGIN_NAME)
	@if [ -z "$(p)" ]; then \
		echo "$(YELLOW)Error: Plugin name required. Usage: make plugin-build p=PLUGIN_NAME$(NC)"; \
		echo "Available transcoding plugins:"; \
		find plugins/ -maxdepth 1 -type d -name "ffmpeg_*" -printf "  %f\n" 2>/dev/null | sort; \
		exit 1; \
	fi
	@./scripts/plugin-dev.sh build $(p)

plugin-reload: ## Hot reload plugin (usage: make plugin-reload p=PLUGIN_NAME)
	@if [ -z "$(p)" ]; then \
		echo "$(YELLOW)Error: Plugin name required. Usage: make plugin-reload p=PLUGIN_NAME$(NC)"; \
		exit 1; \
	fi
	@./scripts/plugin-dev.sh reload $(p)

plugin-enable: ## Enable plugin (usage: make plugin-enable p=PLUGIN_NAME)
	@if [ -z "$(p)" ]; then \
		echo "$(YELLOW)Error: Plugin name required. Usage: make plugin-enable p=PLUGIN_NAME$(NC)"; \
		exit 1; \
	fi
	@./scripts/plugin-dev.sh enable $(p)

plugin-disable: ## Disable plugin (usage: make plugin-disable p=PLUGIN_NAME)
	@if [ -z "$(p)" ]; then \
		echo "$(YELLOW)Error: Plugin name required. Usage: make plugin-disable p=PLUGIN_NAME$(NC)"; \
		exit 1; \
	fi
	@./scripts/plugin-dev.sh disable $(p)

plugin-test: ## Test plugin functionality (usage: make plugin-test p=PLUGIN_NAME)
	@if [ -z "$(p)" ]; then \
		echo "$(YELLOW)Error: Plugin name required. Usage: make plugin-test p=PLUGIN_NAME$(NC)"; \
		exit 1; \
	fi
	@./scripts/plugin-dev.sh test $(p)

plugin-list: ## List all transcoding plugins and their status
	@./scripts/plugin-dev.sh list

plugin-dev: ## Start complete plugin development workflow
	@echo "$(GREEN)🚀 Starting plugin development workflow...$(NC)"
	@echo ""
	@echo "$(GREEN)Step 1:$(NC) Setting up environment..."
	@./scripts/plugin-dev.sh setup
	@echo ""
	@echo "$(GREEN)Step 2:$(NC) Available commands:"
	@echo "  make plugin-build p=ffmpeg_software    # Build specific plugin"
	@echo "  make plugin-reload p=ffmpeg_software   # Hot reload plugin"
	@echo "  make plugin-test p=ffmpeg_software     # Test plugin"
	@echo "  make plugin-list                       # List all plugins"
	@echo ""
	@echo "$(GREEN)✅ Plugin development environment ready!$(NC)"
