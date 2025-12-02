# ADR 029: Settings Infrastructure

## Status

Proposed

## Date

December 2, 2025

## Context

ViewRA needs a flexible settings system to support:

1. System-wide configuration (admin-controlled)
2. Per-user preferences (user-controlled)
3. Runtime changes without restart
4. Future plugin settings integration

Currently, all configuration is file-based (`config.yaml`) and requires a server restart to apply changes. This limits flexibility and prevents users from customizing their experience.

This ADR depends on:
- [ADR 026 - App Package Restructuring](026-app-restructuring-and-auth.md)
- [ADR 028 - User Authentication](028-user-authentication.md) (for per-user settings)

## Decision

Implement a database-backed settings system with in-memory caching and runtime reloading.

### Settings Categories

```text
┌─────────────────────────────────────────────────────────────┐
│                    Settings Hierarchy                        │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│   System Settings (admin-only)                               │
│   ├── Server configuration                                   │
│   ├── Transcoding defaults                                   │
│   ├── Scanning behavior                                      │
│   └── Security policies                                      │
│                                                              │
│   User Settings (per-user)                                   │
│   ├── Playback preferences                                   │
│   ├── UI preferences                                         │
│   ├── Notification settings                                  │
│   └── Default quality                                        │
│                                                              │
│   Plugin Settings (future)                                   │
│   ├── Per-plugin configuration                               │
│   └── User overrides for plugin defaults                     │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

### Database Schema

```sql
CREATE TABLE system_settings (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,          -- JSON-encoded value
    value_type TEXT NOT NULL,     -- string, int, bool, json
    category TEXT NOT NULL,       -- server, transcoding, scanning, security
    description TEXT,
    updated_at TEXT NOT NULL,
    updated_by TEXT REFERENCES users(id)
);

CREATE TABLE user_settings (
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    key TEXT NOT NULL,
    value TEXT NOT NULL,          -- JSON-encoded value
    updated_at TEXT NOT NULL,
    PRIMARY KEY (user_id, key)
);

CREATE INDEX idx_system_settings_category ON system_settings(category);
```

### Settings Service Interface

```go
type SettingsService interface {
    // System settings (admin)
    GetSystem(ctx context.Context, key string) (any, error)
    SetSystem(ctx context.Context, key string, value any) error
    GetSystemByCategory(ctx context.Context, category string) (map[string]any, error)

    // User settings
    GetUser(ctx context.Context, userID, key string) (any, error)
    SetUser(ctx context.Context, userID, key string, value any) error
    GetUserAll(ctx context.Context, userID string) (map[string]any, error)

    // Effective settings (user override or system default)
    GetEffective(ctx context.Context, userID, key string) (any, error)

    // Cache management
    Reload(ctx context.Context) error
}
```

### Caching Strategy

```text
┌─────────────────────────────────────────────────────────────┐
│                    Cache Architecture                        │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│   Write Path:                                                │
│   API Request ───► Validate ───► Write DB ───► Invalidate   │
│                                                  Cache       │
│                                                              │
│   Read Path:                                                 │
│   API Request ───► Check Cache ───► Return (hit)            │
│                        │                                     │
│                        └───► Read DB ───► Populate Cache     │
│                                              (miss)          │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

- **Read-through cache**: Cache miss triggers database read
- **Write-through**: Database write invalidates cache entry
- **TTL**: Optional TTL for cache entries (default: no expiry)
- **Warm on startup**: Load all settings into cache at server start

### API Endpoints

```
GET    /api/settings/system              # Get all system settings (admin)
GET    /api/settings/system/:key         # Get specific system setting (admin)
PUT    /api/settings/system/:key         # Update system setting (admin)

GET    /api/settings/user                # Get current user's settings
GET    /api/settings/user/:key           # Get specific user setting
PUT    /api/settings/user/:key           # Update user setting
DELETE /api/settings/user/:key           # Reset to default

GET    /api/settings/schema              # Get settings schema (for UI)
```

### Settings Schema (for UI Generation)

```go
type SettingDefinition struct {
    Key         string       `json:"key"`
    Type        string       `json:"type"`        // string, int, bool, select, json
    Category    string       `json:"category"`
    Label       string       `json:"label"`
    Description string       `json:"description"`
    Default     any          `json:"default"`
    Options     []Option     `json:"options,omitempty"`  // for select type
    Validation  *Validation  `json:"validation,omitempty"`
    AdminOnly   bool         `json:"adminOnly"`
    Restartable bool         `json:"restartable"`  // requires restart
}

type Option struct {
    Value string `json:"value"`
    Label string `json:"label"`
}

type Validation struct {
    Min      *int   `json:"min,omitempty"`
    Max      *int   `json:"max,omitempty"`
    Pattern  string `json:"pattern,omitempty"`
    Required bool   `json:"required,omitempty"`
}
```

### Initial Settings

**System Settings:**

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `transcoding.default_quality` | select | `720p` | Default transcoding quality |
| `transcoding.hw_accel` | select | `none` | Hardware acceleration |
| `scanning.watch_interval` | int | `300` | Directory watch interval (seconds) |
| `scanning.ignore_patterns` | json | `[".*", "*.nfo"]` | Patterns to ignore |
| `server.base_url` | string | `""` | External base URL |

**User Settings:**

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `playback.default_quality` | select | `auto` | Preferred quality |
| `playback.autoplay` | bool | `true` | Auto-play next episode |
| `ui.theme` | select | `system` | UI theme |
| `ui.language` | select | `en` | UI language |

### Domain Model

```text
internal/domain/settings/
├── setting.go        # Setting value object
├── definition.go     # Setting definition/schema
├── repository.go     # Repository interfaces
└── errors.go         # Domain errors
```

### Plugin Extension Points

Reserved for future plugin integration:

1. **Plugin Settings Registration**: Plugins declare their settings schema
2. **Namespace Isolation**: Plugin settings prefixed with `plugin.<id>.`
3. **User Overrides**: Users can override plugin defaults
4. **Settings Events**: Plugins notified of relevant setting changes

## Consequences

### Positive

- Runtime configuration without restarts
- Per-user preferences
- Self-documenting settings via schema
- Foundation for plugin settings
- Type-safe settings access

### Negative

- Added complexity (cache management)
- Migration of existing config.yaml values
- Settings UI development required

### Neutral

- Breaking change for some config.yaml options
- New dependency on auth system for user settings

## Alternatives Considered

### File-Based Only

Keep all settings in `config.yaml`:
- Simpler implementation
- But: No per-user settings, requires restarts

### Redis/External Cache

Use Redis for settings cache:
- Distributed caching
- But: Added infrastructure dependency, overkill for single-instance

### No Caching

Read from database on every access:
- Simpler, always consistent
- But: Performance impact on hot paths

## Implementation Phases

### Phase 3A: Core Settings (1-2 days)

1. Create settings tables migration
2. Implement SettingsRepository
3. Implement SettingsService with caching
4. Add settings API endpoints

### Phase 3B: Settings Integration (0.5-1 day)

1. Migrate key config.yaml values to settings
2. Add schema endpoint
3. Wire settings into existing services

### Phase 3C: User Settings (0.5-1 day)

1. Add user settings endpoints
2. Implement effective settings (user + system)
3. Basic settings in frontend

**Total Effort**: 2-3 days

## References

- Prerequisite: [ADR 026 - App Package Restructuring](026-app-restructuring-and-auth.md)
- Prerequisite: [ADR 028 - User Authentication](028-user-authentication.md)
- Related: [ADR 027 - Plugin System Architecture](027-plugin-system-architecture.md)
