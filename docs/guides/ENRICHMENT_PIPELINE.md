# Enrichment Pipeline Guide

This guide explains how ViewRA's enrichment pipeline works, how media flows through enrichment stages, and how to configure the pipeline for your needs.

## Overview

After the library scanner discovers media files, the **enrichment pipeline** fetches metadata from various sources:

```text
Scanner discovers file
        │
        ▼
┌───────────────────────────────────────────────────┐
│              Enrichment Pipeline                  │
│                                                   │
│   ┌─────────┐   ┌─────────┐   ┌─────────────┐    │
│   │   NFO   │ → │  Local  │ → │    TMDb     │    │
│   │ Parser  │   │ Images  │   │   Lookup    │    │
│   └─────────┘   └─────────┘   └─────────────┘    │
│                                                   │
└───────────────────────────────────────────────────┘
        │
        ▼
  Metadata stored
```

Each **stage** is handled by an **enricher** (plugin) that can:
- Read local files (NFO, images)
- Query external APIs (TMDb, MusicBrainz)
- Discover external IDs for downstream stages

## Architecture Components

### Pipeline Manager

The Pipeline Manager coordinates all enrichment:

```go
type Manager struct {
    workerPools map[string]*WorkerPool  // One pool per stage
    enrichers   map[string]Enricher     // Registered enrichers
}
```

Key responsibilities:
- Registers enrichers at startup
- Creates worker pools with appropriate concurrency
- Coordinates job flow between stages

### Worker Pools

Each enricher stage has its own worker pool:

| Enricher Type | Concurrency | Rate Limit |
|--------------|-------------|------------|
| NFO Parser | High (CPU × 2) | None |
| Local Images | High (CPU × 2) | None |
| TMDb | Low (3-5) | 40/min |
| Fanart.tv | Very Low (1-2) | 10/min |

```go
type WorkerPool struct {
    enricher  Enricher
    config    StageWorkerConfig
    limiter   *rate.Limiter      // Optional rate limiting
}
```

### Enrichment Queue

Jobs are persisted in `enrichment_queue` for reliability:

```sql
CREATE TABLE enrichment_queue (
    id INTEGER PRIMARY KEY,
    media_id INTEGER NOT NULL,
    library_id INTEGER NOT NULL,
    media_type TEXT NOT NULL,      -- movie, tv, tv_show, music
    stage TEXT NOT NULL,           -- nfo, local_images, tmdb
    priority INTEGER DEFAULT 0,
    status TEXT DEFAULT 'pending', -- pending, processing, completed, failed
    attempts INTEGER DEFAULT 0,
    error TEXT,
    locked_by TEXT,
    locked_at TIMESTAMP,
    next_retry_at TIMESTAMP,
    UNIQUE(media_id, media_type, stage)
);
```

Benefits:
- **Persistence**: Jobs survive server restarts
- **Retry logic**: Failed jobs retry with exponential backoff
- **Progress visibility**: Users see real-time progress
- **Non-blocking scanner**: Scanner enqueues and moves on

## How Jobs Flow

### 1. Scanner Enqueues First Stage

When the scanner discovers/updates media:

```go
func (uc *ScanUseCase) processMovie(ctx context.Context, movie *domain.Movie) error {
    // ... save movie to database ...

    // Single line: enqueue for enrichment
    uc.pipeline.EnqueueFirstStage(ctx, movie.ID, movie.LibraryID, enrichment.MediaTypeMovie)
    return nil
}
```

### 2. Worker Claims Job

Workers poll for pending jobs in their stage:

```go
func (p *WorkerPool) worker(ctx context.Context, workerID int) {
    for {
        // Claim batch of jobs (atomic lock)
        jobs, err := p.deps.QueueRepo.ClaimBatch(ctx, p.config.Stage, workerName, batchSize)

        // Process each job
        for _, job := range jobs {
            p.jobProcessor.Process(ctx, job)
        }
    }
}
```

### 3. Enricher Processes Job

The JobProcessor:
1. Builds `EnrichRequest` from job
2. Calls enricher's `Enrich()` method
3. Applies response (metadata, images, IDs)
4. Enqueues next stage

```go
func (p *JobProcessor) Process(ctx context.Context, job *QueueJob) {
    // Build request with existing IDs
    req, err := p.requestBuilder.Build(ctx, job)

    // Call enricher
    resp, err := p.enricher.Enrich(ctx, req)

    // Apply results to database
    err = p.responseApplier.Apply(ctx, job, resp)

    // Enqueue next stage
    p.enqueueNext(ctx, job.MediaID, job.LibraryID, mediaType, currentPosition)
}
```

### 4. Stage Completion → Next Stage

When a stage completes, the next enabled stage is enqueued:

```text
NFO (position=1) completes
    → Query GetNextStage(movie, position=1)
    → Returns Local Images (position=2)
    → Enqueue local_images job

Local Images completes
    → Query GetNextStage(movie, position=2)
    → Returns TMDb (position=3)
    → Enqueue tmdb job

TMDb completes
    → Query GetNextStage(movie, position=3)
    → Returns nil (no more stages)
    → Enrichment complete
```

## Pipeline Configuration

### Default Pipelines

ViewRA ships with pre-configured pipelines per media type:

**Movies Pipeline:**
```
Position 1: NFO Parser
Position 2: Local Images
Position 3: TMDb
```

**TV Shows Pipeline:**
```
Position 1: NFO Parser
Position 2: Local Images
Position 3: TMDb
```

**Music Pipeline:**
```
Position 1: NFO Parser
Position 2: Local Images
Position 3: MusicBrainz
```

### Database Schema

Pipeline configuration is stored in `enrichment_pipelines`:

```sql
CREATE TABLE enrichment_pipelines (
    id INTEGER PRIMARY KEY,
    media_type TEXT NOT NULL,       -- movie, tv, tv_show, music
    plugin_id TEXT NOT NULL,        -- nfo, local_images, tmdb
    stage_name TEXT NOT NULL,
    position INTEGER NOT NULL,
    enabled INTEGER DEFAULT 1,
    config_json TEXT,
    UNIQUE(media_type, position),
    UNIQUE(media_type, plugin_id)
);
```

### Enabling/Disabling Stages

Users can toggle stages in Settings → Enrichment Pipeline:

```go
// Disable a stage
pipeline.Disable(ctx, stageID)

// Enable a stage
pipeline.Enable(ctx, stageID)
```

Disabled stages are skipped during flow.

### Reordering Stages

Stage order affects which enricher provides metadata first:

```go
// Move TMDb before Local Images
pipeline.Update(ctx, &PipelineStage{
    ID:        3,
    MediaType: "movie",
    PluginID:  "tmdb",
    Position:  2,  // Was 3
})
```

## ID Propagation

External IDs discovered by early stages are passed to later stages:

```text
1. NFO Parser finds: imdb=tt0133093
   → Saves to media_external_ids table

2. TMDb receives EnrichRequest with:
   existing_ids: {"imdb": "tt0133093"}

3. TMDb uses IMDB ID for direct lookup (no search needed)
   → Returns: discovered_ids: {"tmdb": "603"}
```

This enables:
- Precise matching without title ambiguity
- Faster lookups via direct ID queries
- Cascading enrichment across providers

## Job States

| Status | Description |
|--------|-------------|
| `pending` | Waiting to be processed |
| `processing` | Currently being worked on (locked) |
| `completed` | Successfully finished |
| `failed` | Failed after all retries |
| `skipped` | Intentionally skipped (e.g., no NFO file) |

### State Transitions

```text
           ┌───────────────────────────────────────┐
           │                                       │
           ▼                                       │
       pending ──────────► processing ─────────► completed
           ▲                   │
           │                   │ (error)
           │                   ▼
           │              temporary ───► pending (retry)
           │                   │
           │                   │ (max retries)
           │                   ▼
           └────────────── failed
```

## Error Handling & Retries

### Error Categories

```go
const (
    ErrorCategoryNetwork   = "network"    // Retry with backoff
    ErrorCategoryRateLimit = "rate_limit" // Retry after delay
    ErrorCategoryNotFound  = "not_found"  // Skip (no retry)
    ErrorCategoryParsing   = "parsing"    // Skip (no retry)
    ErrorCategoryPlugin    = "plugin"     // Restart plugin, retry once
)
```

### Retry Logic

```go
func (p *JobProcessor) handleFailure(ctx context.Context, job *QueueJob, err error) {
    category := categorizeError(err)

    if category.IsRetryable() && job.Attempts < job.MaxAttempts {
        // Calculate exponential backoff
        delay := time.Duration(math.Pow(2, float64(job.Attempts))) * time.Minute
        nextRetry := time.Now().Add(delay)

        p.deps.QueueRepo.Fail(ctx, job.ID, err.Error(), category, &nextRetry)
    } else {
        // Mark as permanently failed
        p.deps.QueueRepo.Fail(ctx, job.ID, err.Error(), category, nil)
    }
}
```

### Stuck Job Recovery

On startup, the pipeline manager recovers stuck jobs:

```go
func (m *Manager) recoverStuckJobs(ctx context.Context) error {
    // Reset jobs stuck in "processing" (server crashed)
    count, err := m.deps.StatusRepo.ResetStuck(ctx)
    m.deps.Logger.Info("recovered stuck jobs", "count", count)

    // Release locks held too long
    return m.deps.QueueRepo.ReleaseStuck(ctx, 300) // 5 minutes
}
```

## Progress Tracking

### Enrichment Status Table

Each media item's progress is tracked per stage:

```sql
CREATE TABLE enrichment_status (
    media_id INTEGER NOT NULL,
    media_type TEXT NOT NULL,
    stage TEXT NOT NULL,
    status TEXT DEFAULT 'pending',
    completed_at TIMESTAMP,
    error TEXT,
    PRIMARY KEY (media_id, media_type, stage)
);
```

### Library Progress

Aggregate progress is computed for the UI:

```go
func (r *StatusRepository) GetLibraryProgress(ctx context.Context, libraryID int64) (map[string]*QueueStats, error) {
    // Returns per-stage statistics:
    // {
    //   "nfo":         {pending: 0, completed: 100, failed: 2},
    //   "local_images": {pending: 10, completed: 90, failed: 0},
    //   "tmdb":        {pending: 50, completed: 48, failed: 2},
    // }
}
```

### UI Display

```text
Library: Movies (100 items)

Enrichment Pipeline:
  ✓ NFO Metadata      100 / 100  complete
  ✓ Local Images       90 / 100  10 not found
  ⏳ TMDb              48 / 100  in progress (est. 5 min)
```

## Real-Time Events

The pipeline publishes events via the Event Bus for SSE streaming:

```go
// When job starts
p.deps.EventBus.Publish(events.NewEvent(events.EventEnrichmentStarted, "worker").
    WithMediaID(job.MediaID).
    WithLibraryID(job.LibraryID).
    WithStage(p.config.Stage).
    Build())

// When stage completes
p.deps.EventBus.Publish(events.NewEvent(events.EventEnrichmentStageComplete, "worker").
    WithMediaID(job.MediaID).
    WithLibraryID(job.LibraryID).
    WithStage(p.config.Stage).
    Build())
```

The frontend subscribes to these events for live progress updates.

## Configuration Options

### Per-Stage Settings

```go
type StageWorkerConfig struct {
    Stage       string
    Concurrency int           // Worker count
    BatchSize   int           // Jobs per claim
    RateLimit   float64       // Requests/second
    Timeout     int           // Seconds per job
    MaxRetries  int           // Max retry attempts
}
```

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `VIEWRA_ENRICHMENT_CONCURRENCY` | Default worker count | `4` |
| `VIEWRA_ENRICHMENT_TIMEOUT` | Per-job timeout (seconds) | `60` |
| `VIEWRA_TMDB_API_KEY` | TMDb API key | Required |

## Troubleshooting

### Jobs Stuck in Processing

```sql
-- Check for stuck jobs
SELECT * FROM enrichment_queue
WHERE status = 'processing'
AND locked_at < datetime('now', '-5 minutes');

-- Manual release
UPDATE enrichment_queue
SET status = 'pending', locked_by = NULL, locked_at = NULL
WHERE status = 'processing'
AND locked_at < datetime('now', '-5 minutes');
```

### High Failure Rate

Check the error categories:

```sql
SELECT stage, error_category, COUNT(*) as count
FROM enrichment_queue
WHERE status = 'failed'
GROUP BY stage, error_category;
```

Common issues:
- `network`: Check API connectivity
- `rate_limit`: Reduce concurrency or add delays
- `not_found`: Normal for items not in external DB

### Re-Enriching Items

To re-run enrichment for specific items:

```sql
-- Re-enqueue failed items for a stage
INSERT OR REPLACE INTO enrichment_queue (media_id, library_id, media_type, stage, status)
SELECT media_id, library_id, media_type, 'tmdb', 'pending'
FROM enrichment_queue
WHERE stage = 'tmdb' AND status = 'failed';
```

## See Also

- [Plugin Development Guide](./PLUGIN_DEVELOPMENT.md)
- [Real-Time Updates Guide](./REAL_TIME_UPDATES.md)
- [ADR 027: Plugin System Architecture](../decisions/027-plugin-system-architecture.md)
