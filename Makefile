.PHONY: help dev build clean test migrate-up migrate-down migrate-create sqlc-gen swagger-gen api-client-gen install-tools

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

dev: ## Run development servers (backend + frontend) with hot reload
	@echo "🚀 Starting development servers with hot reload..."
	@echo ""
	@echo "📡 Backend:  http://localhost:8080"
	@echo "🎨 Frontend: http://localhost:5173"
	@echo ""
	@echo "💡 Tip: Edit any .go or .tsx file and see changes instantly!"
	@echo "🛑 To stop: Ctrl+C or 'overmind quit'"
	@echo ""
	@which overmind > /dev/null 2>&1 && overmind start || (echo "❌ Overmind not found. Install: brew install overmind (Mac) or go install github.com/DarthSim/overmind/v2@latest")

dev-backend: ## Run backend with hot reload (standalone)
	@echo "🔥 Starting backend with Air hot reload..."
	@echo "📡 Backend: http://localhost:8080"
	~/go/bin/air

dev-frontend: ## Run frontend dev server (standalone)
	@echo "⚛️  Starting Vite dev server..."
	@echo "🎨 Frontend: http://localhost:5173"
	cd web && npm run dev

restart: ## Restart development servers
	@echo "🔄 Restarting all services..."
	overmind restart

restart-backend: ## Restart only backend
	@echo "🔄 Restarting backend..."
	overmind restart backend

restart-frontend: ## Restart only frontend
	@echo "🔄 Restarting frontend..."
	overmind restart frontend

stop: ## Stop all development servers
	@echo "🛑 Stopping all services..."
	overmind quit

logs: ## Show logs from all services
	@echo "📋 Showing logs (Ctrl+C to exit)..."
	overmind echo

logs-backend: ## Show backend logs only
	@echo "📋 Showing backend logs..."
	overmind connect backend

logs-frontend: ## Show frontend logs only
	@echo "📋 Showing frontend logs..."
	overmind connect frontend

build: ## Build production binaries with version info
	@echo "Building backend..."
	@VERSION=$$(git describe --tags --always --dirty 2>/dev/null || echo "dev"); \
	COMMIT=$$(git rev-parse --short HEAD 2>/dev/null || echo "unknown"); \
	BUILD_DATE=$$(date -u '+%Y-%m-%d_%H:%M:%S'); \
	echo "Version: $$VERSION, Commit: $$COMMIT, Built: $$BUILD_DATE"; \
	go build -ldflags "-X github.com/mantonx/viewra/internal/version.Version=$$VERSION -X github.com/mantonx/viewra/internal/version.Commit=$$COMMIT -X github.com/mantonx/viewra/internal/version.BuildDate=$$BUILD_DATE" -o bin/viewra ./cmd/viewra
	@echo "Building frontend..."
	cd web && npm run build
	@echo "Build complete! Binary: bin/viewra"

clean: ## Clean build artifacts and temporary files
	rm -rf tmp/
	rm -rf bin/
	rm -rf web/dist/
	rm -rf data/*.db
	rm -rf data/*.db-shm
	rm -rf data/*.db-wal
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

lint: ## Run linters
	golangci-lint run
	cd web && npm run lint

fmt: ## Format code
	go fmt ./...
	cd web && npm run format || true

tidy: ## Tidy go modules
	go mod tidy

setup: install-tools ## Initial project setup
	@echo "Setting up project..."
	mkdir -p data
	mkdir -p tmp
	cd web && npm install
	@echo "Setup complete! Run 'make dev' to start development servers"

docker-build: ## Build Docker image
	docker build -t viewra:latest .

docker-run: ## Run Docker container
	docker run -p 8080:8080 -v $(PWD)/data:/app/data viewra:latest

audit: ## Run incomplete implementation audit
	@./scripts/audit-incomplete.sh

audit-fix: ## Update INCOMPLETE_IMPLEMENTATIONS.md after fixes
	@./scripts/audit-incomplete.sh || true
	@echo ""
	@echo "Update docs/INCOMPLETE_IMPLEMENTATIONS.md with current status"
