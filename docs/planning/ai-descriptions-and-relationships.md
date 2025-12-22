# AI Descriptions & Media Relationships

## Overview

Two related features that enhance semantic search quality:

1. **Media Relationships** - Generic storage for similar/related media from any source (TMDB, MusicBrainz, AI, etc.)
2. **AI Description Generator** - Generates rich descriptions using LLM, incorporating relationships + AI suggestions

These features address the core problem that thin metadata (e.g., "A man returns home") doesn't embed well for semantic search. By generating rich, semantically dense descriptions, we dramatically improve search quality for mood-based, context-based, and similarity queries.

---

## Problem Statement

### Current Limitations

| Query Type | Current Behavior | Expected Behavior |
|------------|-----------------|-------------------|
| "bittersweet sci-fi about love" | Misses relevant films | Matches mood_tags + rich_description |
| "movies like Heat" | Random results | Finds films via media_relationships |
| "something for date night" | No context matching | Matches viewing_contexts |
| "films about grief" | Only if plot mentions it | Matches themes |

### Test Results (December 2024)

We tested LLM generation with Ollama (llama3.1:8b):

| Metric | Result |
|--------|--------|
| Average generation time | 5-7 seconds per movie |
| Estimated time for 2,530 movies | 3.5-5 hours (background) |
| JSON parsing reliability | 100% with `format: "json"` |
| Description quality | High - relevant, specific, useful |

### Recency Consideration

LLM knowledge cutoff (December 2023) affects "similar_to" suggestions for recent films. Solution: Combine TMDB recommendations (up-to-date) with AI suggestions (classic comparisons).

---

## Part 1: Media Relationships

### Purpose

Store similar/related media from multiple sources during enrichment, enabling:
- "Movies like X" queries
- Cross-source relationship discovery
- Library-aware recommendations (show only what user has)

### Database Schema

```sql
CREATE TABLE media_relationships (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    
    -- Source media
    media_type TEXT NOT NULL,              -- 'movie', 'tv_show', 'music_album', 'music_artist'
    media_id INTEGER NOT NULL,             -- Our internal media ID
    
    -- Related media
    related_media_type TEXT NOT NULL,      -- Can differ from source (cross-media support)
    related_media_id INTEGER,              -- Our media ID if in library (NULL otherwise)
    related_title TEXT NOT NULL,           -- Title for display/search
    related_year INTEGER,                  -- Helps with disambiguation
    
    -- External IDs for matching
    related_external_id TEXT,              -- TMDB ID, MusicBrainz ID, etc.
    related_external_source TEXT,          -- 'tmdb', 'musicbrainz', 'imdb', etc.
    
    -- Relationship metadata
    relationship_type TEXT NOT NULL,       -- See types below
    source TEXT NOT NULL,                  -- 'tmdb', 'musicbrainz', 'ai', 'lastfm', 'user'
    score REAL,                            -- Relevance/confidence (0-1)
    
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now')),
    
    UNIQUE(media_type, media_id, related_external_source, related_external_id, relationship_type)
);

CREATE INDEX idx_media_relationships_source ON media_relationships(media_type, media_id);
CREATE INDEX idx_media_relationships_external ON media_relationships(related_external_source, related_external_id);
CREATE INDEX idx_media_relationships_in_library ON media_relationships(related_media_id) WHERE related_media_id IS NOT NULL;
CREATE INDEX idx_media_relationships_type ON media_relationships(relationship_type, source);
```

### Relationship Types

| Type | Description | Sources |
|------|-------------|---------|
| `similar` | Similar in style/content | TMDB, MusicBrainz, Last.fm, AI |
| `recommendation` | Recommended if you liked this | TMDB, Last.fm, AI |
| `ai_suggested` | AI-generated similarity by feel/tone | AI (LLM) |
| `same_director` | Same director/creator | Derived from credits |
| `same_artist` | Same artist (for music) | MusicBrainz |
| `same_universe` | Same franchise/universe | TMDB collections, manual |
| `sequel` | Direct sequel/prequel | TMDB, manual |
| `adaptation` | Book/remake/adaptation | Manual, AI |
| `user_suggested` | User-submitted similarity | User input |

### Sources by Media Type

| Media Type | Potential Sources |
|------------|-------------------|
| Movies | TMDB, IMDb, AI |
| TV Shows | TMDB, IMDb, AI |
| Music Albums | MusicBrainz, Last.fm, Spotify, AI |
| Music Artists | MusicBrainz, Last.fm, Spotify, AI |

### Design Decisions

1. **No bidirectional storage** - If A similar to B, don't also store B similar to A. Query both directions when needed.
2. **Top 10 per source** - Limit recommendations stored to keep data manageable.
3. **Keep forever with updated_at** - No expiration; user can trigger re-enrichment if needed.
4. **Cross-media support** - Schema supports "if you liked the movie, try the soundtrack" (implement later).

### Data Flow

```
┌─────────────────────────────────────────────────────────────────┐
│                    TMDB Enrichment                               │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  1. Get movie details (existing)                                 │
│     GET /movie/{id}?append_to_response=credits,images,keywords  │
│                                                                  │
│  2. NEW: Get recommendations                                     │
│     GET /movie/{id}/recommendations                              │
│     Returns: [{id: 949, title: "Heat"}, ...]                    │
│                                                                  │
│  3. Return in EnrichResponse.related_media                      │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                 Enrichment Pipeline                              │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  RelationshipsApplier:                                          │
│  1. Store relationships in media_relationships table            │
│  2. Try to match related_media_id to library items              │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                 Post-Scan Matching                               │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  After library scan completes:                                   │
│  UPDATE media_relationships                                      │
│  SET related_media_id = (match from media_external_ids)         │
│  WHERE related_media_id IS NULL                                  │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

---

## Part 2: AI Description Generator

### Purpose

Generate semantically rich descriptions for each media item:
- **rich_description** - 2-3 sentences about viewing/listening experience
- **mood_tags** - Emotional tone, energy, atmosphere
- **similar_to** - Combined TMDB + AI suggestions
- **contexts** - Viewing/listening situations ("date night", "late night alone")
- **themes** - Core themes explored ("grief", "identity", "found family")
- **content_warnings** - Notable content advisories

### Database Schema

```sql
CREATE TABLE ai_descriptions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    media_type TEXT NOT NULL,              -- 'movie', 'tv_show', 'music_album', 'music_artist'
    media_id INTEGER NOT NULL,
    
    -- Generated content (JSON arrays stored as TEXT)
    rich_description TEXT,
    mood_tags TEXT,                        -- ["dark", "tense", "atmospheric"]
    similar_to TEXT,                       -- ["Heat", "Drive"] (merged TMDB + AI)
    contexts TEXT,                         -- ["late night alone", "date night"]
    themes TEXT,                           -- ["identity", "grief"]
    content_warnings TEXT,                 -- ["violence", "language"]
    
    -- Source tracking
    tmdb_similar TEXT,                     -- From media_relationships (in-library only)
    ai_similar TEXT,                       -- From LLM suggestions
    
    -- Metadata
    model_used TEXT,                       -- "llama3.1:8b", "gpt-4o-mini"
    provider_type TEXT,                    -- "ollama", "openai"
    source_hash TEXT,                      -- SHA256 of input metadata (detect changes)
    tokens_used INTEGER DEFAULT 0,
    generation_time_ms INTEGER DEFAULT 0,
    
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now')),
    
    UNIQUE(media_type, media_id)
);

CREATE INDEX idx_ai_descriptions_lookup ON ai_descriptions(media_type, media_id);
```

### Scope

| Media Type | Generate? | Notes |
|------------|-----------|-------|
| Movies | Yes | Full support |
| TV Shows | Yes | Show-level only, not episodes |
| Music Albums | Yes | Full support |
| Music Artists | Yes | Full support |
| TV Episodes | No | Future consideration (expensive at scale) |
| Music Tracks | No | Too expensive for large libraries |

### Replaces MoodTagService

The AI description generator replaces the existing `MoodTagService` (`plugins/ai-search/internal/mood.go`). The new system generates richer mood tags as part of the combined prompt, eliminating redundancy.

### Prompts by Media Type

**Movies:**
```
Analyze this movie. Respond with ONLY valid JSON.

Title: {title} ({year})
Genre: {genre}
Director: {director}
Plot: {plot}
Cast: {cast}

TMDB recommends: {tmdb_similar}

{
  "rich_description": "2-3 sentences about viewing experience, tone, style - not plot",
  "mood_tags": ["3-5 lowercase mood/atmosphere words"],
  "similar_to": ["3-5 similar movies by feel - add to TMDB suggestions"],
  "viewing_contexts": ["2-4 situations: 'late night alone', 'date night'"],
  "themes": ["2-4 core themes: 'identity', 'grief', 'redemption'"],
  "content_warnings": ["any notable warnings, or empty array"]
}
```

**TV Shows:**
```
Analyze this TV show. Respond with ONLY valid JSON.

Title: {title} ({year})
Genre: {genre}
Creator: {creator}
Plot: {plot}

TMDB recommends: {tmdb_similar}

{
  "rich_description": "2-3 sentences about viewing experience, binge-worthiness, tone",
  "mood_tags": ["3-5 lowercase mood/atmosphere words"],
  "similar_to": ["3-5 similar TV shows by feel"],
  "viewing_contexts": ["2-4 situations: 'weekend binge', 'comfort rewatch'"],
  "themes": ["2-4 core themes explored across the series"],
  "content_warnings": ["any notable warnings, or empty array"]
}
```

**Music Albums:**
```
Analyze this music album. Respond with ONLY valid JSON.

Album: {title} by {artist} ({year})
Genre: {genre}

{
  "rich_description": "2-3 sentences about listening experience, sound, emotional journey",
  "mood_tags": ["3-5 lowercase mood/energy words"],
  "similar_to": ["3-5 similar artists or albums by sound/feel"],
  "listening_contexts": ["2-4 situations: 'working out', 'studying', 'road trip'"],
  "themes": ["2-4 lyrical or musical themes"],
  "content_warnings": ["explicit lyrics, heavy themes, or empty array"]
}
```

**Music Artists:**
```
Analyze this music artist. Respond with ONLY valid JSON.

Artist: {name}
Genre: {genre}
From: {country}

{
  "rich_description": "2-3 sentences about their sound, style, what makes them distinctive",
  "mood_tags": ["3-5 lowercase mood/energy words typical of their music"],
  "similar_to": ["3-5 similar artists by sound and appeal"],
  "listening_contexts": ["2-4 situations their music fits well"],
  "themes": ["2-4 recurring themes in their work"],
  "content_warnings": ["typical content advisories, or empty array"]
}
```

### Example Generated Content

**The Big Short (2015):**
```json
{
  "rich_description": "A thought-provoking comedy-drama offering a unique perspective on the 2008 financial crisis through witty dialogue and satirical humor. The film's engaging tone makes complex economic concepts accessible while maintaining dramatic tension.",
  "mood_tags": ["dark", "sarcastic", "informative", "thought-provoking"],
  "similar_to": ["The Wolf of Wall Street", "Boiler Room", "Margin Call", "Enron: The Smartest Guys in the Room"],
  "viewing_contexts": ["watching with friends for discussion", "studying financial history", "business ethics exploration"],
  "themes": ["greed vs. morality", "unchecked capitalism", "systemic failure"],
  "content_warnings": ["strong language"]
}
```

**Drive (2011):**
```json
{
  "rich_description": "A visually stunning, methodically paced thriller with a haunting synth score that immerses viewers in its neon-lit atmosphere. Long stretches of quiet tension punctuated by bursts of brutal violence.",
  "mood_tags": ["atmospheric", "tense", "stylish", "melancholic"],
  "similar_to": ["Thief", "Collateral", "Nightcrawler", "Only God Forgives"],
  "viewing_contexts": ["late night alone", "focused watching", "cinephile appreciation"],
  "themes": ["isolation", "identity", "violence as consequence"],
  "content_warnings": ["graphic violence", "strong language"]
}
```

### Integration with Search Indexing

Update text builders to include AI-generated content:

```go
func (s *IndexingService) buildMovieText(m *pluginv1.MediaDetails) string {
    var b strings.Builder
    
    // ... existing structured fields (title, year, genre, plot, cast) ...
    
    // AI-enhanced content (when available)
    if desc := m.GetAiDescription(); desc != nil {
        if desc.RichDescription != "" {
            b.WriteString(fmt.Sprintf("Experience: %s\n", desc.RichDescription))
        }
        if len(desc.SimilarTo) > 0 {
            b.WriteString(fmt.Sprintf("Similar to: %s\n", strings.Join(desc.SimilarTo, ", ")))
        }
        if len(desc.Contexts) > 0 {
            b.WriteString(fmt.Sprintf("Perfect for: %s\n", strings.Join(desc.Contexts, ", ")))
        }
        if len(desc.Themes) > 0 {
            b.WriteString(fmt.Sprintf("Explores: %s\n", strings.Join(desc.Themes, ", ")))
        }
        if len(desc.MoodTags) > 0 {
            b.WriteString(fmt.Sprintf("Mood: %s\n", strings.Join(desc.MoodTags, ", ")))
        }
    }
    
    return b.String()
}
```

---

## Background Processing

### Idle Detection

The AI description generator runs during idle periods to avoid impacting user experience.

```go
type IdleDetector struct {
    lastActivity    time.Time
    idleThreshold   time.Duration  // Default: 5 minutes
}

// Activities that reset idle timer:
// - Playback start/stop/seek
// - Library scan activity
// - Search queries
// - UI navigation (significant API calls)
```

**Configuration:**
- Default idle threshold: 5 minutes
- Can be overridden via API (force generation regardless of idle state)

### Adaptive Rate Limiting

```go
type AdaptiveRateLimiter struct {
    minDelay       time.Duration  // 500ms
    maxDelay       time.Duration  // 30s
    currentDelay   time.Duration
}

// Adapts based on:
// - Response times (slow → increase delay)
// - Error rates (errors → exponential backoff)
// - Provider type (Ollama local vs cloud)
```

### Cost Guard (Cloud Providers Only)

```go
type CostGuard struct {
    enabled         bool           // Only for non-Ollama
    dailyBudgetUSD  float64        // User-configured, 0 = unlimited
    spentTodayUSD   float64
}
```

**Behavior:**
- Disabled for Ollama (free)
- Tracks token usage for OpenAI/Anthropic
- Warns before high-cost operations
- Stops if daily budget exceeded

---

## API Endpoints

### AI Descriptions

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/ai/descriptions/generate` | Start background generation |
| `GET` | `/api/ai/descriptions/status` | Get progress |
| `POST` | `/api/ai/descriptions/stop` | Stop generation |
| `GET` | `/api/ai/descriptions/estimate` | Estimate time/cost |
| `DELETE` | `/api/ai/descriptions/clear` | Clear all (or by type/library) |

**Generate Request:**
```json
{
  "library_id": 123,              // optional, 0 = all
  "media_types": ["movie"],       // optional, empty = all
  "force": false,                 // regenerate existing?
  "wait_for_idle": true           // respect idle detection?
}
```

**Status Response:**
```json
{
  "is_running": true,
  "is_paused": false,
  "media_types": ["movie", "tv_show"],
  "total": 2530,
  "processed": 500,
  "failed": 3,
  "skipped": 50,
  "est_time_left_seconds": 7200,
  "started_at": 1703203200,
  "last_updated": 1703205000
}
```

**Estimate Response:**
```json
{
  "items_needing_generation": 2530,
  "by_type": {
    "movie": 2000,
    "tv_show": 200,
    "music_album": 300,
    "music_artist": 30
  },
  "estimated_time_minutes": 150,
  "estimated_cost": {
    "provider": "ollama",
    "cost_usd": 0.00,
    "note": "Local provider - no API costs"
  }
}
```

---

## Implementation Plan

### Files to Create

| File | Purpose |
|------|---------|
| `migrations/000065_add_media_relationships.up.sql` | SQLite migration |
| `migrations/000065_add_media_relationships.down.sql` | Rollback |
| `migrations/postgres/000065_add_media_relationships.up.sql` | Postgres migration |
| `migrations/postgres/000065_add_media_relationships.down.sql` | Rollback |
| `migrations/000066_add_ai_descriptions.up.sql` | SQLite migration |
| `migrations/000066_add_ai_descriptions.down.sql` | Rollback |
| `migrations/postgres/000066_add_ai_descriptions.up.sql` | Postgres migration |
| `migrations/postgres/000066_add_ai_descriptions.down.sql` | Rollback |
| `internal/domain/media/relationship.go` | Domain types |
| `internal/domain/media/ai_description.go` | Domain types |
| `internal/infrastructure/database/queries/sqlite/media_relationships.sql` | SQLC queries |
| `internal/infrastructure/database/queries/postgres/media_relationships.sql` | SQLC queries |
| `internal/infrastructure/database/queries/sqlite/ai_descriptions.sql` | SQLC queries |
| `internal/infrastructure/database/queries/postgres/ai_descriptions.sql` | SQLC queries |
| `internal/infrastructure/persistence/relationships/repository.go` | Repository |
| `internal/infrastructure/persistence/ai_descriptions/repository.go` | Repository |
| `internal/application/enrichment/pipeline/relationships_applier.go` | Store relationships |
| `plugins/ai-search/internal/description.go` | Generator service |
| `plugins/ai-search/internal/description_test.go` | Unit tests |
| `plugins/ai-search/internal/idle.go` | Idle detection |
| `plugins/ai-search/internal/costguard.go` | Cost tracking |

### Files to Modify

| File | Changes |
|------|---------|
| `api/proto/plugin/enricher.proto` | Add `RelatedMedia` message |
| `api/proto/plugin/host_services.proto` | Add AI description storage RPCs |
| `plugins/tmdb/internal/types.go` | Add `RecommendationsResponse` |
| `plugins/tmdb/internal/client.go` | Add `GetMovieRecommendations`, `GetTVRecommendations` |
| `plugins/tmdb/internal/enrich.go` | Fetch and return recommendations |
| `internal/application/enrichment/pipeline/deps.go` | Add relationships repository |
| `internal/infrastructure/plugins/host_data.go` | Implement description storage |
| `plugins/ai-search/internal/plugin.go` | Register handlers, remove mood handlers |
| `plugins/ai-search/internal/indexing.go` | Include AI descriptions in text builders |
| `internal/app/repositories/repositories.go` | Wire up new repositories |

### Files to Delete

| File | Reason |
|------|--------|
| `plugins/ai-search/internal/mood.go` | Replaced by description.go |

### Implementation Order

**Phase 1: Media Relationships**
1. Create migrations (SQLite + Postgres)
2. Run `make sqlc-gen`
3. Create domain types
4. Create repository
5. Update proto, run `make proto-gen`
6. Update TMDB plugin (types, client, enrich)
7. Create relationships applier
8. Wire into enrichment pipeline
9. Add library-matching job
10. Test: `make reload-plugin NAME=tmdb`

**Phase 2: AI Descriptions**
1. Create migrations (SQLite + Postgres)
2. Run `make sqlc-gen`
3. Create domain types
4. Create repository
5. Update proto, run `make proto-gen`
6. Implement host service
7. Create idle detector
8. Create cost guard
9. Create description generator
10. Update plugin handlers
11. Delete mood.go
12. Update indexing
13. Create tests
14. Test: `make reload-plugin NAME=ai-search`

**Phase 3: Integration**
1. Update all text builders
2. Re-index library
3. Test search queries

---

## Summary

| Metric | Value |
|--------|-------|
| New tables | 2 |
| New files | ~20 |
| Modified files | ~15 |
| Deleted files | 1 |
| Estimated implementation time | 4-6 hours |

---

## Future Enhancements

1. **TV Episode descriptions** - Currently skipped; could add for premium/important episodes
2. **Music track descriptions** - Expensive but could be valuable for classical/jazz
3. **Cross-media relationships** - "If you liked the movie, try the soundtrack"
4. **User-submitted relationships** - Let users suggest similarities
5. **MusicBrainz integration** - Similar artists from MusicBrainz/Last.fm
6. **Relationship-based recommendations** - "Because you watched Heat..."
