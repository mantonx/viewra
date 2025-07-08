# Database Documentation

## Overview

Viewra uses SQLite for development and supports PostgreSQL for production deployments. The database stores media metadata, user sessions, transcoding information, and plugin data.

## Database Configuration

### Development (SQLite)
```yaml
database:
  type: sqlite
  database_path: ./viewra-data/viewra.db
```

### Production (PostgreSQL)
```yaml
database:
  type: postgres
  host: localhost
  port: 5432
  user: viewra
  password: secret
  database: viewra
```

## Core Tables

### Media Module Tables

#### media_libraries
Stores media library configurations.
```sql
CREATE TABLE media_libraries (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    path TEXT NOT NULL UNIQUE,
    type TEXT NOT NULL, -- movie, tv, music
    scan_interval INTEGER DEFAULT 3600,
    last_scan TIMESTAMP,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
);
```

#### media_files
Tracks all media files in libraries.
```sql
CREATE TABLE media_files (
    id TEXT PRIMARY KEY,
    library_id TEXT REFERENCES media_libraries(id),
    path TEXT NOT NULL,
    filename TEXT NOT NULL,
    size INTEGER,
    mime_type TEXT,
    container TEXT,
    duration INTEGER, -- seconds
    width INTEGER,
    height INTEGER,
    video_codec TEXT,
    audio_codec TEXT,
    created_at TIMESTAMP,
    updated_at TIMESTAMP,
    UNIQUE(library_id, path)
);
```

#### movies
Movie-specific metadata.
```sql
CREATE TABLE movies (
    id TEXT PRIMARY KEY,
    media_file_id TEXT REFERENCES media_files(id),
    title TEXT NOT NULL,
    original_title TEXT,
    overview TEXT,
    release_date DATE,
    runtime INTEGER,
    genres TEXT, -- JSON array
    director TEXT,
    cast TEXT, -- JSON array
    rating REAL,
    poster_path TEXT,
    backdrop_path TEXT,
    tmdb_id INTEGER,
    imdb_id TEXT
);
```

#### tv_shows, seasons, episodes
TV show hierarchy tables with similar metadata structure.

#### albums, tracks
Music metadata tables.

### Playback Module Tables

#### playback_sessions
Active and historical playback sessions.
```sql
CREATE TABLE playback_sessions (
    id TEXT PRIMARY KEY,
    media_file_id TEXT REFERENCES media_files(id),
    user_id TEXT,
    device_id TEXT,
    method TEXT, -- direct, transcode
    state TEXT, -- playing, paused, stopped
    position REAL, -- seconds
    duration REAL,
    start_time TIMESTAMP,
    last_activity TIMESTAMP,
    end_time TIMESTAMP
);
```

#### user_media_progress
Tracks viewing progress per user.
```sql
CREATE TABLE user_media_progress (
    user_id TEXT,
    media_file_id TEXT REFERENCES media_files(id),
    position REAL,
    completed BOOLEAN DEFAULT FALSE,
    updated_at TIMESTAMP,
    PRIMARY KEY (user_id, media_file_id)
);
```

#### playback_history
Historical playback records.
```sql
CREATE TABLE playback_history (
    id TEXT PRIMARY KEY,
    user_id TEXT,
    media_file_id TEXT,
    played_at TIMESTAMP,
    duration_watched INTEGER,
    completed BOOLEAN
);
```

### Transcoding Module Tables

#### transcode_sessions
Transcoding session tracking.
```sql
CREATE TABLE transcode_sessions (
    id TEXT PRIMARY KEY,
    media_file_id TEXT REFERENCES media_files(id),
    status TEXT, -- pending, running, completed, failed
    provider TEXT,
    content_hash TEXT,
    input_path TEXT,
    output_path TEXT,
    container TEXT,
    video_codec TEXT,
    audio_codec TEXT,
    quality INTEGER,
    progress REAL,
    error_message TEXT,
    created_at TIMESTAMP,
    started_at TIMESTAMP,
    completed_at TIMESTAMP
);
```

#### transcode_cache
Caches transcoded content references.
```sql
CREATE TABLE transcode_cache (
    id TEXT PRIMARY KEY,
    media_file_id TEXT,
    content_hash TEXT UNIQUE,
    container TEXT,
    video_codec TEXT,
    audio_codec TEXT,
    quality INTEGER,
    file_size INTEGER,
    created_at TIMESTAMP,
    last_accessed TIMESTAMP,
    access_count INTEGER DEFAULT 0
);
```

### Plugin Module Tables

#### plugins
Registered plugins and their status.
```sql
CREATE TABLE plugins (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    type TEXT NOT NULL, -- transcoder, enrichment, scanner
    version TEXT,
    enabled BOOLEAN DEFAULT TRUE,
    config TEXT, -- JSON configuration
    capabilities TEXT, -- JSON capabilities
    last_health_check TIMESTAMP,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
);
```

#### plugin_metadata
Metadata extracted by plugins.
```sql
CREATE TABLE plugin_metadata (
    id TEXT PRIMARY KEY,
    plugin_id TEXT REFERENCES plugins(id),
    media_file_id TEXT REFERENCES media_files(id),
    metadata_type TEXT,
    metadata TEXT, -- JSON data
    confidence REAL,
    created_at TIMESTAMP
);
```

## Indexes

Key indexes for performance:

```sql
-- Media queries
CREATE INDEX idx_media_files_library ON media_files(library_id);
CREATE INDEX idx_media_files_type ON media_files(mime_type);

-- Playback queries
CREATE INDEX idx_sessions_user_time ON playback_sessions(user_id, start_time DESC);
CREATE INDEX idx_sessions_active ON playback_sessions(state) WHERE state IN ('playing', 'paused');

-- History queries
CREATE INDEX idx_history_user_recent ON playback_history(user_id, played_at DESC);

-- Transcoding queries
CREATE INDEX idx_transcode_media ON transcode_sessions(media_file_id);
CREATE INDEX idx_transcode_status ON transcode_sessions(status);
CREATE INDEX idx_cache_hash ON transcode_cache(content_hash);
```

## Migrations

Database migrations are handled automatically by each module during startup:

```go
// Each module implements Migrate()
func (m *Module) Migrate(db *gorm.DB) error {
    return db.AutoMigrate(
        &MediaLibrary{},
        &MediaFile{},
        // ... other models
    )
}
```

## Best Practices

### 1. Use Transactions
```go
err := db.Transaction(func(tx *gorm.DB) error {
    // Multiple operations
    return nil
})
```

### 2. Avoid N+1 Queries
```go
// Preload related data
db.Preload("Movies").Find(&mediaFiles)
```

### 3. Use Appropriate Indexes
Create indexes for frequently queried columns, especially foreign keys and filters.

### 4. Regular Maintenance
- Vacuum SQLite database periodically
- Monitor query performance
- Archive old playback history

## Database Tools

### SQLite Web Interface
```bash
make db-web
# Visit http://localhost:8081
```

### Direct Access
```bash
# SQLite
sqlite3 ./viewra-data/viewra.db

# PostgreSQL
psql -h localhost -U viewra -d viewra
```

### Backup
```bash
# SQLite backup
sqlite3 viewra.db ".backup viewra-backup.db"

# PostgreSQL backup
pg_dump -h localhost -U viewra viewra > backup.sql
```