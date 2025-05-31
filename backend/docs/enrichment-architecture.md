# Enrichment Architecture

This document describes the enrichment system architecture that follows the priority and merge strategy for metadata enrichment.

## Overview

The enrichment system is designed with two levels of plugins:

1. **Core/Internal Plugins** - Live within the main application, use direct Go interfaces
2. **External Plugins** - Separate processes, communicate via gRPC

## Architecture Components

### Core Enrichment Module

- **Location**: `backend/internal/modules/enrichmentmodule/`
- **Purpose**: Centralized enrichment application with priority-based merging
- **Features**:
  - Priority-based source management
  - Field-specific merge strategies
  - Background application worker
  - HTTP API for management
  - gRPC API for external plugins

### Priority & Merge Strategy

| **Field**      | **Media Type** | **Source Priority**                           | **Merge Strategy**   |
| -------------- | -------------- | --------------------------------------------- | -------------------- |
| `Title`        | All            | TMDb > MusicBrainz > Filename > Embedded Tags | Replace if different |
| `Artist Name`  | Music          | MusicBrainz > Embedded Tag                    | Replace              |
| `Album Name`   | Music          | MusicBrainz > Embedded Tag                    | Replace              |
| `Release Year` | All            | TMDb > MusicBrainz > Filename                 | Replace              |
| `Genres`       | All            | TMDb > MusicBrainz > Embedded Tags            | Merge (union)        |
| `Duration`     | All            | Embedded > TMDb > MusicBrainz                 | Replace              |

## Internal Plugin Development

Internal plugins live within the main application and communicate directly with the enrichment module.

### Example: MusicBrainz Internal Plugin

```go
// Create plugin instance
plugin := NewMusicBrainzInternalPlugin(enrichmentModule)

// Initialize
if err := plugin.Initialize(); err != nil {
    log.Fatalf("Failed to initialize plugin: %v", err)
}

// Use in media file processing
if err := plugin.OnMediaFileScanned(mediaFile, metadata); err != nil {
    log.Printf("Error enriching file: %v", err)
}
```

### Internal Plugin Interface

```go
type InternalEnrichmentPlugin interface {
    GetName() string
    Initialize() error
    CanEnrich(*database.MediaFile) bool
    EnrichMediaFile(*database.MediaFile, map[string]string) error
    OnMediaFileScanned(*database.MediaFile, map[string]string) error
}
```

### Registering Enrichment Data (Internal)

```go
// Convert source data to enrichment format
enrichments := map[string]interface{}{
    "artist_name": "The Beatles",
    "album_name":  "Abbey Road",
    "release_year": "1969",
}

// Register with core module
err := enrichmentModule.RegisterEnrichmentData(
    mediaFileID,
    "musicbrainz",  // source name
    enrichments,
    0.95,          // confidence score 0.0-1.0
)
```

## External Plugin Development

External plugins are separate processes that communicate with the core system via gRPC.

### Setting Up gRPC

1. **Generate protobuf code**:

```bash
cd backend
chmod +x scripts/generate-proto.sh
./scripts/generate-proto.sh
```

2. **Start core enrichment service** (runs on port 50051)

### Example: External Plugin Client

```go
package main

import (
    "log"
    enrichmentpb "github.com/mantonx/viewra/api/proto/enrichment"
    "google.golang.org/grpc"
)

func main() {
    // Connect to enrichment service
    conn, err := grpc.Dial("localhost:50051", grpc.WithInsecure())
    if err != nil {
        log.Fatalf("Failed to connect: %v", err)
    }
    defer conn.Close()

    client := enrichmentpb.NewEnrichmentServiceClient(conn)

    // Register enrichment data
    req := &enrichmentpb.RegisterEnrichmentRequest{
        MediaFileId: "media-file-123",
        SourceName:  "my_external_plugin",
        Enrichments: map[string]string{
            "artist_name": "The Beatles",
            "album_name":  "Abbey Road",
        },
        ConfidenceScore: 0.90,
    }

    resp, err := client.RegisterEnrichment(context.Background(), req)
    if err != nil {
        log.Fatalf("Failed to register: %v", err)
    }

    log.Printf("Success: %s (Job: %s)", resp.Message, resp.JobId)
}
```

### External Plugin Helper Client

For easier external plugin development, use the provided client:

```go
import "github.com/mantonx/viewra/internal/plugins/enrichment"

// Create client
client := enrichment.NewEnrichmentClient("localhost:50051")

// Connect
if err := client.Connect(); err != nil {
    log.Fatalf("Failed to connect: %v", err)
}
defer client.Close()

// Register enrichment
enrichments := map[string]string{
    "artist_name": "The Beatles",
    "album_name":  "Abbey Road",
}

err := client.RegisterEnrichmentData("media-file-123", "my_plugin", enrichments, 0.95)
```

## API Endpoints

### HTTP API (Core Management)

- `GET /api/enrichment/status/:mediaFileId` - Get enrichment status
- `POST /api/enrichment/apply/:mediaFileId/:fieldName/:sourceName` - Force apply enrichment
- `GET /api/enrichment/sources` - List enrichment sources
- `PUT /api/enrichment/sources/:sourceName` - Update source configuration
- `GET /api/enrichment/jobs` - List enrichment jobs
- `POST /api/enrichment/jobs/:mediaFileId` - Trigger enrichment job

### gRPC API (External Plugins)

- `RegisterEnrichment` - Register enrichment data
- `GetEnrichmentStatus` - Get enrichment status
- `ListEnrichmentSources` - List enrichment sources
- `UpdateEnrichmentSource` - Update source configuration
- `TriggerEnrichmentJob` - Trigger enrichment application

## Integration Example

### Core Application Setup

```go
// Initialize enrichment module
enrichmentModule := enrichmentmodule.NewModule(db, eventBus)
if err := enrichmentModule.Start(); err != nil {
    log.Fatalf("Failed to start enrichment module: %v", err)
}

// Initialize internal plugins
mbPlugin := enrichment.NewMusicBrainzInternalPlugin(enrichmentModule)
if err := mbPlugin.Initialize(); err != nil {
    log.Fatalf("Failed to initialize MusicBrainz plugin: %v", err)
}

// Register HTTP routes
api := r.Group("/api")
enrichmentModule.RegisterRoutes(api)

// Plugin hooks for file scanning
func onFileScanned(mediaFile *database.MediaFile, metadata map[string]string) {
    // Internal plugins
    mbPlugin.OnMediaFileScanned(mediaFile, metadata)

    // External plugins will connect via gRPC automatically
}
```

## Configuration

### Source Priority Configuration

```json
{
  "sources": [
    { "name": "tmdb", "priority": 1, "enabled": true },
    { "name": "musicbrainz", "priority": 2, "enabled": true },
    { "name": "embedded", "priority": 4, "enabled": true }
  ]
}
```

### Field Rules

Field rules are defined in the enrichment module and can be customized:

```go
rules := map[string]FieldRule{
    "artist_name": {
        FieldName:     "artist_name",
        MediaTypes:    []string{"track"},
        SourcePriority: []string{"musicbrainz", "embedded"},
        MergeStrategy: MergeStrategyReplace,
        ValidateFunc:  func(value string) bool {
            return strings.TrimSpace(value) != "" && value != "Unknown Artist"
        },
    },
}
```

## Benefits

1. **Unified Priority System**: All enrichment sources follow the same priority rules
2. **Flexible Architecture**: Internal plugins for performance, external plugins for modularity
3. **Automatic Application**: Background worker applies enrichments based on priority
4. **Event-Driven**: Integration with the event system for real-time updates
5. **API Management**: Both HTTP and gRPC APIs for different use cases
6. **Conflict Resolution**: Automatic handling of conflicting enrichment data

## Migration

### From Old Plugin System

1. **Internal Plugins**: Update existing plugins to use `enrichmentModule.RegisterEnrichmentData()`
2. **External Plugins**: Migrate to use gRPC client instead of direct database access
3. **Database**: Enrichment data is now centralized in the enrichment module tables

### Example Migration

**Before (Direct Database)**:

```go
enrichment := MusicBrainzEnrichment{
    MediaFileID: mediaFileID,
    EnrichedTitle: recording.Title,
    // ... other fields
}
db.Create(&enrichment)
```

**After (Enrichment Module)**:

```go
enrichments := map[string]interface{}{
    "title": recording.Title,
    "artist_name": recording.Artist,
}
enrichmentModule.RegisterEnrichmentData(mediaFileID, "musicbrainz", enrichments, confidence)
```
