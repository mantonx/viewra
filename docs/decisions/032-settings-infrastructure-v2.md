# ADR 032: Settings Infrastructure v2 - Environment Variable Awareness

## Status

Proposed

## Date

December 2, 2025

## Context

ADR 029 established a database-backed settings system, but the current implementation has significant gaps:

### Current Problems

1. **Disconnected from actual config**: Settings like `transcoding.hw_accel` show "none" even when hardware acceleration is auto-detected and active
2. **No env var awareness**: Users can't tell if a setting is overridden by environment variable
3. **Incomplete settings coverage**: Many useful config options aren't exposed
4. **No read-only/display mode**: Some values should be visible but not editable (detected hardware, system info)
5. **Settings don't affect runtime**: Database settings exist but aren't wired to actual config consumption

### Current Environment Variable Usage

From `config.go` and related files, ViewRA uses these environment variables:

**Database & Server:**
- `DB_DRIVER`, `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, `DB_NAME`, `DB_SSL_MODE`
- `PORT`, `CORS_ALLOWED_ORIGINS`, `CORS_ALLOW_CREDENTIALS`
- `ENVIRONMENT` (development/production)

**Auth:**
- `JWT_SECRET` (required in production)
- `ACCESS_TOKEN_TTL`, `REFRESH_TOKEN_TTL`, `MAX_SESSIONS_PER_USER`

**Media & Scanning:**
- `TRANSCODE_OUTPUT_DIR`, `TRANSCODE_WORKERS`, `TRANSCODE_POLL_INTERVAL`, `TRANSCODE_IDLE_TIMEOUT`
- `SCAN_TIMEOUT`, `SCAN_PARALLEL_WALKERS`, `SCAN_PROGRESS_INTERVAL`, `SCAN_JOB_RETENTION_MINUTES`
- `AUTO_SCAN_ENABLED`, `AUTO_SCAN_INTERVAL`
- `IMAGE_CACHE_DIR`

**Hardware Acceleration (from transcoding/config.go):**
- `HARDWARE_ACCEL` (none/nvenc/qsv/vaapi/videotoolbox)
- `HARDWARE_DEVICE` (e.g., /dev/dri/renderD128)
- `TONE_MAPPING_ENABLED`, `TONE_MAPPING_ALGORITHM`, `TONE_MAPPING_BACKEND`
- `LIBPLACEBO_PEAK_DETECT`, `LIBPLACEBO_CONTRAST_RECOVERY`

**Transcode Cleanup:**
- `TRANSCODE_CLEANUP_ENABLED`, `TRANSCODE_CLEANUP_DISK_THRESHOLD`, etc.

### Setting Categories by Mutability

After analysis, settings fall into three categories:

1. **Environment-Only** (security-sensitive, path-based, or startup-time only)
2. **UI-Configurable** (safe to change at runtime, stored in database)
3. **Read-Only Display** (detected/computed values, informational only)

## Decision

Enhance the settings infrastructure with:

1. **Environment variable override awareness**
2. **Clear categorization of what's editable vs read-only**
3. **Comprehensive coverage of useful settings**
4. **System information display section**

### Settings Taxonomy

```text
┌─────────────────────────────────────────────────────────────────────────┐
│                        Settings Classification                           │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  ENVIRONMENT-ONLY (not in UI, set via env vars/config)                  │
│  ├── Database connection (DB_*, security sensitive)                      │
│  ├── JWT secret (security sensitive)                                     │
│  ├── File paths (TRANSCODE_OUTPUT_DIR, IMAGE_CACHE_DIR)                 │
│  ├── Server port (requires restart, typically set once)                  │
│  ├── CORS settings (security sensitive)                                  │
│  └── Environment mode (development/production)                           │
│                                                                          │
│  UI-CONFIGURABLE (database-backed, runtime changeable)                   │
│  ├── Transcoding: default quality, tone mapping on/off                   │
│  ├── Scanning: auto-scan enabled, scan interval, ignore patterns        │
│  ├── Playback: autoplay, skip intro (future)                            │
│  └── Notifications: (future)                                             │
│                                                                          │
│  READ-ONLY DISPLAY (informational, detected values)                      │
│  ├── Hardware acceleration: detected type, GPU devices                   │
│  ├── System profile: CPU cores, memory, storage type                     │
│  ├── Server info: version, uptime, environment                          │
│  └── Effective settings: showing env var overrides                       │
│                                                                          │
│  ENV-VAR OVERRIDABLE (UI default, but env var wins)                      │
│  ├── Hardware acceleration mode (auto-detect, but HARDWARE_ACCEL wins)  │
│  ├── Transcode workers (computed default, but env var wins)             │
│  └── Scan parallel walkers (computed default, but env var wins)          │
│                                                                          │
└─────────────────────────────────────────────────────────────────────────┘
```

### Enhanced Definition Schema

```go
type Definition struct {
    Key         string      `json:"key"`
    Type        ValueType   `json:"type"`
    Category    Category    `json:"category"`
    Label       string      `json:"label"`
    Description string      `json:"description"`
    Default     any         `json:"default"`
    Options     []Option    `json:"options,omitempty"`
    Validation  *Validation `json:"validation,omitempty"`
    AdminOnly   bool        `json:"adminOnly"`
    Restartable bool        `json:"restartable"`

    // NEW: Environment variable awareness
    EnvVar      string      `json:"envVar,omitempty"`      // Associated env var name
    EnvLocked   bool        `json:"envLocked"`             // True if env var is set (computed at runtime)
    ReadOnly    bool        `json:"readOnly"`              // True for display-only values
    Source      SettingSource `json:"source"`              // Where the current value comes from
}

type SettingSource string

const (
    SourceDefault    SettingSource = "default"    // Using default value
    SourceDatabase   SettingSource = "database"   // Set via UI/API
    SourceEnvVar     SettingSource = "env_var"    // Overridden by env var
    SourceDetected   SettingSource = "detected"   // Auto-detected (read-only)
)
```

### System Settings (Admin)

#### Transcoding Category

| Key | Type | Default | Env Var | Editable | Description |
|-----|------|---------|---------|----------|-------------|
| `transcoding.hw_accel` | select | auto-detect | `HARDWARE_ACCEL` | When no env var | Hardware acceleration mode |
| `transcoding.hw_accel_detected` | string | - | - | Read-only | Detected GPU type and devices |
| `transcoding.default_quality` | select | `720p` | - | Yes | Default transcode quality |
| `transcoding.workers` | int | computed | `TRANSCODE_WORKERS` | When no env var | Concurrent transcode workers |
| `transcoding.tone_mapping_enabled` | bool | `true` | `TONE_MAPPING_ENABLED` | When no env var | HDR to SDR tone mapping |
| `transcoding.tone_mapping_algorithm` | select | `bt.2390` | `TONE_MAPPING_ALGORITHM` | When no env var | Tone mapping algorithm |
| `transcoding.min_free_disk_gb` | int | `10` | - | Yes | Min free disk for transcoding |

#### Scanning Category

| Key | Type | Default | Env Var | Editable | Description |
|-----|------|---------|---------|----------|-------------|
| `scanning.auto_enabled` | bool | `true` | `AUTO_SCAN_ENABLED` | When no env var | Auto-scan libraries |
| `scanning.auto_interval` | string | `*/15 * * * *` | `AUTO_SCAN_INTERVAL` | When no env var | Cron schedule for auto-scan |
| `scanning.parallel_walkers` | int | computed | `SCAN_PARALLEL_WALKERS` | When no env var | Concurrent directory walkers |
| `scanning.ignore_patterns` | json | `[".*"]` | - | Yes | File patterns to ignore |
| `scanning.job_retention_minutes` | int | `30` | `SCAN_JOB_RETENTION_MINUTES` | When no env var | How long to keep scan job records |

#### Server Category (mostly read-only)

| Key | Type | Default | Env Var | Editable | Description |
|-----|------|---------|---------|----------|-------------|
| `server.version` | string | - | - | Read-only | Server version |
| `server.environment` | string | - | `ENVIRONMENT` | Read-only | Running mode |
| `server.uptime` | string | - | - | Read-only | Server uptime |
| `server.base_url` | string | `""` | - | Yes | External URL for webhooks |

#### System Info Category (read-only display)

| Key | Type | Default | Env Var | Editable | Description |
|-----|------|---------|---------|----------|-------------|
| `system.cpu_model` | string | - | - | Read-only | CPU model name |
| `system.cpu_cores` | string | - | - | Read-only | Physical/logical cores |
| `system.memory_total` | string | - | - | Read-only | Total system RAM |
| `system.gpu_type` | string | - | - | Read-only | Detected GPU type |
| `system.gpu_devices` | json | - | - | Read-only | List of GPU devices |
| `system.vaapi_available` | bool | - | - | Read-only | VAAPI support detected |
| `system.opencl_available` | bool | - | - | Read-only | OpenCL support detected |

### User Settings

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `playback.default_quality` | select | `auto` | Preferred video quality |
| `playback.autoplay_next` | bool | `true` | Auto-play next episode |
| `playback.remember_position` | bool | `true` | Resume from last position |
| `ui.theme` | select | `system` | UI color theme |
| `ui.language` | select | `en` | UI language |
| `ui.sidebar_collapsed` | bool | `false` | Sidebar default state |

### API Changes

**Enhanced Settings Response:**

```json
{
  "settings": [
    {
      "key": "transcoding.hw_accel",
      "value": "nvenc",
      "source": "detected",
      "envVar": "HARDWARE_ACCEL",
      "envLocked": false,
      "readOnly": false
    },
    {
      "key": "system.gpu_devices",
      "value": ["NVIDIA GeForce RTX 3080"],
      "source": "detected",
      "readOnly": true
    }
  ]
}
```

**New System Info Endpoint:**

```
GET /api/system/info    # Returns detected system profile (public, authenticated)
```

```json
{
  "version": "0.0.1",
  "environment": "development",
  "uptime": "2h 34m",
  "profile": {
    "cpu": {
      "model": "AMD Ryzen 9 5900X",
      "physical_cores": 12,
      "logical_cores": 24
    },
    "memory": {
      "total_gb": 32,
      "available_gb": 24
    },
    "gpu": {
      "type": "nvidia",
      "available": true,
      "devices": ["NVIDIA GeForce RTX 3080"],
      "hw_accel": "nvenc",
      "vaapi": true,
      "opencl": true
    }
  }
}
```

### UI Implementation

#### Settings Page Sections

1. **System Information** (read-only card)
   - Server version, environment, uptime
   - CPU, memory, GPU detection results
   - Visual indicators for capabilities (green checkmarks for available features)

2. **Hardware Acceleration** (mixed editable/read-only)
   - Show detected GPU with visual indicator
   - Hardware accel mode dropdown (disabled with "Set by HARDWARE_ACCEL env var" if locked)
   - Tone mapping settings

3. **Library Scanning**
   - Auto-scan toggle and interval
   - Parallel walkers (with env var lock indicator)
   - Ignore patterns editor

4. **Transcoding Defaults**
   - Default quality
   - Worker count (with computed recommendation shown)

#### Visual Indicators

```
┌─────────────────────────────────────────────────────────────────────────┐
│  Hardware Acceleration                                                   │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  🟢 GPU Detected: NVIDIA GeForce RTX 3080                               │
│                                                                          │
│  Acceleration Mode                                                       │
│  ┌──────────────────────────────────────┐                               │
│  │ NVIDIA NVENC                     ▼  │  🔒 Set by HARDWARE_ACCEL     │
│  └──────────────────────────────────────┘                               │
│  Using hardware encoding via NVIDIA GPU                                  │
│                                                                          │
│  Tone Mapping                                                            │
│  [✓] Enable HDR to SDR conversion                                        │
│  Algorithm: BT.2390 (Broadcast Standard)                                │
│                                                                          │
└─────────────────────────────────────────────────────────────────────────┘
```

Lock icon (🔒) indicates value is set by environment variable and cannot be changed in UI.

### Settings Resolution Logic

```go
func (s *SettingsService) GetEffectiveValue(ctx context.Context, key string) (EffectiveValue, error) {
    def := GetDefinition(key)

    // 1. Check if env var is set
    if def.EnvVar != "" {
        if envVal := os.Getenv(def.EnvVar); envVal != "" {
            return EffectiveValue{
                Value:    parseEnvValue(envVal, def.Type),
                Source:   SourceEnvVar,
                Locked:   true,
                EnvVar:   def.EnvVar,
            }, nil
        }
    }

    // 2. Check database value
    if dbVal, err := s.repo.Get(ctx, key); err == nil && dbVal != nil {
        return EffectiveValue{
            Value:  dbVal,
            Source: SourceDatabase,
            Locked: false,
        }, nil
    }

    // 3. Check if it's a detected/computed value
    if def.ReadOnly {
        return EffectiveValue{
            Value:  s.getDetectedValue(key),
            Source: SourceDetected,
            Locked: true,
        }, nil
    }

    // 4. Return default
    return EffectiveValue{
        Value:  def.Default,
        Source: SourceDefault,
        Locked: false,
    }, nil
}
```

### Integration with Config

Settings should flow back into the application config. Two approaches:

**Option A: Settings Service as Config Source**
```go
// In container startup
if !isEnvVarSet("HARDWARE_ACCEL") {
    if val, err := settingsService.Get(ctx, "transcoding.hw_accel"); err == nil {
        cfg.Transcode.HardwareAccel = val.(string)
    }
}
```

**Option B: Config Watches Settings (reactive)**
```go
// Settings service notifies config when relevant settings change
settingsService.OnChange("transcoding.*", func(key string, value any) {
    // Update running config (may require component restart)
})
```

Recommend **Option A** for simplicity - settings are read at startup and on explicit "Apply" action.

## Consequences

### Positive

- Clear visibility into what's actually configured
- Users understand when env vars override UI settings
- System information helps troubleshooting
- Comprehensive settings coverage
- Foundation for future settings (notifications, plugins)

### Negative

- More complex settings resolution logic
- Need to maintain sync between env vars and settings definitions
- Some settings require restart even when changed in UI

### Neutral

- Breaking change to settings API response format
- Settings UI needs redesign

## Implementation Phases

### Phase 1: Enhanced Backend (1-2 days)

1. Add `EnvVar`, `EnvLocked`, `ReadOnly`, `Source` to Definition
2. Create `GetEffectiveValue` resolution logic
3. Add `/api/system/info` endpoint
4. Expand `SystemSettingDefinitions` with all useful settings
5. Add system profile exposure

### Phase 2: Config Integration (1 day)

1. Settings service reads system profile at startup
2. Populate detected values (GPU, CPU, memory)
3. Wire settings into actual config consumption
4. Add "restartable" warning for settings that need restart

### Phase 3: Frontend Redesign (1-2 days)

1. System Information card with capabilities
2. Env var lock indicators on form fields
3. Grouped settings by category
4. Read-only fields for detected values
5. "Requires restart" badges

### Phase 4: User Settings (0.5 day)

1. Playback preferences
2. UI preferences
3. Per-user settings page

**Total Effort**: 3.5-5.5 days

## References

- Supersedes: [ADR 029 - Settings Infrastructure](029-settings-infrastructure.md)
- Related: [ADR 026 - App Package Restructuring](026-app-restructuring-and-auth.md)
- Related: [ADR 028 - User Authentication](028-user-authentication.md)
