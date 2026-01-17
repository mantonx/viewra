# Environment Variables

Complete reference for ViewRA configuration via environment variables.

**Priority**: Environment variables override config file values.

---

## Core

| Variable | Default | Description |
|----------|---------|-------------|
| `ENVIRONMENT` | `development` | `development` or `production` |
| `DATA_DIR` | `./data` | Base directory for all data (db, cache, transcodes) |
| `PORT` | `8080` | HTTP server port |
| `SHUTDOWN_TIMEOUT` | `30s` | Graceful shutdown timeout |
| `LOG_LEVEL` | `info` | Log level: `debug`, `info`, `warn`, `error` |

## Database

### SQLite (Default)

| Variable | Default | Description |
|----------|---------|-------------|
| `DB_DRIVER` | `sqlite` | Database driver (`sqlite` or `postgres`) |
| `DB_PATH` | `{DATA_DIR}/viewra.db` | SQLite database file path |

### PostgreSQL

| Variable | Default | Description |
|----------|---------|-------------|
| `DB_DRIVER` | `sqlite` | Set to `postgres` for PostgreSQL |
| `DB_HOST` | `localhost` | PostgreSQL host |
| `DB_PORT` | `5432` | PostgreSQL port |
| `DB_USER` | `viewra` | PostgreSQL user |
| `DB_PASSWORD` | (none) | PostgreSQL password (required) |
| `DB_NAME` | `viewra` | PostgreSQL database name |
| `DB_SSL_MODE` | `disable` | SSL mode: `disable`, `require`, `verify-ca`, `verify-full` |

### Connection Pool

| Variable | Default | Description |
|----------|---------|-------------|
| `DB_MAX_OPEN_CONNS` | `25` | Maximum open connections |
| `DB_MAX_IDLE_CONNS` | `10` | Maximum idle connections |
| `DB_CONN_MAX_LIFETIME` | `1h` | Maximum connection lifetime |
| `DB_CONN_MAX_IDLE_TIME` | `10m` | Maximum idle time before closing |

### Migrations

| Variable | Default | Description |
|----------|---------|-------------|
| `AUTO_MIGRATE` | `true` | Run migrations on startup |
| `MIGRATIONS_PATH` | `./migrations` | Path to migration files |

## Authentication

| Variable | Default | Description |
|----------|---------|-------------|
| `JWT_SECRET` | (generated) | Secret for signing JWTs. **Required in production** |
| `ACCESS_TOKEN_TTL` | `15m` | Access token lifetime |
| `REFRESH_TOKEN_TTL` | `168h` (7d) | Refresh token lifetime |

## FFmpeg / Transcoding

| Variable | Default | Description |
|----------|---------|-------------|
| `VIEWRA_FFMPEG_PATH` | `ffmpeg` | Path to FFmpeg binary |
| `VIEWRA_FFPROBE_PATH` | `ffprobe` | Path to FFprobe binary |
| `VIEWRA_FFMPEG_LIB_PATH` | (none) | LD_LIBRARY_PATH for custom FFmpeg builds |
| `VIEWRA_HW_ACCEL` | `auto` | Hardware acceleration: `auto`, `nvenc`, `qsv`, `vaapi`, `videotoolbox`, `none` |
| `VIEWRA_TRANSCODE_DIR` | `{DATA_DIR}/transcodes` | Directory for transcode cache |

### Transcode Workers

| Variable | Default | Description |
|----------|---------|-------------|
| `TRANSCODE_WORKERS` | `4` | Number of concurrent transcode workers (1-20) |
| `TRANSCODE_POLL_INTERVAL` | `5s` | Interval to check for new transcode jobs |
| `TRANSCODE_IDLE_TIMEOUT` | `10m` | Kill idle transcode sessions after this duration |

### Transcode Cleanup

| Variable | Default | Description |
|----------|---------|-------------|
| `TRANSCODE_CLEANUP_ENABLED` | `true` | Enable automatic transcode cleanup |
| `TRANSCODE_DISK_THRESHOLD_PERCENT` | `90` | Start cleanup when disk usage exceeds this % |
| `TRANSCODE_DISK_WARNING_PERCENT` | `80` | Log warning when disk usage exceeds this % |
| `TRANSCODE_MIN_FREE_SPACE_GB` | `10` | Minimum free space to maintain |
| `TRANSCODE_MAX_AGE_DAYS` | `7` | Delete transcodes older than this |
| `TRANSCODE_MAX_IDLE_DAYS` | `3` | Delete transcodes not accessed in this time |
| `TRANSCODE_MAX_STORAGE_GB` | `100` | Maximum storage for transcodes (0 = unlimited) |

## Library Scanning

| Variable | Default | Description |
|----------|---------|-------------|
| `SCAN_TIMEOUT` | `24h` | Maximum time for a library scan |
| `SCAN_PARALLEL_WALKERS` | `0` | Concurrent directory walkers (0 = sequential) |
| `SCAN_PROGRESS_INTERVAL` | `1000` | Log progress every N files (0 = disabled) |
| `SCAN_JOB_RETENTION_MINUTES` | `60` | Keep completed scan jobs for this duration |

## Images

| Variable | Default | Description |
|----------|---------|-------------|
| `IMAGE_CACHE_DIR` | `{DATA_DIR}/cache/images` | Image cache directory |

## Plugins

| Variable | Default | Description |
|----------|---------|-------------|
| `PLUGINS_ENABLED` | `true` | Enable external plugin loading |
| `PLUGINS_DIR` | `{DATA_DIR}/plugins` | Directory containing plugin binaries |
| `PLUGINS_STORAGE_DIR` | `{DATA_DIR}/plugins/storage` | Plugin data storage directory |
| `VIEWRA_DEV_MODE` | `0` | Enable development mode features (pprof, etc.) |

## CORS

| Variable | Default | Description |
|----------|---------|-------------|
| `CORS_ALLOWED_ORIGINS` | `*` | Comma-separated allowed origins |
| `CORS_ALLOW_CREDENTIALS` | `false` | Allow credentials in CORS requests |

---

## Examples

### Development (SQLite)

```bash
export DATA_DIR=./data
export PORT=8080
export LOG_LEVEL=debug
./bin/viewra
```

### Production (PostgreSQL)

```bash
export ENVIRONMENT=production
export DATA_DIR=/var/lib/viewra
export PORT=8080
export JWT_SECRET="your-secure-secret-here"
export DB_DRIVER=postgres
export DB_HOST=db.example.com
export DB_USER=viewra
export DB_PASSWORD="secure-password"
export DB_NAME=viewra
export DB_SSL_MODE=require
./bin/viewra
```

### Hardware Transcoding (NVIDIA)

```bash
export VIEWRA_HW_ACCEL=nvenc
export TRANSCODE_WORKERS=8
./bin/viewra
```

---

## Config File

Environment variables can also be set via `{DATA_DIR}/config.yaml`:

```yaml
database:
  driver: postgres  # or sqlite
  postgres:
    host: localhost
    port: 5432
    user: viewra
    database: viewra
    ssl_mode: disable
  sqlite:
    path: viewra.db
server:
  shutdown_timeout: 30s
```

**Note**: Environment variables always take precedence over config file values. Passwords must be set via environment variables (never stored in config files).
