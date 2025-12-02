# ADR 026: App Package Restructuring, Authentication, and Settings Infrastructure

## Status

Proposed

## Date

December 2, 2025

## Context

The ViewRA codebase has grown organically and now has several pain points:

1. **`NewServer` has 30+ parameters** - Individual use cases passed instead of an aggregate
2. **Inconsistent handler creation** - Some handlers created in `NewServer`, others in `app/handlers`
3. **Wasted rebuilds** - `startup.go` rebuilds repos/services/usecases just to recover stuck scans
4. **No authentication** - Single-user only, no multi-user support
5. **No runtime settings** - All config requires restart

These are interconnected: clean app structure makes auth easier, auth enables per-user settings.

## Decision

Implement three initiatives in order:

### 1. App Package Restructuring

**Target structure:**

```text
internal/app/
├── config/           # Split config by domain
├── wire/             # Dependency wiring
│   ├── repositories.go
│   ├── services.go
│   ├── usecases.go   # Aggregate struct
│   └── handlers.go   # -> *api.Handlers
├── tasks/            # Scheduled task registration
└── container.go      # Main container
```

**Key changes:**

- Create `api.Handlers` aggregate struct
- Simplify `NewServer(config, logger, handlers)`
- Move all wiring to `app/wire/`
- Extract task registration to `app/tasks/`
- Fix startup to reuse container instead of rebuilding

### 2. User Authentication

**Approach:** JWT access tokens (15 min) + database-backed refresh tokens (7 days)

**New tables:**

- `users` - username, email, password_hash, is_admin
- `sessions` - refresh token tracking, enables revocation

**Components:**

- Argon2id password hashing
- JWT service for token generation/validation
- Auth middleware (`RequireAuth`, `RequireAdmin`)
- Login/logout/register/refresh endpoints

**Migration:** Add `user_id` to `watch_progress` for per-user tracking

### 3. Settings Infrastructure

**Approach:** Database-backed settings with in-memory cache, runtime reloadable

**Features:**

- System-wide settings (admin only)
- Per-user settings (after auth)
- Change subscribers for live updates
- Schema endpoint for future settings UI

**Categories:** Server, Transcoding, Scanning, Library, Playback, UI

## Consequences

### Positive

- Cleaner dependency management
- Multi-user support
- Per-user watch progress and preferences
- Runtime configuration without restarts
- Foundation for settings UI

### Negative

- Significant refactoring effort (7-10 days)
- Migration complexity for existing single-user data
- More moving parts in auth flow

### Neutral

- Breaking API change (auth headers required)
- First-run experience changes (admin setup)

## Alternatives Considered

### Session-only auth (no JWT)

Simpler but requires DB lookup on every request. JWT allows stateless validation for most requests.

### File-based settings

Simpler but no per-user settings, requires restart for changes.

### Gradual restructuring

Could do auth first without restructuring, but would make the code messier before cleaning it up.

## Implementation Order

1. **Phase 1: App Restructuring** (2-3 days) - Prerequisite for clean auth
2. **Phase 2: Authentication** (3-4 days) - Enables multi-user
3. **Phase 3: Settings** (2-3 days) - Runtime config

## References

- [OWASP Password Storage](https://cheatsheetseries.owasp.org/cheatsheets/Password_Storage_Cheat_Sheet.html)
- [JWT Best Practices RFC 8725](https://datatracker.ietf.org/doc/html/rfc8725)
- Current: `internal/app/`, `internal/api/`, `cmd/viewra/bootstrap/`
