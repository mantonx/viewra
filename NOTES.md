# ViewRA Development Notes

> Personal development journal - track progress, decisions, issues, and useful commands

---

## 2025-11-11 - Project Kickoff

### ✅ Session 1: Planning & Documentation
- [x] Created comprehensive documentation
  - ARCHITECTURE.md - DDD design complete
  - DATABASE_SCHEMA.md - All tables defined (including missing People, Credits, Collections, Genres)
  - API_SPECIFICATION.md - All endpoints documented
  - PLUGIN_ARCHITECTURE.md - Plugin system designed
  - TECH_STACK.md - Technology decisions
  - PROJECT_PLAN.md - 8-phase roadmap
- [x] Created README.md (Claude-friendly)
- [x] Created NOTES.md (this file!)
- [x] Created .agent.md with AI assistant guidelines

### ✅ Session 2: Phase 0.1 - Repository & Structure Complete
- [x] Initialized Git repository with main branch
- [x] Created comprehensive .gitignore for Go/Node.js/SQLite
- [x] Set up project directory structure
  - cmd/viewra/ - Go application entry point
  - internal/ - Domain, Application, Infrastructure, Interfaces layers
  - migrations/ - Database migrations
  - web/ - React frontend
  - scripts/, data/, test-data/ directories
- [x] Initialized Go module (github.com/viewra/viewra)
- [x] Set up Vite + React + TypeScript frontend
- [x] Configured Air for Go hot reload (.air.toml)
- [x] Configured sqlc for type-safe SQL (sqlc.yaml)
- [x] Created Makefile with common development tasks
- [x] Created Procfile for concurrent backend/frontend dev
- [x] Created initial database migration (000001_init.up.sql)
  - Core tables: libraries, media, movies, tv_shows, tv_seasons, tv_episodes, music_tracks
  - Progress tables: watch_progress, transcode_jobs
- [x] Created basic Go HTTP server skeleton with Gin
- [x] Installed Gin web framework dependency
- [x] Updated .agent.md with documentation guidelines
  - ❌ Never create redundant/superfluous docs
  - ✅ Always update existing documentation
- [x] Made initial git commit

### 🎯 Next Session: Phase 0.2 - Development Tools
- [ ] Install golang-migrate/migrate
- [ ] Set up Swagger/swag configuration  
- [ ] Set up Orval configuration (frontend API client)
- [ ] Set up VS Code workspace settings
- [ ] Configure linters (golangci-lint, ESLint)

### 💡 Decisions Made
- **Database**: Start with SQLite (easier development), PostgreSQL support in Phase 8
- **Architecture**: DDD with clean layers (Domain → Application → Infrastructure → Interfaces)
- **Real-time Updates**: SSE instead of WebSockets (simpler, good enough)
- **Frontend Embedding**: Embedded in production binary (single binary deployment)
- **Frontend State Management**: Zustand + TanStack Query (not Redux)
- **Frontend Organization**: Feature-based (mirrors backend DDD)
- **Frontend Routing**: File-based routing with TanStack Router
- **Frontend UI**: Shadcn/ui + Tailwind CSS (custom styling, no bloat)
- **API Client**: Auto-generated with Orval from Swagger (type-safe)
- **Code Quality**: Biome for formatting + linting (fast, all-in-one)
- **Frontend Testing**: Vitest + React Testing Library + MSW

### 🤔 Questions / Open Items
- None yet

---

## Commands I'll Need

### Go / Backend

```bash
# Initialize project
go mod init github.com/yourusername/viewra2
go mod tidy

# Install Air (hot reload)
go install github.com/air-verse/air@latest

# Install sqlc (type-safe SQL)
go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest

# Install swagger
go install github.com/swaggo/swag/cmd/swag@latest

# Install migrate
go install -tags 'sqlite3' github.com/golang-migrate/migrate/v4/cmd/migrate@latest

# Run backend with hot reload
air

# Generate sqlc code
sqlc generate

# Generate Swagger docs
swag init -g cmd/server/main.go

# Run tests
go test ./...
go test -v ./internal/domain/...
```

### Database Migrations

```bash
# Create new migration
migrate create -ext sql -dir migrations -seq init
migrate create -ext sql -dir migrations -seq add_tv_shows
migrate create -ext sql -dir migrations -seq add_music

# Run migrations up
migrate -path migrations -database "sqlite3://data/viewra2.db" up

# Rollback one migration
migrate -path migrations -database "sqlite3://data/viewra2.db" down 1

# Force version (if migrations are stuck)
migrate -path migrations -database "sqlite3://data/viewra2.db" force <version>

# Check current version
migrate -path migrations -database "sqlite3://data/viewra2.db" version
```

### Frontend

```bash
# Initialize Vite + React + TypeScript
npm create vite@latest web -- --template react-ts
cd web
npm install

# Install dependencies
npm install @tanstack/react-router @tanstack/react-query
npm install tailwindcss postcss autoprefixer
npm install shaka-player
npm install @shadcn/ui

# Install dev dependencies
npm install -D orval

# Run frontend dev server
npm run dev

# Generate API client from Swagger
npm run generate:api  # (need to configure orval.config.js first)

# Build for production
npm run build
```

### Git

```bash
# Initial commit
git init
git add .
git commit -m "Initial commit - Planning phase complete"

# Create .gitignore
cat > .gitignore << 'EOF'
# Go
*.exe
*.exe~
*.dll
*.so
*.dylib
*.test
*.out
vendor/
.air/

# Data
data/
*.db
*.db-shm
*.db-wal

# Logs
*.log

# Frontend
web/node_modules/
web/dist/
web/.vite/

# IDE
.vscode/
.idea/
*.swp
*.swo
*~

# OS
.DS_Store
Thumbs.db

# Env
.env
.env.local
EOF
```

### Docker (Phase 8)

```bash
# Build image
docker build -t viewra2 .

# Run container
docker run -p 3000:3000 -v $(pwd)/data:/data viewra2

# Docker compose
docker-compose up -d
docker-compose logs -f
docker-compose down
```

---

## Directory Structure Reference

```
viewra2/
├── cmd/
│   └── server/
│       └── main.go              # Entry point
├── internal/
│   ├── domain/                  # Business logic (NO external deps)
│   │   ├── library/
│   │   │   ├── entity.go        # Library struct
│   │   │   ├── repository.go   # Interface only
│   │   │   ├── service.go       # Business rules
│   │   │   └── errors.go        # Domain errors
│   │   ├── media/
│   │   │   ├── entity.go
│   │   │   ├── repository.go
│   │   │   ├── service.go
│   │   │   └── scanner.go       # Scanning logic
│   │   └── watch/
│   │       ├── entity.go
│   │       ├── repository.go
│   │       └── service.go
│   ├── application/             # Use cases, DTOs
│   │   ├── library/
│   │   │   ├── create_library.go
│   │   │   ├── scan_library.go
│   │   │   └── dto.go
│   │   └── media/
│   │       ├── get_media.go
│   │       ├── stream_media.go
│   │       └── dto.go
│   ├── infrastructure/          # External dependencies
│   │   ├── database/
│   │   │   ├── connection.go
│   │   │   ├── migrate.go
│   │   │   ├── repository/      # Repository implementations
│   │   │   │   ├── library.go
│   │   │   │   └── media.go
│   │   │   └── sqlc/
│   │   │       ├── schema.sql
│   │   │       ├── queries/
│   │   │       │   ├── library.sql
│   │   │       │   └── media.sql
│   │   │       └── db/          # Generated code
│   │   ├── ffmpeg/
│   │   │   ├── client.go
│   │   │   ├── metadata.go
│   │   │   ├── thumbnail.go
│   │   │   └── transcoder.go
│   │   ├── filesystem/
│   │   │   ├── scanner.go
│   │   │   └── watcher.go
│   │   └── queue/
│   │       └── transcode_queue.go
│   └── interfaces/              # HTTP, CLI
│       └── http/
│           ├── server.go
│           ├── router.go
│           ├── middleware/
│           │   ├── cors.go
│           │   └── logging.go
│           └── handlers/
│               ├── library/
│               │   └── handler.go
│               └── media/
│                   └── handler.go
├── web/                         # React frontend
│   ├── src/
│   │   ├── components/
│   │   ├── routes/
│   │   ├── lib/
│   │   └── App.tsx
│   └── package.json
├── migrations/                  # Database migrations
│   ├── 000001_init.up.sql
│   ├── 000001_init.down.sql
│   ├── 000002_add_tv_shows.up.sql
│   └── 000002_add_tv_shows.down.sql
├── docs/                        # Documentation
├── configs/
│   ├── sqlc.yaml
│   └── air.toml
├── data/                        # Runtime (gitignored)
│   ├── viewra2.db
│   ├── thumbnails/
│   ├── dash/
│   └── cache/
├── go.mod
├── go.sum
├── Makefile
└── README.md
```

---

## Useful Code Snippets

### sqlc.yaml Configuration

```yaml
version: "2"
sql:
  - schema: "internal/infrastructure/database/sqlc/schema.sql"
    queries: "internal/infrastructure/database/sqlc/queries"
    engine: "sqlite"
    gen:
      go:
        package: "db"
        out: "internal/infrastructure/database/sqlc/db"
        sql_package: "database/sql"
        emit_json_tags: true
        emit_interface: true
        emit_exact_table_names: false
        emit_empty_slices: true
```

### air.toml Configuration

```toml
root = "."
testdata_dir = "testdata"
tmp_dir = "tmp"

[build]
  args_bin = []
  bin = "./tmp/main"
  cmd = "go build -o ./tmp/main ./cmd/server"
  delay = 1000
  exclude_dir = ["assets", "tmp", "vendor", "testdata", "web", "data"]
  exclude_file = []
  exclude_regex = ["_test.go"]
  exclude_unchanged = false
  follow_symlink = false
  full_bin = ""
  include_dir = []
  include_ext = ["go", "tpl", "tmpl", "html"]
  include_file = []
  kill_delay = "0s"
  log = "build-errors.log"
  poll = false
  poll_interval = 0
  rerun = false
  rerun_delay = 500
  send_interrupt = false
  stop_on_error = false

[color]
  app = ""
  build = "yellow"
  main = "magenta"
  runner = "green"
  watcher = "cyan"

[log]
  main_only = false
  time = false

[misc]
  clean_on_exit = false

[screen]
  clear_on_rebuild = false
  keep_scroll = true
```

### Makefile

```makefile
.PHONY: help dev build test clean migrate-up migrate-down sqlc swagger

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-15s\033[0m %s\n", $$1, $$2}'

dev: ## Run development server with hot reload
	air

build: ## Build production binary
	go build -o bin/viewra2 ./cmd/server

test: ## Run tests
	go test ./...

clean: ## Clean build artifacts
	rm -rf tmp/ bin/ data/*.db

migrate-up: ## Run database migrations up
	migrate -path migrations -database "sqlite3://data/viewra2.db" up

migrate-down: ## Rollback last migration
	migrate -path migrations -database "sqlite3://data/viewra2.db" down 1

sqlc: ## Generate sqlc code
	sqlc generate

swagger: ## Generate Swagger docs
	swag init -g cmd/server/main.go

frontend: ## Run frontend dev server
	cd web && npm run dev

install-tools: ## Install development tools
	go install github.com/air-verse/air@latest
	go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
	go install github.com/swaggo/swag/cmd/swag@latest
	go install -tags 'sqlite3' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
```

---

## Common Issues & Solutions

### Issue: sqlc generation fails
**Solution**: Make sure `schema.sql` is valid SQLite and all queries reference correct table names

### Issue: Air not reloading
**Solution**: Check `air.toml` exclude patterns - might be ignoring changed files

### Issue: Migration fails with "dirty database"
**Solution**: 
```bash
# Force to clean version
migrate -path migrations -database "sqlite3://data/viewra2.db" force <last_good_version>
# Then re-run migrations
migrate -path migrations -database "sqlite3://data/viewra2.db" up
```

### Issue: CORS errors in frontend
**Solution**: Add frontend URL to CORS middleware in backend:
```go
config := cors.DefaultConfig()
config.AllowOrigins = []string{"http://localhost:5173"}
```

### Issue: FFmpeg not found
**Solution**: 
```bash
# macOS
brew install ffmpeg

# Ubuntu/Debian
sudo apt-get install ffmpeg

# Verify installation
ffmpeg -version
```

---

## Environment Variables

Create `.env` file in root (gitignored):

```bash
# Server
PORT=3000
ENV=development

# Database
DATABASE_URL=sqlite://data/viewra2.db
DATA_DIR=./data

# Logging
LOG_LEVEL=debug
LOG_FORMAT=text

# Frontend (development)
FRONTEND_URL=http://localhost:5173
CORS_ORIGINS=http://localhost:5173,http://localhost:3000

# Metadata (Phase 4)
TMDB_API_KEY=
TVDB_API_KEY=
METADATA_ENABLED=false

# Transcoding
MAX_TRANSCODE_JOBS=2
TRANSCODE_QUALITY=720p
```

---

## Testing Checklist

### Phase 1 Tests
- [ ] Library CRUD operations work
- [ ] Scanner detects movies correctly
- [ ] Scanner detects TV shows (S01E01 format)
- [ ] FFmpeg extracts metadata (duration, codec, resolution)
- [ ] Thumbnails generate successfully
- [ ] Media list API returns results
- [ ] Direct streaming works (H.264 files)
- [ ] Frontend displays media grid
- [ ] Frontend plays video

### Phase 2 Tests
- [ ] Watch progress saves
- [ ] Progress resumes on reload
- [ ] Auto-mark watched at 90%
- [ ] Transcode request creates job
- [ ] 360p transcodes quickly
- [ ] DASH manifest generated
- [ ] Shaka Player switches quality
- [ ] Background jobs queue properly

---

## Performance Benchmarks

_To be filled in as development progresses_

### Library Scanning
- Target: 1000 files in < 30 seconds
- Actual: TBD

### Thumbnail Generation
- Target: < 1 second per thumbnail
- Actual: TBD

### Transcoding Speed
- 360p: Target < 30 seconds for 2-hour movie
- 720p: Target < 10 minutes for 2-hour movie
- Actual: TBD

### Database Queries
- Media list (1000 items): Target < 100ms
- Search: Target < 200ms
- Actual: TBD

---

## Ideas / Future Enhancements

- [ ] Browser extension for remote "Add to ViewRA"
- [ ] Telegram bot for notifications
- [ ] Automatic anime absolute numbering detection
- [ ] Hardware transcoding (NVENC, QuickSync, VAAPI)
- [ ] Download episodes for offline viewing
- [ ] Watch parties (synchronized playback)
- [ ] Intro/outro detection using audio fingerprinting
- [ ] Automatic collection creation from metadata
- [ ] Smart playlists (unwatched, recently added, etc.)
- [ ] Mobile app (React Native?)

---

## Resources & Links

### Documentation
- [Go Documentation](https://go.dev/doc/)
- [Gin Framework](https://gin-gonic.com/docs/)
- [sqlc Documentation](https://docs.sqlc.dev/)
- [golang-migrate](https://github.com/golang-migrate/migrate)
- [FFmpeg Documentation](https://ffmpeg.org/documentation.html)

### Frontend
- [React Docs](https://react.dev/)
- [TanStack Router](https://tanstack.com/router/latest)
- [TanStack Query](https://tanstack.com/query/latest)
- [Shadcn UI](https://ui.shadcn.com/)
- [Shaka Player](https://shaka-player-demo.appspot.com/)

### Reference Projects
- [Jellyfin](https://github.com/jellyfin/jellyfin) - .NET media server
- [Plex](https://www.plex.tv/) - Commercial media server
- [Navidrome](https://github.com/navidrome/navidrome) - Music server in Go

---

## Daily Log Template

```markdown
## YYYY-MM-DD - [Title]

### ✅ Today's Progress
- [ ] Task 1
- [ ] Task 2

### 🐛 Issues Encountered
- **Issue**: Description
  **Solution**: How I fixed it

### 💡 Decisions Made
- Decision and reasoning

### 🎯 Next Session
- [ ] Next task

### ⏱️ Time Spent
- X hours

### 📝 Notes
- Random thoughts, learnings
```

---

**Last Updated**: 2025-11-11  
**Current Phase**: Phase 0 - Project Setup  
**Next Milestone**: MVP (End of Phase 2)
