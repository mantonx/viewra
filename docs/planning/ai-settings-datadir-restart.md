# AI Settings, Central DataDir, and Server Restart

## Overview

This document captures the implementation plan for three major features:

1. **Central DataDir** - Single `DATA_DIR` configuration for all data paths
2. **AI Settings Backend** - Encryption, settings definitions, API, and plugin integration
3. **Server Restart + Admin Status** - Restart endpoint with SSE admin status stream

## Status

- **Created**: 2024-12-20
- **Status**: Planning Complete, Ready for Implementation
- **Estimated Effort**: ~4 hours

---

## Background

### Current State

- AI settings are hardcoded in plugin `config.yml` files
- Data paths are scattered across config with hardcoded `./data/` prefixes
- No server restart capability via UI
- No encrypted storage for API keys
- `ai_settings` table exists but is unused (migration 000054)

### Goals

1. Move AI configuration from per-plugin config to global ViewRA settings
2. Centralize data directory configuration for easier deployment
3. Enable admins to restart the server from the UI
4. Encrypt sensitive values (API keys) at rest
5. Provide real-time admin status via SSE

---

## Phase 1: Central DataDir Refactor

### Current Data Directory Usage

| Component | Current Env Var | Current Default |
|-----------|-----------------|-----------------|
| SQLite DB | `DB_PATH` | `data/viewra.db` |
| Transcode cache | `TRANSCODE_OUTPUT_DIR` | `./data/cache/transcodes` |
| Image cache | `IMAGE_CACHE_DIR` | `./data/cache/images` |
| Plugin binaries | `PLUGINS_DIR` | `./data/plugins` |
| Plugin storage | `PLUGINS_STORAGE_DIR` | `./data/plugins/storage` |

### New Design

Add `DataDir` to `Config` struct with `DATA_DIR` env var override (default: `./data`).

All paths derive from `DataDir`:

| Component | New Derived Path |
|-----------|------------------|
| SQLite DB | `{DataDir}/viewra.db` |
| Transcode cache | `{DataDir}/cache/transcodes` |
| Image cache | `{DataDir}/cache/images` |
| Plugin binaries | `{DataDir}/plugins` |
| Plugin storage | `{DataDir}/plugins/storage` |
| **Encryption key** | `{DataDir}/encryption.key` |

### Implementation

**File: `internal/app/config/config.go`**

```go
type Config struct {
    Environment   string
    DataDir       string  // Base directory for all data (default: "./data")
    Database      DatabaseConfig
    // ... rest unchanged
}

// DataPath returns the full path for a data subdirectory.
func (c *Config) DataPath(subpath string) string {
    return filepath.Join(c.DataDir, subpath)
}
```

---

## Phase 1.5: Server Restart + Admin Status Infrastructure

### Restart Mechanism

The server uses exit-with-code approach for restart:

1. Admin clicks "Restart Server" in UI (with confirmation dialog)
2. Server receives `POST /api/system/restart`
3. Server sends 200 response
4. Server sends `SIGTERM` to itself (triggers graceful shutdown)
5. Server exits with code `42`
6. Process manager (air/Docker/systemd) restarts the process

**Exit Code Convention:**
- `0` = Normal shutdown
- `1` = Error
- `42` = Restart requested

### Admin Status SSE Stream

Instead of polling, use SSE for real-time admin status:

**Endpoint**: `GET /api/admin/status/stream`

**Events**:
```
event: status
data: {"restartRequired": true, "restartRequiredSettings": ["transcoding.hw_accel"], "aiConfigured": false, "ollamaConnected": true}
```

The SSE stream:
1. Sends initial status immediately on connection
2. Subscribes to `settings.changed` events
3. Sends updated status whenever relevant settings change
4. Keeps connection alive with periodic heartbeats

### New Event Types

```go
// Settings events
EventSettingsChanged EventType = "settings.changed"

// System events
EventServerRestartRequested EventType = "server.restart_requested"
```

### Restart Flow Diagram

```
+------------------+      POST /api/system/restart      +------------------+
|    Admin UI      | ---------------------------------> |     Server       |
+------------------+                                    +------------------+
        |                                                       |
        |  200 OK {"message": "Restarting..."}                 |
        |<------------------------------------------------------|
        |                                                       |
        |  Show "Restarting..." dialog                         |
        |                                                       |
        |                                          Send SIGTERM to self
        |                                          (triggers graceful shutdown)
        |                                          Exit with code 42
        |                                                       |
        |                                     +------------------+
        |                                     |  Process Mgr     |
        |                                     |  (air/Docker/    |
        |                                     |   systemd)       |
        |                                     +------------------+
        |                                             |
        |                                      Restart process
        |                                             |
        |                                     +------------------+
        |  SSE: GET /api/admin/status/stream  |   New Server    |
        |-----------------------------------> |                  |
        |                                     +------------------+
        |  data: {"status": "online", ...}           |
        |<-------------------------------------------|
        |                                            
        |  Hide dialog, optionally show success toast
```

### Tracking Restartable Settings

Settings service tracks which `Restartable: true` settings have changed:

```go
type Service struct {
    // ... existing fields ...
    pendingRestartSettings map[string]bool
}

func (s *Service) GetPendingRestartSettings() []string
func (s *Service) ClearRestartPending()
```

---

## Phase 2: Settings Domain Updates

### New Category

**File: `internal/domain/settings/setting.go`**

```go
const (
    // ... existing categories ...
    CategoryAI Category = "ai"
)
```

### New Definition Field

**File: `internal/domain/settings/definition.go`**

```go
type Definition struct {
    // ... existing fields ...
    Sensitive bool `json:"sensitive"` // Values are encrypted at rest
}
```

### AI Setting Definitions

| Key | Type | Default | Sensitive | Description |
|-----|------|---------|-----------|-------------|
| `ai.embedding.provider` | string | `"ollama"` | No | Embedding provider |
| `ai.embedding.model` | string | `"nomic-embed-text"` | No | Embedding model |
| `ai.llm.provider` | string | `"ollama"` | No | Chat provider |
| `ai.llm.model` | string | `"llama3.1:8b"` | No | Chat model |
| `ai.ollama.base_url` | string | `"http://localhost:11434"` | No | Ollama API URL |
| `ai.openai.api_key` | string | `""` | **Yes** | OpenAI API key |
| `ai.anthropic.api_key` | string | `""` | **Yes** | Anthropic API key |
| `ai.openrouter.api_key` | string | `""` | **Yes** | OpenRouter API key |

---

## Phase 3: Encryption Infrastructure

### Design

**File: `internal/infrastructure/crypto/encryption.go`**

```go
type Encryptor interface {
    Encrypt(plaintext string) (string, error)
    Decrypt(ciphertext string) (string, error)
}

type AESEncryptor struct {
    key []byte // 32 bytes for AES-256
}

func NewAESEncryptor(dataDir string) (*AESEncryptor, error)
```

### Key Management

Key loading priority:
1. `VIEWRA_ENCRYPTION_KEY` env var (hex or base64 encoded, 32 bytes)
2. `{DataDir}/encryption.key` file (raw 32 bytes)
3. Auto-generate and save to `{DataDir}/encryption.key`

### Integration with Settings Service

- `SetSystem()`: If `Definition.Sensitive == true`, encrypt before storing
- `GetSystemValue()`: If `Definition.Sensitive == true`, decrypt before returning
- `GetSystemValueMasked()`: Returns `"********"` for sensitive values (for UI display)

---

## Phase 4: Ollama Enhancements

### New Methods

**File: `internal/infrastructure/ai/providers/ollama.go`**

```go
// PullProgress represents progress during model download.
type PullProgress struct {
    Status    string  `json:"status"`
    Digest    string  `json:"digest,omitempty"`
    Total     int64   `json:"total,omitempty"`
    Completed int64   `json:"completed,omitempty"`
    Percent   float64 `json:"percent,omitempty"`
    Done      bool    `json:"done"`
    Error     string  `json:"error,omitempty"`
}

// Pull downloads a model with progress streaming.
func (p *OllamaProvider) Pull(ctx context.Context, model string) (<-chan PullProgress, error)

// DeleteModel removes an installed model.
func (p *OllamaProvider) DeleteModel(ctx context.Context, model string) error
```

### Dynamic Model Recommendations

**File: `internal/infrastructure/ai/recommendations.go`**

Based on system profile (RAM, GPU), recommend appropriate models:

| RAM | GPU | Recommended LLM |
|-----|-----|-----------------|
| >= 32GB | Yes | `llama3.1:8b` |
| >= 16GB | Yes | `llama3.2:3b` |
| >= 16GB | No | `llama3.2:3b` (slower) |
| >= 8GB | Any | `llama3.2:1b` |
| < 8GB | Any | Cloud provider recommended |

---

## Phase 5: AI Settings API

### Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/settings/ai` | GET | Get AI config + installed models + recommendations |
| `/api/settings/ai` | PUT | Update AI settings (validates API keys before saving) |
| `/api/settings/ai/providers/:id/status` | GET | Check provider health |
| `/api/settings/ai/models` | GET | List installed Ollama models |
| `/api/settings/ai/models/pull` | POST | Start model pull, returns pull ID |
| `/api/settings/ai/models/pull/:id/progress` | GET | SSE stream of pull progress |
| `/api/settings/ai/models/pull/:id` | DELETE | Cancel active pull |
| `/api/settings/ai/models/:name` | DELETE | Delete installed model |

### GET /api/settings/ai Response

```json
{
  "settings": {
    "ai.embedding.provider": "ollama",
    "ai.embedding.model": "nomic-embed-text",
    "ai.llm.provider": "ollama",
    "ai.llm.model": "llama3.1:8b",
    "ai.ollama.base_url": "http://localhost:11434",
    "ai.openai.api_key": "********",
    "ai.anthropic.api_key": "",
    "ai.openrouter.api_key": ""
  },
  "ollama": {
    "status": "connected"
  },
  "installedModels": [
    {"id": "llama3.1:8b", "name": "llama3.1:8b", "isChat": true, "isEmbedding": false},
    {"id": "nomic-embed-text", "name": "nomic-embed-text", "isChat": false, "isEmbedding": true}
  ],
  "recommendations": {
    "embeddingModel": "nomic-embed-text",
    "llmModel": "llama3.1:8b",
    "reason": "32GB+ RAM with GPU detected - recommended for best quality",
    "canRunLocal": true
  }
}
```

### API Key Validation

When saving API keys via `PUT /api/settings/ai`, validate immediately:

| Provider | Validation Method |
|----------|-------------------|
| OpenAI | `GET /v1/models` with API key header |
| Anthropic | Minimal chat request |
| OpenRouter | `GET /api/v1/models` |
| Ollama | `GET /api/tags` health check |

---

## Phase 6: HostLLMServer Integration

### AIConfigReader Interface

**File: `internal/application/settings/ai_config.go`**

```go
type AIConfigReader struct {
    service *Service
}

func (r *AIConfigReader) GetEmbeddingConfig(ctx context.Context) (provider, model, baseURL, apiKey string, err error)
func (r *AIConfigReader) GetLLMConfig(ctx context.Context) (provider, model, baseURL, apiKey string, err error)
```

### HostLLMServer Changes

**File: `internal/infrastructure/plugins/host_llm.go`**

- Add `configReader AIConfigReader` field
- Add `cachedConfig` with mutex for thread-safe access
- Add `RefreshConfig()` method called on `settings.changed` events
- Remove hardcoded defaults, read from settings

### Event Bus Subscription

In `services.go`, subscribe HostLLMServer to settings changes:

```go
sub := eventBus.Subscribe(events.WithEventTypes(events.EventSettingsChanged))
go func() {
    for event := range sub.Events() {
        if cat, ok := event.Data["category"].(string); ok && cat == "ai" {
            hostLLMServer.RefreshConfig(context.Background())
        }
    }
}()
```

---

## Phase 7: Migration

### Migration 000056: Drop ai_settings Table

**Up:**
```sql
DROP TABLE IF EXISTS ai_settings;
```

**Down:**
```sql
CREATE TABLE IF NOT EXISTS ai_settings (
    id INTEGER PRIMARY KEY,
    key TEXT NOT NULL UNIQUE,
    value TEXT NOT NULL,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

---

## File Summary

### Backend Files

| File | Action | Description |
|------|--------|-------------|
| `internal/app/config/config.go` | Modify | Add `DataDir`, refactor all paths |
| `internal/domain/events/event.go` | Modify | Add `EventSettingsChanged`, `EventServerRestartRequested` |
| `internal/domain/settings/setting.go` | Modify | Add `CategoryAI` |
| `internal/domain/settings/definition.go` | Modify | Add `Sensitive` field, AI definitions |
| `internal/infrastructure/crypto/encryption.go` | **New** | AES-256-GCM encryption |
| `internal/application/settings/service.go` | Modify | Add encryption, event publishing, restart tracking |
| `internal/application/settings/ai_config.go` | **New** | `AIConfigReader` implementation |
| `internal/infrastructure/ai/providers/ollama.go` | Modify | Add `Pull()`, `DeleteModel()` |
| `internal/infrastructure/ai/recommendations.go` | **New** | Dynamic model recommendations |
| `internal/app/lifecycle/lifecycle.go` | **New** | Restart coordinator |
| `internal/api/handlers/admin.go` | **New** | Restart + admin status SSE |
| `internal/api/handlers/ai_settings.go` | **New** | AI settings endpoints |
| `internal/api/routes/admin.go` | **New** | Admin route registration |
| `internal/api/routes/ai_settings.go` | **New** | AI settings route registration |
| `internal/api/server.go` | Modify | Register new routes |
| `internal/infrastructure/plugins/host_llm.go` | Modify | Add config reader, cache |
| `internal/app/services/services.go` | Modify | Wire encryptor, config reader |
| `cmd/viewra/bootstrap/bootstrap.go` | Modify | Handle restart exit code |
| `migrations/000056_drop_ai_settings_table.up.sql` | **New** | Drop unused table |
| `migrations/000056_drop_ai_settings_table.down.sql` | **New** | Rollback |

### Frontend Files (Phase 2 - After Backend)

| File | Description |
|------|-------------|
| `web/src/components/admin/AdminBanner.tsx` | Dismissible/persistent banner component |
| `web/src/components/admin/AdminBannerContainer.tsx` | Container for stacking banners |
| `web/src/contexts/AdminStatusContext.tsx` | Admin status from SSE |
| `web/src/hooks/useAdminStatus.ts` | SSE connection management |
| `web/src/pages/settings/maintenance.tsx` | Maintenance page with Restart button |
| `web/src/pages/settings/ai.tsx` | AI Settings page |

---

## API Endpoints Summary

### Admin/System Endpoints

| Endpoint | Method | Auth | Description |
|----------|--------|------|-------------|
| `/api/system/restart` | POST | Admin | Trigger graceful restart |
| `/api/admin/status/stream` | GET | Admin | SSE stream of admin status |

### AI Settings Endpoints

| Endpoint | Method | Auth | Description |
|----------|--------|------|-------------|
| `/api/settings/ai` | GET | Admin | Get AI config + models + recommendations |
| `/api/settings/ai` | PUT | Admin | Update AI settings |
| `/api/settings/ai/providers/:id/status` | GET | Admin | Check provider health |
| `/api/settings/ai/models` | GET | Admin | List installed Ollama models |
| `/api/settings/ai/models/pull` | POST | Admin | Start model pull |
| `/api/settings/ai/models/pull/:id/progress` | GET | Admin | SSE pull progress |
| `/api/settings/ai/models/pull/:id` | DELETE | Admin | Cancel pull |
| `/api/settings/ai/models/:name` | DELETE | Admin | Delete model |

---

## Open Questions (Resolved)

1. **Encryption key location**: `{DataDir}/encryption.key`
2. **Pull progress tracking**: SSE (existing patterns)
3. **Config refresh**: Event bus subscription for cache invalidation
4. **API key validation**: Test connection immediately on save
5. **Model recommendations**: Dynamic based on system resources
6. **Restart mechanism**: Exit with code 42, process manager restarts
7. **Admin status delivery**: SSE stream
8. **Maintenance page location**: `/settings/maintenance`
9. **Restart confirmation**: Confirmation dialog required

---

## Estimated Effort

| Phase | Files | Complexity | Est. Time |
|-------|-------|------------|-----------|
| 1. DataDir Refactor | 1 | Medium | 20 min |
| 1.5. Restart + Admin Status | 6 | High | 45 min |
| 2. Settings Domain | 2 | Low | 15 min |
| 3. Encryption | 2 | Medium | 30 min |
| 4. Ollama Enhancements | 2 | Medium | 30 min |
| 5. AI Settings API | 3 | High | 45 min |
| 6. HostLLMServer Integration | 3 | Medium | 30 min |
| 7. Migration | 2 | Low | 5 min |
| 8. Testing | - | Medium | 30 min |
| **Total** | **~21 files** | | **~4 hours** |
