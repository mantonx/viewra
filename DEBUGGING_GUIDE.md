# Debugging Guide for Viewra

## Common Issues and Solutions

### 1. Docker Container Not Reflecting Code Changes

**Problem**: Changes to Go code aren't reflected when restarting containers.

**Solutions**:
```bash
# Force rebuild without cache
docker-compose build --no-cache backend

# Or use the Makefile target
make clean && make dev-setup

# For plugin changes specifically
make build-plugins DOCKER_BUILD_ARGS="--no-cache"
```

### 2. Plugin Discovery Issues

**Problem**: Plugins aren't being discovered or registered.

**Debug Steps**:
1. Check plugin is built: `ls plugins/*/plugin_binary`
2. Check plugin is enabled in CUE: `enabled: true`
3. Check logs for discovery: `docker-compose logs backend | grep -i plugin`
4. Force re-discovery: `curl -X POST http://localhost:8080/api/playback/plugins/refresh`

### 3. Request Parsing Issues

**Problem**: API requests aren't being parsed correctly.

**Debug Steps**:
1. Add raw body logging:
```go
bodyBytes, _ := c.GetRawData()
logger.Info("raw request", "body", string(bodyBytes))
```

2. Test with curl to see exact response:
```bash
curl -X POST http://localhost:8080/api/endpoint \
  -H "Content-Type: application/json" \
  -d '{"your": "data"}' | jq
```

### 4. FFmpeg Process Issues

**Problem**: FFmpeg process starts but is immediately killed.

**Common Causes**:
- Empty input path
- Invalid FFmpeg arguments
- Container resource limits
- File permissions

**Debug Steps**:
1. Check FFmpeg command: Look for `[ffmpeg command]` in logs
2. Verify input file exists: `docker exec viewra-backend-1 ls -la /path/to/file`
3. Test FFmpeg directly: `docker exec viewra-backend-1 ffmpeg -i /path/to/file`

## Development Best Practices

### 1. Use Development Mode Bindings
Add volume mounts for hot-reload in docker-compose.yml:
```yaml
volumes:
  - ./backend:/app
  - ./sdk:/sdk
```

### 2. Implement Comprehensive Logging
- Log at entry and exit of key functions
- Log all external calls (gRPC, FFmpeg, etc.)
- Include request IDs for tracing

### 3. Add Health Check Endpoints
```go
// Check plugin connectivity
GET /api/playback/plugins/health

// Check transcoding service
GET /api/playback/health/detailed
```

### 4. Use Structured Errors
```go
type PlaybackError struct {
    Code    string
    Message string
    Details map[string]interface{}
}
```

### 5. Implement Request Tracing
Add request IDs that flow through all layers:
```go
requestID := uuid.New().String()
ctx = context.WithValue(ctx, "request_id", requestID)
```

## Quick Debug Commands

```bash
# View all logs with timestamps
docker-compose logs -f --timestamps backend

# Check if backend is receiving requests
docker-compose logs backend | grep -E "(HandleStartTranscode|raw request)"

# Verify plugin is running
docker-compose exec backend ps aux | grep plugin

# Check file permissions
docker-compose exec backend ls -la /app/viewra-data/

# Test plugin directly
docker-compose exec backend /app/plugins/ffmpeg_software/ffmpeg_software

# Force clean rebuild
make clean && docker-compose down -v && make dev-setup
```

## Architecture Improvements Needed

1. **Centralized Configuration Validation**
   - Validate all configs at startup
   - Clear error messages for misconfigurations

2. **Better Plugin Interface**
   - Simplified plugin SDK
   - Clear separation between SDK and internal types
   - Consistent naming conventions

3. **Request/Response Logging Middleware**
   - Log all incoming requests with bodies
   - Log all responses with status codes
   - Configurable verbosity

4. **Development Mode**
   - Disable caching in development
   - Auto-reload on file changes
   - Verbose logging by default

5. **Integration Test Suite**
   - End-to-end tests for common workflows
   - Automated testing of plugin discovery
   - Mock plugin for testing