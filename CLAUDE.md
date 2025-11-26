# ViewRA - Media Server

## Project Overview
ViewRA is a self-hosted media server built with Go (backend) and React/TypeScript (frontend).

## Database Location
The SQLite database is located at `./data/viewra.db`, NOT in the project root.

When running migrations or database commands, use:
```bash
~/go/bin/migrate -database "sqlite3://./data/viewra.db" -path ./migrations up
```

## Project Structure
- `internal/` - Go backend code
  - `internal/api/` - HTTP handlers and routes
  - `internal/application/` - Application services
  - `internal/domain/` - Domain models and interfaces
  - `internal/infrastructure/` - Database, persistence, external services
- `web/` - React/TypeScript frontend
- `migrations/` - SQLite migrations
- `migrations/postgres/` - PostgreSQL migrations
- `data/` - Runtime data (database, thumbnails, transcoded files)

## Build Commands
- `make build` - Build frontend and backend
- `make dev` - Run development server
- `make migrate-up` - Run database migrations

## Code Generation
- `make openapi` - Generate OpenAPI spec from handlers
- `make api-client-gen` - Generate TypeScript API client from OpenAPI spec
- `~/go/bin/sqlc generate` - Generate Go code from SQL queries
