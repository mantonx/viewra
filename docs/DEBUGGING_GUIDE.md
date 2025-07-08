# Debugging Guide

This guide helps troubleshoot common issues in Viewra development and deployment.

## Table of Contents
- [Development Issues](#development-issues)
- [Plugin Issues](#plugin-issues)
- [API Debugging](#api-debugging)
- [Transcoding Issues](#transcoding-issues)
- [Performance Debugging](#performance-debugging)
- [Database Issues](#database-issues)
- [Docker Issues](#docker-issues)
- [Logging and Monitoring](#logging-and-monitoring)

## Development Issues

### Code Changes Not Reflecting

**Problem:** Changes to Go code aren't visible after restart.

**Solutions:**
```bash
# Force rebuild without cache
docker-compose build --no-cache backend

# Or use Makefile
make clean && make dev-setup

# For hot reload (if using Air)
docker-compose logs backend | grep -i air
```

### Import Errors

**Problem:** Go imports not resolving correctly.

**Solutions:**
```bash
# Update dependencies
docker-compose exec backend go mod tidy

# Clear module cache
docker-compose exec backend go clean -modcache

# Verify module path
grep "module" go.mod
```

## Plugin Issues

### Plugin Not Discovered

**Problem:** Plugins aren't being registered.

**Debug Steps:**
```bash
# 1. Check plugin binary exists
docker-compose exec backend ls -la /app/data/plugins/*/

# 2. Verify plugin.cue
docker-compose exec backend cat /app/data/plugins/ffmpeg_software/plugin.cue

# 3. Check discovery logs
docker-compose logs backend | grep -i "plugin\|discover"

# 4. Force refresh
curl -X POST http://localhost:8080/api/v1/plugins/refresh

# 5. List all plugins
curl http://localhost:8080/api/v1/plugins | jq
```

### Plugin Build Failures

**Problem:** Plugin won't compile.

**Solutions:**
```bash
# Build with verbose output
make plugin-build p=my_plugin VERBOSE=1

# Manual build in container
docker-compose exec backend sh -c \
  "cd /app/plugins/my_plugin && go build -v"

# Check dependencies
cd plugins/my_plugin && go mod tidy
```

### Plugin Crashes

**Problem:** Plugin starts but crashes immediately.

**Debug Steps:**
```bash
# Run plugin directly
docker-compose exec backend \
  /app/data/plugins/my_plugin/my_plugin

# Check plugin logs
docker-compose logs backend | grep my_plugin

# Enable debug logging
export PLUGIN_LOG_LEVEL=debug
```

## API Debugging

### Request Parsing Issues

**Problem:** API requests fail with parsing errors.

**Debug Techniques:**

1. **Log Raw Request Body:**
```go
bodyBytes, _ := io.ReadAll(c.Request.Body)
c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
logger.Info("Raw request", "body", string(bodyBytes))
```

2. **Test with curl:**
```bash
curl -X POST http://localhost:8080/api/v1/endpoint \
  -H "Content-Type: application/json" \
  -d '{"key": "value"}' \
  -v
```

3. **Check Content-Type:**
```bash
# Must be application/json for JSON bodies
curl -H "Content-Type: application/json"
```

### Response Formatting Issues

**Problem:** API responses are malformed.

**Solutions:**
```go
// Debug response before sending
logger.Info("API response", 
  "status", c.Writer.Status(),
  "data", responseData)

// Use middleware for response logging
router.Use(responseLoggerMiddleware())
```

### CORS Issues

**Problem:** Frontend can't access API.

**Solutions:**
```go
// Check CORS middleware
router.Use(cors.New(cors.Config{
    AllowOrigins:     []string{"http://localhost:3000"},
    AllowMethods:     []string{"GET", "POST", "PUT", "DELETE"},
    AllowHeaders:     []string{"Content-Type"},
    AllowCredentials: true,
}))
```

## Transcoding Issues

### FFmpeg Not Found

**Problem:** Transcoding fails with "ffmpeg not found".

**Debug Steps:**
```bash
# Check FFmpeg in container
docker-compose exec backend which ffmpeg
docker-compose exec backend ffmpeg -version

# Verify plugin can access FFmpeg
docker-compose exec backend sh -c \
  "cd /app/data/plugins/ffmpeg_software && ./test_ffmpeg.sh"
```

### Transcoding Hangs

**Problem:** Transcoding starts but never completes.

**Debug Techniques:**

1. **Check FFmpeg Process:**
```bash
# List FFmpeg processes
docker-compose exec backend ps aux | grep ffmpeg

# Check process output
docker-compose exec backend \
  cat /tmp/viewra-transcoding/session_*/ffmpeg.log
```

2. **Enable FFmpeg Logging:**
```go
cmd := exec.Command("ffmpeg",
    "-loglevel", "debug",
    "-progress", progressFile,
    // other args...
)
```

3. **Monitor Progress:**
```bash
# Watch progress file
docker-compose exec backend \
  tail -f /tmp/viewra-transcoding/session_*/progress.log
```

### Quality Issues

**Problem:** Output quality is poor.

**Debug Steps:**
```bash
# Check transcoding parameters
curl http://localhost:8080/api/v1/transcoding/sessions/{id} | jq

# Test with different quality settings
curl -X POST http://localhost:8080/api/v1/transcoding/transcode \
  -d '{"quality": 85, "video_codec": "h264"}'

# Analyze output
docker-compose exec backend \
  ffprobe /app/viewra-data/content/{hash}/output.mp4
```

## Performance Debugging

### High CPU Usage

**Problem:** Application using too much CPU.

**Debug Tools:**

1. **Go Profiling:**
```go
import _ "net/http/pprof"

// In main.go
go func() {
    log.Println(http.ListenAndServe("localhost:6060", nil))
}()
```

2. **Access Profiles:**
```bash
# CPU profile
go tool pprof http://localhost:6060/debug/pprof/profile

# Memory profile
go tool pprof http://localhost:6060/debug/pprof/heap
```

3. **Container Stats:**
```bash
docker stats viewra-backend
```

### Memory Leaks

**Problem:** Memory usage grows over time.

**Debug Steps:**
```bash
# Take heap snapshots
curl http://localhost:6060/debug/pprof/heap > heap1.pprof
# Wait some time...
curl http://localhost:6060/debug/pprof/heap > heap2.pprof

# Compare
go tool pprof -base heap1.pprof heap2.pprof
```

### Slow Queries

**Problem:** Database queries are slow.

**Solutions:**
```go
// Enable query logging
db.LogMode(true)

// Add query timing
db.Set("gorm:query_time", true)

// Check slow queries
SELECT * FROM pg_stat_statements 
ORDER BY mean_exec_time DESC LIMIT 10;
```

## Database Issues

### Connection Pool Exhaustion

**Problem:** "too many connections" errors.

**Debug Steps:**
```bash
# Check connection status
curl http://localhost:8080/api/db-status

# Monitor connections
docker-compose exec postgres \
  psql -U viewra -c "SELECT count(*) FROM pg_stat_activity;"
```

### Migration Failures

**Problem:** Database migrations fail.

**Solutions:**
```bash
# Check migration status
docker-compose exec backend \
  go run cmd/migrate/main.go status

# Run migrations manually
docker-compose exec backend \
  go run cmd/migrate/main.go up

# Rollback if needed
docker-compose exec backend \
  go run cmd/migrate/main.go down
```

## Docker Issues

### Container Won't Start

**Problem:** Backend container exits immediately.

**Debug Steps:**
```bash
# Check logs
docker-compose logs backend

# Run with shell
docker-compose run --entrypoint sh backend

# Check environment
docker-compose config
```

### Volume Permission Issues

**Problem:** Can't write to mounted volumes.

**Solutions:**
```bash
# Fix permissions
sudo chown -R $(id -u):$(id -g) ./viewra-data

# Check mount points
docker-compose exec backend ls -la /app/viewra-data
```

### Network Issues

**Problem:** Containers can't communicate.

**Debug Steps:**
```bash
# List networks
docker network ls

# Inspect network
docker network inspect viewra_default

# Test connectivity
docker-compose exec backend ping postgres
```

## Logging and Monitoring

### Enable Debug Logging

**Environment Variables:**
```bash
# Global log level
LOG_LEVEL=debug

# Module-specific
VIEWRA_LOG_PLAYBACK=trace
VIEWRA_LOG_TRANSCODING=debug
VIEWRA_LOG_PLUGIN=trace
```

### Structured Logging

**Add Context:**
```go
logger := log.With().
    Str("module", "playback").
    Str("session", sessionID).
    Logger()

logger.Debug().
    Str("provider", provider.Name).
    Int("quality", quality).
    Msg("Starting transcoding")
```

### Log Aggregation

**Search Logs:**
```bash
# Find errors
docker-compose logs | grep -i error

# Filter by module
docker-compose logs | grep -E "module=playback"

# JSON logs
docker-compose logs | jq 'select(.level=="error")'
```

### Metrics Collection

**Prometheus Endpoint:**
```go
// Add metrics
prometheus.MustRegister(
    transcodingSessions,
    transcodingDuration,
    transcodingErrors,
)

// Expose endpoint
http.Handle("/metrics", promhttp.Handler())
```

## Common Error Messages

### "no capable providers found"
- No plugins support the requested format
- Check plugin capabilities
- Verify plugins are enabled

### "session limit exceeded"
- Too many concurrent sessions
- Increase limit in configuration
- Check for stuck sessions

### "content hash not found"
- Transcoded content missing
- Check content store directory
- Verify cleanup settings

### "plugin handshake failed"
- Plugin binary incompatible
- Rebuild plugin
- Check protocol version

## Debug Utilities

### Health Checks
```bash
# System health
curl http://localhost:8080/api/health

# Module health
curl http://localhost:8080/api/v1/playback/health
curl http://localhost:8080/api/v1/transcoding/health

# Database health
curl http://localhost:8080/api/db-health
```

### Test Endpoints
```bash
# Test transcoding
curl -X POST http://localhost:8080/api/v1/debug/test-transcode

# Test plugin
curl http://localhost:8080/api/v1/plugins/{id}/test

# Generate test data
curl -X POST http://localhost:8080/api/v1/debug/generate-test-data
```

### Debug Mode
```go
// Enable debug endpoints
if config.DebugMode {
    router.GET("/debug/vars", expvar.Handler())
    router.GET("/debug/pprof/*", pprof.Handler())
    router.GET("/debug/requests", requestDebugger())
}
```

## Getting Help

### Log Collection
When reporting issues, include:
1. Error messages and stack traces
2. Relevant log sections
3. Configuration (sanitized)
4. Steps to reproduce

### Debug Information Script
```bash
#!/bin/bash
# collect-debug-info.sh

echo "=== System Info ==="
docker version
docker-compose version

echo "=== Container Status ==="
docker-compose ps

echo "=== Recent Logs ==="
docker-compose logs --tail=100

echo "=== Configuration ==="
docker-compose config

echo "=== Health Status ==="
curl -s http://localhost:8080/api/health | jq
```

### Community Support
- GitHub Issues: Report bugs
- Discussions: Ask questions
- Discord: Real-time help