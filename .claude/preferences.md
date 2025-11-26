# Claude Code Preferences for Viewra2

This file contains project-specific preferences and guidelines for Claude Code when working on this project.

## Development Workflow

### Use Air for Auto-Reload
- **ALWAYS use `~/go/bin/air` for development** instead of manual `go build` commands
- Air provides automatic rebuilding and hot-reloading when Go files change
- Start Air once at the beginning of a session and let it handle rebuilds
- Only use manual `go build` for final production builds or when Air is not appropriate

**Example:**
```bash
# Start Air (runs in background, auto-rebuilds on file changes)
~/go/bin/air &

# Make code changes - Air automatically rebuilds
# No need to run go build again!
```

### When to Use Manual Builds
- Creating production binaries: `make build`
- Testing builds without running: `go build -o bin/viewra ./cmd/viewra`
- CI/CD pipelines

## Code Generation Guidelines

### DO NOT Create:
- **Example usage files** (e.g., `example_usage.go`, `examples.go`)
- **Extra documentation files** unless explicitly requested
- **README files** unless explicitly requested
- **Tutorial or demo files** unless explicitly requested

### Focus On:
- **Production code only** - implement actual features
- **Essential tests** - unit tests for business logic
- **Inline documentation** - comments within code files
- **Updating existing documentation** when features are completed

## Rationale
Keep the codebase lean and focused. Documentation should be concise and integrated into the code itself or in planned documentation files (like PROJECT_PLAN.md, ARCHITECTURE.md, etc.). Avoid creating supplementary files that won't be maintained or used in production.

## Frontend Conventions

### No Classes

- **NEVER use classes in frontend code** - use functional patterns instead
- Use pure functions with state objects instead of class instances
- Use React hooks for stateful integration
- Exception: Extending `Error` for custom error types is acceptable

**Instead of:**

```typescript
class NetworkMonitor {
  private state: State
  constructor() { this.state = initialState }
  update() { /* mutates this.state */ }
}
```

**Use:**

```typescript
interface NetworkState { /* ... */ }
const createNetworkState = (): NetworkState => ({ /* ... */ })
const updateNetwork = (state: NetworkState): NetworkState => ({ /* ... */ })
```

### Why Functional?

- Aligns with React's functional paradigm
- Pure functions are easier to test
- Immutable state prevents subtle bugs
- Better tree-shaking and dead code elimination

### No Backwards Compatibility Shims

- We are in dev mode - don't create backwards compatibility wrappers
- Just refactor consumers to use the new API directly
- Tech debt from compatibility layers compounds quickly

### No Plan/Phase References in Code

- Don't mention "Phase 1", "Phase 2", ADR numbers, or plan details in code or comments
- Code should be self-documenting without referencing planning artifacts
- Comments should describe what the code does, not when it was planned

## Last Updated
2025-11-25
