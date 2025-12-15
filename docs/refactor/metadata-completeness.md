# Metadata Completeness Refactor

## Overview

This document outlines the gaps in ViewRA's metadata pipeline and the changes needed to fully support metadata from external providers like TMDB, TVDb, and MusicBrainz.

## Current Architecture

```
External Provider (TMDB/MusicBrainz)
         ↓
    Plugin (enricher)
         ↓
    Proto (EnrichedMetadata)
         ↓
    MetadataApplier
         ↓
    Domain Entity
         ↓
    Repository (mapper/params)
         ↓
    Database Schema
```

Data can be lost at any layer if that layer doesn't support the field.

---

## Gap Analysis by Media Type

### Movies

| Field | TMDB | Proto | DB | Domain | Applier | Status |
|-------|------|-------|-----|--------|---------|--------|
| title | ✓ | ✓ | ✓ | ✓ | ✓ | **OK** |
| original_title | ✓ | ✓ | ✓ | ✓ | ✓ | **OK** |
| sort_title | ✓ | ✓ | ✓ | ✓ | ✓ | **OK** |
| year | ✓ | ✓ | ✓ | ✓ | ✓ | **OK** |
| release_date | ✓ | ❌ | ✓ | ✓ | ❌ | **PROTO GAP** |
| plot | ✓ | ✓ | ✓ | ✓ | ✓ | **OK** |
| tagline | ✓ | ✓ | ✓ | ✓ | ✓ | **OK** |
| genres | ✓ | ✓ | ✓ | ✓ | ✓ | **OK** |
| content_rating | ✓ | ✓ | ✓ | ✓ | ✓ | **OK** |
| runtime_minutes | ✓ | ✓ | ✓ | ✓ | ✓ | **OK** |
| rating | ✓ | ✓ | ❌ | ❌ | ❌ | **DB GAP** |
| rating_votes | ✓ | ✓ | ❌ | ❌ | ❌ | **DB GAP** |
| directors | ✓ | ✓ | ✓ | ✓ | ✓ | **OK** (flat text) |
| writers | ✓ | ✓ | ❌ | ❌ | ❌ | **DB GAP** |
| cast (names) | ✓ | ✓ | ✓ | ✓ | ✓ | **OK** (flat text) |
| cast (roles) | ✓ | ✓ | ❌ | ❌ | ❌ | **STRUCTURAL GAP** |
| cast (photos) | ✓ | ✓ | ❌ | ❌ | ❌ | **STRUCTURAL GAP** |
| studios | ✓ | ✓ | ❌ | ❌ | ❌ | **DB GAP** |
| budget | ✓ | ❌ | ✓ | ✓ | ❌ | **PROTO GAP** |
| revenue | ✓ | ❌ | ✓ | ✓ | ❌ | **PROTO GAP** |
| original_language | ✓ | ❌ | ✓ | ✓ | ❌ | **PROTO GAP** |
| country_of_origin | ✓ | ❌ | ✓ | ✓ | ❌ | **PROTO GAP** |

### TV Shows

| Field | TMDB | Proto | DB | Domain | Applier | Status |
|-------|------|-------|-----|--------|---------|--------|
| title | ✓ | ✓ | ✓ | ✓ | ✓ | **OK** |
| original_title | ✓ | ✓ | ✓ | ✓ | ✓ | **OK** |
| sort_title | ✓ | ✓ | ✓ | ✓ | ✓ | **OK** |
| year | ✓ | ✓ | ✓ | ✓ | ✓ | **OK** |
| first_air_date | ✓ | ✓ | ✓ | ✓ | ✓ | **OK** |
| last_air_date | ✓ | ❌ | ✓ | ✓ | ❌ | **PROTO GAP** |
| plot | ✓ | ✓ | ✓ | ✓ | ✓ | **OK** |
| tagline | ✓ | ✓ | ❌ | ❌ | ❌ | **DB GAP** |
| genres | ✓ | ✓ | ✓ | ✓ | ✓ | **OK** |
| status | ✓ | ✓ | ✓ | ✓ | ✓ | **OK** |
| content_rating | ✓ | ✓ | ✓ | ✓ | ✓ | **OK** |
| network | ✓ | ✓ | ✓ | ✓ | ✓ | **OK** |
| rating | ✓ | ✓ | ❌ | ❌ | ❌ | **DB GAP** |
| rating_votes | ✓ | ✓ | ❌ | ❌ | ❌ | **DB GAP** |
| creators | ✓ | ❌ | ❌ | ❌ | ❌ | **FULL GAP** |
| cast | ✓ | ✓ | ❌ | ❌ | ❌ | **DB GAP** |
| studios | ✓ | ✓ | ❌ | ❌ | ❌ | **DB GAP** |
| original_language | ✓ | ❌ | ✓ | ✓ | ❌ | **PROTO GAP** |
| country_of_origin | ✓ | ❌ | ✓ | ✓ | ❌ | **PROTO GAP** |

### TV Episodes

| Field | TMDB | Proto | DB | Domain | Applier | Status |
|-------|------|-------|-----|--------|---------|--------|
| episode_title | ✓ | ✓ | ✓ | ✓ | ✓ | **OK** |
| plot | ✓ | ✓ | ✓ | ✓ | ✓ | **OK** |
| air_date | ✓ | ✓ | ✓ | ✓ | ✓ | **OK** |
| runtime_minutes | ✓ | ✓ | ❌ | ❌ | ❌ | **DB GAP** |
| rating | ✓ | ✓ | ❌ | ❌ | ❌ | **DB GAP** |
| rating_votes | ✓ | ✓ | ❌ | ❌ | ❌ | **DB GAP** |
| directors | ✓ | ✓ | ❌ | ❌ | ❌ | **DB GAP** |
| writers | ✓ | ✓ | ❌ | ❌ | ❌ | **DB GAP** |
| guest_cast | ✓ | ✓ | ❌ | ❌ | ❌ | **DB GAP** |

### Music Tracks

| Field | MusicBrainz | Proto | DB | Domain | Applier | Status |
|-------|-------------|-------|-----|--------|---------|--------|
| title | ✓ | ✓ | ✓ | ✓ | ✓ | **OK** |
| genres | ✓ | ✓ | ✓ | ✓ | ✓ | **OK** |
| year | ✓ | ✓ | ✓ | ✓ | ✓ | **OK** |
| release_date | ✓ | ✓ | ✓ | ✓ | ❌ | **APPLIER GAP** |
| record_label | ✓ | ✓ | ✓ | ✓ | ❌ | **APPLIER GAP** |
| artist | ✓ | ❌ | ✓ | ✓ | ❌ | **PROTO GAP** |
| album | ✓ | ❌ | ✓ | ✓ | ❌ | **PROTO GAP** |
| album_artist | ✓ | ❌ | ✓ | ✓ | ❌ | **PROTO GAP** |
| track_number | ✓ | ❌ | ✓ | ✓ | ❌ | **PROTO GAP** |
| disc_number | ✓ | ❌ | ✓ | ✓ | ❌ | **PROTO GAP** |

### Music Albums & Artists

**Complete TODO** - The applier has placeholder comments but no implementation:
- `metadata_applier.go:46` - "TODO: Add album metadata update"
- `metadata_applier.go:49` - "TODO: Add artist metadata update"

---

## Proposed Schema Changes

### 1. People Table (New)

Normalized storage for cast, directors, writers, creators.

```sql
CREATE TABLE people (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    sort_name TEXT,
    photo_path TEXT,           -- Local cached path
    photo_url TEXT,            -- Original remote URL
    imdb_id TEXT,
    tmdb_id INTEGER,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX idx_people_tmdb_id ON people(tmdb_id) WHERE tmdb_id IS NOT NULL;
CREATE UNIQUE INDEX idx_people_imdb_id ON people(imdb_id) WHERE imdb_id IS NOT NULL;
CREATE INDEX idx_people_name ON people(name);
```

### 2. Credits Table (New)

Links people to media with role information.

```sql
CREATE TABLE credits (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    person_id INTEGER NOT NULL,
    media_type TEXT NOT NULL,  -- 'movie', 'tv_show', 'tv_episode'
    entity_id INTEGER NOT NULL,
    credit_type TEXT NOT NULL, -- 'cast', 'director', 'writer', 'creator', 'guest'
    character_name TEXT,       -- For cast: "Tony Stark"
    department TEXT,           -- For crew: "Directing", "Writing"
    job TEXT,                  -- For crew: "Director", "Screenplay"
    billing_order INTEGER,     -- Cast ordering
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (person_id) REFERENCES people(id) ON DELETE CASCADE
);

CREATE INDEX idx_credits_person_id ON credits(person_id);
CREATE INDEX idx_credits_entity ON credits(media_type, entity_id);
CREATE INDEX idx_credits_type ON credits(credit_type);
```

### 3. Studios Table (New)

```sql
CREATE TABLE studios (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    logo_path TEXT,
    tmdb_id INTEGER,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE media_studios (
    media_type TEXT NOT NULL,  -- 'movie', 'tv_show'
    entity_id INTEGER NOT NULL,
    studio_id INTEGER NOT NULL,
    PRIMARY KEY (media_type, entity_id, studio_id),
    FOREIGN KEY (studio_id) REFERENCES studios(id) ON DELETE CASCADE
);
```

### 4. Rating Columns (Add to existing tables)

```sql
-- Movies
ALTER TABLE movies ADD COLUMN rating REAL;
ALTER TABLE movies ADD COLUMN rating_votes INTEGER;

-- TV Shows
ALTER TABLE tv_shows ADD COLUMN rating REAL;
ALTER TABLE tv_shows ADD COLUMN rating_votes INTEGER;
ALTER TABLE tv_shows ADD COLUMN tagline TEXT;

-- TV Episodes
ALTER TABLE tv_episodes ADD COLUMN rating REAL;
ALTER TABLE tv_episodes ADD COLUMN rating_votes INTEGER;
ALTER TABLE tv_episodes ADD COLUMN runtime_minutes INTEGER;
```

---

## Proto Changes

### EnrichedMetadata Additions

```protobuf
message EnrichedMetadata {
  // ... existing fields ...

  // Add these:
  optional string release_date = 21;      // YYYY-MM-DD for movies
  optional string last_air_date = 22;     // For TV shows
  optional int32 budget = 23;             // Movies
  optional int64 revenue = 24;            // Movies
  optional string original_language = 25;
  optional string country_of_origin = 26;

  // Music-specific (move from EnrichRequest to allow applying)
  optional string artist = 27;
  optional string album = 28;
  optional string album_artist = 29;
  optional int32 track_number = 30;
  optional int32 disc_number = 31;
  optional int32 total_tracks = 32;
  optional int32 total_discs = 33;

  // Creators (TV shows)
  repeated string creators = 34;
}
```

---

## Domain Entity Changes

### Movie

Add to `internal/domain/media/movie.go`:
```go
Rating       float32
RatingVotes  int
Writers      string   // Comma-separated (same pattern as Director)
```

### TVShow

Add to `internal/domain/media/repository.go` (TVShow struct):
```go
Rating       float32
RatingVotes  int
Tagline      string
```

### TVEpisode

Add to `internal/domain/media/tv.go`:
```go
RuntimeMinutes int
Rating         float32
RatingVotes    int
```

### People (New)

Create `internal/domain/media/people.go`:
```go
type Person struct {
    ID        int64
    Name      string
    SortName  string
    PhotoPath string
    PhotoURL  string
    IMDbID    string
    TMDbID    int
}

type Credit struct {
    ID            int64
    PersonID      int64
    Person        *Person  // Populated on fetch
    MediaType     string
    EntityID      int64
    CreditType    string   // "cast", "director", "writer", "creator"
    CharacterName string
    Department    string
    Job           string
    BillingOrder  int
}
```

---

## Repository Changes

### New PeopleRepository Interface

```go
type PeopleRepository interface {
    // People
    CreatePerson(ctx context.Context, person *Person) error
    GetPersonByID(ctx context.Context, id int64) (*Person, error)
    FindPersonByTMDbID(ctx context.Context, tmdbID int) (*Person, error)
    FindPersonByIMDbID(ctx context.Context, imdbID string) (*Person, error)
    UpdatePerson(ctx context.Context, person *Person) error

    // Credits
    AddCredit(ctx context.Context, credit *Credit) error
    GetCreditsForEntity(ctx context.Context, mediaType string, entityID int64) ([]*Credit, error)
    GetCreditsForPerson(ctx context.Context, personID int64) ([]*Credit, error)
    DeleteCreditsForEntity(ctx context.Context, mediaType string, entityID int64) error
}
```

---

## Applier Changes

The `MetadataApplier` needs significant expansion:

1. **Movies**: Wire rating, rating_votes, writers, budget, revenue, release_date
2. **TV Shows**: Wire rating, rating_votes, tagline, last_air_date
3. **TV Episodes**: Wire runtime_minutes, rating, rating_votes
4. **Music**: Wire release_date, record_label, artist, album fields
5. **Music Albums**: Implement the TODO
6. **Music Artists**: Implement the TODO
7. **Credits**: New method to process CastMember array into people + credits tables

### New Method: ApplyCredits

```go
func (a *MetadataApplier) ApplyCredits(
    ctx context.Context,
    mediaType string,
    entityID int64,
    directors []string,
    writers []string,
    cast []*pluginv1.CastMember,
) error {
    // 1. Delete existing credits for this entity
    // 2. For each person:
    //    a. Find or create Person record (by TMDB ID if available)
    //    b. Create Credit record linking person to entity
    // 3. Download actor photos asynchronously (queue for later)
}
```

---

## Migration Plan

### Phase 1: Schema & Simple Fields
1. Create migration 000035 for people, credits, studios tables
2. Create migration 000036 for rating/tagline/runtime columns
3. Update domain entities
4. Update repository mappers
5. Regenerate sqlc

### Phase 2: Proto & Applier
1. Update proto with new fields
2. Regenerate proto
3. Update applier for simple fields (ratings, tagline, etc.)

### Phase 3: Credits System
1. Implement PeopleRepository
2. Update applier to process credits
3. Wire credits to API responses

### Phase 4: Music Completion
1. Update proto for music fields
2. Implement album/artist applier methods
3. Test end-to-end with MusicBrainz enricher

---

## API Impact

New endpoints needed:
- `GET /api/people/{id}` - Person details with filmography
- `GET /api/people/{id}/credits` - All credits for a person
- `GET /api/movies/{id}/credits` - Cast & crew for a movie
- `GET /api/tv/{id}/credits` - Cast & crew for a show

Existing endpoints enhanced:
- Movie/show detail responses include rating, cast with roles

---

## Testing Strategy

1. Unit tests for new repository methods
2. Integration tests for credit deduplication (same actor in multiple movies)
3. Migration tests (up and down)
4. E2E test with mock TMDB responses

---

## Provider-Specific Fields

Beyond TMDB, these providers offer unique fields we should consider:

### TVDb (TheTVDB)
- `absolute_episode_number` - For anime/non-sequential ordering
- `dvd_season`, `dvd_episode_number` - DVD release ordering
- `aired_season`, `aired_episode_number` - Original broadcast ordering
- `zap2it_id` - Zap2It external ID
- `production_code` - Internal production codes

**Recommendation**: Add `absolute_number`, `dvd_season`, `dvd_episode` to episodes (DB already has these columns, need to wire through proto/applier).

### MusicBrainz
- `disambiguation` - Differentiates recordings/artists with same name
- `isrc` - International Standard Recording Code (per track)
- `release_group` - Groups different releases of same album
- `artist_credit` with `joinphrase` - "Artist A feat. Artist B"
- `area` / `country` - Geographic origin

**Recommendation**: Most music fields already exist in schema. Focus on wiring proto and applier.

### Fanart.tv
- 40+ image types vs our current ~10
- `likes` count per image
- Season/disc-specific artwork

**Recommendation**: Current image system handles this. No schema changes needed.

### OMDb
- `metacritic_score` (0-100 scale)
- `rotten_tomatoes_rating`
- `awards` text (e.g., "Won 5 Golden Globes")
- `dvd_release_date`
- `box_office`

**Recommendation**: Add `awards` to movies (already in schema, wire through). Consider separate ratings table for multiple sources.

---

## Open Questions

1. **Photo storage**: Download actor photos immediately or lazily on first view?
   - **Recommendation**: Lazy download on first API request, cache locally.

2. **Credit deduplication**: Match by TMDB ID, IMDb ID, or name?
   - **Recommendation**: TMDB ID first, then IMDb ID, then name+year as fallback.

3. **Backward compatibility**: Keep existing flat `director`/`cast` columns or migrate fully?
   - **Recommendation**: Keep flat columns for quick display, populate from credits table.

4. **Music credits**: Do we need composer/performer credits for music tracks?
   - **Recommendation**: Yes, reuse credits table with `media_type='music_track'`.

5. **Multiple rating sources**: Store TMDB, IMDb, RT, Metacritic separately?
   - **Recommendation**: Phase 1: single rating field. Phase 2: ratings table if needed.

---

## Files to Modify

### Migrations
- `migrations/000035_add_people_credits.up.sql` (new)
- `migrations/000035_add_people_credits.down.sql` (new)
- `migrations/000036_add_rating_columns.up.sql` (new)
- `migrations/000036_add_rating_columns.down.sql` (new)
- `migrations/postgres/` (same migrations for PostgreSQL)

### Proto
- `api/proto/plugin/enricher.proto`

### Domain
- `internal/domain/media/movie.go`
- `internal/domain/media/tv.go`
- `internal/domain/media/repository.go`
- `internal/domain/media/people.go` (new)

### Infrastructure
- `internal/infrastructure/database/queries/sqlite/people.sql` (new)
- `internal/infrastructure/database/queries/postgres/people.sql` (new)
- `internal/infrastructure/database/queries/sqlite/movies.sql` (update)
- `internal/infrastructure/database/queries/postgres/movies.sql` (update)
- `internal/infrastructure/persistence/people/repository.go` (new)
- `internal/infrastructure/persistence/movie/types.go` (update)
- `internal/infrastructure/persistence/tvshow/types.go` (update)

### Application
- `internal/application/enrichment/pipeline/metadata_applier.go`
- `internal/application/enrichment/pipeline/credits_applier.go` (new)

### API
- `internal/api/handlers/people.go` (new)
- `internal/api/routes.go` (update)
