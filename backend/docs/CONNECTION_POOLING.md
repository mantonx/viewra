# Database Connection Pooling

This document explains the database connection pooling implementation in the Viewra media server, designed to optimize performance for high-throughput scanning operations.

## Overview

The Viewra media server implements intelligent database connection pooling to:

- **Optimize scanner performance** - Handle thousands of concurrent file operations
- **Prevent connection exhaustion** - Manage database connections efficiently
- **Reduce latency** - Reuse existing connections instead of creating new ones
- **Handle burst workloads** - Scale connections based on actual usage

## Features

### 🎯 Intelligent Configuration

- **Automatic optimization** based on database type (SQLite vs PostgreSQL)
- **System-aware settings** that consider CPU cores and available memory
- **Container detection** with appropriate resource limits
- **Environment variable overrides** for custom configurations

### 📊 Real-time Monitoring

- **Connection pool statistics** via `/api/connection-pool` endpoint
- **Health monitoring** via `/api/db-health` endpoint
- **Performance indicators** to detect bottlenecks
- **Utilization metrics** with warnings for high usage

### 🔧 Performance Optimizations

- **GORM optimizations** including batch operations and prepared statements
- **SQLite enhancements** with WAL mode and optimized pragmas
- **PostgreSQL tuning** for high-concurrency workloads
- **Adaptive scaling** based on actual usage patterns

## Configuration

### Environment Variables

Configure connection pooling using these environment variables:

```bash
# Maximum number of open connections to the database
DB_MAX_OPEN_CONNS=25

# Maximum number of idle connections in the pool
DB_MAX_IDLE_CONNS=5

# Maximum lifetime of a connection
DB_CONN_MAX_LIFETIME=1h

# Maximum time a connection can be idle
DB_CONN_MAX_IDLE_TIME=15m
```

### Database-Specific Defaults

#### SQLite (Default)

```bash
DB_MAX_OPEN_CONNS=25      # Conservative for SQLite
DB_MAX_IDLE_CONNS=5       # Keep some connections warm
DB_CONN_MAX_LIFETIME=1h   # Shorter lifetime
DB_CONN_MAX_IDLE_TIME=15m # Aggressive idle cleanup
```

#### PostgreSQL

```bash
DB_MAX_OPEN_CONNS=100     # Higher concurrency
DB_MAX_IDLE_CONNS=20      # More idle connections
DB_CONN_MAX_LIFETIME=2h   # Longer lifetime
DB_CONN_MAX_IDLE_TIME=30m # Less aggressive cleanup
```

## Optimization Script

Use the included optimization script to automatically configure optimal settings:

```bash
# Analyze system and show recommendations
./backend/scripts/optimize-connection-pool.sh

# Apply recommendations to .env file automatically
./backend/scripts/optimize-connection-pool.sh --apply
```

The script analyzes:

- **CPU cores** and memory available
- **Database type** (SQLite vs PostgreSQL)
- **Container environment** detection
- **Current usage patterns** if backend is running

## Monitoring

### Connection Pool Statistics

Access detailed connection pool metrics:

```bash
curl http://localhost:8080/api/connection-pool
```

Response includes:

```json
{
  "connection_pool": {
    "open_connections": 8,
    "max_open_connections": 25,
    "in_use": 3,
    "idle": 5,
    "wait_count": 0,
    "wait_duration": "0s"
  },
  "utilization": {
    "open_connection_percent": 32.0,
    "idle_connection_percent": 62.5,
    "busy_connection_percent": 37.5
  },
  "performance_indicators": {
    "connection_waits": false,
    "high_utilization": false,
    "connection_churning": false,
    "idle_timeout_issues": false
  },
  "health_status": "healthy"
}
```

### Health Monitoring

Check overall database health:

```bash
curl http://localhost:8080/api/db-health
```

### Key Metrics to Monitor

| Metric                    | Description                            | Warning Threshold |
| ------------------------- | -------------------------------------- | ----------------- |
| `wait_count`              | Connections waiting for availability   | > 0               |
| `open_connection_percent` | Pool utilization                       | > 80%             |
| `connection_churning`     | Connections being recreated frequently | High              |
| `idle_timeout_issues`     | Connections timing out when idle       | High              |

## Performance Tuning

### For High-Throughput Scanning

When running large media scans (10,000+ files):

1. **Increase connection limits**:

   ```bash
   DB_MAX_OPEN_CONNS=50    # More concurrent operations
   DB_MAX_IDLE_CONNS=10    # Keep more connections ready
   ```

2. **Monitor for bottlenecks**:

   ```bash
   # Watch for connection waits during scan
   watch -n 2 'curl -s http://localhost:8080/api/connection-pool | jq .connection_pool.wait_count'
   ```

3. **Consider PostgreSQL** for very large libraries (100,000+ files)

### For Memory-Constrained Systems

On systems with limited RAM (< 4GB):

1. **Reduce connection limits**:

   ```bash
   DB_MAX_OPEN_CONNS=10
   DB_MAX_IDLE_CONNS=2
   ```

2. **Shorter connection lifetimes**:
   ```bash
   DB_CONN_MAX_LIFETIME=30m
   DB_CONN_MAX_IDLE_TIME=5m
   ```

### SQLite Optimizations

The implementation includes several SQLite-specific optimizations:

```go
// Applied automatically in connectSQLite()
dsn := dbPath + "?" +
    "cache=shared&" +          // Enable shared cache
    "mode=rwc&" +              // Read-write-create mode
    "_journal_mode=WAL&" +     // Write-Ahead Logging
    "_synchronous=NORMAL&" +   // Balance safety and performance
    "_busy_timeout=30000&" +   // 30 second busy timeout
    "_cache_size=-64000&" +    // 64MB cache size
    "_temp_store=MEMORY&" +    // Memory temp storage
    "_foreign_keys=ON"         // Enable FK constraints
```

## Troubleshooting

### Common Issues

#### Connection Waits

**Symptom**: `wait_count > 0` in connection pool stats
**Solution**:

- Increase `DB_MAX_OPEN_CONNS`
- Check for long-running queries
- Monitor system resources

#### High Memory Usage

**Symptom**: System memory pressure during scans
**Solution**:

- Reduce `DB_MAX_OPEN_CONNS` and `DB_MAX_IDLE_CONNS`
- Decrease connection lifetimes
- Monitor with `htop` or similar

#### SQLite Database Locked Errors

**Symptom**: "database is locked" errors
**Solution**:

- Increase `_busy_timeout` in SQLite DSN
- Reduce concurrent connections
- Consider PostgreSQL for high concurrency

#### Connection Pool Exhaustion

**Symptom**: "connection pool exhausted" errors
**Solution**:

- Check for connection leaks in application code
- Increase pool size temporarily
- Monitor connection usage patterns

### Debugging

Enable detailed database logging:

```bash
# Set GORM log level to Info or Debug
export DB_LOG_LEVEL=info
```

Monitor real-time connection usage:

```bash
# Watch connection pool stats during operations
watch -n 1 'curl -s http://localhost:8080/api/connection-pool | jq .utilization'
```

## Integration with Scanner

The media scanner is optimized to work with the connection pool:

1. **Batch operations** - Scanner uses batched inserts (500-1000 records)
2. **Connection efficiency** - Prepared statements for repeated queries
3. **Resource awareness** - Adaptive throttling considers connection pool pressure
4. **Graceful degradation** - Scanner reduces concurrency if connections are limited

## Best Practices

### Development

- Use the optimization script to set initial values
- Monitor connection pool during testing
- Test with realistic data volumes

### Production

- Set up monitoring alerts for connection waits
- Monitor pool utilization during peak loads
- Scale database resources before connection limits

### Container Deployments

- Set appropriate memory limits for containers
- Use the script's container detection for optimal settings
- Consider dedicated database containers for large deployments

## Performance Impact

Proper connection pooling provides significant performance improvements:

- **Scanner throughput**: 2-3x faster file processing
- **Reduced latency**: Eliminate connection setup overhead
- **Memory efficiency**: Controlled connection resource usage
- **Stability**: Prevent connection exhaustion under load

The optimizations are particularly effective for:

- Large library scans (10,000+ files)
- Concurrent user operations
- Plugin-heavy configurations
- Resource-constrained environments
