# Quick Reference: Command Cheat Sheet

**Essential commands for ViewRA development**

---

## Development Commands

### Running the Application

```bash
# Run both backend and frontend with live reload
make dev

# Backend only
make run
go run cmd/viewra/main.go

# Frontend only
cd web && npm run dev
```

### Code Generation

```bash
# Generate SQL code from queries
make sqlc
sqlc generate

# Generate Swagger API documentation
make swagger-gen
swag init -g cmd/viewra/main.go

# Generate frontend API client
cd web && npm run generate:api
```

### Database

```bash
# Run migrations (automatic on startup if VIEWRA_AUTO_MIGRATE=true)
migrate -path migrations -database "sqlite3://data/viewra.db" up

# Create new migration
make migrate-create NAME=feature_name
```

### Testing

```bash
# Run all tests
make test

# Run with coverage
make test-coverage
go test -v -coverprofile=coverage.out ./...

# View coverage report
go tool cover -html=coverage.out

# Coverage summary
go tool cover -func=coverage.out | grep total
```

### Transcode Cleanup

```bash
# Show disk usage
./bin/transcode-cleanup --stats

# Clean failed transcodes (dry run)
./bin/transcode-cleanup --failed --dry-run

# Clean failed transcodes
./bin/transcode-cleanup --failed

# Clean old transcodes (older than 30 days)
./bin/transcode-cleanup --older-than 720h

# Find orphaned files
./bin/transcode-cleanup --orphans --dry-run
```

### Linting & Formatting

```bash
# Backend linting
golangci-lint run
gofmt -s -w .

# Frontend linting
cd web && npm run lint
```

### Build

```bash
# Build backend
make build
go build -o bin/viewra ./cmd/viewra

# Build frontend
cd web && npm run build
```

---

## Search & Inspection Commands

```bash
# Find field usage in repositories
grep -r "FieldName" internal/infrastructure/persistence/

# Check for incomplete implementations
make audit

# Find TODOs in code
grep -r "TODO" internal/

# Count lines of code
cloc internal/ web/src/
```

---

## Git Commands

```bash
# Check status
git status

# Stage all changes
git add .

# Commit with message
git commit -m "feat: add feature description"

# Push to remote
git push origin branch-name

# Pull latest changes
git pull origin main
```

---

## Docker (if used)

```bash
# Build image
docker build -t viewra .

# Run container
docker run -p 8080:8080 viewra

# View logs
docker logs -f <container-id>
```

---

## API Testing

```bash
# Get libraries
curl http://localhost:8080/api/libraries

# Get disk usage
curl http://localhost:8080/api/transcode/disk-usage

# Trigger cleanup (dry run)
curl -X POST http://localhost:8080/api/transcode/cleanup \
  -H "Content-Type: application/json" \
  -d '{"failed": true, "dry_run": true}'
```

---

## Useful Aliases (add to ~/.bashrc or ~/.zshrc)

```bash
# ViewRA project shortcuts
alias vr-dev='cd /path/to/viewra2 && make dev'
alias vr-test='cd /path/to/viewra2 && make test'
alias vr-build='cd /path/to/viewra2 && make build'
alias vr-clean='cd /path/to/viewra2 && ./bin/transcode-cleanup --stats'
```

---

## Documentation

See these files for detailed guides:

- **[CONVENTIONS.md](./CONVENTIONS.md)** - Code style and best practices
- **[TESTING.md](./TESTING.md)** - Testing strategy and coverage
- **[PROJECT_STATUS.md](./PROJECT_STATUS.md)** - Current implementation status
- **[TRANSCODE_CLEANUP.md](./TRANSCODE_CLEANUP.md)** - Cleanup system guide

---

**Last Updated**: 2025-11-13
