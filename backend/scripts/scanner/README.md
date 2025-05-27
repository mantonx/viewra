# Viewra Media Scanner Testing

This document describes the testing setup for the Viewra media scanner functionality.

## Test Script

The comprehensive test script `test-scanner.sh` provides complete testing for all scanner functionality including:

- **Endpoint Testing**: Validates all scanner API endpoints
- **Pause/Resume Testing**: Tests pause and resume functionality with progress preservation
- **Progress Tracking**: Verifies that scan progress is properly tracked and preserved
- **Error Handling**: Tests various error conditions and recovery scenarios

## Prerequisites

Before running tests, ensure you have:

1. **Dependencies installed**:

   ```bash
   # On Ubuntu/Debian
   sudo apt install jq curl

   # On Arch Linux
   sudo pacman -S jq curl

   # On macOS
   brew install jq curl
   ```

2. **Server running**:

   ```bash
   # Start the development environment
   docker-compose up -d

   # Or start manually
   cd backend && go run cmd/viewra/main.go
   ```

3. **Media libraries configured**: Ensure you have at least one media library set up in the system.

## Usage

### Basic Endpoint Testing

Test all scanner endpoints to ensure they're working:

```bash
./backend/test-scanner.sh endpoints
```

This will test:

- Health endpoint (`/api/health`)
- Scanner status endpoint (`/api/scanner/status`)
- Scan jobs list endpoint (`/api/scanner/jobs`)

### Pause/Resume Testing

Test the complete pause/resume functionality with a new scan:

```bash
./backend/test-scanner.sh pause-resume <library_id>
```

Example:

```bash
./backend/test-scanner.sh pause-resume 19
```

This test will:

1. Start a new scan for the specified library
2. Wait for initial progress
3. Pause the scan and verify progress is preserved
4. Resume the scan and verify it continues from where it left off
5. Monitor for continued progress after resume
6. Clean up by pausing the job

### Resume Existing Job

Test resuming an existing paused scan job:

```bash
./backend/test-scanner.sh resume <job_id>
```

Example:

```bash
./backend/test-scanner.sh resume 56
```

This test will:

1. Check the current status of the specified job
2. Pause it if it's not already paused
3. Resume the job
4. Monitor for progress to ensure it's working

### Full Test Suite

Run all tests with a single command:

```bash
./backend/test-scanner.sh full <library_id>
```

Example:

```bash
./backend/test-scanner.sh full 19
```

This runs both endpoint testing and pause/resume testing.

## Test Output

The script provides colored output for easy reading:

- ✅ **Green**: Successful operations
- ❌ **Red**: Failed operations or errors
- ⚠️ **Yellow**: Warnings or unexpected conditions
- ℹ️ **Blue**: Informational messages

## Common Library IDs

Based on your setup, common library IDs are:

- `19`: `/media/music` (main music library)
- `20`: `/app/data/test-music` (test music library)

You can check available libraries with:

```bash
curl -s http://localhost:8080/api/libraries | jq
```

## Troubleshooting

### Server Not Found

If you get "Server not found" errors:

1. Check if the server is running:

   ```bash
   docker-compose ps
   # or
   curl http://localhost:8080/api/health
   ```

2. Start the server if needed:
   ```bash
   docker-compose up -d
   ```

### Permission Denied

If you get permission denied errors:

```bash
chmod +x backend/test-scanner.sh
```

### jq Command Not Found

Install jq package:

```bash
# Ubuntu/Debian
sudo apt install jq

# Arch Linux
sudo pacman -S jq

# macOS
brew install jq
```

### Test Failures

If tests fail:

1. Check the server logs:

   ```bash
   docker-compose logs backend
   ```

2. Verify the library ID exists:

   ```bash
   curl -s http://localhost:8080/api/libraries | jq
   ```

3. Check for any running scans that might interfere:
   ```bash
   curl -s http://localhost:8080/api/scanner/jobs | jq
   ```

## Development

The test script is designed to be:

- **Self-contained**: No external dependencies except jq and curl
- **Robust**: Includes timeouts and error handling
- **Informative**: Provides detailed output about what's being tested
- **Clean**: Cleans up after itself by pausing test jobs

When modifying the scanner functionality, run the full test suite to ensure nothing is broken:

```bash
./backend/test-scanner.sh full 19
```

## API Endpoints Tested

The script tests these key endpoints:

- `GET /api/health` - Server health check
- `GET /api/scanner/status` - Scanner module status
- `GET /api/scanner/jobs` - List all scan jobs
- `GET /api/scanner/jobs/{id}` - Get specific job details
- `POST /api/scanner/scan` - Start new scan
- `DELETE /api/scanner/jobs/{id}` - Pause scan job
- `POST /api/scanner/resume/{id}` - Resume paused job

## Test Coverage

The test script covers:

- ✅ **Basic connectivity**: Server availability and endpoint accessibility
- ✅ **Scan lifecycle**: Start → Running → Pause → Resume → Progress
- ✅ **Progress preservation**: Ensures progress isn't lost during pause/resume
- ✅ **Error handling**: Timeouts, invalid responses, and edge cases
- ✅ **Performance monitoring**: Tracks files processed and scan progress
- ✅ **Cleanup**: Properly manages test jobs to avoid interference

This provides comprehensive coverage of the scanner's core functionality and ensures reliability in production environments.
