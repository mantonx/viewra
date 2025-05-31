# Configuration Management

Viewra uses a centralized configuration management system that supports multiple sources, validation, and hot-reloading.

## Configuration Sources

Configuration is loaded from multiple sources in the following priority order (higher priority overrides lower):

1. **Environment Variables** (highest priority)
2. **Configuration File** (YAML or JSON)
3. **Default Values** (lowest priority)

## Configuration File

### Location

The configuration file can be placed in several locations:

- `/app/viewra-data/viewra.yaml` (default for containers)
- `./viewra.yaml` (current working directory)
- Custom path via `--config` flag or `VIEWRA_CONFIG_PATH` environment variable

### Format

Both YAML and JSON formats are supported. YAML is recommended for readability.

### Example Configuration

See `backend/data/viewra.yaml.example` for a complete example with all available options.

## Configuration Sections

### Server Configuration

Controls HTTP server behavior:

```yaml
server:
  host: '0.0.0.0' # Server bind address
  port: 8080 # Server port
  read_timeout: '30s' # HTTP read timeout
  write_timeout: '30s' # HTTP write timeout
  max_header_bytes: 1048576 # Max HTTP header size
  enable_cors: true # Enable CORS support
  trusted_proxies: [] # Trusted proxy IPs
```

**Environment Variables:**

- `VIEWRA_HOST` - Server host
- `VIEWRA_PORT` - Server port
- `VIEWRA_READ_TIMEOUT` - Read timeout
- `VIEWRA_WRITE_TIMEOUT` - Write timeout
- `VIEWRA_ENABLE_CORS` - Enable CORS

### Database Configuration

Database connection and pool settings:

```yaml
database:
  type: 'sqlite' # Database type: sqlite, postgres
  data_dir: '/app/viewra-data' # Data directory
  database_path: '' # SQLite file path (auto-generated)
  url: '' # Database connection URL

  # PostgreSQL settings
  host: 'localhost'
  port: 5432
  username: 'viewra'
  password: ''
  database: 'viewra'

  # Connection pool
  max_open_conns: 100
  max_idle_conns: 20
  conn_max_lifetime: '2h'
  conn_max_idle_time: '30m'

  # Monitoring
  enable_metrics: true
  log_queries: false
```

**Environment Variables:**

- `DATABASE_TYPE` - Database type
- `VIEWRA_DATA_DIR` - Data directory
- `VIEWRA_DATABASE_PATH` - SQLite database path
- `DATABASE_URL` - Complete database URL
- `POSTGRES_HOST`, `POSTGRES_PORT`, `POSTGRES_USER`, `POSTGRES_PASSWORD`, `POSTGRES_DB` - PostgreSQL settings
- `DB_MAX_OPEN_CONNS`, `DB_MAX_IDLE_CONNS`, `DB_CONN_MAX_LIFETIME`, `DB_CONN_MAX_IDLE_TIME` - Connection pool settings

### Asset Management

Controls media asset storage and processing:

```yaml
assets:
  data_dir: '' # Asset storage directory
  max_file_size: 52428800 # Max asset size (50MB)
  default_quality: 95 # Default image quality
  enable_webp: true # Enable WebP format
  enable_thumbnails: true # Enable thumbnail generation
  thumbnail_sizes: [150, 300, 600] # Thumbnail sizes
  cache_duration: '24h' # Asset cache duration
  cleanup_interval: '6h' # Cleanup interval
```

**Environment Variables:**

- `VIEWRA_ASSETS_DIR` - Asset directory
- `VIEWRA_MAX_ASSET_SIZE` - Maximum asset size
- `VIEWRA_ASSET_QUALITY` - Default quality
- `VIEWRA_ENABLE_WEBP` - Enable WebP
- `VIEWRA_ENABLE_THUMBNAILS` - Enable thumbnails
- `VIEWRA_THUMBNAIL_SIZES` - Thumbnail sizes (comma-separated)

### Scanner Configuration

File scanning and metadata extraction settings:

```yaml
scanner:
  parallel_scanning: true # Enable parallel scanning
  worker_count: 0 # Worker count (0 = auto-detect)
  batch_size: 50 # Database batch size
  channel_buffer_size: 100 # Channel buffer size
  smart_hash_enabled: true # Enable smart hashing
  async_metadata: true # Async metadata extraction
  metadata_workers: 2 # Metadata worker count
  scan_interval: '1h' # Auto-scan interval
  auto_scan_enabled: false # Enable auto-scanning
  ignore_patterns: ['.*', 'Thumbs.db', '.DS_Store']
  max_file_size: 10737418240 # Max file size (10GB)
```

**Environment Variables:**

- `VIEWRA_PARALLEL_SCANNING` - Enable parallel scanning
- `VIEWRA_WORKER_COUNT` - Number of workers
- `VIEWRA_BATCH_SIZE` - Database batch size
- `VIEWRA_SMART_HASH` - Enable smart hashing
- `VIEWRA_ASYNC_METADATA` - Enable async metadata
- `VIEWRA_METADATA_WORKERS` - Metadata workers
- `VIEWRA_IGNORE_PATTERNS` - Ignore patterns (comma-separated)

### Plugin System

Plugin management and security settings:

```yaml
plugins:
  plugin_dir: './data/plugins' # Plugin directory
  enable_hot_reload: true # Enable hot-reloading
  default_enabled: false # Enable new plugins by default
  max_execution_time: '30s' # Max execution time
  enable_sandbox: true # Enable sandboxing
  memory_limit: 536870912 # Memory limit (512MB)
  allow_network_access: true # Allow network access
  allow_filesystem_write: false # Allow file writes
```

**Environment Variables:**

- `PLUGIN_DIR` - Plugin directory
- `VIEWRA_PLUGIN_HOT_RELOAD` - Enable hot-reload
- `VIEWRA_PLUGINS_DEFAULT_ENABLED` - Default enabled state
- `VIEWRA_PLUGIN_SANDBOX` - Enable sandboxing
- `VIEWRA_PLUGIN_MEMORY_LIMIT` - Memory limit

### Logging Configuration

Application logging settings:

```yaml
logging:
  level: 'info' # Log level: debug, info, warn, error
  format: 'json' # Format: json, text
  output: 'stdout' # Output: stdout, stderr, file
  file_path: '' # Log file path
  max_file_size: 100 # Max file size (MB)
  max_backups: 3 # Max backup files
  max_age: 30 # Max age (days)
  enable_colors: true # Enable colors
  enable_stack_trace: false # Enable stack traces
```

**Environment Variables:**

- `VIEWRA_LOG_LEVEL` - Log level
- `VIEWRA_LOG_FORMAT` - Log format
- `VIEWRA_LOG_OUTPUT` - Log output
- `VIEWRA_LOG_FILE` - Log file path

### Security Configuration

Authentication and security settings:

```yaml
security:
  enable_authentication: false # Enable authentication
  jwt_secret: '' # JWT signing secret
  jwt_expiration: '24h' # JWT expiration
  session_timeout: '30m' # Session timeout
  rate_limit_enabled: true # Enable rate limiting
  rate_limit_rpm: 1000 # Requests per minute
  allowed_origins: ['*'] # CORS origins
  secure_headers: true # Security headers
```

**Environment Variables:**

- `VIEWRA_ENABLE_AUTH` - Enable authentication
- `VIEWRA_JWT_SECRET` - JWT secret
- `VIEWRA_RATE_LIMIT` - Enable rate limiting
- `VIEWRA_RATE_LIMIT_RPM` - Rate limit

### Performance Configuration

Performance tuning and monitoring:

```yaml
performance:
  enable_pprof: false # Enable pprof endpoints
  enable_metrics: true # Enable metrics
  max_concurrent_scans: 2 # Max concurrent scans
  gc_percent: 100 # GC target percentage
  max_procs: 0 # Max OS threads
  memory_threshold: 85.0 # Memory threshold (%)
  cpu_threshold: 80.0 # CPU threshold (%)
  enable_adaptive_throttling: true # Adaptive throttling
```

**Environment Variables:**

- `VIEWRA_ENABLE_PPROF` - Enable pprof
- `VIEWRA_ENABLE_METRICS` - Enable metrics
- `VIEWRA_MAX_CONCURRENT_SCANS` - Max concurrent scans
- `GOGC` - Go GC percent
- `GOMAXPROCS` - Max OS threads

## API Endpoints

The configuration system provides REST API endpoints for runtime management:

### Get Configuration

```http
GET /api/config
```

Returns the complete configuration (with sensitive data redacted).

### Get Configuration Section

```http
GET /api/config/{section}
```

Returns a specific configuration section. Available sections:

- `server`
- `database`
- `assets`
- `scanner`
- `plugins`
- `logging`
- `security`
- `performance`

### Update Configuration Section

```http
PUT /api/config/{section}
```

Updates a specific configuration section. Send the new configuration as JSON in the request body.

**Note:** Some changes require a server restart to take effect.

### Reload Configuration

```http
POST /api/config/reload?path=/path/to/config.yaml
```

Reloads configuration from file.

### Save Configuration

```http
POST /api/config/save
```

Saves the current configuration to file.

### Validate Configuration

```http
POST /api/config/validate
```

Validates the current configuration and returns any issues.

### Get Configuration Defaults

```http
GET /api/config/defaults
```

Returns the default configuration values.

### Get Configuration Info

```http
GET /api/config/info
```

Returns information about the configuration system.

## Hot-Reload Support

The configuration system supports hot-reloading for most settings. When configuration changes are detected:

1. The new configuration is validated
2. Registered watchers are notified
3. Components can react to configuration changes
4. Some changes may require restart (marked in API responses)

## Validation

All configuration values are validated when loaded:

- **Type checking**: Ensures correct data types
- **Range checking**: Validates numeric ranges
- **Format validation**: Checks URLs, durations, etc.
- **Dependency validation**: Ensures required settings are present

## Best Practices

### Development

- Use the example configuration file as a starting point
- Set `VIEWRA_LOG_LEVEL=debug` for detailed logging
- Enable `VIEWRA_ENABLE_PPROF=true` for performance profiling

### Production

- Use environment variables for sensitive data (passwords, secrets)
- Set appropriate resource limits based on your hardware
- Enable metrics and monitoring
- Use PostgreSQL for better performance with large libraries
- Configure proper connection pools for your workload

### Security

- Generate a strong random `jwt_secret` if using authentication
- Set `allowed_origins` to specific domains instead of `["*"]`
- Use HTTPS in production with `secure_headers: true`
- Consider enabling `enable_sandbox: true` for plugins

### Performance

- Tune `worker_count` based on your CPU cores and I/O capacity
- Adjust `batch_size` based on your database performance
- Set appropriate `memory_threshold` and `cpu_threshold` for your system
- Use `smart_hash_enabled: true` to avoid re-processing unchanged files

## Migration from Old Configuration

The new system maintains backward compatibility with existing environment variables. Old configurations will continue to work, but you're encouraged to migrate to the new centralized system for better management and new features.

## Troubleshooting

### Configuration Not Loading

1. Check file permissions and path
2. Verify YAML/JSON syntax
3. Check logs for validation errors
4. Use `/api/config/validate` endpoint

### Environment Variables Not Working

1. Verify exact variable names (case-sensitive)
2. Check for typos in variable names
3. Ensure proper data types (use quotes for strings)
4. Restart the application after setting variables

### Performance Issues

1. Use `/api/config/info` to check current settings
2. Monitor `/api/metrics` for bottlenecks
3. Adjust worker counts and batch sizes
4. Enable adaptive throttling for automatic tuning
