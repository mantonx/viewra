# Claude Code Preferences for Viewra2

This file contains project-specific preferences and guidelines for Claude Code when working on this project.

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
2025-11-13
