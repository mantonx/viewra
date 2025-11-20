# ADR 008: Music Artist Artwork Extraction from Local Files

**Status**: Proposed
**Date**: 2025-11-17
**Author**: ViewRA Team
**Supersedes**: N/A
**Related**: [ADR 006: Image Handling Strategy](006-image-handling-strategy.md)

## Context

Music artists in ViewRA are currently **virtual entities** - they don't have a dedicated database table. Instead, artist data is aggregated from the `music_tracks.artist` field, with each `ArtistSummary` using the first track's `media_id` as a representative ID.

While album artwork is successfully extracted and displayed (100 covers across 9 albums from 3 artists), artist artwork is not currently being extracted, leaving artist pages with placeholder emojis (🎤).

### Discovery: Local Artist Artwork Already Exists

Investigation of the music file structure reveals **artist-level artwork is already present** in the file system:

```
/cifs/fictionalserver/music/A Perfect Circle/
├── folder.jpg         ← Artist photo/image
├── banner.jpg         ← Artist banner
├── fanart.jpg         ← Artist background art
├── logo.png           ← Artist logo
└── Eat the Elephant (2018)[FLAC 24bit]/
    ├── folder.jpg     ← Album cover (✅ already extracted)
    └── discart.png    ← Disc art (✅ already extracted)
```

This pattern is consistent across all artists with organized music libraries. **No external API calls are needed** - the artwork is local.

### Current Architecture

**Album Image Extraction** (working):
```go
// scan_library.go:622-628
if uc.extractMusicImages != nil && track.Album != "" {
    albumDir := filepath.Dir(result.FilePath)  // Album directory
    entityID := int(track.Media.ID)            // Track ID as entity ID
    uc.extractMusicImages.Execute(ctx, albumDir, images.MediaTypeMusicAlbum, entityID)
}
```

**Artist Representation** (virtual):
```go
// dto.go:8-15
type ArtistSummary struct {
    ID         int64            `json:"id"`         // First track's media_id
    Name       string           `json:"name"`
    AlbumCount int              `json:"album_count"`
    TrackCount int              `json:"track_count"`
}
```

## Decision

Implement **local artist artwork extraction** during library scan, following the same pattern as album artwork extraction, with artists continuing to be virtual entities.

### Architecture

#### 1. Entity ID Strategy

**Use the same ID system as `ArtistSummary`**: First track's `media_id` represents the artist.

**Benefits**:
- ✅ Consistent with existing artist ID system
- ✅ No new database tables needed
- ✅ Frontend already uses this ID (`artist.id`)
- ✅ Simple JOIN in queries: `media_images.entity_id = first_track.media_id`

**Storage**:
```sql
-- Artist images stored in existing media_images table
INSERT INTO media_images (
    entity_id,      -- First track's media_id (same as ArtistSummary.ID)
    media_type,     -- 'music_artist'
    image_type,     -- 'folder', 'fanart', 'banner', 'logo'
    file_path,      -- Path to artist-level image
    ...
)
```

#### 2. Extraction Process

Add artist image extraction **after** album image extraction in scan flow:

```go
// scan_library.go (after line 646)
// Extract artist images from parent directory
if uc.extractArtistImages != nil && track.Artist != "" {
    artistDir := filepath.Dir(filepath.Dir(result.FilePath))  // Parent of album dir
    entityID := int(track.Media.ID)  // Same as artist.id

    // Only extract once per artist (check if already done)
    if !isArtistProcessed(track.Artist) {
        if err := uc.extractArtistImages.Execute(
            ctx,
            artistDir,
            images.MediaTypeMusicArtist,
            entityID,
        ); err != nil {
            fmt.Printf("failed to extract artist images for %s: %v\n", track.Artist, err)
        }
        markArtistProcessed(track.Artist)
    }
}
```

**Deduplication**: Track processed artists in-memory during scan to avoid extracting the same artist multiple times (once per track).

#### 3. Image Types to Extract

Scan for these files in artist directory (case-insensitive):

| File Pattern | Image Type | Purpose | Priority |
|-------------|------------|---------|----------|
| `folder.jpg/png` | `folder` | Primary artist image | 1 (highest) |
| `fanart.jpg/png` | `fanart` | Background art | 2 |
| `banner.jpg/png` | `banner` | Wide banner | 3 |
| `logo.png` | `logo` | Artist logo | 4 |
| `clearlogo.png` | `clearlogo` | Transparent logo | 4 |

#### 4. Use Case Implementation

Create new use case following existing pattern:

```go
// internal/application/images/extract_artist_images.go
type ExtractMusicArtistImagesUseCase struct {
    imageRepo       images.Repository
    cacheService    ImageCacheService
    transformer     ImageTransformService
}

func (uc *ExtractMusicArtistImagesUseCase) Execute(
    ctx context.Context,
    artistDir string,
    mediaType images.MediaType,
    entityID int,
) error {
    // 1. Scan for artist-level images
    artistImages := scanArtistDirectory(artistDir)

    // 2. For each image found:
    for _, imgPath := range artistImages {
        imgType := detectImageType(filepath.Base(imgPath))

        // 3. Store metadata in database
        image := &images.Image{
            EntityID:   entityID,
            MediaType:  mediaType,
            ImageType:  imgType,
            FilePath:   imgPath,
            SourceType: images.SourceTypeLocal,
            Priority:   getPriority(imgType),
        }

        // 4. Create cached versions
        if err := uc.cacheService.CacheImage(ctx, image); err != nil {
            return err
        }

        if err := uc.imageRepo.Create(ctx, image); err != nil {
            return err
        }
    }

    return nil
}
```

#### 5. API Endpoint

Add new endpoint following existing pattern:

```go
// internal/api/handlers/images.go
// GET /api/music/artists/:id/images
func (h *ImagesHandler) GetMusicArtistImages(c *gin.Context) {
    id, err := parseID(c.Param("id"))  // Artist's representative track ID
    if err != nil {
        c.JSON(400, ErrorResponse{...})
        return
    }

    images, err := h.getImages.Execute(c.Request.Context(), id, images.MediaTypeMusicArtist)
    if err != nil {
        handleError(c, err)
        return
    }

    c.JSON(200, ListImagesResponse{Images: images})
}
```

Route registration:
```go
// internal/api/routes/images.go
music.GET("/artists/:id/images", imagesHandler.GetMusicArtistImages)
```

#### 6. Frontend Integration

**Hook** (web/src/lib/hooks/useMediaImages.ts):
```typescript
export function useMusicArtistImages(
  artistId: number | undefined,
  options: UseMediaImagesOptions = {}
) {
  const { enabled = true } = options

  return useQuery({
    queryKey: ['music-artist-images', artistId],
    queryFn: async () => {
      if (!artistId) return { images: [] }
      return imagesApi.getMusicArtistImages(artistId)
    },
    enabled: enabled && artistId !== undefined,
    staleTime: 1000 * 60 * 5, // 5 minutes
  })
}
```

**API Client** (web/src/lib/api/images.ts):
```typescript
getMusicArtistImages: async (artistId: number): Promise<ListImagesResponse> => {
  const response = await fetch(`/api/music/artists/${artistId}/images`)
  if (!response.ok) throw new Error('Failed to fetch artist images')
  return response.json()
},
```

**MediaPoster Support**:
```typescript
// Add 'music-artist' to MediaType union
export type MediaType =
  | 'media'
  | 'tv-show'
  | 'tv-season'
  | 'tv-episode'
  | 'music-album'
  | 'music-artist'  // ← Add this

// Add artist images query
const artistImagesQuery = useMusicArtistImages(mediaId, {
  enabled: mediaType === 'music-artist'
})

// Add to query selection
const { data: imagesData, isLoading } =
  mediaType === 'music-artist'
    ? artistImagesQuery
    : // ... other types

// Add artist image helper
export function getArtistImage(images: Image[]): Image | undefined {
  // Prefer folder.jpg, fallback to fanart
  return findImageByType(images, 'folder') ||
         findImageByType(images, 'fanart')
}

// Use in image selection
const image =
  mediaType === 'music-artist'
    ? imagesData?.images ? getArtistImage(imagesData.images) : null
    : // ... other types
```

**ArtistCard Update**:
```typescript
// web/src/components/music/ArtistCard/ArtistCard.tsx
<MediaPoster
  mediaId={artist.id}
  mediaType="music-artist"  // ← Change from default to music-artist
  alt={artist.name}
  className="w-full h-full absolute inset-0"
  preset="medium"
  fallbackIcon="🎤"
/>
```

## Consequences

### Positive

✅ **No External Dependencies**: Uses local files, no API calls needed
✅ **DRY**: Reuses exact same pattern as album extraction
✅ **Consistent**: Same entity ID system as virtual artists
✅ **Performant**: Local file access, cached serving
✅ **Extensible**: Easy to add more image types later
✅ **Minimal Changes**: Works with existing virtual artist architecture

### Negative

⚠️ **Deduplication Required**: Must track processed artists during scan
⚠️ **File Organization Dependent**: Requires organized music library structure
⚠️ **Same ID for Multiple Artists**: If artists share a first track ID (unlikely but theoretically possible)

### Neutral

🔹 **No Artist Table**: Continues using virtual artist pattern
🔹 **Manual Artwork**: Relies on user-maintained artwork files

## Implementation Checklist

### Backend

- [ ] Create `ExtractMusicArtistImagesUseCase` in `internal/application/images/`
- [ ] Add `GetMusicArtistImages` handler in `internal/api/handlers/images.go`
- [ ] Register route `GET /api/music/artists/:id/images` in `internal/api/routes/images.go`
- [ ] Update `ScanLibraryUseCase` to call artist image extraction (with deduplication)
- [ ] Wire up use case in `internal/app/container.go`
- [ ] Add artist image type detection logic (folder.jpg → 'folder', etc.)
- [ ] Test with A Perfect Circle directory structure

### Frontend

- [ ] Add `useMusicArtistImages` hook in `web/src/lib/hooks/useMediaImages.ts`
- [ ] Add `getMusicArtistImages` to `web/src/lib/api/images.ts`
- [ ] Add `'music-artist'` to MediaType union in `MediaPoster.tsx`
- [ ] Add `getArtistImage` helper in `web/src/lib/types/images.ts`
- [ ] Update MediaPoster to support music-artist type
- [ ] Update ArtistCard to use `mediaType="music-artist"`
- [ ] Test with A Perfect Circle artist page

### Testing

- [ ] Unit tests for artist image extraction
- [ ] Integration test for artist image API endpoint
- [ ] Verify deduplication (scan same artist's albums multiple times)
- [ ] Verify fallback behavior (no artist images present)
- [ ] Test with different file naming conventions (folder.jpg vs Folder.JPG)

## Alternatives Considered

### 1. Use Album Cover as Artist Image (Quick Fix)
```typescript
// Just change ArtistCard to use music-album type
<MediaPoster mediaType="music-album" ... />
```
**Pros**: 5-minute fix, zero backend changes
**Cons**: All artists show same album cover, not ideal UX
**Verdict**: Good temporary solution, not ideal long-term

### 2. Create Dedicated Artist Table
**Pros**: Proper normalization, unique IDs
**Cons**: Major refactor, breaks existing API contracts
**Verdict**: Over-engineered for current needs

### 3. External API (MusicBrainz/Last.fm)
**Pros**: Automated, professional artist photos
**Cons**: API dependencies, rate limits, may not have all artists
**Verdict**: Local files already exist - use them first, external API as future enhancement

## References

- [ADR 006: Image Handling Strategy](006-image-handling-strategy.md)
- Current album extraction: `internal/application/library/scan_library.go:622-628`
- Music track structure: `internal/domain/media/music.go`
- Artist aggregation: `internal/application/music/list_artists.go`

## Notes

**Future Enhancements**:
- Add external API fallback (MusicBrainz) for artists without local artwork
- Support embedded album art extraction from audio files (ID3 tags)
- Add artist image priority/selection preferences
- Create artist table for proper normalization (if needed)

**File Structure Example**:
```
/cifs/fictionalserver/music/
├── A Perfect Circle/              ← Artist directory
│   ├── folder.jpg                 ← Primary artist image
│   ├── fanart.jpg                 ← Background art
│   ├── banner.jpg                 ← Banner
│   ├── logo.png                   ← Logo
│   ├── Eat the Elephant (2018)/   ← Album directory
│   │   ├── folder.jpg             ← Album cover
│   │   ├── discart.png            ← Disc art
│   │   └── *.flac                 ← Tracks
│   └── Mer de noms (2000)/
│       └── ...
```
