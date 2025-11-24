# LLM Documentation

This directory contains documentation specifically optimized for Large Language Models (LLMs) to understand and work with the ViewRA codebase effectively.

## Quick Start for LLMs

1. **First Time?** → Read [CONTEXT.md](CONTEXT.md) for 30-second overview
2. **Writing Code?** → Copy patterns from [PATTERNS.md](PATTERNS.md)
3. **Need Details?** → Check root [.agent.md](../../.agent.md) and [docs/](../) directory

## Files in This Directory

### [CONTEXT.md](CONTEXT.md)
**Quick codebase context for LLMs**

- 30-second project overview
- Architecture at a glance
- Key concepts and patterns
- File placement rules
- Common questions and answers
- Critical rules checklist

**Use when**: You need to quickly understand the project structure and core principles.

### [PATTERNS.md](PATTERNS.md)
**Copy-paste code patterns**

Complete, production-ready patterns for:
- Domain entities with validation
- Repository implementations
- SQL queries (SQLite + PostgreSQL)
- Use cases
- HTTP handlers
- Integration tests
- TypeScript/React components
- TanStack Query hooks
- Common utilities

**Use when**: You need to generate new code following project conventions.

## Configuration Files for LLM Tools

These files are located in the project root:

### [.cursorrules](../../.cursorrules)
Configuration for Cursor IDE with:
- Critical architecture rules
- File placement guidelines
- TypeScript/React style enforcement
- Anti-patterns to avoid
- Quick command reference

### [.clinerules](../../.clinerules)
Configuration for Cline AI assistant with:
- Detailed patterns and examples
- Definition of done checklist
- Development workflow
- Common mistakes

### [.aider.conf.yml](../../.aider.conf.yml)
Configuration for Aider with:
- Model configuration (Claude Sonnet 4.5)
- Read-only documentation files
- Git integration settings
- Lint/test commands

### [.github/copilot-instructions.md](../../.github/copilot-instructions.md)
Instructions for GitHub Copilot with:
- Project overview
- Core principles
- Preferred code patterns
- What NOT to suggest
- Success checklist

## Main Documentation

For comprehensive documentation, see the main [docs/](../) directory:

| File | Purpose |
|------|---------|
| [ARCHITECTURE.md](../ARCHITECTURE.md) | Complete DDD layer breakdown |
| [DATABASE_SCHEMA.md](../DATABASE_SCHEMA.md) | All tables, relations, queries |
| [PROJECT_PLAN.md](../PROJECT_PLAN.md) | 8-phase implementation roadmap |
| [DEVELOPMENT_WORKFLOW.md](../DEVELOPMENT_WORKFLOW.md) | Step-by-step workflows |
| [API_SPECIFICATION.md](../API_SPECIFICATION.md) | All REST endpoints |
| [QUICK_REFERENCE.md](../QUICK_REFERENCE.md) | 1-page cheat sheet |

## Root Configuration

The root [.agent.md](../../.agent.md) contains the most comprehensive AI assistant guidelines with:
- Critical rules (DRY, Layer boundaries, Dual DB support)
- Complete development workflow
- Vertical slice development methodology
- Anti-patterns and red flags
- TypeScript/React coding standards
- Definition of done checklist

## Decision Making Flow

When working on ViewRA as an LLM:

```
1. Quick orientation?
   → Read CONTEXT.md (this directory)

2. Writing new code?
   → Copy patterns from PATTERNS.md (this directory)

3. Need architectural details?
   → Check docs/ARCHITECTURE.md

4. What's already implemented?
   → Check docs/PROJECT_PLAN.md

5. Database schema questions?
   → Check docs/DATABASE_SCHEMA.md

6. Confused about conventions?
   → Read .agent.md (root)

7. Still unclear?
   → Read docs/DEVELOPMENT_WORKFLOW.md
```

## Critical Principles (Repeat from CONTEXT.md)

### Architecture Layers
```
Interfaces → Application → Domain ← Infrastructure
```
Domain is PURE (no external imports).

### File Placement
- Entity: `internal/domain/<entity>/entity.go`
- Repository interface: `internal/domain/<entity>/repository.go`
- Repository impl: `internal/infrastructure/persistence/<entity>/repository.go`
- Use case: `internal/application/<entity>/<verb>_<noun>.go`
- Handler: `internal/interfaces/http/<entity>/handler.go`

### TypeScript Style
```typescript
// ✅ Arrow functions, exports at end
const Component = () => { }
export { Component }

// ❌ Function keyword, inline exports
function Component() { } // WRONG
export const Component = () => { } // WRONG
```

### Dual Database Support
Every feature MUST work on SQLite AND PostgreSQL.

## Before Generating Code

Ask yourself:
1. ✅ Does this follow Clean Architecture principles?
2. ✅ Are all entity fields mapped (no placeholders)?
3. ✅ Does it work on both databases?
4. ✅ Am I using TypeScript arrow functions?
5. ✅ Are exports at the end of the file?
6. ✅ Is this a complete vertical slice?

## Getting Help

If uncertain about implementation:
1. Check existing similar code in the codebase
2. Reference [PATTERNS.md](PATTERNS.md) for templates
3. Consult [.agent.md](../../.agent.md) for detailed rules
4. Review [docs/DEVELOPMENT_WORKFLOW.md](../DEVELOPMENT_WORKFLOW.md)

## Contributing to LLM Docs

When adding new patterns or context:
- Keep CONTEXT.md concise (30-second read)
- Add complete examples to PATTERNS.md
- Update this README.md if adding new files
- Maintain consistency with .agent.md

## Version

**Last Updated**: 2025-11-24
**Compatible with**: ViewRA Phase 1+
**LLM Tools Supported**: Cursor, Cline, Aider, GitHub Copilot, Claude Code
