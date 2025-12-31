# Database Choice, Migration & Kubernetes Support

## Overview

Enable users to choose between SQLite and PostgreSQL databases, view their current database configuration in the UI, migrate seamlessly between database types, and deploy Viewra in Kubernetes environments with proper health checks and scaling support.

## Goals

1. **Visibility**: Users can see their current database type and connection details in System Settings
2. **Flexibility**: Users can migrate between SQLite and PostgreSQL in either direction
3. **Safety**: Migrations are atomic with verification, rollback support, and automatic backups
4. **Kubernetes-Ready**: Proper health probes, graceful shutdown, and Helm chart for K8s deployment
5. **User Experience**: Clear warnings about SQLite limitations in containerized environments

## Non-Goals

- Automatic failover between databases
- Real-time replication
- Support for databases other than SQLite and PostgreSQL
- Zero-downtime migration (maintenance mode is required)

---

## Key Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Password storage | Environment variable only (`DB_PASSWORD`) | Security best practice; never store secrets in config files |
| Config format | YAML (`data/config.yaml`) | Human-readable, widely supported, good for K8s ConfigMaps |
| Config load priority | Env vars → Config file → Defaults | K8s pattern: env vars override file config |
| First-run behavior | Default to SQLite | Zero-config start; users can migrate later |
| Migration approach | Direct DB-to-DB transfer | No intermediate files = no data exposure risk |
| During migration | Maintenance mode, blocking | Ensures data consistency |
| Source DB after migration | Preserved as backup | Easy rollback if issues discovered |
| K8s manifests | Helm chart + Kustomize | Industry standard deployment tools |
| SQLite in K8s | Show warning | Help users avoid common pitfalls |
| Multiple instances | Detect via advisory lock | Prevent SQLite corruption from concurrent writes |

---

## Architecture

### Configuration Loading

```
┌─────────────────────────────────────────────────────────────┐
│                    Configuration Sources                     │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│   Priority 1: Environment Variables (highest)               │
│   ├── DB_DRIVER, DB_HOST, DB_PORT, DB_USER, DB_PASSWORD    │
│   ├── DB_NAME, DB_SSL_MODE, DB_PATH                        │
│   └── Set via K8s ConfigMaps/Secrets                       │
│                           │                                 │
│                           ▼                                 │
│   Priority 2: Config File (data/config.yaml)               │
│   ├── Persistent settings                                  │
│   ├── Written by migration wizard                          │
│   └── Human-editable                                       │
│                           │                                 │
│                           ▼                                 │
│   Priority 3: Defaults (lowest)                            │
│   ├── driver: sqlite                                       │
│   ├── sqlite.path: data/viewra.db                          │
│   └── postgres.host: localhost, port: 5432                 │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

### Migration Flow

```
┌─────────────────────────────────────────────────────────────┐
│                     Migration Process                        │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  1. Pre-Migration                                           │
│     ├── Test target connection                              │
│     ├── Verify target is empty or compatible                │
│     ├── Estimate time and size                              │
│     ├── Enter maintenance mode                              │
│     └── Create backup of source DB                          │
│                           │                                 │
│                           ▼                                 │
│  2. Schema Setup                                            │
│     ├── Run migrations on target DB                         │
│     └── Verify schema version matches                       │
│                           │                                 │
│                           ▼                                 │
│  3. Data Transfer (in transaction)                          │
│     ├── Copy tables in dependency order                     │
│     ├── Track progress (tables, rows)                       │
│     └── On any error → Rollback target, abort              │
│                           │                                 │
│                           ▼                                 │
│  4. Verification                                            │
│     ├── Row count verification (all tables)                 │
│     ├── Checksum spot-checks (sample rows)                  │
│     ├── Foreign key integrity check                         │
│     ├── Media file path verification                        │
│     └── Admin authentication test                           │
│                           │                                 │
│                           ▼                                 │
│  5. Finalization                                            │
│     ├── Update config file to point to new DB              │
│     ├── Exit maintenance mode                               │
│     ├── Prompt user to restart                              │
│     └── Source DB preserved for rollback                   │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

### Table Dependency Order

Tables must be copied in order to satisfy foreign key constraints:

```
Level 0 (no dependencies):
├── users
├── libraries
├── system_settings
├── scheduled_tasks
└── plugins

Level 1 (depends on Level 0):
├── sessions (→ users)
├── user_settings (→ users)
├── media_items (→ libraries)
├── scan_jobs (→ libraries)
├── plugin_kv (→ plugins)
└── scheduler_executions (→ scheduled_tasks)

Level 2 (depends on Level 1):
├── watch_progress (→ users, media_items)
├── movie_details (→ media_items)
├── tv_show_details (→ media_items)
├── music_artists (→ libraries)
├── scan_checkpoints (→ scan_jobs)
├── scan_errors (→ scan_jobs)
├── media_tracks (→ media_items)
├── images (→ media_items)
├── enrichment_status (→ media_items)
└── enrichment_queue (→ media_items)

Level 3 (depends on Level 2):
├── tv_seasons (→ tv_show_details)
├── music_albums (→ music_artists)
└── media_cast (→ media_items, people)

Level 4 (depends on Level 3):
├── tv_episodes (→ tv_seasons)
└── music_tracks (→ music_albums)

Plugin tables (last):
└── plugin_* (all plugin-owned tables)
```

### Health Check Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                     Health Endpoints                         │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  /health/live (Liveness Probe)                              │
│  ├── Purpose: "Is the process running?"                     │
│  ├── Checks: None (always 200 if handler reached)           │
│  ├── K8s action on failure: Restart pod                     │
│  └── Response: {"status": "alive"}                          │
│                                                             │
│  /health/ready (Readiness Probe)                            │
│  ├── Purpose: "Can we serve traffic?"                       │
│  ├── Checks:                                                │
│  │   ├── Database connected                                 │
│  │   ├── Migrations complete                                │
│  │   └── Not in maintenance mode                            │
│  ├── K8s action on failure: Remove from service             │
│  └── Response: {"status": "ready"} or 503                   │
│                                                             │
│  /health (Full Status)                                      │
│  ├── Purpose: Detailed health for monitoring/debugging      │
│  ├── Checks: All components                                 │
│  └── Response: Full component status, metrics               │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

---

## Configuration File Schema

**Location:** `data/config.yaml`

```yaml
# Viewra Configuration
# Environment variables take precedence over this file.
# Password must be set via DB_PASSWORD environment variable.

database:
  # Database driver: "sqlite" or "postgres"
  driver: sqlite
  
  # Set to false to disable auto-migration (for external migration tools)
  auto_migrate: true
  
  # SQLite configuration
  sqlite:
    # Path to SQLite database file (relative to data directory or absolute)
    path: viewra.db
  
  # PostgreSQL configuration
  postgres:
    host: localhost
    port: 5432
    user: viewra
    # Password must be set via DB_PASSWORD environment variable
    database: viewra
    ssl_mode: disable  # disable, require, verify-ca, verify-full
    
    # Connection pool settings
    max_open_conns: 25      # Maximum open connections
    max_idle_conns: 5       # Maximum idle connections
    conn_max_lifetime: 5m   # Maximum connection lifetime
    conn_max_idle_time: 1m  # Maximum idle time before close

server:
  # Graceful shutdown timeout
  shutdown_timeout: 30s

# Migration settings (used during database migration)
migration:
  # Days to keep old database after successful migration
  retention_days: 7
```

---

## API Specification

### Health Endpoints

#### GET /health/live
Liveness probe for Kubernetes.

**Response (always 200):**
```json
{
  "status": "alive",
  "timestamp": "2024-01-15T10:30:00Z"
}
```

#### GET /health/ready
Readiness probe for Kubernetes.

**Response (200 if ready, 503 if not):**
```json
{
  "status": "ready",
  "checks": {
    "database": "ok",
    "migrations": "ok",
    "maintenance_mode": false
  },
  "timestamp": "2024-01-15T10:30:00Z"
}
```

#### GET /health
Full health status (existing endpoint, enhanced).

**Response:**
```json
{
  "status": "healthy",
  "version": "1.0.0",
  "uptime": "24h15m30s",
  "timestamp": "2024-01-15T10:30:00Z",
  "components": {
    "database": {
      "status": "pass",
      "message": "Ping: 1.2ms",
      "details": {
        "driver": "postgres",
        "version": "16.1",
        "pool_active": 3,
        "pool_idle": 2,
        "pool_max": 25
      }
    },
    "scheduler": {
      "status": "pass",
      "message": "5 tasks registered"
    },
    "transcode_queue": {
      "status": "pass",
      "message": "Queue operational"
    }
  },
  "system": {
    "num_goroutines": 45,
    "memory_usage_mb": 128,
    "num_cpu": 8
  }
}
```

### System Info Endpoint

#### GET /api/system/info
Returns system information including database details.

**Response:**
```json
{
  "cpu": {
    "model": "AMD Ryzen 9 5900X",
    "cores": 12,
    "threads": 24
  },
  "memory": {
    "total_gb": 32,
    "available_gb": 24
  },
  "gpu": {
    "name": "NVIDIA RTX 3080",
    "vram_gb": 10
  },
  "database": {
    "driver": "postgres",
    "host": "localhost",
    "port": 5432,
    "name": "viewra",
    "version": "16.1",
    "size_bytes": 52428800,
    "table_count": 45,
    "pool": {
      "active": 3,
      "idle": 2,
      "max": 25
    }
  },
  "environment": {
    "is_container": true,
    "is_kubernetes": true,
    "instance_id": "viewra-abc123"
  },
  "warnings": [
    {
      "code": "SQLITE_IN_K8S",
      "message": "SQLite is not recommended in Kubernetes environments",
      "severity": "warning"
    }
  ]
}
```

### Maintenance Mode Endpoints

#### GET /api/admin/system/maintenance
Get current maintenance mode status.

**Response:**
```json
{
  "enabled": false,
  "reason": null,
  "started_at": null,
  "estimated_end": null
}
```

#### POST /api/admin/system/maintenance
Enable or disable maintenance mode.

**Request:**
```json
{
  "enabled": true,
  "reason": "Database migration in progress"
}
```

**Response:**
```json
{
  "enabled": true,
  "reason": "Database migration in progress",
  "started_at": "2024-01-15T10:30:00Z"
}
```

### Database Migration Endpoints

#### POST /api/admin/system/database/test-connection
Test connection to a target database.

**Request:**
```json
{
  "driver": "postgres",
  "postgres": {
    "host": "localhost",
    "port": 5432,
    "user": "viewra",
    "password": "secret",
    "database": "viewra",
    "ssl_mode": "disable"
  }
}
```

**Response (success):**
```json
{
  "success": true,
  "message": "Successfully connected to PostgreSQL 16.1",
  "details": {
    "version": "16.1",
    "server_time": "2024-01-15T10:30:00Z",
    "is_empty": true,
    "existing_tables": 0
  }
}
```

**Response (failure):**
```json
{
  "success": false,
  "message": "Connection failed: connection refused",
  "error": "dial tcp 127.0.0.1:5432: connect: connection refused"
}
```

#### POST /api/admin/system/database/estimate
Estimate migration time and data size.

**Request:**
```json
{
  "target_driver": "postgres"
}
```

**Response:**
```json
{
  "source": {
    "driver": "sqlite",
    "size_bytes": 52428800,
    "table_count": 45,
    "total_rows": 125000
  },
  "estimate": {
    "duration_seconds": 180,
    "duration_human": "~3 minutes",
    "data_size_bytes": 52428800,
    "tables": [
      {"name": "media_items", "rows": 50000, "size_bytes": 25000000},
      {"name": "images", "rows": 45000, "size_bytes": 15000000},
      {"name": "watch_progress", "rows": 10000, "size_bytes": 5000000}
    ]
  },
  "warnings": []
}
```

#### POST /api/admin/system/database/migrate
Start database migration.

**Request:**
```json
{
  "target_driver": "postgres",
  "postgres": {
    "host": "localhost",
    "port": 5432,
    "user": "viewra",
    "password": "secret",
    "database": "viewra",
    "ssl_mode": "disable"
  }
}
```

**Response (started):**
```json
{
  "started": true,
  "migration_id": "mig_abc123",
  "message": "Migration started. Server will enter maintenance mode."
}
```

**Response (already in progress):**
```json
{
  "started": false,
  "error": "Migration already in progress",
  "migration_id": "mig_abc123"
}
```

#### GET /api/admin/system/database/migrate
Get migration progress.

**Response (in progress):**
```json
{
  "status": "in_progress",
  "migration_id": "mig_abc123",
  "phase": "copying",
  "started_at": "2024-01-15T10:30:00Z",
  "progress": {
    "current_table": "media_items",
    "tables_completed": 12,
    "tables_total": 45,
    "rows_copied": 35000,
    "rows_total": 125000,
    "bytes_copied": 25000000,
    "bytes_total": 52428800,
    "percent_complete": 48,
    "elapsed_seconds": 90,
    "estimated_remaining_seconds": 95
  },
  "phases": [
    {"name": "maintenance_mode", "status": "completed"},
    {"name": "backup", "status": "completed"},
    {"name": "connect_target", "status": "completed"},
    {"name": "create_schema", "status": "completed"},
    {"name": "copying", "status": "in_progress"},
    {"name": "verification", "status": "pending"},
    {"name": "update_config", "status": "pending"}
  ]
}
```

**Response (completed):**
```json
{
  "status": "completed",
  "migration_id": "mig_abc123",
  "started_at": "2024-01-15T10:30:00Z",
  "completed_at": "2024-01-15T10:33:15Z",
  "duration_seconds": 195,
  "result": {
    "tables_migrated": 45,
    "rows_migrated": 125000,
    "verification_passed": true,
    "old_database_path": "data/viewra.db.pre-migration-20240115",
    "requires_restart": true
  }
}
```

**Response (failed):**
```json
{
  "status": "failed",
  "migration_id": "mig_abc123",
  "started_at": "2024-01-15T10:30:00Z",
  "failed_at": "2024-01-15T10:31:45Z",
  "error": {
    "phase": "copying",
    "table": "media_items",
    "message": "Foreign key constraint violation",
    "details": "Key (library_id)=(999) is not present in table \"libraries\""
  },
  "rollback": {
    "performed": true,
    "success": true,
    "message": "Target database transaction rolled back. Source database unchanged."
  }
}
```

#### DELETE /api/admin/system/database/migrate
Cancel an in-progress migration.

**Response:**
```json
{
  "cancelled": true,
  "message": "Migration cancelled. Rolling back changes to target database.",
  "rollback_status": "completed"
}
```

#### POST /api/admin/system/database/rollback
Rollback to previous database after a completed migration.

**Request:**
```json
{
  "confirm": true
}
```

**Response:**
```json
{
  "success": true,
  "message": "Configuration updated to use previous database. Restart required.",
  "previous_database": {
    "driver": "sqlite",
    "path": "data/viewra.db.pre-migration-20240115"
  },
  "requires_restart": true
}
```

---

## UI Components

### DatabaseInfoCard

Displays current database information in System Settings.

**Location:** `web/src/components/DatabaseInfoCard.tsx`

**Design:**
```
┌─────────────────────────────────────────────────────────────┐
│ Database                                              [i]   │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  ┌──────────┐                                               │
│  │ 🐘       │  PostgreSQL 16.1                              │
│  │          │  localhost:5432/viewra                        │
│  └──────────┘                                               │
│                                                             │
│  Size         52.4 MB                                       │
│  Tables       45                                            │
│  Connection   ✓ Healthy (3/25 active)                       │
│                                                             │
│  ┌─────────────────────────────────────────────────────┐   │
│  │           [Migrate to SQLite...]                     │   │
│  └─────────────────────────────────────────────────────┘   │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

**For SQLite:**
```
┌─────────────────────────────────────────────────────────────┐
│ Database                                              [i]   │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  ┌──────────┐                                               │
│  │ 📄       │  SQLite 3.45.0                                │
│  │          │  data/viewra.db                               │
│  └──────────┘                                               │
│                                                             │
│  Size         52.4 MB                                       │
│  Tables       45                                            │
│  Connection   ✓ Healthy                                     │
│                                                             │
│  ┌─────────────────────────────────────────────────────┐   │
│  │           [Migrate to PostgreSQL...]                 │   │
│  └─────────────────────────────────────────────────────┘   │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

### DatabaseWarningBanner

Warning shown when SQLite is detected in Kubernetes/container environment.

**Location:** `web/src/components/DatabaseWarningBanner.tsx`

**Design:**
```
┌─────────────────────────────────────────────────────────────┐
│ ⚠️  SQLite in Container Environment                         │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│ SQLite is not recommended for containerized or Kubernetes   │
│ deployments due to:                                         │
│                                                             │
│   • Single-writer limitation prevents scaling               │
│   • Data loss risk on unexpected pod termination            │
│   • Poor performance with network-attached storage          │
│                                                             │
│ Consider migrating to PostgreSQL for better reliability.    │
│                                                             │
│   [Migrate to PostgreSQL]                    [Dismiss]      │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

### MaintenanceBanner

Banner shown when server is in maintenance mode.

**Location:** `web/src/components/MaintenanceBanner.tsx`

**Design:**
```
┌─────────────────────────────────────────────────────────────┐
│ 🔧 Maintenance Mode                                         │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│ The server is currently undergoing maintenance.             │
│ Reason: Database migration in progress                      │
│                                                             │
│ Please wait while the operation completes.                  │
│ This page will automatically refresh when ready.            │
│                                                             │
│   ████████████████░░░░░░░░░░░░ 48%                          │
│   Copying data... (media_items: 35,000 / 50,000 rows)       │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

### Migration Wizard

Multi-step wizard for database migration.

**Location:** `web/src/views/settings/DatabaseMigration/`

#### Step 1: Choose Target

```
┌─────────────────────────────────────────────────────────────┐
│ Migrate Database                                      1/4   │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│ Current Database: SQLite (data/viewra.db)                   │
│                                                             │
│ Choose your target database:                                │
│                                                             │
│   ┌─────────────────────────────────────────────────────┐   │
│   │ ○ SQLite                                     (current)│  │
│   │   Self-contained, zero configuration                  │   │
│   │   Best for: Single user, local deployments           │   │
│   └─────────────────────────────────────────────────────┘   │
│                                                             │
│   ┌─────────────────────────────────────────────────────┐   │
│   │ ● PostgreSQL                                         │   │
│   │   Separate server, robust and scalable               │   │
│   │   Best for: Multi-user, Kubernetes, production       │   │
│   └─────────────────────────────────────────────────────┘   │
│                                                             │
│                                    [Cancel]    [Next →]     │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

#### Step 2: Configure Connection (PostgreSQL)

```
┌─────────────────────────────────────────────────────────────┐
│ Migrate Database                                      2/4   │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│ Configure PostgreSQL Connection                             │
│                                                             │
│   Host         ┌─────────────────────────────────────┐     │
│                │ localhost                            │     │
│                └─────────────────────────────────────┘     │
│                                                             │
│   Port         ┌─────────────────────────────────────┐     │
│                │ 5432                                 │     │
│                └─────────────────────────────────────┘     │
│                                                             │
│   Database     ┌─────────────────────────────────────┐     │
│                │ viewra                               │     │
│                └─────────────────────────────────────┘     │
│                                                             │
│   Username     ┌─────────────────────────────────────┐     │
│                │ viewra                               │     │
│                └─────────────────────────────────────┘     │
│                                                             │
│   Password     ┌─────────────────────────────────────┐     │
│                │ ••••••••                             │     │
│                └─────────────────────────────────────┘     │
│                                                             │
│   SSL Mode     ┌─────────────────────────────────────┐     │
│                │ Disable                          ▼  │     │
│                └─────────────────────────────────────┘     │
│                                                             │
│   ┌──────────────────────────────────────────────────┐     │
│   │  ✓ Connection successful (PostgreSQL 16.1)       │     │
│   └──────────────────────────────────────────────────┘     │
│                                                             │
│                     [← Back]   [Test Connection]   [Next →] │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

#### Step 2: Configure Connection (SQLite)

```
┌─────────────────────────────────────────────────────────────┐
│ Migrate Database                                      2/4   │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│ Configure SQLite Database                                   │
│                                                             │
│   Database Path                                             │
│   ┌─────────────────────────────────────────────────────┐   │
│   │ data/viewra.db                                      │   │
│   └─────────────────────────────────────────────────────┘   │
│   Path relative to application data directory               │
│                                                             │
│   ℹ️  A new SQLite database will be created at this path.   │
│                                                             │
│                              [← Back]              [Next →] │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

#### Step 3: Review & Confirm

```
┌─────────────────────────────────────────────────────────────┐
│ Migrate Database                                      3/4   │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│ Review Migration                                            │
│                                                             │
│   From        SQLite (data/viewra.db)                       │
│   To          PostgreSQL (localhost:5432/viewra)            │
│                                                             │
│ ┌───────────────────────────────────────────────────────┐   │
│ │ Estimation                                            │   │
│ ├───────────────────────────────────────────────────────┤   │
│ │ Data size:        52.4 MB                             │   │
│ │ Tables:           45                                  │   │
│ │ Rows:             125,000                             │   │
│ │ Estimated time:   ~3 minutes                          │   │
│ └───────────────────────────────────────────────────────┘   │
│                                                             │
│ ⚠️  Important:                                               │
│                                                             │
│   • Server will enter maintenance mode during migration     │
│   • All users will be temporarily disconnected              │
│   • A backup of your current database will be created       │
│   • Server restart required after completion                │
│                                                             │
│   ☑️  I understand and want to proceed                       │
│                                                             │
│                              [← Back]    [Start Migration]  │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

#### Step 4: Migration Progress

```
┌─────────────────────────────────────────────────────────────┐
│ Migrate Database                                      4/4   │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│ Migration in Progress                                       │
│                                                             │
│   ████████████████████░░░░░░░░░░░░ 65%                      │
│                                                             │
│   Elapsed: 1m 57s    Remaining: ~1m 03s                     │
│                                                             │
│ ┌───────────────────────────────────────────────────────┐   │
│ │ ✓ Entered maintenance mode                            │   │
│ │ ✓ Created backup (viewra.db.pre-migration-20240115)   │   │
│ │ ✓ Connected to PostgreSQL                             │   │
│ │ ✓ Created schema (45 tables)                          │   │
│ │ ● Copying data...                                     │   │
│ │   ├── ✓ users (3 rows)                                │   │
│ │   ├── ✓ libraries (2 rows)                            │   │
│ │   ├── ✓ media_items (50,000 rows)                     │   │
│ │   ├── ● images (32,450 / 45,000 rows)                 │   │
│ │   │     ████████████████░░░░░░ 72%                    │   │
│ │   └── ○ 38 tables remaining                           │   │
│ │ ○ Verifying integrity                                 │   │
│ │ ○ Updating configuration                              │   │
│ └───────────────────────────────────────────────────────┘   │
│                                                             │
│                                              [Cancel]       │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

#### Step 4: Migration Complete

```
┌─────────────────────────────────────────────────────────────┐
│ Migrate Database                                      4/4   │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│                         ✓                                   │
│                                                             │
│              Migration Completed Successfully               │
│                                                             │
│ ┌───────────────────────────────────────────────────────┐   │
│ │ Summary                                               │   │
│ ├───────────────────────────────────────────────────────┤   │
│ │ Duration:         3m 12s                              │   │
│ │ Tables migrated:  45                                  │   │
│ │ Rows migrated:    125,000                             │   │
│ │ Verification:     ✓ Passed                            │   │
│ └───────────────────────────────────────────────────────┘   │
│                                                             │
│ Your old database has been preserved at:                    │
│ data/viewra.db.pre-migration-20240115                       │
│                                                             │
│ It will be automatically deleted after 7 days, or you can   │
│ delete it manually from System Settings.                    │
│                                                             │
│ A server restart is required to use the new database.       │
│                                                             │
│                     [Restart Later]     [Restart Now]       │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

---

## Kubernetes Deployment

### Helm Chart Structure

```
deploy/helm/viewra/
├── Chart.yaml
├── values.yaml
├── README.md
├── templates/
│   ├── _helpers.tpl
│   ├── deployment.yaml
│   ├── service.yaml
│   ├── configmap.yaml
│   ├── secret.yaml
│   ├── ingress.yaml
│   ├── pvc.yaml
│   ├── serviceaccount.yaml
│   └── NOTES.txt
└── charts/
    └── postgresql/          # Optional subchart
```

### values.yaml

```yaml
# Default values for Viewra Helm chart

# Number of replicas (requires PostgreSQL for >1)
replicaCount: 1

image:
  repository: viewra/viewra
  tag: ""  # Defaults to chart appVersion
  pullPolicy: IfNotPresent

imagePullSecrets: []
nameOverride: ""
fullnameOverride: ""

serviceAccount:
  create: true
  annotations: {}
  name: ""

# Database configuration
database:
  # Type: "sqlite" or "postgres"
  type: postgres
  
  # External PostgreSQL configuration
  external:
    enabled: true
    host: ""
    port: 5432
    database: viewra
    user: viewra
    # Name of existing secret with 'password' key
    existingSecret: ""
    # Or specify password directly (not recommended for production)
    password: ""
    sslMode: disable
  
  # Deploy PostgreSQL as subchart
  postgresql:
    enabled: false
    auth:
      database: viewra
      username: viewra
      # Uses existing secret or generates one
      existingSecret: ""
      password: ""
    primary:
      persistence:
        enabled: true
        size: 10Gi
  
  # Connection pool settings
  pool:
    maxOpenConns: 25
    maxIdleConns: 5
    connMaxLifetime: "5m"
    connMaxIdleTime: "1m"

# Server configuration
server:
  port: 8080
  shutdownTimeout: "30s"

# Media storage
media:
  persistence:
    enabled: true
    # Use existing PVC
    existingClaim: ""
    # Storage class (leave empty for default)
    storageClass: ""
    # Access mode - use ReadWriteMany for multiple replicas
    accessMode: ReadWriteOnce
    size: 100Gi
    # Mount path inside container
    mountPath: /data/media

# Data directory (config, database for SQLite, cache)
data:
  persistence:
    enabled: true
    existingClaim: ""
    storageClass: ""
    accessMode: ReadWriteOnce
    size: 10Gi
    mountPath: /data

# Service configuration
service:
  type: ClusterIP
  port: 80
  annotations: {}

# Ingress configuration
ingress:
  enabled: false
  className: ""
  annotations: {}
    # kubernetes.io/ingress.class: nginx
    # cert-manager.io/cluster-issuer: letsencrypt-prod
  hosts:
    - host: viewra.local
      paths:
        - path: /
          pathType: Prefix
  tls: []
    # - secretName: viewra-tls
    #   hosts:
    #     - viewra.local

# Resource limits
resources:
  requests:
    cpu: 100m
    memory: 256Mi
  limits:
    cpu: 2000m
    memory: 2Gi

# Probes
probes:
  liveness:
    enabled: true
    path: /health/live
    initialDelaySeconds: 10
    periodSeconds: 10
    timeoutSeconds: 5
    failureThreshold: 3
  readiness:
    enabled: true
    path: /health/ready
    initialDelaySeconds: 5
    periodSeconds: 5
    timeoutSeconds: 3
    failureThreshold: 3
  startup:
    enabled: true
    path: /health/ready
    initialDelaySeconds: 0
    periodSeconds: 5
    timeoutSeconds: 3
    failureThreshold: 30  # 150s for migrations

# Node selector
nodeSelector: {}

# Tolerations
tolerations: []

# Affinity rules
affinity: {}

# Additional environment variables
extraEnv: []
  # - name: LOG_LEVEL
  #   value: debug

# Additional volume mounts
extraVolumeMounts: []

# Additional volumes
extraVolumes: []
```

### deployment.yaml (template)

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ include "viewra.fullname" . }}
  labels:
    {{- include "viewra.labels" . | nindent 4 }}
spec:
  replicas: {{ .Values.replicaCount }}
  {{- if gt (int .Values.replicaCount) 1 }}
  strategy:
    type: RollingUpdate
    rollingUpdate:
      maxUnavailable: 1
      maxSurge: 1
  {{- end }}
  selector:
    matchLabels:
      {{- include "viewra.selectorLabels" . | nindent 6 }}
  template:
    metadata:
      labels:
        {{- include "viewra.selectorLabels" . | nindent 8 }}
      annotations:
        checksum/config: {{ include (print $.Template.BasePath "/configmap.yaml") . | sha256sum }}
    spec:
      {{- with .Values.imagePullSecrets }}
      imagePullSecrets:
        {{- toYaml . | nindent 8 }}
      {{- end }}
      serviceAccountName: {{ include "viewra.serviceAccountName" . }}
      terminationGracePeriodSeconds: {{ trimSuffix "s" .Values.server.shutdownTimeout | int | add 5 }}
      containers:
        - name: {{ .Chart.Name }}
          image: "{{ .Values.image.repository }}:{{ .Values.image.tag | default .Chart.AppVersion }}"
          imagePullPolicy: {{ .Values.image.pullPolicy }}
          ports:
            - name: http
              containerPort: {{ .Values.server.port }}
              protocol: TCP
          env:
            - name: PORT
              value: {{ .Values.server.port | quote }}
            - name: DB_DRIVER
              value: {{ .Values.database.type | quote }}
            {{- if eq .Values.database.type "postgres" }}
            {{- if .Values.database.external.enabled }}
            - name: DB_HOST
              value: {{ .Values.database.external.host | quote }}
            - name: DB_PORT
              value: {{ .Values.database.external.port | quote }}
            - name: DB_USER
              value: {{ .Values.database.external.user | quote }}
            - name: DB_NAME
              value: {{ .Values.database.external.database | quote }}
            - name: DB_SSL_MODE
              value: {{ .Values.database.external.sslMode | quote }}
            - name: DB_PASSWORD
              valueFrom:
                secretKeyRef:
                  name: {{ .Values.database.external.existingSecret | default (printf "%s-db" (include "viewra.fullname" .)) }}
                  key: password
            {{- else if .Values.database.postgresql.enabled }}
            - name: DB_HOST
              value: {{ printf "%s-postgresql" (include "viewra.fullname" .) | quote }}
            - name: DB_PORT
              value: "5432"
            - name: DB_USER
              value: {{ .Values.database.postgresql.auth.username | quote }}
            - name: DB_NAME
              value: {{ .Values.database.postgresql.auth.database | quote }}
            - name: DB_PASSWORD
              valueFrom:
                secretKeyRef:
                  name: {{ .Values.database.postgresql.auth.existingSecret | default (printf "%s-postgresql" (include "viewra.fullname" .)) }}
                  key: password
            {{- end }}
            - name: DB_MAX_OPEN_CONNS
              value: {{ .Values.database.pool.maxOpenConns | quote }}
            - name: DB_MAX_IDLE_CONNS
              value: {{ .Values.database.pool.maxIdleConns | quote }}
            - name: DB_CONN_MAX_LIFETIME
              value: {{ .Values.database.pool.connMaxLifetime | quote }}
            {{- end }}
            - name: SHUTDOWN_TIMEOUT
              value: {{ .Values.server.shutdownTimeout | quote }}
            {{- with .Values.extraEnv }}
            {{- toYaml . | nindent 12 }}
            {{- end }}
          {{- if .Values.probes.liveness.enabled }}
          livenessProbe:
            httpGet:
              path: {{ .Values.probes.liveness.path }}
              port: http
            initialDelaySeconds: {{ .Values.probes.liveness.initialDelaySeconds }}
            periodSeconds: {{ .Values.probes.liveness.periodSeconds }}
            timeoutSeconds: {{ .Values.probes.liveness.timeoutSeconds }}
            failureThreshold: {{ .Values.probes.liveness.failureThreshold }}
          {{- end }}
          {{- if .Values.probes.readiness.enabled }}
          readinessProbe:
            httpGet:
              path: {{ .Values.probes.readiness.path }}
              port: http
            initialDelaySeconds: {{ .Values.probes.readiness.initialDelaySeconds }}
            periodSeconds: {{ .Values.probes.readiness.periodSeconds }}
            timeoutSeconds: {{ .Values.probes.readiness.timeoutSeconds }}
            failureThreshold: {{ .Values.probes.readiness.failureThreshold }}
          {{- end }}
          {{- if .Values.probes.startup.enabled }}
          startupProbe:
            httpGet:
              path: {{ .Values.probes.startup.path }}
              port: http
            initialDelaySeconds: {{ .Values.probes.startup.initialDelaySeconds }}
            periodSeconds: {{ .Values.probes.startup.periodSeconds }}
            timeoutSeconds: {{ .Values.probes.startup.timeoutSeconds }}
            failureThreshold: {{ .Values.probes.startup.failureThreshold }}
          {{- end }}
          resources:
            {{- toYaml .Values.resources | nindent 12 }}
          volumeMounts:
            - name: data
              mountPath: {{ .Values.data.persistence.mountPath }}
            {{- if .Values.media.persistence.enabled }}
            - name: media
              mountPath: {{ .Values.media.persistence.mountPath }}
            {{- end }}
            {{- with .Values.extraVolumeMounts }}
            {{- toYaml . | nindent 12 }}
            {{- end }}
      volumes:
        - name: data
          {{- if .Values.data.persistence.enabled }}
          persistentVolumeClaim:
            claimName: {{ .Values.data.persistence.existingClaim | default (printf "%s-data" (include "viewra.fullname" .)) }}
          {{- else }}
          emptyDir: {}
          {{- end }}
        {{- if .Values.media.persistence.enabled }}
        - name: media
          persistentVolumeClaim:
            claimName: {{ .Values.media.persistence.existingClaim | default (printf "%s-media" (include "viewra.fullname" .)) }}
        {{- end }}
        {{- with .Values.extraVolumes }}
        {{- toYaml . | nindent 8 }}
        {{- end }}
      {{- with .Values.nodeSelector }}
      nodeSelector:
        {{- toYaml . | nindent 8 }}
      {{- end }}
      {{- with .Values.affinity }}
      affinity:
        {{- toYaml . | nindent 8 }}
      {{- end }}
      {{- with .Values.tolerations }}
      tolerations:
        {{- toYaml . | nindent 8 }}
      {{- end }}
```

### Kustomize Base

```
deploy/kustomize/
├── base/
│   ├── kustomization.yaml
│   ├── deployment.yaml
│   ├── service.yaml
│   ├── configmap.yaml
│   └── pvc.yaml
└── overlays/
    ├── development/
    │   ├── kustomization.yaml
    │   └── patches/
    └── production/
        ├── kustomization.yaml
        ├── patches/
        └── secrets/
```

---

## Implementation Phases

### Phase 1: Config File & Database Info Display (~3 hours)
- [ ] Create `internal/app/config/file.go` - YAML config loading
- [ ] Modify `internal/app/config/config.go` - Integrate config file
- [ ] Modify `internal/infrastructure/database/connection.go` - Pool settings from config
- [ ] Modify `internal/api/handlers/settings.go` - Add database info to system info
- [ ] Create `web/src/components/DatabaseInfoCard.tsx`
- [ ] Modify `web/src/views/settings/SystemSettings/SystemSettings.tsx`

### Phase 2: Health Check Enhancements (~2 hours)
- [ ] Create `internal/api/handlers/probes.go` - Liveness/readiness handlers
- [ ] Modify `internal/api/routes/routes.go` - Add probe routes
- [ ] Modify `internal/api/handlers/health.go` - Enhance full health response
- [ ] Verify graceful shutdown in `cmd/viewra/main.go`

### Phase 3: Connection Pool Configuration (~1 hour)
- [ ] Add pool settings to config schema
- [ ] Expose via environment variables
- [ ] Add `auto_migrate` toggle
- [ ] Update `internal/infrastructure/database/connection.go`

### Phase 4: Environment Detection & Warnings (~2 hours)
- [ ] Create `internal/application/system/environment.go` - Detection logic
- [ ] Create `internal/application/system/instance_lock.go` - Advisory lock
- [ ] Add warnings to system info response
- [ ] Create `web/src/components/DatabaseWarningBanner.tsx`
- [ ] Integrate warning banner in App.tsx

### Phase 5: Maintenance Mode (~2 hours)
- [ ] Create `internal/application/system/maintenance.go` - State management
- [ ] Create `internal/api/middleware/maintenance.go` - Request blocking
- [ ] Add SSE event for maintenance mode changes
- [ ] Create `web/src/components/MaintenanceBanner.tsx`
- [ ] Add maintenance endpoints to admin routes

### Phase 6: Migration Service (~6 hours)
- [ ] Create `internal/application/system/migration/service.go` - Orchestration
- [ ] Create `internal/application/system/migration/transfer.go` - Table copy logic
- [ ] Create `internal/application/system/migration/verify.go` - Integrity checks
- [ ] Create `internal/application/system/migration/estimate.go` - Time estimation
- [ ] Create `internal/application/system/migration/progress.go` - Progress tracking
- [ ] Add table dependency ordering
- [ ] Implement media file path verification
- [ ] Implement plugin table migration

### Phase 7: Migration API & UI (~4 hours)
- [ ] Add migration endpoints to `internal/api/handlers/system.go`
- [ ] Add routes to `internal/api/routes/admin.go`
- [ ] Create `web/src/views/settings/DatabaseMigration/Wizard.tsx`
- [ ] Create `web/src/views/settings/DatabaseMigration/ConnectionForm.tsx`
- [ ] Create `web/src/views/settings/DatabaseMigration/Progress.tsx`
- [ ] Integrate wizard in System Settings page

### Phase 8: First-Run Setup Enhancement (~2 hours)
- [ ] Modify `internal/api/handlers/auth.go` - Add DB choice to setup
- [ ] Modify `web/src/routes/setup.tsx` - Add database selection step
- [ ] Add connection test to setup flow

### Phase 9: Rollback & Cleanup (~2 hours)
- [ ] Create `internal/application/system/migration/rollback.go`
- [ ] Create `internal/application/system/migration/cleanup.go`
- [ ] Add rollback endpoint
- [ ] Add scheduled task for old database cleanup
- [ ] Add "Delete old database" UI option

### Phase 10: Kubernetes Deployment (~3 hours)
- [ ] Create `deploy/helm/viewra/Chart.yaml`
- [ ] Create `deploy/helm/viewra/values.yaml`
- [ ] Create Helm templates (deployment, service, configmap, etc.)
- [ ] Create `deploy/helm/viewra/README.md`
- [ ] Create Kustomize base and overlays
- [ ] Test deployment in local K8s (minikube/kind)

### Phase 11: Documentation (~2 hours)
- [ ] Create `docs/guides/DATABASE_MIGRATION.md`
- [ ] Create `docs/guides/KUBERNETES_DEPLOYMENT.md`
- [ ] Update main README with database options
- [ ] Add examples for common deployment scenarios

---

## Testing Plan

### Unit Tests
- Config file loading with various scenarios
- Migration service table ordering
- Verification logic
- Environment detection

### Integration Tests
- SQLite → PostgreSQL migration
- PostgreSQL → SQLite migration
- Rollback after completed migration
- Connection pool behavior under load
- Maintenance mode request blocking

### End-to-End Tests
- Full migration wizard flow
- First-run setup with PostgreSQL
- Kubernetes deployment with Helm
- Multi-replica deployment

### Manual Testing Checklist
- [ ] Fresh install defaults to SQLite
- [ ] Database info displays correctly in UI
- [ ] SQLite warning appears in container
- [ ] Migration wizard completes successfully
- [ ] Progress updates in real-time
- [ ] Cancellation works mid-migration
- [ ] Rollback restores previous database
- [ ] Helm chart deploys to Kubernetes
- [ ] Multiple replicas work with PostgreSQL
- [ ] Graceful shutdown completes cleanly

---

## Rollout Plan

1. **Alpha**: Internal testing with both database types
2. **Beta**: Limited release, gather feedback on migration UX
3. **GA**: Full release with documentation and Helm chart

---

## Future Considerations

- **Automatic backups**: Scheduled backups to S3/cloud storage
- **Read replicas**: Support for PostgreSQL read replicas
- **Connection proxy**: Built-in PgBouncer for high-connection scenarios
- **Database metrics**: Prometheus metrics for connection pool, query latency
- **Migration from other media servers**: Import from Jellyfin, Plex databases
