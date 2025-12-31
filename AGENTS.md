# Agent Guidelines

## Commands
**ALWAYS use make commands when available.** Run `make help` to see all targets.

```bash
# Testing & Linting
make test                              # Run all tests
go test -v -run TestName ./path/to/pkg # Run single test
golangci-lint run                      # Go lint
cd web && npm run lint                 # Frontend lint

# Code Generation
make sqlc-gen                          # Generate DB code after SQL changes
make swagger-gen                       # Generate Swagger docs (after API changes)
make api-client-gen                    # Generate TypeScript API client (runs swagger-gen first)
make proto-gen                         # Generate Go code from protobuf definitions

# Plugin Development
make reload-plugin NAME=semantic-search # Build + reload plugin (picks up changes immediately)
make reload-plugins                     # Build + reload all plugins
make build-plugin NAME=semantic-search  # Build plugin only (no reload)
make build-plugins                     # Build all plugins
```

## Architecture
- **Clean Architecture**: domain/ (stdlib only) → application/ → infrastructure/ → api/
- **Dual DB**: All SQL must work on SQLite AND PostgreSQL
- **Use Cases vs Services**: Use `*Service` for CRUD operations combining related methods (e.g., `LibraryService`). Use `*UseCase` for single-purpose operations (e.g., `GetNextEpisodeUseCase`). Both patterns are valid - choose based on cohesion.

## Go Style
- Max line 120 chars, max function 150 lines, max complexity 25
- Errors: `var ErrNotFound = errors.New("not found")`, wrap with `fmt.Errorf("context: %w", err)`
- Imports: stdlib first, then external, then local (`github.com/viewra/viewra`)

## TypeScript Style
- Arrow functions only, no classes, exports at end of file
- Prettier: no semicolons, single quotes, trailing commas

## Critical Rules
- **NEVER** delete user data or run `rm -rf` on user directories
- **NEVER** run `make dev` or restart the server - user manages it
- **NO** stub code, TODOs, example files, or extra documentation
- **NO** adapter/wrapper patterns - implement interfaces directly. Adapters add tech debt and indirection.
- **NO** "simplest fixes" or "quick workarounds" - they aren't comprehensive and often turn out wrong. Always think through the full solution before implementing. If a proper fix requires more work, do the work.
- **STOP and THINK** before implementing - don't jump to the first solution that comes to mind. Consider the root cause and fix that, not the symptoms.
- Use `~/go/bin/air` for auto-reload instead of manual rebuilds

## MCP Tools
- Use `context7` to search documentation for Go, React, TypeScript, FFmpeg, HLS, etc.
- Use `gh_grep` to search GitHub for real-world code examples when unsure about implementation patterns
- Use `playwright` for browser automation and E2E testing
- Use `filesystem` for enhanced file operations (search, read, write across the project)
