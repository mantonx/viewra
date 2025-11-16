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

## Last Updated
2025-11-16
