# ViewRA Documentation

**Navigation hub for all ViewRA documentation**

---

## Quick Start

**New to ViewRA?** Start here:
1. **[README.md](../README.md)** - Project overview
2. **[ARCHITECTURE.md](core/ARCHITECTURE.md)** - Understand the system design
3. **[QUICK_REFERENCE.md](development/QUICK_REFERENCE.md)** - Essential commands
4. **[PROJECT_STATUS.md](planning/PROJECT_STATUS.md)** - Current implementation status

---

## Core Documentation (Architecture & Design)

**Understand how ViewRA works**

- **[ARCHITECTURE.md](core/ARCHITECTURE.md)** - System architecture, DDD layers, clean architecture patterns
- **[DATABASE_SCHEMA.md](core/DATABASE_SCHEMA.md)** - Complete database schema for SQLite and PostgreSQL
- **[API_SPECIFICATION.md](core/API_SPECIFICATION.md)** - REST API endpoints, requests, responses
- **[TECH_STACK.md](core/TECH_STACK.md)** - Technology choices and rationale
- **[DATABASE_SETUP.md](core/DATABASE_SETUP.md)** - Database configuration guide
- **[PLUGIN_ARCHITECTURE.md](core/PLUGIN_ARCHITECTURE.md)** - Plugin system design (Phase 8)

---

## Development Guides

**How to work on ViewRA**

- **[QUICK_REFERENCE.md](development/QUICK_REFERENCE.md)** - Command cheat sheet for daily development
- **[CONVENTIONS.md](development/CONVENTIONS.md)** - Code style, naming conventions, best practices
- **[TESTING.md](development/TESTING.md)** - Testing strategy, patterns, coverage guidelines
- **[PORT_CONFIGURATION.md](development/PORT_CONFIGURATION.md)** - Service ports and networking

---

## Project Planning & Status

**Where we are and where we're going**

- **[PROJECT_STATUS.md](planning/PROJECT_STATUS.md)** - Current phase, recent work, implementation metrics
- **[PROJECT_PLAN.md](planning/PROJECT_PLAN.md)** - Complete 8-phase roadmap with detailed tasks
- **[ROADMAP.md](planning/ROADMAP.md)** - Historical implementation timeline and milestones

---

## Architecture Decision Records (ADRs)

**Why we made specific technical decisions**

See **[decisions/README.md](decisions/README.md)** for the complete index.

Key decisions:
- [005: On-Demand Transcoding Strategy](decisions/005-on-demand-transcoding-strategy.md)
- [006: Image Handling Strategy](decisions/006-image-handling-strategy.md)
- [007: Unified Task Scheduler](decisions/007-unified-task-scheduler.md)
- [012: Music Database Architecture](decisions/012-music-database-architecture.md)
- [014: Library Scanner Resilience Improvements](decisions/014-library-scanner-resilience-improvements.md)
- [020: Segment-Based Transcoding](decisions/020-segment-based-on-demand-transcoding.md) (REJECTED)
- [021: Progressive HLS Transcoding](decisions/021-progressive-hls-transcoding.md) (Proposed)

---

## Feature Documentation

**Implementation details for complex features**

- **[HARDWARE_ACCELERATION.md](features/HARDWARE_ACCELERATION.md)** - GPU transcoding with VAAPI/NVENC/QSV
- **[FFMPEG_7_8_FEATURES.md](features/FFMPEG_7_8_FEATURES.md)** - FFmpeg 7.x/8.x feature usage
- **[TONE_MAPPING.md](features/TONE_MAPPING.md)** - HDR to SDR tone mapping
- **[LIBPLACEBO_IMPLEMENTATION_SUMMARY.md](features/LIBPLACEBO_IMPLEMENTATION_SUMMARY.md)** - High-quality tone mapping
- **[SCALING.md](features/SCALING.md)** - Performance scaling strategies
- **[TRANSCODE_CLEANUP.md](features/TRANSCODE_CLEANUP.md)** - Transcode cleanup tools and automation

---

## Research & Analysis

**Data and investigations that informed design decisions**

- **[filename-parsing-patterns.md](research/filename-parsing-patterns.md)** - Media filename parsing patterns
- **[real-world-filename-analysis.md](research/real-world-filename-analysis.md)** - Analysis of real media files
- **[REAL_WORLD_VALIDATION.md](research/REAL_WORLD_VALIDATION.md)** - Scanner validation results
- **[scanner-v1-analysis.md](research/scanner-v1-analysis.md)** - Scanner v1 architecture analysis

---

## Code Reviews

**Comprehensive reviews of codebase quality and architecture**

- **[Backend Code Review (Nov 21, 2025)](reviews/backend-code-review-2025-11-21.md)** - Complete backend analysis, test infrastructure improvements, Phase 1/2/3 work summary

---

## For AI Agents

**Essential reading for Claude and other AI assistants**

### Core Understanding
1. **[ARCHITECTURE.md](core/ARCHITECTURE.md)** - Understand DDD layers, file organization, dependency flow
2. **[CONVENTIONS.md](development/CONVENTIONS.md)** - Code style rules, naming patterns, file structure
3. **[DATABASE_SCHEMA.md](core/DATABASE_SCHEMA.md)** - Table structures, relationships, query patterns
4. **[PROJECT_STATUS.md](planning/PROJECT_STATUS.md)** - What phase we're in, what's been done

### Code Organization Rules
- **Domain layer**: NO external dependencies, only business logic and interfaces
- **Application layer**: Use cases orchestrate domain + infrastructure
- **Infrastructure layer**: External concerns (DB, filesystem, FFmpeg, etc.)
- **API layer**: HTTP handlers, thin wrappers around use cases

### File Naming Patterns
- Use cases: `<verb>_<noun>.go` (e.g., `create_library.go`, `scan_library.go`)
- Repositories: `<entity>_repository.go` (interfaces in domain, implementations in infrastructure)
- Handlers: `<resource>.go` (e.g., `library.go`, `media.go`)

### Database Compatibility
- **CRITICAL**: All SQL must work on BOTH SQLite and PostgreSQL
- Use `$1, $2` placeholders (PostgreSQL style), sqlc generates correct bindings
- Test migrations on both databases
- Use QueryRouter in repositories for database-specific logic

### Common Patterns
See [CONVENTIONS.md](development/CONVENTIONS.md) for detailed examples of:
- Entity definitions with validation
- Repository interfaces and implementations
- Use case structure with error handling
- Handler patterns with Gin

---

## Documentation Guidelines

### When to Update Docs

**Always update these when making changes:**
- `PROJECT_STATUS.md` - After completing significant features
- `ADRs` - When making architectural decisions
- `CONVENTIONS.md` - When establishing new patterns
- `API_SPECIFICATION.md` - When adding/changing endpoints

**Never create:**
- Duplicate documentation
- One-off status reports (update PROJECT_STATUS instead)
- Implementation work logs (let git history speak)

### Creating New ADRs

See [decisions/README.md](decisions/README.md) for the ADR template and numbering convention.

---

**Last Updated**: 2025-11-21
