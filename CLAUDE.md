# ViewRA - Media Server

## Quick Context

ViewRA is a self-hosted media server for organizing and streaming personal media collections.

- **Architecture**: Clean Architecture / DDD
- **Database**: SQLite (default) or PostgreSQL
- **Streaming**: HLS with on-demand transcoding via FFmpeg

## Database Location

SQLite database: `./data/viewra.db`

```bash
# Run migrations
make migrate-up
# Or manually:
~/go/bin/migrate -database "sqlite3://./data/viewra.db" -path ./migrations up
```

## Project Structure

```text
internal/
├── domain/          # Business logic (NO external deps)
├── application/     # Use cases
├── infrastructure/  # DB, FFmpeg, filesystem
├── api/             # HTTP handlers
└── app/             # Dependency wiring
web/                 # React frontend
migrations/          # SQLite migrations
migrations/postgres/ # PostgreSQL migrations
data/                # Runtime data (db, cache, transcodes)
```

## Development

```bash
make dev          # Start backend (8080) + frontend (5173)
make build        # Production build
make test         # Run tests
```

## Code Generation

```bash
make openapi          # Generate OpenAPI spec
make api-client-gen   # Generate TypeScript API client
~/go/bin/sqlc generate # Generate Go code from SQL
```

## API Testing

Dev credentials: `dev` / `dev`

```bash
# Helper script with auto-auth (caches token for 10 min)
./scripts/api /api/media/147440              # GET request
./scripts/api /api/media/147440/tracks       # GET tracks
./scripts/api POST /api/libraries/1/scan     # POST request
./scripts/api /health                        # Health check

# Manual curl (if needed)
TOKEN=$(curl -s -X POST http://localhost:8080/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"dev","password":"dev"}' | jq -r '.AccessToken')
curl -s http://localhost:8080/api/media/147440 -H "Authorization: Bearer $TOKEN"
```

## Key Rules

1. **Dual DB**: All SQL must work on SQLite AND PostgreSQL
2. **Domain purity**: Domain layer imports only stdlib
3. **TypeScript**: Use arrow functions, exports at end of file
