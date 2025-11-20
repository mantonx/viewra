# ADR 012: Music Database Architecture - Virtual Entities vs Normalized Tables

**Status**: Accepted
**Date**: 2025-11-18
**Author**: ViewRA Team
**Context**: Music library implementation with artists, albums, and tracks

## Context

ViewRA's music library currently uses a **virtual entity** approach where artists and albums are not stored in dedicated database tables. Instead, they are aggregated from the `music_tracks` table at query time, with each `ArtistSummary`/`AlbumSummary` using the first track's `media_id` as a representative ID.

This differs from a traditional normalized approach where `artists`, `albums`, and `tracks` would be separate tables with foreign key relationships.

### Current Implementation

```
┌─────────────────────────────────────────┐
│ media (base table)                      │
│  - id                                   │
│  - library_id                           │
│  - file_path                            │
│  - duration, codec, etc.                │
└────────────┬────────────────────────────┘
             │
             ↓
┌─────────────────────────────────────────┐
│ music_tracks                            │
│  - media_id (FK to media.id)            │
│  - artist (TEXT)           ← Denormalized
│  - album (TEXT)            ← Denormalized
│  - album_artist (TEXT)     ← Denormalized
│  - track_number, year, genre, etc.      │
└─────────────────────────────────────────┘

Artists & Albums = Aggregated at query time
  - ArtistSummary.ID = first track's media_id
  - AlbumSummary.ID = first track's media_id
```

**Example Aggregation** (from [list_artists.go](../../internal/application/music/list_artists.go)):
```go
artistMap := make(map[string]*ArtistSummary)
for _, track := range tracks {
    artistName := track.AlbumArtist
    if artistName == "" {
        artistName = track.Artist
    }
    if _, exists := artistMap[artistName]; !exists {
        artistMap[artistName] = &ArtistSummary{
            ID:   track.ID,  // First track's media_id
            Name: artistName,
        }
    }
    artistMap[artistName].TrackCount++
}
```

### Alternative: Normalized Schema

```
┌──────────────┐      ┌──────────────┐      ┌──────────────┐
│   artists    │      │   albums     │      │   tracks     │
│────────────  │      │────────────  │      │────────────  │
│ id (PK)      │◄─┐   │ id (PK)      │◄─┐   │ id (PK)      │
│ name         │  │   │ title        │  │   │ title        │
│ mb_artist_id │  │   │ artist_id(FK)│  │   │ album_id(FK) │
│ ...          │  │   │ year         │  │   │ track_number │
└──────────────┘  │   │ ...          │  │   │ ...          │
                  │   └──────────────┘  │   └──────────────┘
                  │                     │
                  └─────────────────────┘
                    Many-to-Many for
                    multi-artist albums
```

## Decision

**We will keep the virtual entities approach** (denormalized `music_tracks` table with aggregated artists/albums).

## Rationale

### Performance Characteristics

| Operation | Virtual Entities | Normalized Tables |
|-----------|-----------------|-------------------|
| **List Artists** | `SELECT artist, COUNT(*) FROM music_tracks GROUP BY artist` | `SELECT * FROM artists JOIN tracks...` |
| **List Albums** | `SELECT album, COUNT(*) FROM music_tracks GROUP BY album` | `SELECT * FROM albums JOIN tracks...` |
| **Track Scan** | 1 INSERT per track | 3-4 INSERTs (artist, album, track, junction) |
| **Image Association** | Direct: `entity_id = track.media_id` | Complex: requires JOIN or duplicate logic |
| **Artist Page** | Aggregate from tracks | JOIN artists + albums + tracks |

### Advantages of Virtual Entities

**1. Read-Heavy Optimization** ✅
- Media servers are **95%+ reads, <5% writes**
- No JOIN overhead for common operations (list artists, list albums)
- Simple `GROUP BY` aggregation is very fast on indexed columns

**2. Simplicity** ✅
- Easy to understand and reason about
- Minimal migration complexity
- Consistent with file-first philosophy (metadata comes from files)
- Fewer tables to maintain

**3. File-Based Source of Truth** ✅
- Music metadata lives in audio files (ID3 tags), not database
- Database is a cache/index, not authoritative source
- Rescan recreates all metadata from files (no user edits to preserve)

**4. Current Scale Appropriateness** ✅
- Tested with production library (thousands of tracks)
- Performance is excellent for <100k tracks
- No reported slowdowns or issues

**5. Image Association Consistency** ✅
- Already using `media_id` from first track as entity ID
- Image extraction during scan already works with this model
- No need to refactor image handling

**6. Industry Alignment** ✅
- Similar to Jellyfin's virtual entity pattern
- Lightweight media servers (Navidrome, Airsonic) use similar approach
- Plex's normalized approach criticized for slow scans and complexity

### Disadvantages (Acknowledged)

**1. No Referential Integrity** ⚠️
- Can't enforce FK constraints on artist/album relationships
- Typos in artist names create duplicate entries
- Mitigated by: file metadata is authoritative, rescans fix issues

**2. Storage Duplication** ⚠️
- Artist/album names repeated for each track
- Example: "A Perfect Circle" stored 100+ times
- Impact: Minimal - text is cheap, indexes mitigate

**3. Limited Multi-Artist Support** ⚠️
- Hard to represent albums with multiple artists
- Compilation albums less elegant
- Workaround: `album_artist` field handles most cases

**4. User-Editable Metadata Challenges** ⚠️
- Editing artist name requires updating all tracks
- No single "artist" record to update
- Acceptable: ViewRA is file-first, not edit-first

### When to Reconsider

This decision should be revisited if:

1. **Library size exceeds 100,000 tracks** - Aggregation queries may slow down
2. **User-editable metadata becomes required** - Need single source of truth per artist/album
3. **Complex music relationships needed** - Multi-artist collaborations, VA compilations
4. **Performance issues arise** - Aggregation queries become bottleneck
5. **External integrations require normalization** - MusicBrainz, Last.fm matching

## Comparison: Real-World Systems

### Jellyfin
- Uses virtual entities for artists/albums
- Fast library scanning
- File-based metadata priority
- **Similar to ViewRA's approach** ✅

### Plex
- Uses normalized schema (`metadata_items` with hierarchical `parent_id`)
- Slower library scanning
- Complex database structure
- More features but higher complexity
- **Different from ViewRA's approach**

### Navidrome/Airsonic
- Lightweight, file-first
- Virtual entity pattern
- Excellent performance
- **Similar to ViewRA's approach** ✅

## Migration Strategy (If Needed)

If normalization becomes necessary, the hybrid approach allows gradual migration:

```sql
-- Phase 1: Add optional normalized tables
CREATE TABLE artists (
    id INTEGER PRIMARY KEY,
    name TEXT UNIQUE NOT NULL,
    musicbrainz_id TEXT
);

CREATE TABLE albums (
    id INTEGER PRIMARY KEY,
    title TEXT NOT NULL,
    artist_id INTEGER REFERENCES artists(id),
    year INTEGER
);

-- Phase 2: Add optional FK columns (keep existing text columns)
ALTER TABLE music_tracks ADD COLUMN artist_id INTEGER REFERENCES artists(id);
ALTER TABLE music_tracks ADD COLUMN album_id INTEGER REFERENCES albums(id);

-- Phase 3: Backfill normalized data
-- Keep music_tracks.artist/album for backward compatibility

-- Phase 4: Update queries to use FKs when available, fall back to text
```

This provides:
- ✅ Zero downtime migration
- ✅ Backward compatibility
- ✅ Gradual data migration
- ✅ Ability to revert if issues arise

## Consequences

### Positive

✅ **Optimal read performance** - No JOINs for common operations
✅ **Simple schema** - Easy to understand and maintain
✅ **Fast library scanning** - One insert per track
✅ **File-first alignment** - Metadata from files, not database
✅ **Proven at current scale** - Works well with production library
✅ **No migration cost** - Keep working code
✅ **Industry precedent** - Jellyfin, Navidrome use similar patterns

### Negative

⚠️ **Text duplication** - Artist/album names repeated
⚠️ **No referential integrity** - Can't enforce FK constraints
⚠️ **Limited multi-artist support** - Collaboration albums are awkward
⚠️ **Harder to edit** - No single artist record to update

### Neutral

🔹 **Denormalization trade-off** - Accepted for read optimization
🔹 **Future flexibility** - Can add normalized tables without breaking changes
🔹 **Migration path exists** - Hybrid approach available if needed

## Implementation Notes

**Current Files**:
- Domain: [media/music.go](../../internal/domain/media/music.go) - `MusicTrack` entity
- DTO: [music/dto.go](../../internal/application/music/dto.go) - `ArtistSummary`, `AlbumSummary`
- Repository: [persistence/music/repository.go](../../internal/infrastructure/persistence/music/repository.go)
- Use Cases: [application/music/list_artists.go](../../internal/application/music/list_artists.go)

**Database Schema**: [migrations/008_music_metadata.sql](../../migrations/008_music_metadata.sql)

**Indexes** (critical for performance):
```sql
CREATE INDEX idx_music_tracks_artist ON music_tracks(artist);
CREATE INDEX idx_music_tracks_album ON music_tracks(album);
CREATE INDEX idx_music_tracks_album_artist ON music_tracks(album_artist);
```

## Metrics for Monitoring

Track these to know when to reconsider:

- **Query performance**: `GROUP BY artist` execution time
- **Library scan time**: Time to scan 10k tracks
- **Database size**: `music_tracks` table size vs normalized equivalent
- **User complaints**: Artist duplication, missing relationships
- **Library growth**: Track count over time

**Current Benchmarks** (Nov 18, 2025):
- Library scan: ~1000 tracks in <10 seconds
- List artists: ~400 artists in <100ms
- Database size: Acceptable for thousands of tracks

## References

- [ADR 008: Music Artist Artwork Extraction](./008-music-artist-artwork-extraction.md) - Uses virtual entity IDs
- Database Design Best Practices: [StackExchange Discussion](https://dba.stackexchange.com/questions/293822/how-to-design-a-relational-database-schema-for-music-recordings)
- Jellyfin Music Documentation: [jellyfin.org/docs/general/server/media/music](https://jellyfin.org/docs/general/server/media/music/)
- Current schema: [migrations/008_music_metadata.sql](../../migrations/008_music_metadata.sql)

## Decision Log

- **2025-11-18**: Initial decision to keep virtual entities approach
- Reviewed by: Architecture review with industry research
- Approved by: Project maintainer
- Next review: When library exceeds 50k tracks or performance issues arise
