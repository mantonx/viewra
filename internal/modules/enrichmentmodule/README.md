# Enrichment Module

The enrichment module provides centralized metadata enrichment with priority-based merging for Viewra's media library.

## Quick Start

```go
// Initialize enrichment module
enrichmentModule := enrichmentmodule.NewModule(db, eventBus)
if err := enrichmentModule.Start(); err != nil {
    log.Fatalf("Failed to start enrichment module: %v", err)
}

// Register enrichment data from any source
enrichments := map[string]interface{}{
    "artist_name": "The Beatles",
    "album_name":  "Abbey Road",
    "release_year": "1969",
}

err := enrichmentModule.RegisterEnrichmentData(
    mediaFileID,     // Media file to enrich
    "musicbrainz",   // Source name
    enrichments,     // Data to register
    0.95,           // Confidence score (0.0-1.0)
)
```

## Architecture

### Core Components

- **Module** (`module.go`) - Main enrichment orchestrator
- **Models** (`models.go`) - Database models for enrichment data
- **Worker** (`worker.go`) - Background application worker
- **Field Rules** (`field_rules.go`) - Priority and merge logic
- **HTTP Handlers** (`handlers.go`) - REST API endpoints
- **gRPC Server** (`grpc_server.go`) - gRPC API for external plugins

### Priority System

Sources are prioritized by numeric value (lower = higher priority):

1. TMDb (1)
2. MusicBrainz (2)
3. Filename parsing (3)
4. Embedded tags (4)

### Field Rules

Each field has specific merge strategies:

- **Replace**: Use highest priority source (Title, Artist, Album, Year)
- **Merge**: Combine values from multiple sources (Genres)
- **User Override**: Skip if user has manually set value

## Internal Plugins

Internal plugins run within the main process for better performance:

```go
// Create internal MusicBrainz plugin (no database dependency - uses centralized system)
plugin := enrichment.NewMusicBrainzInternalPlugin(enrichmentModule)

// Use in file scanning
manager.OnMediaFileScanned(mediaFile, metadata)
```

## External Plugins

External plugins communicate via gRPC for modularity:

```go
// External plugin client
client := enrichment.NewEnrichmentClient("localhost:50051")
if err := client.Connect(); err != nil {
    log.Fatalf("Failed to connect: %v", err)
}

// Register enrichment data
err := client.RegisterEnrichmentData(
    mediaFileID, "my_plugin", enrichments, 0.90,
)
```

## HTTP API

- `GET /api/enrichment/status/:mediaFileId` - Get enrichment status
- `POST /api/enrichment/apply/:mediaFileId/:fieldName/:sourceName` - Force apply
- `GET /api/enrichment/sources` - List sources
- `PUT /api/enrichment/sources/:sourceName` - Update source config
- `GET /api/enrichment/jobs` - List jobs
- `POST /api/enrichment/jobs/:mediaFileId` - Trigger job

## gRPC API

- `RegisterEnrichment` - Register enrichment data
- `GetEnrichmentStatus` - Get status
- `ListEnrichmentSources` - List sources
- `UpdateEnrichmentSource` - Update source
- `TriggerEnrichmentJob` - Trigger job

## Database Schema

### EnrichmentSource

- Source name and priority configuration
- Media type restrictions
- Enable/disable flags

### FieldEnrichment

- Individual field enrichments from sources
- Confidence scores and metadata
- Application tracking

### EnrichmentJob

- Background application jobs
- Progress tracking and error handling

## Configuration

```json
{
  "sources": [
    { "name": "tmdb", "priority": 1, "enabled": true },
    { "name": "musicbrainz", "priority": 2, "enabled": true }
  ],
  "worker": {
    "enabled": true,
    "interval": "10s",
    "batch_size": 50
  },
  "grpc": {
    "enabled": true,
    "port": 50051
  }
}
```

## Events

The module integrates with the event system:

- `enrichment.data_registered` - New enrichment data available
- `enrichment.applied` - Enrichment applied to entity
- `enrichment.job_completed` - Background job finished

## Development

### Adding New Sources

1. Create enrichment plugin (internal or external)
2. Register with appropriate priority
3. Follow field naming conventions
4. Provide confidence scores

### Custom Field Rules

```go
rules := map[string]FieldRule{
    "custom_field": {
        FieldName:     "custom_field",
        MediaTypes:    []string{"track", "album"},
        SourcePriority: []string{"source1", "source2"},
        MergeStrategy: MergeStrategyReplace,
        ValidateFunc:  func(value string) bool {
            return strings.TrimSpace(value) != ""
        },
    },
}
```

### Testing

```bash
# Run tests
go test ./internal/modules/enrichmentmodule/...

# Generate protobuf (for gRPC development)
./scripts/generate-proto.sh

# Check for issues
go vet ./internal/modules/enrichmentmodule/...
```

## Troubleshooting

### Common Issues

1. **"Unknown Artist" still showing**: Check if enrichment data is registered and background worker is running
2. **Plugin not working**: Verify plugin registration and `CanEnrich()` method
3. **gRPC connection failed**: Ensure gRPC server is started and port is correct
4. **Low confidence data ignored**: Check confidence thresholds in field rules

### Debugging

```go
// Enable debug logging
log.SetLevel(log.DebugLevel)

// Check enrichment status
status, err := enrichmentModule.GetEnrichmentStatus(mediaFileID)

// List registered sources
sources := enrichmentModule.ListEnrichmentSources()

// View job queue
jobs := enrichmentModule.GetEnrichmentJobs(EnrichmentJobStatusPending)
```



### Usage Example

```go
// The MusicBrainz functionality is now handled entirely by the external plugin
// located in data/plugins/musicbrainz_enricher/
//
// No internal plugin setup needed - the external plugin system handles
// all MusicBrainz enrichment through gRPC communication

// Example of manual enrichment (if needed):
enrichmentModule := enrichmentmodule.New(db)
enrichmentModule.Start()

// Register enrichment data from any source
mediaFileID := uint32(123)
pluginName := "external_plugin_example"
enrichments := map[string]interface{}{
    "title": "Enriched Title",
    "artist": "Enriched Artist",
}
confidence := 0.9

err := enrichmentModule.RegisterEnrichmentData(mediaFileID, pluginName, enrichments, confidence)
```
