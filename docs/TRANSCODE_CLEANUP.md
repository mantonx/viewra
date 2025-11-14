# Transcode Cleanup System

## Overview

The transcode cleanup system provides both **manual** and **automated** tools for managing disk space used by transcode files. With on-demand transcoding, disk usage can grow quickly during testing and production use.

**Two Approaches:**
1. **Manual Cleanup** - CLI tool and API endpoints for on-demand cleanup
2. **Automated Cleanup** - Background scheduler that runs periodically

---

# Manual Cleanup Tools

Use these tools when you need immediate cleanup control or want to clean specific transcodes.

## CLI Tool

### Installation

```bash
go build -o bin/transcode-cleanup ./cmd/transcode-cleanup
```

### Commands

#### Show Disk Usage Statistics

```bash
./bin/transcode-cleanup --stats
```

**Output Example:**
```
📊 Transcode Disk Usage
======================
Output Directory: ./data/transcode
Total Size:       24.06 GB
File Count:       2835

Transcode Jobs:
  Total:       18
  Completed:   4
  Failed:      5
  Queued:      0
  Processing:  9
```

#### Clean Failed Transcodes

```bash
# Dry run (shows what would be deleted)
./bin/transcode-cleanup --failed --dry-run

# Actually delete (default: older than 24 hours)
./bin/transcode-cleanup --failed

# Delete failed jobs older than 1 hour
./bin/transcode-cleanup --failed --older-than 1h
```

#### Clean Old Transcodes

```bash
# Delete transcodes older than 30 days (dry run)
./bin/transcode-cleanup --older-than 720h --dry-run

# Delete transcodes older than 7 days
./bin/transcode-cleanup --older-than 168h
```

#### Clean Transcodes for Specific Media

```bash
# Clean all transcodes for media ID 123
./bin/transcode-cleanup --media-id 123 --dry-run
./bin/transcode-cleanup --media-id 123
```

#### Find and Clean Orphaned Files

Orphaned files are transcode files on disk without corresponding database records.

```bash
# Find orphans
./bin/transcode-cleanup --orphans --dry-run

# Delete orphans
./bin/transcode-cleanup --orphans
```

#### Clean ALL Transcodes (Dangerous!)

```bash
# This will prompt for confirmation if not using --dry-run
./bin/transcode-cleanup --all --dry-run
./bin/transcode-cleanup --all
```

### CLI Options

| Option | Description | Default |
|--------|-------------|---------|
| `--db` | Path to SQLite database | `./data/viewra.db` |
| `--output-dir` | Transcode output directory | `./data/transcode` |
| `--stats` | Show disk usage statistics | - |
| `--all` | Clean all transcodes (dangerous!) | false |
| `--failed` | Clean failed transcode jobs | false |
| `--orphans` | Clean orphaned files | false |
| `--media-id` | Clean transcodes for specific media | 0 |
| `--quality` | Clean specific quality only | "" |
| `--older-than` | Clean transcodes older than duration | 0 |
| `--dry-run` | Show what would be deleted without deleting | false |

## API Endpoints

### Get Disk Usage

```bash
curl http://localhost:8080/api/transcode/disk-usage
```

**Response:**
```json
{
  "output_dir": "./data/transcode",
  "total_size_bytes": 25820000000,
  "total_size_human": "24.06 GB",
  "file_count": 2835,
  "total_jobs": 18,
  "completed_count": 4,
  "failed_count": 5,
  "queued_count": 0,
  "processing_count": 9
}
```

### Cleanup Transcodes

```bash
# Dry run to see what would be deleted
curl -X POST http://localhost:8080/api/transcode/cleanup \
  -H "Content-Type: application/json" \
  -d '{
    "failed": true,
    "older_than_hours": 24,
    "dry_run": true
  }'

# Actually delete failed jobs
curl -X POST http://localhost:8080/api/transcode/cleanup \
  -H "Content-Type: application/json" \
  -d '{
    "failed": true,
    "older_than_hours": 24,
    "dry_run": false
  }'
```

**Request Body Options:**
```json
{
  "media_id": 123,           // Optional: Clean transcodes for specific media
  "quality": "720p",         // Optional: Clean specific quality only
  "failed": true,            // Clean failed jobs
  "orphans": true,           // Clean orphaned files
  "older_than_hours": 168,   // Clean items older than X hours
  "dry_run": true            // Preview without actually deleting
}
```

**Response:**
```json
{
  "deleted_count": 5,
  "deleted_size_bytes": 12800000000,
  "deleted_size_human": "11.92 GB",
  "failed_count": 0,
  "errors": [],
  "dry_run": true
}
```

---

# Automated Cleanup Scheduler

The automated cleanup scheduler runs in the background, monitoring disk usage and cleaning up transcode files based on configurable policies. It starts automatically with the application and requires no manual intervention.

## Configuration

All configuration is done through environment variables. If not set, sensible defaults are used.

### Environment Variables

```bash
# Enable/Disable Automated Cleanup
TRANSCODE_CLEANUP_ENABLED=true               # Default: true

# Cleanup Interval
TRANSCODE_CLEANUP_INTERVAL_HOURS=6          # Default: 6 (run every 6 hours)

# Disk Usage Thresholds
TRANSCODE_CLEANUP_DISK_THRESHOLD=85         # Default: 85% (start cleanup at this %)
TRANSCODE_CLEANUP_DISK_WARNING=80           # Default: 80% (log warnings at this %)
TRANSCODE_MIN_FREE_SPACE_GB=10              # Default: 10GB minimum free space

# Age-Based Policies
TRANSCODE_MAX_AGE_DAYS=30                   # Default: 30 (delete transcodes older than 30 days)
TRANSCODE_MAX_IDLE_DAYS=7                   # Default: 7 (delete if not accessed in 7 days)

# Storage Limits
TRANSCODE_MAX_STORAGE_GB=50                 # Default: 0 (unlimited)

# Cleanup Behavior
TRANSCODE_CLEANUP_BATCH_SIZE=10             # Default: 10 (max transcodes per run)
TRANSCODE_KEEP_FAILED_HOURS=24              # Default: 24 (keep failed jobs for 24 hours)
```

## How It Works

### 1. Startup
When the application starts, the cleanup scheduler:
- Reads configuration from environment variables
- Performs an immediate cleanup check
- Schedules recurring cleanup based on `TRANSCODE_CLEANUP_INTERVAL_HOURS`

### 2. Each Cleanup Cycle

The scheduler performs checks in this order:

#### A. Policy-Based Cleanup (Always Runs)

1. **Failed Jobs** - Deletes failed transcodes older than `TRANSCODE_KEEP_FAILED_HOURS`
2. **Old Transcodes** - Deletes completed transcodes older than `TRANSCODE_MAX_AGE_DAYS`
3. **Idle Transcodes** - Deletes transcodes not accessed in `TRANSCODE_MAX_IDLE_DAYS`
4. **Orphaned Files** - Removes files without database records

#### B. Disk Threshold Cleanup (Triggered by Thresholds)

If any of these conditions are met:
- Disk usage >= `TRANSCODE_CLEANUP_DISK_THRESHOLD`
- Free space < `TRANSCODE_MIN_FREE_SPACE_GB`
- Total transcode storage > `TRANSCODE_MAX_STORAGE_GB`

Then:
- Performs **LRU (Least Recently Used)** cleanup
- Deletes up to `TRANSCODE_CLEANUP_BATCH_SIZE` oldest accessed transcodes
- Logs which transcodes were removed

### 3. Logging

The scheduler logs:
- **INFO**: Cleanup runs, disk usage stats, successful cleanup operations
- **WARN**: Disk usage approaching/exceeding thresholds
- **ERROR**: Failed cleanup operations
- **DEBUG**: Individual transcode deletions (LRU)

## Example Configurations

### Conservative (Production)
Keep more data, clean less aggressively:

```bash
TRANSCODE_CLEANUP_ENABLED=true
TRANSCODE_CLEANUP_INTERVAL_HOURS=12          # Check twice daily
TRANSCODE_CLEANUP_DISK_THRESHOLD=90          # Only cleanup at 90% full
TRANSCODE_MAX_AGE_DAYS=90                    # Keep for 3 months
TRANSCODE_MAX_IDLE_DAYS=30                   # Keep if accessed in last month
TRANSCODE_MAX_STORAGE_GB=100                 # Allow 100GB total
```

### Aggressive (Testing/Development)
Clean frequently to save space:

```bash
TRANSCODE_CLEANUP_ENABLED=true
TRANSCODE_CLEANUP_INTERVAL_HOURS=1           # Check every hour
TRANSCODE_CLEANUP_DISK_THRESHOLD=75          # Cleanup at 75% full
TRANSCODE_MAX_AGE_DAYS=7                     # Keep for 1 week
TRANSCODE_MAX_IDLE_DAYS=1                    # Delete if not accessed in 24 hours
TRANSCODE_MAX_STORAGE_GB=20                  # Limit to 20GB
TRANSCODE_CLEANUP_BATCH_SIZE=20              # Delete more per run
```

### Minimal (Low Disk Space)
For systems with very limited storage:

```bash
TRANSCODE_CLEANUP_ENABLED=true
TRANSCODE_CLEANUP_INTERVAL_HOURS=1
TRANSCODE_CLEANUP_DISK_THRESHOLD=70
TRANSCODE_MIN_FREE_SPACE_GB=20               # Require 20GB free
TRANSCODE_MAX_AGE_DAYS=3                     # Delete after 3 days
TRANSCODE_MAX_IDLE_DAYS=1                    # Delete if idle for 1 day
TRANSCODE_MAX_STORAGE_GB=10                  # Strict 10GB limit
```

### Disabled
To turn off automated cleanup (use manual cleanup only):

```bash
TRANSCODE_CLEANUP_ENABLED=false
```

## Monitoring

### Check Logs

The scheduler logs its activity. Look for:

```
INFO automated cleanup scheduler started interval=6h0m0s disk_threshold=85
INFO running automated cleanup check
INFO disk usage check used_percent=78 free_gb=45 total_gb=200
INFO cleaned failed transcode jobs count=3 size_bytes=5242880
INFO cleaned idle transcodes count=5 size_bytes=12582912 max_idle_hours=168
```

### Warning Signs

```
WARN disk usage approaching threshold used_percent=82 warning_threshold=80
WARN disk threshold exceeded, performing aggressive cleanup reason="disk usage 87% exceeds threshold 85%"
INFO LRU cleanup completed deleted_count=10 deleted_size_bytes=25165824
```

### Manual Override

You can still use the manual cleanup tools even with automation enabled:

```bash
# CLI tool works alongside automation
./bin/transcode-cleanup --stats
./bin/transcode-cleanup --failed --dry-run

# API endpoints also available
curl http://localhost:8080/api/transcode/disk-usage
```

## How Access Tracking Works

The system automatically tracks when transcodes are used:

1. **On Segment Request** - Every time an HLS segment is served, the database records:
   - `last_accessed_at` - Updated to current time
   - `access_count` - Incremented by 1

2. **On Transcode Completion** - When a transcode finishes:
   - `file_path` - Path to output directory
   - `file_size_bytes` - Total size of all files

3. **LRU Cleanup Uses This Data** - When disk is full:
   - Queries transcodes ordered by `last_accessed_at ASC`
   - Deletes oldest accessed first
   - Preserves frequently-used content

## Best Practices

### 1. Start Conservative
Use default settings first, then adjust based on your usage patterns.

### 2. Monitor for a Week
Watch the logs to see how much cleanup happens naturally.

### 3. Adjust Thresholds
If you see frequent LRU cleanup, consider:
- Increasing `TRANSCODE_MAX_STORAGE_GB`
- Decreasing `TRANSCODE_MAX_AGE_DAYS`
- Increasing disk threshold

### 4. Balance Age vs Idle
- **Age-based**: Good for predictable retention (e.g., keep 30 days)
- **Idle-based**: Good for optimizing popular content (keeps what's watched)

### 5. Failed Job Cleanup
Keep `TRANSCODE_KEEP_FAILED_HOURS` at 24h minimum for debugging.

## Troubleshooting

### Cleanup Not Running

Check if enabled:
```bash
# Should see this in logs
automated cleanup scheduler started
```

If you see:
```
automated cleanup scheduler is disabled
```

Then set: `TRANSCODE_CLEANUP_ENABLED=true`

### Disk Still Full

1. Check current usage:
```bash
./bin/transcode-cleanup --stats
```

2. Run manual cleanup immediately:
```bash
./bin/transcode-cleanup --all --dry-run
./bin/transcode-cleanup --all
```

3. Lower thresholds:
```bash
TRANSCODE_CLEANUP_DISK_THRESHOLD=70
TRANSCODE_MAX_AGE_DAYS=7
```

### Too Aggressive

If cleanup is deleting content you want to keep:

1. Increase retention periods:
```bash
TRANSCODE_MAX_AGE_DAYS=90
TRANSCODE_MAX_IDLE_DAYS=30
```

2. Increase storage limits:
```bash
TRANSCODE_MAX_STORAGE_GB=200
```

3. Reduce cleanup frequency:
```bash
TRANSCODE_CLEANUP_INTERVAL_HOURS=24  # Once daily
```

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│           Cleanup Scheduler (Background Service)         │
├─────────────────────────────────────────────────────────┤
│                                                           │
│  ┌──────────────────────────────────────────────────┐  │
│  │  Runs every TRANSCODE_CLEANUP_INTERVAL_HOURS     │  │
│  └────────────────────┬─────────────────────────────┘  │
│                       │                                  │
│       ┌───────────────┴───────────────┐                │
│       │                               │                │
│  ┌────▼─────┐                  ┌─────▼────┐           │
│  │ Policy   │                  │  Disk    │           │
│  │ Cleanup  │                  │Threshold │           │
│  │ (Always) │                  │ Cleanup  │           │
│  └────┬─────┘                  │(If       │           │
│       │                        │ Needed)  │           │
│  ┌────▼───────────────┐        └─────┬────┘           │
│  │ - Failed (24h)     │              │                │
│  │ - Old (30d)        │         ┌────▼────┐           │
│  │ - Idle (7d)        │         │   LRU   │           │
│  │ - Orphans          │         │ Cleanup │           │
│  └────────────────────┘         └─────────┘           │
│                                                          │
└──────────────────────────────────────────────────────────┘
```

## Related Documentation

- [Manual Cleanup Tools](./TRANSCODE_CLEANUP.md)
- [On-Demand Transcoding ADR](./decisions/005-on-demand-transcoding-strategy.md)
