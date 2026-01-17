# Operations Guide

Documentation for deploying and running ViewRA in production.

## Quick Start

```bash
# Build everything
make build

# Run with default settings (SQLite)
./bin/viewra

# Run with PostgreSQL
VIEWRA_DB_TYPE=postgres VIEWRA_DB_URL="postgres://user:pass@host/viewra" ./bin/viewra
```

## Documents

| Document | Purpose |
|----------|---------|
| [ENVIRONMENT_VARIABLES.md](ENVIRONMENT_VARIABLES.md) | Complete configuration reference |
| [DEPLOYMENT.md](DEPLOYMENT.md) | Docker, systemd, Kubernetes examples |
| [TROUBLESHOOTING.md](TROUBLESHOOTING.md) | Common issues and solutions |

## Requirements

### Runtime Dependencies

- **FFmpeg 6.0+**: Video transcoding (7.x/8.x recommended for best features)
- **subtitle-extractor**: Rust tool for subtitle extraction (built with `make build-tools`)

### Optional

- **PostgreSQL 14+**: Alternative to SQLite for larger deployments
- **NVIDIA GPU**: Hardware transcoding with NVENC
- **Intel CPU/GPU**: Hardware transcoding with QuickSync/VAAPI

## Ports

| Port | Service | Description |
|------|---------|-------------|
| 8080 | HTTP API | Main server (configurable) |

## Health Check

```bash
curl http://localhost:8080/health
```

Returns JSON with server status, database connectivity, and resource usage.

## Logging

ViewRA uses structured logging (JSON in production, text in development).

```bash
# Set log level
LOG_LEVEL=debug ./bin/viewra

# Log levels: debug, info, warn, error
```

## Related

- [../CLAUDE.md](../../CLAUDE.md) - Build requirements
- [../guides/HLS_TRANSCODING.md](../guides/HLS_TRANSCODING.md) - Hardware acceleration setup
