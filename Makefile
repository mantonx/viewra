.PHONY: help dev dev-debug dev-clean stop build clean test migrate-up migrate-down migrate-create sqlc-gen swagger-gen api-client-gen install-tools setup

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

build: ## Build production binaries with version info
	@echo "Building frontend..."
	cd web && npm run build
	@echo "Building backend with embedded frontend..."
	@VERSION=$$(git describe --tags --always --dirty 2>/dev/null || echo "dev"); \
	COMMIT=$$(git rev-parse --short HEAD 2>/dev/null || echo "unknown"); \
	BUILD_DATE=$$(date -u '+%Y-%m-%d_%H:%M:%S'); \
	echo "Version: $$VERSION, Commit: $$COMMIT, Built: $$BUILD_DATE"; \
	go build -ldflags "-X github.com/mantonx/viewra/internal/version.Version=$$VERSION -X github.com/mantonx/viewra/internal/version.Commit=$$COMMIT -X github.com/mantonx/viewra/internal/version.BuildDate=$$BUILD_DATE" -o bin/viewra ./cmd/viewra
	@echo "✓ Build complete! Binary: bin/viewra (includes embedded frontend)"

clean: ## Clean build artifacts and temporary files
	rm -rf tmp/
	rm -rf bin/
	rm -rf web/dist/
	@echo "Cleaned build artifacts"

test: ## Run tests
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

migrate-up: ## Run database migrations up
	~/go/bin/migrate -path migrations -database "sqlite3://data/viewra.db" up

migrate-down: ## Run database migrations down
	~/go/bin/migrate -path migrations -database "sqlite3://data/viewra.db" down

migrate-create: ## Create a new migration (usage: make migrate-create NAME=add_users)
	@if [ -z "$(NAME)" ]; then echo "Usage: make migrate-create NAME=migration_name"; exit 1; fi
	migrate create -ext sql -dir migrations -seq $(NAME)

sqlc-gen: ## Generate sqlc code from queries
	~/go/bin/sqlc generate

swagger-gen: ## Generate Swagger documentation
	~/go/bin/swag init -g cmd/viewra/main.go -o docs/swagger --parseDependency --parseInternal

api-client-gen: swagger-gen ## Generate TypeScript API client from Swagger
	cd web && npm run generate:api
	@echo "API client generated successfully in web/src/lib/api/generated/"

setup: install-tools ## Initial project setup
	@echo "Setting up project..."
	mkdir -p data
	mkdir -p tmp
	cd web && npm install
	@echo "Setup complete! Run 'make dev' to start development servers"

