# ADR 028: User Authentication

## Status

Accepted (Implemented December 2, 2025)

Note: Event publishing (Phase 2D) deferred until plugin system (ADR 027) is implemented.

## Date

December 2, 2025

## Context

ViewRA currently operates as a single-user system. To support multiple users with isolated watch progress, preferences, and eventual plugin ecosystem integration, we need a robust authentication system.

Requirements:

1. Secure multi-user support with isolated data
2. Simple initial implementation (username/password)
3. Extension points for future plugin auth integration
4. Device authorization flow for TV/streaming clients (future)
5. API access patterns for both users and plugins

This ADR depends on [ADR 026 - App Package Restructuring](026-app-restructuring-and-auth.md).

## Decision

Implement JWT-based authentication with database-backed refresh tokens.

### Token Strategy

```text
┌─────────────────────────────────────────────────────────────┐
│                      Token Flow                             │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│   Login ───► Access Token (15m) + Refresh Token (7d)        │
│                      │                                      │
│                      ▼                                      │
│   API Request ───► Validate JWT ───► Allow/Deny             │
│                      │                                      │
│                      ▼ (expired)                            │
│   Refresh ───► Validate Refresh Token ───► New Token Pair   │
│                      │                                      │
│                      ▼ (revoked/expired)                    │
│   Re-authenticate                                           │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

- **Access tokens**: Short-lived JWTs (15 min), stateless validation
- **Refresh tokens**: Long-lived (7 days), hashed in database, enables revocation

### Password Hashing: Argon2id

Using OWASP-recommended parameters:

```go
type Argon2Params struct {
    Memory      uint32 // 64 * 1024 (64 MB)
    Iterations  uint32 // 3
    Parallelism uint8  // 4
    SaltLength  uint32 // 16
    KeyLength   uint32 // 32
}
```

Argon2id provides memory-hard hashing resistant to both GPU and side-channel attacks.

### Authorization Model

Phase 1 uses a simple `is_admin` boolean with room for future expansion:

```go
type User struct {
    ID           string
    Username     string
    DisplayName  string
    PasswordHash string
    IsAdmin      bool
    IsDisabled   bool
    CreatedAt    time.Time
    UpdatedAt    time.Time
}
```

### Database Schema

```sql
CREATE TABLE users (
    id TEXT PRIMARY KEY,
    username TEXT NOT NULL UNIQUE,
    display_name TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    is_admin INTEGER NOT NULL DEFAULT 0,
    is_disabled INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE sessions (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    refresh_token_hash TEXT NOT NULL UNIQUE,
    user_agent TEXT,
    ip_address TEXT,
    created_at TEXT NOT NULL,
    last_used_at TEXT NOT NULL,
    expires_at TEXT NOT NULL
);

CREATE INDEX idx_sessions_user_id ON sessions(user_id);
CREATE INDEX idx_sessions_expires_at ON sessions(expires_at);
```

### Session Storage

Refresh tokens are hashed (SHA-256) before storage. Sessions track:

- Token hash (for validation)
- User agent and IP (for session management UI)
- Expiration time
- Created/last used timestamps

This enables users to view active sessions and revoke specific devices.

### API Endpoints

**Authentication:**

```
POST   /api/auth/register      # Self-registration (if enabled)
POST   /api/auth/login         # Authenticate, returns token pair
POST   /api/auth/logout        # Invalidate current session
POST   /api/auth/logout-all    # Invalidate all user sessions
POST   /api/auth/refresh       # Refresh access token
GET    /api/auth/me            # Get current user
PUT    /api/auth/password      # Change password
```

**User Management (Admin):**

```
GET    /api/users              # List users
POST   /api/users              # Create user
GET    /api/users/:id          # Get user
PUT    /api/users/:id          # Update user
DELETE /api/users/:id          # Delete user
POST   /api/users/:id/reset-password  # Admin password reset
```

**Session Management:**

```
GET    /api/auth/sessions      # List current user's sessions
DELETE /api/auth/sessions/:id  # Revoke specific session
```

### Auth Middleware

```go
// RequireAuth validates JWT and populates user context
func RequireAuth(next http.Handler) http.Handler

// RequireAdmin validates JWT and checks is_admin
func RequireAdmin(next http.Handler) http.Handler

// OptionalAuth populates user context if valid token present
func OptionalAuth(next http.Handler) http.Handler
```

### Domain Model

```text
internal/domain/user/
├── user.go           # User aggregate root
├── session.go        # Session entity
├── credentials.go    # Value object for login credentials
├── repository.go     # Repository interfaces
└── errors.go         # Domain errors
```

### Service Interfaces

```go
type PasswordHasher interface {
    Hash(password string) (string, error)
    Verify(password, hash string) error
}

type TokenService interface {
    GenerateAccessToken(userID string, isAdmin bool) (string, error)
    GenerateRefreshToken() (string, error)
    ValidateAccessToken(token string) (*AccessClaims, error)
    HashRefreshToken(token string) string
}

type UserRepository interface {
    Create(ctx context.Context, user *User) error
    GetByID(ctx context.Context, id string) (*User, error)
    GetByUsername(ctx context.Context, username string) (*User, error)
    Update(ctx context.Context, user *User) error
    Delete(ctx context.Context, id string) error
    List(ctx context.Context, opts ListOptions) ([]*User, error)
    Count(ctx context.Context) (int64, error)
}

type SessionRepository interface {
    Create(ctx context.Context, session *Session) error
    GetByTokenHash(ctx context.Context, hash string) (*Session, error)
    GetByUserID(ctx context.Context, userID string) ([]*Session, error)
    UpdateLastUsed(ctx context.Context, id string) error
    Delete(ctx context.Context, id string) error
    DeleteByUserID(ctx context.Context, userID string) error
    DeleteExpired(ctx context.Context) (int64, error)
}
```

### Authentication Events

Auth use cases publish events via the Event Bus (see [ADR 027](027-plugin-system-architecture.md)) for plugin integration:

```go
// Published when a user successfully logs in
type UserLoggedIn struct {
    UserID    string
    SessionID string
    Timestamp time.Time
}

// Published when a user logs out (single session or all)
type UserLoggedOut struct {
    UserID    string
    SessionID string
    Timestamp time.Time
}

// Published when a new user is created
type UserCreated struct {
    UserID   string
    Username string
    IsAdmin  bool
}

// Published when a user is deleted
type UserDeleted struct {
    UserID string
}
```

These events enable plugins to:
- Sync user data to external systems
- Trigger welcome flows
- Log authentication events
- Clean up plugin-specific user data on deletion

### Migration for Existing Data

```sql
-- Add user_id to watch_progress
-- First, create a default admin user, then update existing rows

INSERT INTO users (id, username, display_name, password_hash, is_admin, created_at, updated_at)
VALUES ('usr_default_admin', 'admin', 'Admin', '<generated_hash>', 1, datetime('now'), datetime('now'));

ALTER TABLE watch_progress ADD COLUMN user_id TEXT REFERENCES users(id) ON DELETE CASCADE;
UPDATE watch_progress SET user_id = 'usr_default_admin';
```

### Validation Rules

- **Usernames**: 3-32 chars, alphanumeric + underscore, unique
- **Passwords**: minimum 8 chars (configurable)
- **Display names**: 1-64 chars, any unicode

### Configuration

```yaml
auth:
  jwt_secret: ""           # Generated on first run if empty
  access_token_ttl: 15m
  refresh_token_ttl: 168h  # 7 days

  password:
    min_length: 8

  registration:
    enabled: false         # Admin-only user creation by default

  session:
    max_per_user: 10       # Maximum concurrent sessions
    cleanup_interval: 1h   # How often to purge expired sessions
```

### Plugin Extension Points

Reserved for future plugin integration (see [ADR 027](027-plugin-system-architecture.md)):

1. **Auth Events**: Plugins subscribe to `UserLoggedIn`, `UserLoggedOut`, `UserCreated`, `UserDeleted` via Event Bus
2. **User Metadata**: Key-value storage for plugin-specific per-user data (implemented in ADR 027)
3. **Plugin API Keys**: Server-to-server authentication for plugins (implemented in ADR 027)
4. **Custom Auth Providers** (future): LDAP, OIDC via plugins

### Custom Auth Provider Interface (Future)

Plugins may register authentication providers for enterprise integration:

```go
// Future: Plugin-provided auth
type AuthProvider interface {
    // Unique identifier (e.g., "ldap", "oidc-google")
    ID() string

    // Display name for UI
    Name() string

    // Authenticate user, return user info or error
    Authenticate(ctx context.Context, credentials map[string]string) (*ExternalUser, error)

    // Optional: Handle callback (for OAuth flows)
    HandleCallback(ctx context.Context, params map[string]string) (*ExternalUser, error)
}

type ExternalUser struct {
    ExternalID  string
    Username    string
    DisplayName string
    Email       string
    Groups      []string
}
```

Users authenticated through plugins would be linked to local User records. **Not implementing in Phase 1**, but the architecture should not preclude this.

### Permission Extensions (Future)

When granular permissions are added, plugins can register custom permissions:

```go
type PermissionDefinition struct {
    ID          string   // "myplugin:export"
    Name        string   // "Export to My Service"
    Description string
    Category    string   // For grouping in admin UI
}

// Plugin registers permissions at startup
func (p *MyPlugin) Permissions() []PermissionDefinition {
    return []PermissionDefinition{
        {
            ID:          "myplugin:export",
            Name:        "Export to My Service",
            Description: "Allow exporting watch history to My Service",
            Category:    "Integrations",
        },
    }
}
```

Admins can grant these to users through the standard UI. **Not implementing in Phase 1.**

### Security Boundaries

**Never Exposed:**

- Raw password hashes
- Refresh tokens (only hashes stored)
- JWT signing keys
- Other users' sessions

## Testing Strategy

### Unit Tests

- Password hashing round-trip
- JWT generation and validation
- Token expiration logic
- Session expiration logic
- User validation rules

### Integration Tests

- Full auth flow: register → login → refresh → logout
- Session revocation
- Concurrent session limits
- Expired token handling

### Security Tests

- Invalid token rejection
- Expired token rejection
- Revoked session rejection
- Password hash not exposed in API responses
- Rate limiting (when implemented)

## Consequences

### Positive

- Multi-user support with isolated watch progress
- Secure password storage (Argon2id)
- Session management (view/revoke devices)
- Stateless API validation (JWT)
- Foundation for plugin auth integration

### Negative

- Added complexity in every request (auth middleware)
- Migration complexity for existing single-user data
- First-run experience changes (admin setup required)

### Neutral

- Breaking API change (auth headers required)
- New dependency: `golang.org/x/crypto`

## Alternatives Considered

### Session-Only Auth (No JWT)

Simpler but requires database lookup on every request. JWT allows stateless validation for most requests, with refresh tokens providing revocation capability.

### OAuth2/OIDC from the Start

More feature-complete but significantly more complex. Username/password is sufficient for v1; OIDC can be added via plugins later.

### No Multi-User Support

Could continue single-user, but limits use cases (families, shared servers) and plugin ecosystem.

## Implementation Phases

### Phase 2A: Core Authentication (2-3 days)

1. Add `golang.org/x/crypto` dependency
2. Create migration for users and sessions tables
3. Implement domain layer (User, Session entities)
4. Implement infrastructure (Argon2, JWT, repositories)
5. Implement use cases (login, register, logout, refresh)
6. Add API handlers and routes
7. Add auth middleware

### Phase 2B: User Context Threading (1-2 days)

1. Update watch_progress schema and repository
2. Thread user ID through progress handlers
3. Create default admin during first run
4. Add session cleanup background task

### Phase 2C: Session Management (0.5-1 day)

1. List sessions endpoint
2. Revoke session endpoint
3. Logout all sessions

### Phase 2D: Event Publishing (0.5 day)

1. Define auth event types
2. Publish events from auth use cases (login, logout, create, delete)
3. Document plugin auth patterns

**Total Effort**: 3.5-4.5 days

## Future Considerations

### Device Code Flow (TV Clients)

For streaming devices without keyboards:

```text
┌─────────────────┐                    ┌─────────────────┐
│  TV Client      │                    │   Server        │
├─────────────────┤                    ├─────────────────┤
│                 │ ──── Request ────► │                 │
│                 │ ◄─── Code + URL ── │                 │
│                 │                    │                 │
│ Display QR      │                    │                 │
│                 │ ──── Poll ───────► │                 │
│                 │ ◄─── Pending ───── │                 │
│                 │                    │                 │
│                 │       User scans QR, logs in         │
│                 │                    │                 │
│                 │ ──── Poll ───────► │                 │
│                 │ ◄─── Tokens ────── │                 │
└─────────────────┘                    └─────────────────┘
```

The device requests a code, displays it (or a QR), and polls until the user completes login on another device.

### Profiles (Non-Authenticated Users)

For family scenarios, profiles are lightweight identities under a user account:

```go
type Profile struct {
    ID       string
    UserID   string  // Parent account
    Name     string
    AvatarID *string
    // No credentials - selected from a list after parent logs in
}
```

Profiles have their own watch progress but share the parent's library access (like Netflix profiles).

### OIDC/LDAP Integration

Plugin-based auth providers could enable:
- "Log in with Authelia/Authentik"
- Corporate LDAP integration
- Google/GitHub OAuth (for cloud-hosted scenarios)

The core auth system works standalone, with plugins adding enterprise features.

## Open Questions

1. **Initial Admin Setup**: Generate random password and log it, or force setup flow on first visit?
2. **Username Changes**: Allow users to change username? Adds complexity.
3. **Account Recovery**: For v1, admin resets password manually. Email-based recovery later?
4. **Audit Logging**: Log auth events to database for security review? Or just structured logs?
5. **Device Names**: Let users name sessions ("Living Room TV") or just show user agent?

## References

- [OWASP Password Storage Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Password_Storage_Cheat_Sheet.html)
- [JWT Best Practices RFC 8725](https://datatracker.ietf.org/doc/html/rfc8725)
- Prerequisite: [ADR 026 - App Package Restructuring](026-app-restructuring-and-auth.md)
- Related: [ADR 027 - Plugin System Architecture](027-plugin-system-architecture.md)
- Related: [ADR 029 - Settings Infrastructure](029-settings-infrastructure.md)
