.PHONY: help dev build clean test migrate-up migrate-down migrate-create sqlc-gen swagger-gen install-tools

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-20s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

install-tools: ## Install development tools (Air, sqlc, swag, migrate)
	@echo "Installing development tools..."
	go install github.com/cosmtrek/air@latest
	go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
	go install github.com/swaggo/swag/cmd/swag@latest
	go install -tags 'sqlite3' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
	@echo "Tools installed successfully!"

dev: ## Run development servers (backend + frontend)
	@echo "Starting development servers..."
	@echo "Backend will run on http://localhost:8080"
	@echo "Frontend will run on http://localhost:3000"
	@echo ""
	@which overmind > /dev/null 2>&1 && overmind start || (which foreman > /dev/null 2>&1 && foreman start || echo "Please install overmind or foreman to run dev servers")

dev-backend: ## Run backend with hot reload
	air

dev-frontend: ## Run frontend dev server
	cd web && npm run dev

build: ## Build production binaries
	@echo "Building backend..."
	go build -o bin/viewra ./cmd/viewra
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
	migrate -path migrations -database "sqlite3://data/viewra.db" up

migrate-down: ## Run database migrations down
	migrate -path migrations -database "sqlite3://data/viewra.db" down

migrate-create: ## Create a new migration (usage: make migrate-create NAME=add_users)
	@if [ -z "$(NAME)" ]; then echo "Usage: make migrate-create NAME=migration_name"; exit 1; fi
	migrate create -ext sql -dir migrations -seq $(NAME)

sqlc-gen: ## Generate sqlc code from queries
	sqlc generate

swagger-gen: ## Generate Swagger documentation
	swag init -g cmd/viewra/main.go -o docs/swagger

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
