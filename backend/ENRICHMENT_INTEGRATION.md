# Enrichment System Integration

This document explains how the enrichment system integrates with Viewra's current metadata scanning to solve the "Unknown Artist" problem and ensure proper metadata application.

## Problem Analysis

### Current Flow Issues

1. **File Scanning**: Creates MediaFile records
2. **Core Plugins**: Extract basic metadata, default to "Unknown Artist"/"Unknown Album" when tags missing
3. **Enrichment Plugins**: Collect additional metadata but store in separate tables (`MediaEnrichment`, `MediaExternalIDs`)
4. **Missing Link**: Enriched data never gets applied back to Track/Album/Artist tables
5. **Result**: Users see "Unknown Artist" even when enrichment data exists

### Root Cause

The enrichment plugins (MusicBrainz, AudioDB, TMDb) successfully collect metadata but there was no system to:

- Apply enrichment data based on priority rules
- Replace "Unknown Artist" with enriched data
- Merge data from multiple sources intelligently

## Solution Architecture

### Core Components

1. **Enrichment Module** (`backend/internal/modules/enrichmentmodule/`)

   - Centralized enrichment application with priority-based merging
   - Background worker to apply enrichments automatically
   - Field-specific rules for metadata application

2. **Enhanced MediaEnrichment Table**

   - Stores structured enrichment data as JSON payloads
   - Includes source priority, confidence scores, and external IDs
   - Compatible with existing plugin system

3. **Dual Plugin Architecture**
   - **Internal Plugins**: Direct integration for performance (MusicBrainz internal)
   - **External Plugins**: gRPC communication for modularity
   - **Plugin Manager**: Coordinates internal plugin execution

### Priority & Application Rules

| **Field**      | **Source Priority**                           | **Merge Strategy**   |
| -------------- | --------------------------------------------- | -------------------- |
| `Title`        | TMDb > MusicBrainz > Filename > Embedded Tags | Replace if different |
| `Artist Name`  | MusicBrainz > Embedded Tag                    | Replace              |
| `Album Name`   | MusicBrainz > Embedded Tag                    | Replace              |
| `Release Year` | TMDb > MusicBrainz > Filename                 | Replace              |
| `Genres`       | TMDb > MusicBrainz > Embedded Tags            | Merge (union)        |
| `Duration`     | Embedded > TMDb > MusicBrainz                 | Replace              |

## Integration Flow

### 1. File Scanning (Current + Enhanced)

```
MediaFile Created → Core Plugins Extract Basic Metadata → Track/Album/Artist Created with defaults
                                                        ↓
                  Enrichment Module Hook → Queue Enrichment Job
```

### 2. Enrichment Collection (Current + Enhanced)

```
Plugin (MusicBrainz/AudioDB/TMDb) → Collect enriched metadata → Store in MediaEnrichment table
                                                              ↓
                                   Register with Enrichment Module → Queue Application Job
```

### 3. Enrichment Application (New)

```
Background Worker → Get pending jobs → Parse enrichment data → Apply priority rules → Update Track/Album/Artist
                                                                                   ↓
                                                                Store External IDs → Complete job
```

## Database Schema

### Enhanced MediaEnrichment

Uses existing table with structured JSON payloads:

```json
{
  "source": "musicbrainz",
  "source_priority": 2,
  "confidence_score": 0.95,
  "updated_at": "2024-01-01T00:00:00Z",
  "fields": {
    "title": "Abbey Road",
    "artist_name": "The Beatles",
    "album_name": "Abbey Road",
    "release_year": "1969",
    "genres": "Rock,Progressive Rock"
  },
  "external_ids": {
    "musicbrainz_recording_id": "12345",
    "musicbrainz_artist_id": "67890",
    "musicbrainz_release_id": "abcdef"
  }
}
```

### Enhanced MediaExternalIDs

Uses existing table to store external identifiers separately for cross-reference.

### New EnrichmentSource

Manages source configuration and priorities:

```sql
CREATE TABLE enrichment_sources (
  id INTEGER PRIMARY KEY,
  name VARCHAR NOT NULL,
  priority INTEGER NOT NULL,
  media_types TEXT NOT NULL,  -- JSON array
  enabled BOOLEAN DEFAULT true,
  last_sync TIMESTAMP,
  created_at TIMESTAMP,
  updated_at TIMESTAMP
);
```

### New EnrichmentJob

Tracks background application jobs:

```sql
CREATE TABLE enrichment_jobs (
  id INTEGER PRIMARY KEY,
  media_file_id VARCHAR NOT NULL,
  job_type VARCHAR NOT NULL,
  status VARCHAR DEFAULT 'pending',
  results TEXT,  -- JSON
  error TEXT,
  created_at TIMESTAMP,
  updated_at TIMESTAMP
);
```

## API Integration

### HTTP API

- `GET /api/enrichment/status/:mediaFileId` - Get enrichment status
- `POST /api/enrichment/apply/:mediaFileId/:fieldName/:sourceName` - Force apply
- `GET /api/enrichment/sources` - List sources
- `PUT /api/enrichment/sources/:sourceName` - Update source config
- `GET /api/enrichment/jobs` - List jobs
- `POST /api/enrichment/jobs/:mediaFileId` - Trigger job

### gRPC API (External Plugins)

- `RegisterEnrichment` - Register enrichment data
- `GetEnrichmentStatus` - Get status
- `TriggerEnrichmentJob` - Trigger application

## Solving "Unknown Artist" Problem

### Before

1. File scanned → Basic metadata extracted → "Unknown Artist" created
2. MusicBrainz plugin runs → Finds correct artist → Stores in separate table
3. **Data never applied** → User still sees "Unknown Artist"

### After

1. File scanned → Basic metadata extracted → "Unknown Artist" created
2. MusicBrainz plugin runs → Finds correct artist → Registers with enrichment module
3. **Background worker applies enrichment** → Artist.name updated to "The Beatles"
4. User sees correct artist name

### Example Application

```go
// Original Track/Album/Artist
Track: {Title: "Track01.mp3", Artist: "Unknown Artist", Album: "Unknown Album"}

// MusicBrainz enrichment registered
enrichments := map[string]interface{}{
    "title": "Come Together",
    "artist_name": "The Beatles",
    "album_name": "Abbey Road",
    "release_year": "1969"
}

// After enrichment applied
Track: {Title: "Come Together", Artist: "The Beatles", Album: "Abbey Road"}
Album: {Title: "Abbey Road", ReleaseDate: 1969-01-01}
Artist: {Name: "The Beatles"}
```

## Event-Driven Updates

The system publishes events for real-time UI updates:

- `enrichment.data_registered` - New enrichment data available
- `enrichment.applied` - Enrichment applied to entity
- `enrichment.job_completed` - Background job finished

## Migration from Current System

### Phase 1: Enhanced Data Storage

- ✅ Enhanced MediaEnrichment table usage
- ✅ Enrichment module infrastructure
- ✅ Background worker system

### Phase 2: Plugin Integration

- ✅ Internal plugin architecture (MusicBrainz internal)
- ✅ Plugin manager for coordination
- 🔄 Migrate existing external plugins to use enrichment module

### Phase 3: gRPC External Plugins

- 🔄 Generate protobuf code
- 🔄 Enable gRPC server
- 🔄 Update external plugins to use gRPC

## Configuration

### Source Priorities

```json
{
  "sources": [
    { "name": "tmdb", "priority": 1, "enabled": true },
    { "name": "musicbrainz", "priority": 2, "enabled": true },
    { "name": "embedded", "priority": 4, "enabled": true }
  ]
}
```

### Worker Configuration

```json
{
  "worker": {
    "enabled": true,
    "interval": "30s",
    "batch_size": 10
  }
}
```

## Benefits

1. **Fixes "Unknown Artist"**: Automatically applies enriched metadata to replace defaults
2. **Priority-Based**: TMDb beats MusicBrainz beats embedded tags for movies/TV
3. **Confidence Scoring**: Higher confidence data preferred
4. **Dual Architecture**: Performance (internal) + modularity (external)
5. **Event-Driven**: Real-time UI updates when enrichments applied
6. **Backward Compatible**: Works with existing plugin system
7. **Field-Specific Rules**: Different merge strategies per field type

## Monitoring & Debugging

### Check Enrichment Status

```bash
curl http://localhost:8080/api/enrichment/status/media-file-123
```

### View Jobs Queue

```bash
curl http://localhost:8080/api/enrichment/jobs?status=pending
```

### Force Apply Enrichment

```bash
curl -X POST http://localhost:8080/api/enrichment/apply/media-file-123/artist_name/musicbrainz
```

### Enable Debug Logging

```go
// Check if enrichment data exists but wasn't applied
status, err := enrichmentModule.GetEnrichmentStatus(mediaFileID)

// Trigger manual application
jobID, err := enrichmentModule.TriggerEnrichmentJob(mediaFileID)
```

This integration ensures that enrichment data collected by plugins is automatically applied to fix metadata display issues while maintaining compatibility with the existing system.
