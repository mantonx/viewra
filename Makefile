.PHONY: help dev dev-debug dev-clean stop build build-tools build-ffmpeg clean-ffmpeg clean test migrate-up migrate-down migrate-sqlite-up migrate-sqlite-down migrate-create migrate-sqlite-create sqlc-gen swagger-gen api-client-gen proto-gen install-tools install-ollama ollama-status setup build-plugins build-plugin reload-plugin reload-plugins clean-plugins new-plugin

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-20s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

install-tools: ## Install development tools (Air, sqlc, swag, migrate)
	@echo "Installing development tools..."
	go install github.com/air-verse/air@latest
	go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
	go install github.com/swaggo/swag/cmd/swag@latest
	go install -tags 'sqlite3' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
	@echo "Tools installed successfully!"

install-ollama: ## Install Ollama and pull embedding model for AI features
	@echo "Installing Ollama..."
	@if command -v ollama >/dev/null 2>&1; then \
		echo "✓ Ollama already installed"; \
	else \
		curl -fsSL https://ollama.com/install.sh | sh; \
	fi
	@echo "Pulling nomic-embed-text embedding model..."
	@ollama pull nomic-embed-text
	@echo "✓ Ollama ready! Run 'ollama serve' if not using systemd."

ollama-status: ## Check Ollama status and available models
	@echo "Checking Ollama status..."
	@if curl -s http://localhost:11434/api/tags >/dev/null 2>&1; then \
		echo "✓ Ollama is running"; \
		echo "Available models:"; \
		curl -s http://localhost:11434/api/tags | jq -r '.models[].name' 2>/dev/null || curl -s http://localhost:11434/api/tags; \
	else \
		echo "✗ Ollama is not running. Start with: systemctl start ollama"; \
	fi

dev-clean: ## Clean up stale dev processes and sockets
	@echo "🧹 Cleaning up stale development state..."
	@if pgrep overmind >/dev/null 2>&1; then \
		overmind quit 2>/dev/null || true; \
		sleep 1; \
	fi
	@killall -9 air 2>/dev/null || true
	@killall -9 main 2>/dev/null || true
	@lsof -ti:8080 2>/dev/null | xargs kill -9 2>/dev/null || true
	@lsof -ti:5173 2>/dev/null | xargs kill -9 2>/dev/null || true
	@rm -f .overmind.sock
	@echo "✓ Cleanup complete"

dev: ## Start development servers with INFO logging (use dev-debug for DEBUG logs)
	@if [ ! -x "$$(command -v overmind)" ]; then \
		echo "❌ Overmind not found. Install: brew install overmind (macOS) or go install github.com/DarthSim/overmind/v2@latest"; \
		exit 1; \
	fi
	@if [ -e .overmind.sock ]; then \
		echo "🧹 Cleaning up stale session..."; \
		$(MAKE) -s dev-clean; \
	fi
	@if lsof -i:8080 -sTCP:LISTEN -t >/dev/null 2>&1; then \
		echo "⚠️  Port 8080 in use. Run: make dev-clean"; \
		exit 1; \
	fi
	@if lsof -i:5173 -sTCP:LISTEN -t >/dev/null 2>&1; then \
		echo "⚠️  Port 5173 in use. Run: make dev-clean"; \
		exit 1; \
	fi
	@echo "🚀 Starting dev servers: http://localhost:8080 | http://localhost:5173"
	@overmind start

dev-debug: ## Start development servers with DEBUG logging
	@if [ ! -x "$$(command -v overmind)" ]; then \
		echo "❌ Overmind not found. Install: brew install overmind (macOS) or go install github.com/DarthSim/overmind/v2@latest"; \
		exit 1; \
	fi
	@if [ -e .overmind.sock ]; then \
		echo "🧹 Cleaning up stale session..."; \
		$(MAKE) -s dev-clean; \
	fi
	@if lsof -i:8080 -sTCP:LISTEN -t >/dev/null 2>&1; then \
		echo "⚠️  Port 8080 in use. Run: make dev-clean"; \
		exit 1; \
	fi
	@if lsof -i:5173 -sTCP:LISTEN -t >/dev/null 2>&1; then \
		echo "⚠️  Port 5173 in use. Run: make dev-clean"; \
		exit 1; \
	fi
	@echo "🚀 Starting dev servers with DEBUG logging: http://localhost:8080 | http://localhost:5173"
	@LOG_LEVEL=DEBUG overmind start

stop: ## Stop all development servers
	@echo "🛑 Stopping services..."
	@if pgrep overmind >/dev/null 2>&1; then \
		overmind quit 2>/dev/null || true; \
		sleep 1; \
	fi
	@killall -9 air 2>/dev/null || true
	@killall -9 main 2>/dev/null || true
	@lsof -ti:8080 2>/dev/null | xargs kill -9 2>/dev/null || true
	@lsof -ti:5173 2>/dev/null | xargs kill -9 2>/dev/null || true
	@rm -f .overmind.sock
	@echo "✓ Stopped"

build-tools: ## Build Rust helper tools (subtitle-extractor)
	@echo "Building subtitle-extractor..."
	@if ! command -v cargo >/dev/null 2>&1; then \
		echo "❌ Rust/Cargo not found. Install from https://rustup.rs/"; \
		exit 1; \
	fi
	@mkdir -p bin
	cd tools/subtitle-extractor && cargo build --release
	cp tools/subtitle-extractor/target/release/subtitle-extractor bin/
	@echo "✓ subtitle-extractor built: bin/subtitle-extractor"

build-ffmpeg: ## Build patched FFmpeg with ViewRA fixes (optional, ~10min build)
	@echo "Building patched FFmpeg 7.1..."
	@if [ -f bin/ffmpeg-viewra ]; then \
		echo "✓ FFmpeg already built: bin/ffmpeg-viewra (use 'make clean-ffmpeg' to rebuild)"; \
	else \
		cd tools/ffmpeg-viewra && ./build.sh && \
		mkdir -p $(CURDIR)/bin/ffmpeg-lib && \
		cp dist/bin/ffmpeg $(CURDIR)/bin/ffmpeg-viewra && \
		cp dist/bin/ffprobe $(CURDIR)/bin/ffprobe-viewra && \
		cp -a dist/lib/*.so* $(CURDIR)/bin/ffmpeg-lib/ && \
		echo "✓ Patched FFmpeg built: bin/ffmpeg-viewra"; \
	fi

clean-ffmpeg: ## Clean FFmpeg build artifacts
	rm -rf tools/ffmpeg-viewra/build tools/ffmpeg-viewra/dist
	rm -f bin/ffmpeg-viewra bin/ffprobe-viewra
	rm -rf bin/ffmpeg-lib
	@echo "✓ FFmpeg build cleaned"

build: build-tools ## Build production binaries with version info
	@echo "Building frontend..."
	cd web && npm run build
	@echo "Building backend with embedded frontend..."
	@VERSION=$$(git describe --tags --always --dirty 2>/dev/null || echo "dev"); \
	COMMIT=$$(git rev-parse --short HEAD 2>/dev/null || echo "unknown"); \
	BUILD_DATE=$$(date -u '+%Y-%m-%d_%H:%M:%S'); \
	echo "Version: $$VERSION, Commit: $$COMMIT, Built: $$BUILD_DATE"; \
	go build -ldflags "-X github.com/mantonx/viewra/internal/version.Version=$$VERSION -X github.com/mantonx/viewra/internal/version.Commit=$$COMMIT -X github.com/mantonx/viewra/internal/version.BuildDate=$$BUILD_DATE" -o bin/viewra ./cmd/viewra
	@echo "✓ Build complete! Binaries: bin/viewra, bin/subtitle-extractor"

clean: ## Clean build artifacts and temporary files
	rm -rf tmp/
	rm -rf bin/
	rm -rf web/dist/
	rm -rf tools/subtitle-extractor/target/
	@echo "Cleaned build artifacts"

test: ## Run tests
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

migrate-up: ## Run all database migrations up (main + SQLite-specific)
	@echo "Running main migrations..."
	~/go/bin/migrate -path migrations -database "sqlite3://data/viewra.db" up
	@if ls migrations/sqlite/*.sql 1> /dev/null 2>&1; then \
		echo "Running SQLite-specific migrations..."; \
		~/go/bin/migrate -path migrations/sqlite -database "sqlite3://data/viewra.db?x-migrations-table=schema_migrations_sqlite" up; \
	else \
		echo "No SQLite-specific migrations found, skipping..."; \
	fi

migrate-down: ## Run database migrations down (main only - use migrate-sqlite-down for SQLite-specific)
	~/go/bin/migrate -path migrations -database "sqlite3://data/viewra.db" down

migrate-sqlite-up: ## Run SQLite-specific migrations up
	~/go/bin/migrate -path migrations/sqlite -database "sqlite3://data/viewra.db?x-migrations-table=schema_migrations_sqlite" up

migrate-sqlite-down: ## Run SQLite-specific migrations down
	~/go/bin/migrate -path migrations/sqlite -database "sqlite3://data/viewra.db?x-migrations-table=schema_migrations_sqlite" down

migrate-create: ## Create a new migration (usage: make migrate-create NAME=add_users)
	@if [ -z "$(NAME)" ]; then echo "Usage: make migrate-create NAME=migration_name"; exit 1; fi
	migrate create -ext sql -dir migrations -seq $(NAME)

migrate-sqlite-create: ## Create a new SQLite-specific migration (usage: make migrate-sqlite-create NAME=add_vec0)
	@if [ -z "$(NAME)" ]; then echo "Usage: make migrate-sqlite-create NAME=migration_name"; exit 1; fi
	migrate create -ext sql -dir migrations/sqlite -seq $(NAME)

sqlc-gen: ## Generate sqlc code from queries (runs sqlc + post-processing for unified DB layer)
	~/go/bin/sqlc generate
	go run ./cmd/sqlc-gen

taskgen: ## Generate scheduler task registration code
	go run ./cmd/taskgen

swagger-gen: ## Generate Swagger documentation
	~/go/bin/swag init -g cmd/viewra/main.go -o docs/swagger --parseDependency --parseInternal

api-client-gen: swagger-gen ## Generate TypeScript API client from Swagger
	cd web && npm run generate:api
	@echo "API client generated successfully in web/src/lib/api/generated/"

proto-gen: ## Generate Go code from protobuf definitions
	@echo "Generating protobuf code..."
	PATH="$(HOME)/go/bin:$(PATH)" protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		api/proto/plugin/common.proto \
		api/proto/plugin/enricher.proto \
		api/proto/plugin/host_services.proto \
		api/proto/plugin/plugin_core.proto \
		api/proto/plugin/provider.proto \
		api/proto/plugin/search_provider.proto \
		api/proto/plugin/trending.proto \
		api/proto/plugin/vector_search.proto
	@echo "✓ Protobuf code generated"

setup: install-tools ## Initial project setup
	@echo "Setting up project..."
	@echo "Checking Rust toolchain..."
	@if ! command -v cargo >/dev/null 2>&1; then \
		echo "⚠️  Rust/Cargo not found. Install from https://rustup.rs/ for subtitle extraction support."; \
	else \
		echo "✓ Rust toolchain found"; \
	fi
	mkdir -p data
	mkdir -p tmp
	cd web && npm install
	@echo "Setup complete! Run 'make dev' to start development servers"

# =============================================================================
# Plugin Build System
# =============================================================================

build-plugins: ## Build all plugins from plugins/ directory
	@echo "Building all plugins..."
	@mkdir -p data/plugins
	@for dir in plugins/*/; do \
		if [ -f "$$dir/main.go" ] || [ -f "$$dir/go.mod" ]; then \
			name=$$(basename "$$dir"); \
			echo "Building plugin: $$name"; \
			mkdir -p "data/plugins/$$name"; \
			(cd "$$dir" && go mod download && go build -o "../../data/plugins/$$name/$$name" .); \
			if [ -f "$$dir/plugin.yml" ]; then \
				cp "$$dir/plugin.yml" "data/plugins/$$name/"; \
			fi; \
			if [ -f "$$dir/config.yml" ]; then \
				cp "$$dir/config.yml" "data/plugins/$$name/"; \
			fi; \
			echo "✓ $$name built -> data/plugins/$$name/$$name"; \
		fi; \
	done
	@echo "All plugins built!"

build-plugin: ## Build a single plugin (usage: make build-plugin NAME=tmdb)
	@if [ -z "$(NAME)" ]; then echo "Usage: make build-plugin NAME=plugin_name"; exit 1; fi
	@if [ ! -d "plugins/$(NAME)" ]; then echo "❌ Plugin not found: plugins/$(NAME)"; exit 1; fi
	@echo "Building plugin: $(NAME)"
	@mkdir -p "data/plugins/$(NAME)"
	cd "plugins/$(NAME)" && go mod download && go build -o "../../data/plugins/$(NAME)/$(NAME)" .
	@if [ -f "plugins/$(NAME)/plugin.yml" ]; then \
		cp "plugins/$(NAME)/plugin.yml" "data/plugins/$(NAME)/"; \
	fi
	@if [ -f "plugins/$(NAME)/config.yml" ]; then \
		cp "plugins/$(NAME)/config.yml" "data/plugins/$(NAME)/"; \
	fi
	@echo "✓ $(NAME) built -> data/plugins/$(NAME)/$(NAME)"

reload-plugin: ## Build and reload a plugin in dev server (usage: make reload-plugin NAME=semantic-search)
	@if [ -z "$(NAME)" ]; then echo "Usage: make reload-plugin NAME=plugin_name"; exit 1; fi
	@$(MAKE) build-plugin NAME=$(NAME)
	@echo "Reloading plugin in dev server..."
	@if curl -s -X POST "http://localhost:8080/api/plugins/$(NAME)/restart" | grep -q '"success":true'; then \
		echo "✓ $(NAME) reloaded successfully"; \
	else \
		echo "⚠️  Could not reload plugin (is dev server running?)"; \
		echo "   Plugin binary was built. Restart will occur on next request."; \
	fi

reload-plugins: ## Build and reload all plugins in dev server
	@echo "Building and reloading all plugins..."
	@$(MAKE) build-plugins
	@echo "Reloading plugins in dev server..."
	@for dir in plugins/*/; do \
		if [ -f "$$dir/main.go" ] || [ -f "$$dir/go.mod" ]; then \
			name=$$(basename "$$dir"); \
			if curl -s -X POST "http://localhost:8080/api/plugins/$$name/restart" | grep -q '"success":true'; then \
				echo "✓ $$name reloaded"; \
			else \
				echo "⚠️  $$name: could not reload (will restart on next request)"; \
			fi; \
		fi; \
	done
	@echo "All plugins processed!"

clean-plugins: ## Remove all built plugin binaries
	rm -rf data/plugins/
	@echo "✓ Plugin binaries cleaned"

new-plugin: ## Create a new plugin scaffold (usage: make new-plugin NAME=myplugin)
	@if [ -z "$(NAME)" ]; then echo "Usage: make new-plugin NAME=plugin_name"; exit 1; fi
	@if [ -d "plugins/$(NAME)" ]; then echo "❌ Plugin already exists: plugins/$(NAME)"; exit 1; fi
	@echo "Creating plugin scaffold: $(NAME)"
	@mkdir -p "plugins/$(NAME)/internal"
	@echo "# $(NAME) Plugin Manifest" > "plugins/$(NAME)/plugin.yml"
	@echo "" >> "plugins/$(NAME)/plugin.yml"
	@echo "id: $(NAME)" >> "plugins/$(NAME)/plugin.yml"
	@echo "name: $(NAME)" >> "plugins/$(NAME)/plugin.yml"
	@echo "version: 0.1.0" >> "plugins/$(NAME)/plugin.yml"
	@echo "description: TODO: Add description" >> "plugins/$(NAME)/plugin.yml"
	@echo "" >> "plugins/$(NAME)/plugin.yml"
	@echo "author: TODO" >> "plugins/$(NAME)/plugin.yml"
	@echo "license: MIT" >> "plugins/$(NAME)/plugin.yml"
	@echo "" >> "plugins/$(NAME)/plugin.yml"
	@echo "min_host_version: \"0.1.0\"" >> "plugins/$(NAME)/plugin.yml"
	@echo "" >> "plugins/$(NAME)/plugin.yml"
	@echo "categories:" >> "plugins/$(NAME)/plugin.yml"
	@echo "  - enricher" >> "plugins/$(NAME)/plugin.yml"
	@echo "" >> "plugins/$(NAME)/plugin.yml"
	@echo "capabilities:" >> "plugins/$(NAME)/plugin.yml"
	@echo "  media_types:" >> "plugins/$(NAME)/plugin.yml"
	@echo "    - movie" >> "plugins/$(NAME)/plugin.yml"
	@echo "  provides:" >> "plugins/$(NAME)/plugin.yml"
	@echo "    - metadata" >> "plugins/$(NAME)/plugin.yml"
	@echo "  is_local: false" >> "plugins/$(NAME)/plugin.yml"
	@echo "  rate_limit: 10" >> "plugins/$(NAME)/plugin.yml"
	@echo "" >> "plugins/$(NAME)/plugin.yml"
	@echo "permissions:" >> "plugins/$(NAME)/plugin.yml"
	@echo "  - network" >> "plugins/$(NAME)/plugin.yml"
	@echo "" >> "plugins/$(NAME)/plugin.yml"
	@echo "# $(NAME) Plugin Configuration" > "plugins/$(NAME)/config.yml"
	@echo "" >> "plugins/$(NAME)/config.yml"
	@echo "# Add your configuration options here" >> "plugins/$(NAME)/config.yml"
	@echo "" >> "plugins/$(NAME)/config.yml"
	@echo "✓ Plugin scaffold created: plugins/$(NAME)/"
	@echo "  Next steps:"
	@echo "    1. Edit plugins/$(NAME)/plugin.yml with your plugin details"
	@echo "    2. Create plugins/$(NAME)/go.mod"
	@echo "    3. Create plugins/$(NAME)/main.go"
	@echo "    4. Build with: make build-plugin NAME=$(NAME)"

