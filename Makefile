# Viewra Development Makefile

# Variables
PLUGINS_DIR = plugins
DOCKER_COMPOSE = docker-compose
GO = go

# Colors for output
GREEN = \033[0;32m
YELLOW = \033[1;33m
RED = \033[0;31m
CYAN = \033[0;36m
NC = \033[0m # No Color

# Go tools
GOLANGCI_LINT_VERSION = v1.62.2

.PHONY: help build-plugin build-plugins clean-binaries clean-plugins migrate-db check-db restart-backend logs check-env dev-setup rebuild-troublesome db-web db-web-stop db-web-restart db-web-logs enforce-docker-builds plugins build-plugins-docker build-plugins-host build-plugin-% setup-plugins dev-plugins logs-plugins plugin-dev plugin-setup plugin-build plugin-reload plugin-test plugin-watch plugin-fast plugin-cache refresh-plugins lint lint-fix lint-install lint-docker test test-coverage build fmt vet

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

# Plugin build system
build-plugin: ## Build a specific plugin (usage: make build-plugin p=PLUGIN_NAME)
	@if [ -z "$(p)" ]; then \
		echo "$(YELLOW)Error: Plugin name required. Usage: make build-plugin p=PLUGIN_NAME$(NC)"; \
		echo "Available plugins:"; \
		find plugins/ -maxdepth 1 -type d -name "*_*" -printf "  %f\n" 2>/dev/null | sort; \
		exit 1; \
	fi
	@./scripts/build-plugin.sh build $(p)

# Original slow build for comparison
build-plugin-slow: ## Build plugin using original slow method
	@if [ -z "$(p)" ]; then \
		echo "$(YELLOW)Error: Plugin name required. Usage: make build-plugin-slow p=PLUGIN_NAME$(NC)"; \
		exit 1; \
	fi
	@echo "$(GREEN)Building plugin for container architecture: $(p)$(NC)"
	@mkdir -p viewra-data/plugins/$(p)
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
	@ls -la viewra-data/plugins/$(p)/$(p) || echo "$(RED)❌ Binary not found!$(NC)"
	@echo "$(GREEN)✅ Plugin $(p) built for container architecture$(NC)"

build-plugins: ## Build all plugins
	@./scripts/build-plugin.sh all

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
.PHONY: plugins setup-plugins clean-plugins verify-transcoding

# Build all plugins (alias)
plugins: build-plugins

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



# Plugin development commands
plugin-setup: ## Setup plugin build environment
	@./scripts/build-plugin.sh setup

plugin-watch: ## Watch and auto-rebuild plugin (usage: make plugin-watch p=PLUGIN_NAME)
	@if [ -z "$(p)" ]; then \
		echo "$(YELLOW)Error: Plugin name required. Usage: make plugin-watch p=PLUGIN_NAME$(NC)"; \
		exit 1; \
	fi
	@./scripts/build-plugin.sh watch $(p)

plugin-list: ## List all plugins and their build status
	@./scripts/build-plugin.sh list

plugin-clean: ## Clean plugin build caches
	@./scripts/build-plugin.sh clean

# Shortcut for FFmpeg plugin rebuild
ffmpeg: ## Quick rebuild FFmpeg plugin
	@./scripts/build-plugin.sh build ffmpeg_software

# Plugin refresh commands
refresh-plugins: ## Refresh all plugins after build
	@./scripts/refresh-plugins.sh all

refresh-external: ## Refresh external plugins only
	@./scripts/refresh-plugins.sh external

refresh-playback: ## Refresh playback/transcoding plugins only
	@./scripts/refresh-plugins.sh playback

plugin-status: ## Show plugin status
	@./scripts/refresh-plugins.sh status

##@ Code Quality

lint-install: ## Install golangci-lint locally
	@echo "$(GREEN)Installing golangci-lint $(GOLANGCI_LINT_VERSION)...$(NC)"
	@if ! command -v golangci-lint >/dev/null 2>&1; then \
		curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin $(GOLANGCI_LINT_VERSION); \
	else \
		echo "golangci-lint already installed: $(golangci-lint --version)"; \
	fi

lint: ## Run Go linter on all code
	@echo "$(GREEN)Running Go linter...$(NC)"
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./... --timeout=10m; \
	else \
		echo "$(YELLOW)golangci-lint not installed. Run 'make lint-install' first or use 'make lint-docker'$(NC)"; \
		exit 1; \
	fi

lint-fix: ## Run Go linter with auto-fix
	@echo "$(GREEN)Running Go linter with auto-fix...$(NC)"
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./... --fix --timeout=10m; \
	else \
		echo "$(YELLOW)golangci-lint not installed. Run 'make lint-install' first$(NC)"; \
		exit 1; \
	fi

lint-docker: ## Run Go linter using Docker (no local install needed)
	@echo "$(GREEN)Running Go linter in Docker...$(NC)"
	@docker run --rm -v "$(shell pwd):/app" -w /app golangci/golangci-lint:$(GOLANGCI_LINT_VERSION) golangci-lint run ./... --timeout=10m

lint-plugins: ## Lint all plugins
	@echo "$(GREEN)Linting plugins...$(NC)"
	@for plugin in $(find $(PLUGINS_DIR) -maxdepth 1 -type d -name "*_*" -printf "%f\n" 2>/dev/null); do \
		echo "$(CYAN)Linting plugin: $plugin$(NC)"; \
		if [ -f "$(PLUGINS_DIR)/$plugin/go.mod" ]; then \
			cd "$(PLUGINS_DIR)/$plugin" && golangci-lint run . --timeout=5m || true; \
		fi; \
	done

lint-ci: ## Run linter in CI mode (strict)
	@echo "$(GREEN)Running Go linter in CI mode...$(NC)"
	@golangci-lint run ./... --timeout=10m --out-format=github-actions

##@ Testing & Building

test: ## Run all tests
	@echo "$(GREEN)Running tests...$(NC)"
	@go test -v ./...

test-coverage: ## Run tests with coverage report
	@echo "$(GREEN)Running tests with coverage...$(NC)"
	@go test -v -race -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "$(CYAN)Coverage report generated: coverage.html$(NC)"

test-short: ## Run short tests only
	@echo "$(GREEN)Running short tests...$(NC)"
	@go test -v -short ./...

build: ## Build the main application
	@echo "$(GREEN)Building Viewra...$(NC)"
	@go build -v -o bin/viewra ./cmd/viewra
	@echo "$(GREEN)✅ Build complete: bin/viewra$(NC)"

build-race: ## Build with race detector
	@echo "$(GREEN)Building with race detector...$(NC)"
	@go build -race -v -o bin/viewra-race ./cmd/viewra

fmt: ## Format Go code
	@echo "$(GREEN)Formatting Go code...$(NC)"
	@go fmt ./...
	@echo "$(GREEN)✅ Code formatted$(NC)"

vet: ## Run go vet
	@echo "$(GREEN)Running go vet...$(NC)"
	@go vet ./...
	@echo "$(GREEN)✅ Vet complete$(NC)"

mod-tidy: ## Tidy go modules
	@echo "$(GREEN)Tidying Go modules...$(NC)"
	@go mod tidy
	@echo "$(GREEN)✅ Modules tidied$(NC)"

mod-verify: ## Verify go modules
	@echo "$(GREEN)Verifying Go modules...$(NC)"
	@go mod verify
	@echo "$(GREEN)✅ Modules verified$(NC)"

clean: clean-binaries ## Clean build artifacts
	@echo "$(GREEN)Cleaning build artifacts...$(NC)"
	@rm -rf bin/ coverage.* 
	@echo "$(GREEN)✅ Clean complete$(NC)"

##@ Quick Commands

check: fmt vet lint test ## Run all checks (format, vet, lint, test)
	@echo "$(GREEN)✅ All checks passed!$(NC)"

ci: mod-verify lint-ci test-coverage ## Run CI checks
	@echo "$(GREEN)✅ CI checks complete!$(NC)"
