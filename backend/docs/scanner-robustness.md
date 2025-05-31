# Scanner Robustness Improvements

## Problem Description

The original issue was that when the backend restarted, scanner jobs marked as "running" in the database would become disconnected from the actual scanner state in memory, leading to:

1. API reporting jobs as "paused" while scanning was actually active
2. Frontend confusion between job IDs and library IDs
3. Orphaned scan jobs accumulating in the database
4. Inconsistent state between database and actual scanner operations

## Root Causes

1. **In-Memory vs Database State Disconnect**: Scanner state was stored in memory but job status was persisted in database
2. **Incomplete Recovery Logic**: Backend restart recovery only marked jobs as "paused" without proper synchronization
3. **API Design Issues**: Mixed use of job IDs vs library IDs in different endpoints
4. **No State Validation**: No mechanism to detect or fix state inconsistencies

## Solutions Implemented

### 1. Enhanced Orphaned Job Recovery

**File**: `backend/internal/modules/scannermodule/scanner/manager.go`

- **Comprehensive Recovery Process**: Added 5-step recovery process on startup
- **Library Validation**: Verify libraries still exist before recovering jobs
- **Duplicate Job Cleanup**: Detect and remove duplicate jobs for same library
- **Intelligent Auto-Resume**: Auto-resume jobs with significant progress (>10 files or >1%)
- **State Logging**: Detailed logging for recovery operations

```go
func (m *Manager) recoverOrphanedJobs() error {
    // STEP 1: Find orphaned "running" jobs
    // STEP 2: Find paused jobs for potential auto-resume
    // STEP 3: Clean up duplicate jobs
    // STEP 4: Mark orphaned jobs as paused
    // STEP 5: Auto-resume eligible jobs
}
```

### 2. Library-Based API Methods

**Files**: `backend/internal/modules/scannermodule/scanner/manager.go`, `backend/internal/server/handlers/scanner.go`

- **Consistent Library ID Usage**: New methods that always use library IDs instead of job IDs
- **Intelligent Job Resolution**: Find appropriate jobs based on library and status
- **Graceful Fallbacks**: Auto-start new scan if no paused scan exists

```go
func (m *Manager) PauseScanByLibrary(libraryID uint) error
func (m *Manager) ResumeScanByLibrary(libraryID uint) error
func (m *Manager) GetLibraryScanStatus(libraryID uint) (*database.ScanJob, error)
```

### 3. Background State Synchronization

**File**: `backend/internal/modules/scannermodule/scanner/manager.go`

- **Periodic Health Checks**: Background goroutine checks state every 30 seconds
- **Auto-Fix Inconsistencies**: Automatically corrects database/memory mismatches
- **Comprehensive Validation**: Checks both directions (DB→Memory and Memory→DB)

```go
func (m *Manager) StartStateSynchronizer() {
    // Runs every 30 seconds to detect and fix:
    // - DB jobs marked "running" but no in-memory scanner
    // - In-memory scanners but no DB record
    // - Status mismatches between DB and memory
}
```

### 4. Improved API Handlers

**Files**: `backend/internal/server/handlers/scanner.go`, `backend/internal/server/handlers/scanner_resume.go`

- **Library-Based Operations**: Updated pause/resume to use library IDs consistently
- **Better Error Handling**: Clearer error messages and appropriate HTTP status codes
- **Status Synchronization**: Always return current job status after operations

### 5. Startup Integration

**File**: `backend/internal/modules/scannermodule/module.go`

- **Automatic Recovery**: Recovery runs automatically on module startup
- **Background Services**: State synchronizer starts automatically
- **Graceful Error Handling**: Startup continues even if recovery has issues

## Benefits

### Crash Resilience

- Backend restarts no longer leave orphaned or inconsistent scanner state
- Jobs automatically resume from where they left off
- Duplicate jobs are cleaned up automatically

### API Consistency

- Frontend can always use library IDs for pause/resume operations
- No more confusion between job IDs and library IDs
- Consistent behavior across all scanner endpoints

### State Integrity

- Database and memory state stay synchronized
- Periodic validation catches and fixes drift
- Comprehensive logging for troubleshooting

### User Experience

- Scans automatically resume after backend restarts
- Progress is preserved across crashes
- Clear status reporting and error messages

## Testing the Improvements

### Scenario 1: Backend Restart During Scan

1. Start a scan for a library
2. Restart the backend while scanning
3. **Expected**: Job automatically marked as paused and eligible for auto-resume
4. **Expected**: If significant progress, job auto-resumes
5. **Expected**: API consistently reports correct status

### Scenario 2: Multiple Jobs Per Library

1. Create multiple scan jobs for same library (simulating the bug condition)
2. Restart backend
3. **Expected**: Duplicate jobs cleaned up, best job retained
4. **Expected**: Only one job per library remains

### Scenario 3: State Drift Detection

1. Manually create state inconsistency (DB shows running, no in-memory scanner)
2. Wait 30 seconds for background sync
3. **Expected**: Inconsistency detected and auto-fixed
4. **Expected**: Logging shows the correction

### Scenario 4: Library-Based Operations

1. Use library ID to pause scan: `POST /api/scanner/pause/{library_id}`
2. Use library ID to resume scan: `POST /api/scanner/resume/{library_id}`
3. **Expected**: Operations work regardless of underlying job IDs
4. **Expected**: Consistent API behavior

## Configuration

The improvements are self-contained and require no additional configuration. Key parameters:

- **State Sync Interval**: 30 seconds (hardcoded)
- **Auto-Resume Threshold**: 10 files or 1% progress
- **Recovery Timeout**: 30 seconds for scan stopping during cleanup

## Monitoring

Enhanced logging provides visibility into:

- Recovery operations during startup
- State synchronization activities
- Auto-resume decisions
- Error conditions and resolutions

All logs use structured logging with appropriate levels (Info, Warn, Error, Debug).
