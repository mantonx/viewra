# Agent Guidelines

## Commands
```bash
make test                              # Run all tests
go test -v -run TestName ./path/to/pkg # Run single test
golangci-lint run                      # Go lint
cd web && npm run lint                 # Frontend lint
make sqlc-gen                          # Generate DB code after SQL changes
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
- Use `~/go/bin/air` for auto-reload instead of manual rebuilds
