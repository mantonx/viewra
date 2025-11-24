# Working with Claude Code on ViewRA

> Quick guide for using Claude Code effectively with this codebase

## First Time Setup

When starting a new Claude Code session:

```
Hi Claude! I'm working on ViewRA. Please read:
1. .agent.md - Core rules and conventions
2. docs/llm/CONTEXT.md - Quick project overview
3. docs/llm/PATTERNS.md - Code templates
```

## Key Project Context

**Architecture**: Domain-Driven Design (DDD) with Clean Architecture
- Domain layer: Pure Go, no external dependencies
- Infrastructure: Implements domain interfaces (DB, FFmpeg)
- Application: Use cases
- Interfaces: HTTP handlers

**Dual Database**: Every feature must work on SQLite AND PostgreSQL

**TypeScript Style**: Arrow functions only, exports at end of file

## Common Tasks

### Adding a New Feature
"Add a [feature] that [does X]. Follow vertical slice development - complete all layers (domain, infrastructure, application, API) before moving on."

### Fixing a Bug
"There's a bug in [component] where [behavior]. Please investigate and fix it, ensuring you test on both SQLite and PostgreSQL."

### Refactoring
"Refactor [code] to follow our Clean Architecture principles. Make sure domain layer has no external dependencies."

### Adding Tests
"Add integration tests for [feature] using the pattern in docs/llm/PATTERNS.md. Test with real database."

## What Claude Code Should Know

### File Locations (Critical)
- Entities: `internal/domain/<entity>/entity.go`
- Repository interfaces: `internal/domain/<entity>/repository.go`
- Repository implementations: `internal/infrastructure/persistence/<entity>/repository.go`
- Use cases: `internal/application/<entity>/<verb>_<noun>.go`
- HTTP handlers: `internal/interfaces/http/<entity>/handler.go`

### Anti-Patterns (Never Do)
- ❌ Don't create adapter/wrapper files
- ❌ Don't use empty `sql.NullString{}` placeholders
- ❌ Don't import external packages in domain layer
- ❌ Don't use `function` keyword in TypeScript
- ❌ Don't inline exports in TypeScript

### Definition of Done
Before marking a feature complete:
- [ ] Works on SQLite AND PostgreSQL
- [ ] All entity fields mapped (no placeholders)
- [ ] Integration test passing
- [ ] TypeScript uses arrow functions
- [ ] `make audit` returns 0 issues

## Quick Commands

```bash
make dev              # Start backend + frontend
make test             # Run all tests
make audit            # Find incomplete implementations
sqlc generate         # Generate SQL code
```

## Common Patterns

### Go Repository
```go
func (r *repo) Create(ctx context.Context, m *domain.Media) error {
    result, err := r.queries.CreateMedia(ctx, sqlc.CreateMediaParams{
        Title: m.Title,
        VideoCodec: common.NullString(m.VideoCodec), // Map actual field
    })
    // ...
}
```

### TypeScript Component
```typescript
const Component = ({ prop }: Props) => {
  const handler = () => { }
  return <div onClick={handler}>{prop}</div>
}

export { Component }
```

## Tips for Claude Code

1. **Read docs first**: Check PATTERNS.md before generating code
2. **Complete features**: Don't leave TODOs or placeholders
3. **Test both DBs**: Always test SQLite and PostgreSQL
4. **Use existing patterns**: Follow the codebase style
5. **Ask before big changes**: Clarify architecture decisions

## Getting Unstuck

If Claude Code seems confused:
- "Please read .agent.md for the full context"
- "Check docs/llm/PATTERNS.md for the correct pattern"
- "Follow the vertical slice approach - complete all layers"

## Remember

This codebase values:
- Clean architecture over shortcuts
- Complete implementations over TODOs
- Type safety over flexibility
- Both databases over convenience

---

**Last Updated**: 2025-11-24
