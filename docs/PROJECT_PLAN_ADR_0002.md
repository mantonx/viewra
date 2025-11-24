# Project Plan: ADR 0002 Implementation
## Library UI Consistency and Browsing Experience

**Status**: Ready for Execution
**Created**: 2025-11-23
**Total Estimated Time**: 14-17 hours
**Phases**: 6
**Breaking Change**: Yes

---

## Executive Summary

This project plan outlines the complete implementation of ADR 0002, which unifies the library browsing experience across movies, TV shows, and music. The plan includes creating shared components, rebuilding media cards from scratch, implementing user-controlled badge preferences, and enhancing poster loading for a smooth browsing experience.

**Key Objectives**:
- ✅ Reduce MovieCard from 176 lines to ~60 lines (66% reduction)
- ✅ Enhance TVShowCard from 38 lines to ~70 lines (with full feature parity)
- ✅ Eliminate code duplication through shared components
- ✅ Hide optional badges by default, make user-configurable
- ✅ Preserve all existing hover/play interactions
- ✅ Zero layout shifts during poster loading

---

## Phase 1: Core Architecture (3-4 hours)

### Task 1.1: Create Badge Preference System (1.5 hours)

**Files to Create**:
```
/web/src/lib/types/preferences.ts
/web/src/lib/hooks/useBadgePreferences.ts
```

**Implementation Details**:

**preferences.ts**:
```typescript
export interface BadgePreferences {
  essential: {
    new: true        // Always shown (not configurable)
    watched: true    // Always shown (not configurable)
    progress: true   // Always shown (not configurable)
  }
  optional: {
    resolution: boolean      // Default: false
    contentRating: boolean   // Default: false
    codec: boolean          // Default: false
    extra: boolean          // Default: false
    mediaType: boolean      // Default: false
  }
}

export const DEFAULT_BADGE_PREFERENCES: BadgePreferences = {
  essential: {
    new: true,
    watched: true,
    progress: true,
  },
  optional: {
    resolution: false,
    contentRating: false,
    codec: false,
    extra: false,
    mediaType: false,
  },
}
```

**useBadgePreferences.ts**:
```typescript
import { useState, useEffect } from 'react'
import type { BadgePreferences } from '@/lib/types/preferences'
import { DEFAULT_BADGE_PREFERENCES } from '@/lib/types/preferences'

const STORAGE_KEY = 'viewra_badge_preferences'

export const useBadgePreferences = () => {
  const [preferences, setPreferences] = useState<BadgePreferences>(() => {
    // Load from localStorage on mount
    if (typeof window === 'undefined') return DEFAULT_BADGE_PREFERENCES

    const stored = localStorage.getItem(STORAGE_KEY)
    if (stored) {
      try {
        return JSON.parse(stored)
      } catch {
        return DEFAULT_BADGE_PREFERENCES
      }
    }
    return DEFAULT_BADGE_PREFERENCES
  })

  const updatePreferences = (newPreferences: BadgePreferences) => {
    setPreferences(newPreferences)
    localStorage.setItem(STORAGE_KEY, JSON.stringify(newPreferences))
  }

  return { preferences, updatePreferences }
}
```

**Dependencies**: None
**Testing**: Unit tests for localStorage save/load logic
**Estimated Time**: 1.5 hours

---

### Task 1.2: Create MediaBadges Component (1 hour)

**Files to Create**:
```
/web/src/components/media/MediaBadges/MediaBadges.tsx
/web/src/components/media/MediaBadges/MediaBadges.types.ts
/web/src/components/media/MediaBadges/index.ts
```

**Implementation Details**:

**MediaBadges.types.ts**:
```typescript
import type { BadgePreferences } from '@/lib/types/preferences'

export interface MediaBadgesProps {
  preferences: BadgePreferences
  badges: {
    isNew?: boolean
    isExtra?: boolean
    resolution?: string
    contentRating?: string
    codec?: string
    mediaType?: string
  }
}
```

**MediaBadges.tsx**:
```typescript
import { getCodecBadgeColor } from '@/lib/utils/media'
import type { MediaBadgesProps } from './MediaBadges.types'

export const MediaBadges = ({ preferences, badges }: MediaBadgesProps) => {
  return (
    <div className="flex gap-1 flex-wrap">
      {/* Essential badges - always shown if data exists */}
      {badges.isNew && (
        <span className="px-2 py-1 text-xs font-bold bg-gradient-to-r from-green-500 to-emerald-600 text-white rounded shadow-lg">
          NEW
        </span>
      )}

      {/* Optional badges - only shown if preference enabled AND data exists */}
      {preferences.optional.extra && badges.isExtra && (
        <span className="px-2 py-1 text-xs font-semibold bg-yellow-500 text-black rounded">
          EXTRA
        </span>
      )}

      {preferences.optional.resolution && badges.resolution && (
        <span className="px-2 py-1 text-xs font-semibold bg-black bg-opacity-75 text-white rounded">
          {badges.resolution}
        </span>
      )}

      {preferences.optional.contentRating && badges.contentRating && (
        <span className="px-2 py-1 text-xs font-semibold bg-neutral-800 dark:bg-neutral-700 bg-opacity-90 text-white rounded border border-neutral-600 dark:border-neutral-500">
          {badges.contentRating}
        </span>
      )}

      {preferences.optional.codec && badges.codec && (
        <span
          className={`px-2 py-1 text-xs font-semibold text-white rounded ${getCodecBadgeColor(
            badges.codec
          )}`}
        >
          {badges.codec.toUpperCase()}
        </span>
      )}

      {preferences.optional.mediaType && badges.mediaType && (
        <span className="px-2 py-1 text-xs font-semibold bg-black bg-opacity-75 text-white rounded">
          {badges.mediaType}
        </span>
      )}
    </div>
  )
}
```

**Dependencies**: Task 1.1
**Testing**: Unit tests for badge visibility logic
**Estimated Time**: 1 hour

---

### Task 1.3: Create MediaMetadata Component (45 minutes)

**Files to Create**:
```
/web/src/components/media/MediaMetadata/MediaMetadata.tsx
/web/src/components/media/MediaMetadata/MediaMetadata.types.ts
/web/src/components/media/MediaMetadata/index.ts
```

**Implementation Details**:

**MediaMetadata.types.ts**:
```typescript
export interface MediaMetadataProps {
  title: string
  year?: number
  duration?: number  // in seconds
  genres?: string[]
  plot?: string
  director?: string
  fileSize?: number
  progress?: any  // ProgressData type from existing hooks
  links?: {
    imdb?: string
    tmdb?: string
  }
  // For TV shows
  seasonCount?: number
  episodeCount?: number
}
```

**MediaMetadata.tsx**:
```typescript
import { getProgressPercentage } from '@/lib/utils'
import type { MediaMetadataProps } from './MediaMetadata.types'

export const MediaMetadata = ({
  title,
  year,
  duration,
  genres,
  plot,
  director,
  fileSize,
  progress,
  links,
  seasonCount,
  episodeCount,
}: MediaMetadataProps) => {
  // Format duration to hours and minutes
  const formatDuration = (seconds?: number) => {
    if (!seconds) return null
    const hours = Math.floor(seconds / 3600)
    const minutes = Math.floor((seconds % 3600) / 60)
    if (hours > 0) {
      return `${hours}h ${minutes}m`
    }
    return `${minutes}m`
  }

  const formattedDuration = formatDuration(duration)
  const primaryGenre = genres && genres.length > 0 ? genres[0] : null

  return (
    <>
      {/* Title + Watched Indicator */}
      <div className="flex items-start justify-between gap-2 mb-1">
        <h3 className="font-semibold text-sm line-clamp-2 flex-1 text-neutral-900 dark:text-neutral-50">
          {title}
        </h3>
        {progress?.is_watched && getProgressPercentage(progress) >= 95 && (
          <span className="text-green-500 shrink-0" title="Watched">
            ✓
          </span>
        )}
      </div>

      {/* Year and Duration OR Season/Episode counts */}
      <div className="flex items-center gap-2 text-xs text-neutral-600 dark:text-neutral-400 mb-2">
        {year && <span className="font-medium">{year}</span>}
        {year && (formattedDuration || seasonCount) && <span>•</span>}
        {formattedDuration && <span>{formattedDuration}</span>}
        {seasonCount !== undefined && (
          <span>
            {seasonCount} {seasonCount === 1 ? 'Season' : 'Seasons'}
          </span>
        )}
        {episodeCount !== undefined && seasonCount !== undefined && <span>•</span>}
        {episodeCount !== undefined && (
          <span>
            {episodeCount} {episodeCount === 1 ? 'Episode' : 'Episodes'}
          </span>
        )}
      </div>

      {/* Genre */}
      {primaryGenre && (
        <div className="mb-2">
          <span className="inline-block px-2 py-1 text-xs bg-neutral-100 dark:bg-neutral-800 text-neutral-700 dark:text-neutral-300 rounded">
            {primaryGenre}
          </span>
        </div>
      )}

      {/* Plot preview */}
      {plot && (
        <p className="text-xs text-neutral-600 dark:text-neutral-400 line-clamp-2 mb-2">
          {plot.substring(0, 100)}
          {plot.length > 100 ? '...' : ''}
        </p>
      )}

      {/* Director */}
      {director && (
        <p className="text-xs text-neutral-500 dark:text-neutral-500 truncate">
          <span className="font-medium">Dir:</span> {director}
        </p>
      )}

      {/* Progress or file size */}
      <div className="flex items-center justify-between text-xs text-neutral-500 dark:text-neutral-500 mt-2">
        {progress && getProgressPercentage(progress) > 0 && !progress.is_watched ? (
          <span className="text-blue-600 font-medium">
            {Math.floor(getProgressPercentage(progress))}% watched
          </span>
        ) : fileSize ? (
          <span>{(fileSize / 1024 / 1024 / 1024).toFixed(1)} GB</span>
        ) : (
          <span></span>
        )}

        {/* IMDb/TMDb links */}
        {links && (links.imdb || links.tmdb) && (
          <div className="flex gap-1">
            {links.imdb && (
              <a
                href={`https://www.imdb.com/title/${links.imdb}`}
                target="_blank"
                rel="noopener noreferrer"
                className="text-yellow-600 font-bold hover:underline"
                title="View on IMDb"
                onClick={(e) => e.stopPropagation()}
              >
                IMDb
              </a>
            )}
            {links.tmdb && (
              <a
                href={`https://www.themoviedb.org/movie/${links.tmdb}`}
                target="_blank"
                rel="noopener noreferrer"
                className="text-blue-600 font-bold hover:underline"
                title="View on TMDb"
                onClick={(e) => e.stopPropagation()}
              >
                TMDb
              </a>
            )}
          </div>
        )}
      </div>
    </>
  )
}
```

**Dependencies**: None
**Testing**: Unit tests for formatting logic
**Estimated Time**: 45 minutes

---

### Task 1.4: Create MediaProgress Component (45 minutes)

**Files to Create**:
```
/web/src/components/media/MediaProgress/MediaProgress.tsx
/web/src/components/media/MediaProgress/MediaProgress.types.ts
/web/src/components/media/MediaProgress/index.ts
```

**Implementation Details**:

**MediaProgress.types.ts**:
```typescript
export interface MediaProgressProps {
  progress?: any  // ProgressData type from existing hooks
  showPercentage?: boolean
  showWatchedIndicator?: boolean
}
```

**MediaProgress.tsx**:
```typescript
import { ProgressBar } from '@/components/media/ProgressBar'
import { getProgressPercentage } from '@/lib/utils'
import type { MediaProgressProps } from './MediaProgress.types'

export const MediaProgress = ({
  progress,
  showPercentage = true,
  showWatchedIndicator = true,
}: MediaProgressProps) => {
  if (!progress) return null

  return (
    <>
      <ProgressBar progress={progress} />
      {/* Additional progress indicators can be added here */}
    </>
  )
}
```

**Dependencies**: None (uses existing ProgressBar)
**Testing**: Unit tests for percentage calculations
**Estimated Time**: 45 minutes

---

## Phase 2: Rebuild Cards from Scratch (3-4 hours)

### Task 2.1: Rewrite MovieCard Component (1.5 hours)

**Files to Modify**:
```
/web/src/components/movies/MovieCard/MovieCard.tsx (176 lines → ~60 lines)
```

**Implementation Strategy**:
1. **DELETE** all internal badge rendering (lines 44-77)
2. **DELETE** all metadata formatting logic (lines 15-27, 86-167)
3. **REPLACE** with shared components
4. **PRESERVE** MediaCard wrapper and all hover/play behavior

**New Implementation**:
```typescript
import { useBatchProgress } from '@/lib/hooks'
import { useBadgePreferences } from '@/lib/hooks/useBadgePreferences'
import { formatResolutionLabel } from '@/lib/utils/quality'
import { MediaCard } from '@/components/media/MediaCard'
import { MediaBadges } from '@/components/media/MediaBadges'
import { MediaMetadata } from '@/components/media/MediaMetadata'
import { ProgressBar } from '@/components/media/ProgressBar'
import type { MovieCardProps } from './MovieCard.types'

const MovieCard = ({ movie, onClick }: MovieCardProps) => {
  const { progress } = useBatchProgress(movie.id)
  const { preferences } = useBadgePreferences()

  // Check if movie is newly added (within last 7 days)
  const isNew =
    movie.created_at &&
    Date.now() - new Date(movie.created_at).getTime() < 7 * 24 * 60 * 60 * 1000

  return (
    <MediaCard
      mediaId={movie.id}
      mediaType="movie"
      imageAlt={movie.title}
      aspectRatio="2/3"
      onClick={onClick}
      playIconType="play"
      badges={
        <MediaBadges
          preferences={preferences}
          badges={{
            isNew,
            isExtra: movie.is_extra,
            resolution: formatResolutionLabel(movie.height),
            contentRating: movie.content_rating,
            codec: movie.video_codec,
          }}
        />
      }
      overlays={<ProgressBar progress={progress} />}
      infoContent={
        <MediaMetadata
          title={movie.title}
          year={movie.year}
          duration={movie.duration}
          genres={movie.genre}
          plot={movie.plot}
          director={movie.director}
          fileSize={movie.file_size}
          progress={progress}
          links={{
            imdb: movie.imdb_id,
            tmdb: movie.tmdb_id,
          }}
        />
      }
    />
  )
}

export type { MovieCardProps } from './MovieCard.types'
export { MovieCard }
```

**Dependencies**: Tasks 1.1, 1.2, 1.3
**Testing**: Visual regression tests
**Estimated Time**: 1.5 hours

---

### Task 2.2: Rewrite TVShowCard Component (1.5 hours)

**Files to Modify**:
```
/web/src/components/tv/TVShowCard/TVShowCard.tsx (38 lines → ~70 lines)
```

**New Implementation**:
```typescript
import { useBadgePreferences } from '@/lib/hooks/useBadgePreferences'
import { MediaCard } from '@/components/media/MediaCard'
import { MediaBadges } from '@/components/media/MediaBadges'
import { MediaMetadata } from '@/components/media/MediaMetadata'
import type { TVShowCardProps } from './TVShowCard.types'

const TVShowCard = ({ show, onClick, onPlay }: TVShowCardProps) => {
  const { preferences } = useBadgePreferences()

  // Check if show is newly added (within last 7 days)
  const isNew =
    show.created_at &&
    Date.now() - new Date(show.created_at).getTime() < 7 * 24 * 60 * 60 * 1000

  return (
    <MediaCard
      mediaId={show.id ?? 0}
      mediaType="tv-show"
      imageAlt={show.title ?? 'TV Show'}
      imageFallback="📺"
      aspectRatio="2/3"
      onClick={onClick}
      onPlay={onPlay}
      playIconType="play"
      badges={
        <MediaBadges
          preferences={preferences}
          badges={{
            isNew,
            // Future: Add resolution, rating, codec once backend provides data
          }}
        />
      }
      infoContent={
        <MediaMetadata
          title={show.title ?? 'Unknown Show'}
          year={show.year}           // NEW - needs backend
          genres={show.genre}        // NEW - needs backend
          plot={show.plot}           // NEW - needs backend
          seasonCount={show.season_count}
          episodeCount={show.episode_count}
          links={{
            imdb: show.imdb_id,      // NEW - needs backend
            tmdb: show.tmdb_id,      // NEW - needs backend
          }}
        />
      }
    />
  )
}

export type { TVShowCardProps } from './TVShowCard.types'
export { TVShowCard }
```

**Dependencies**: Tasks 1.1, 1.2, 1.3, Backend updates (Phase 5)
**Testing**: Visual regression tests
**Estimated Time**: 1.5 hours

---

### Task 2.3: Update Music Cards for Consistency (1 hour)

**Files to Review/Modify**:
```
/web/src/components/music/ArtistCard/ArtistCard.tsx
/web/src/components/music/AlbumCard/AlbumCard.tsx
```

**Current Issues**:
- AlbumCard has redundant "ALBUM" badge (users know they're on albums page)
- Year badge appears twice (in badges AND in info section)
- Not using shared MediaBadges component

**Actions**:

1. **ArtistCard** (minimal changes):
   - Remove redundant "ARTIST" badge (users know context)
   - Verify MediaCard base component works correctly
   - Verify square aspect ratio preserved
   - Keep clean, simple design (it's already good)

2. **AlbumCard** (moderate updates):
   - Integrate MediaBadges component
   - Remove redundant "ALBUM" badge by default (optional in settings)
   - Remove duplicate year display (keep in info section only)
   - Add NEW badge for recently added albums (7 days)
   - Maintain square aspect ratio
   - Keep artist name, track count, year in info section

**New AlbumCard Implementation**:
```typescript
import { useBadgePreferences } from '@/lib/hooks/useBadgePreferences'
import { MediaCard } from '@/components/media/MediaCard'
import { MediaBadges } from '@/components/media/MediaBadges'
import type { AlbumCardProps } from './AlbumCard.types'

const AlbumCard = ({ album, onClick }: AlbumCardProps) => {
  const { preferences } = useBadgePreferences()

  // Check if album is newly added (within last 7 days)
  const isNew =
    album.created_at &&
    Date.now() - new Date(album.created_at).getTime() < 7 * 24 * 60 * 60 * 1000

  return (
    <MediaCard
      mediaId={album.id ?? 0}
      mediaType="music-album"
      imageAlt={album.album ?? 'Album'}
      imageFallback="💿"
      aspectRatio="square"
      onClick={onClick}
      playIconType="play"
      badges={
        <MediaBadges
          preferences={preferences}
          badges={{
            isNew,
            mediaType: 'ALBUM', // Only shown if user enables mediaType badge
          }}
        />
      }
      infoContent={
        <>
          <h3 className="font-semibold text-sm line-clamp-2 mb-1 text-neutral-900 dark:text-neutral-50">
            {album.album ?? 'Unknown Album'}
          </h3>
          <p className="text-xs text-neutral-600 dark:text-neutral-400 mb-2 line-clamp-1">
            {album.artist ?? 'Unknown Artist'}
          </p>
          <div className="flex items-center justify-between text-xs">
            <span className="text-neutral-500 dark:text-neutral-500">
              {album.track_count ?? 0} {album.track_count === 1 ? 'Track' : 'Tracks'}
            </span>
            {album.year && (
              <span className="px-2 py-0.5 text-[10px] font-medium bg-blue-100 dark:bg-blue-900 text-blue-700 dark:text-blue-300 rounded">
                {album.year}
              </span>
            )}
          </div>
        </>
      }
    />
  )
}
```

**New ArtistCard Implementation**:
```typescript
import { useBadgePreferences } from '@/lib/hooks/useBadgePreferences'
import { MediaCard } from '@/components/media/MediaCard'
import { MediaBadges } from '@/components/media/MediaBadges'
import type { ArtistCardProps } from './ArtistCard.types'

const ArtistCard = ({ artist, onClick }: ArtistCardProps) => {
  const { preferences } = useBadgePreferences()

  const isNew =
    artist.created_at &&
    Date.now() - new Date(artist.created_at).getTime() < 7 * 24 * 60 * 60 * 1000

  return (
    <MediaCard
      mediaId={artist.id ?? 0}
      mediaType="music-artist"
      imageAlt={artist.name ?? 'Artist'}
      imageFallback="🎤"
      aspectRatio="square"
      onClick={onClick}
      playIconType="play"
      badges={
        <MediaBadges
          preferences={preferences}
          badges={{
            isNew,
            mediaType: 'ARTIST', // Only shown if user enables mediaType badge
          }}
        />
      }
      infoContent={
        <>
          <h3 className="font-semibold text-sm line-clamp-2 mb-2 text-neutral-900 dark:text-neutral-50">
            {artist.name ?? 'Unknown Artist'}
          </h3>
          <div className="flex items-center justify-between text-xs text-neutral-600 dark:text-neutral-400">
            <span>
              {artist.album_count ?? 0} {artist.album_count === 1 ? 'Album' : 'Albums'}
            </span>
            <span>
              {artist.track_count ?? 0} {artist.track_count === 1 ? 'Track' : 'Tracks'}
            </span>
          </div>
        </>
      }
    />
  )
}
```

**Key Changes**:
- ✅ Remove redundant type badges ("ALBUM", "ARTIST") - only show if user enables in settings
- ✅ Add NEW badge for recently added music (consistency with movies/TV)
- ✅ Use shared MediaBadges component
- ✅ Clean up duplicate year display in AlbumCard
- ✅ Preserve square aspect ratio (essential for music)
- ✅ Preserve simple, focused design philosophy

**Dependencies**: Tasks 1.1, 1.2
**Testing**: Visual regression tests, verify square aspect ratio maintained
**Estimated Time**: 1 hour

---

## Phase 3: Progressive Loading System (2 hours)

### Task 3.1: Enhance MediaPoster Component (1.5 hours)

**Files to Modify**:
```
/web/src/components/media/MediaPoster/MediaPoster.tsx
```

**Enhancements to Add**:

1. **CSS aspect-ratio enforcement**:
```tsx
<div
  className={`relative ${className}`}
  style={{ aspectRatio: aspectRatio === 'square' ? '1/1' : '2/3' }}
>
```

2. **Placeholder with gradient**:
```tsx
{!imageLoaded && !imageError && (
  <div
    className="absolute inset-0 bg-gradient-to-br from-neutral-700 to-neutral-900 flex items-center justify-center animate-pulse"
  >
    <span className="text-white text-4xl opacity-50">{fallbackIcon}</span>
  </div>
)}
```

3. **Fade-in transition**:
```tsx
<img
  src={imageUrl}
  alt={alt}
  className={`object-cover transition-opacity duration-300 ${
    imageLoaded ? 'opacity-100' : 'opacity-0'
  } ${className}`}
  onLoad={() => setImageLoaded(true)}
  onError={() => setImageError(true)}
  loading="lazy"
/>
```

4. **IntersectionObserver for priority loading** (optional enhancement)

**PRESERVE**:
- All existing functionality
- Fallback icon behavior
- Error state handling
- Lazy loading attribute

**Dependencies**: None
**Testing**: Visual tests for loading states, CLS measurement
**Estimated Time**: 1.5 hours

---

### Task 3.2: Remove Layout Shift Causes (30 minutes)

**Actions**:
1. Audit all card components for dimension calculations
2. Ensure aspect-ratio CSS is set before image loads
3. Verify no height/width changes during poster load
4. Test with slow network throttling (Chrome DevTools)
5. Measure CLS (Cumulative Layout Shift) - target < 0.1

**Dependencies**: Task 3.1
**Testing**: Performance tests
**Estimated Time**: 30 minutes

---

## Phase 4: Settings UI (2 hours)

### Task 4.1: Create Display Settings Route (1.5 hours)

**Files to Create**:
```
/web/src/routes/_layout/settings.display.tsx
```

**Implementation**:
```tsx
import { createFileRoute } from '@tanstack/react-router'
import { Card, CardHeader, CardContent } from '@/components/ui/Card'
import { useBadgePreferences } from '@/lib/hooks/useBadgePreferences'

const SettingsDisplay = () => {
  const { preferences, updatePreferences } = useBadgePreferences()

  const toggleOptionalBadge = (key: keyof typeof preferences.optional) => {
    updatePreferences({
      ...preferences,
      optional: {
        ...preferences.optional,
        [key]: !preferences.optional[key],
      },
    })
  }

  return (
    <div className="p-6">
      <h1 className="text-2xl font-bold mb-6 text-neutral-900 dark:text-neutral-50">
        Display Settings
      </h1>

      <Card>
        <CardHeader>
          <h2 className="text-lg font-semibold">Library Card Display</h2>
          <p className="text-sm text-neutral-600 dark:text-neutral-400">
            Customize which badges appear on media cards
          </p>
        </CardHeader>
        <CardContent>
          <div className="space-y-6">
            {/* Always Show */}
            <div>
              <h3 className="font-medium mb-3">Always Show</h3>
              <div className="space-y-2 opacity-60">
                <label className="flex items-center gap-2">
                  <input type="checkbox" checked disabled />
                  <span>New content indicator</span>
                </label>
                <label className="flex items-center gap-2">
                  <input type="checkbox" checked disabled />
                  <span>Watched status</span>
                </label>
                <label className="flex items-center gap-2">
                  <input type="checkbox" checked disabled />
                  <span>Progress bars</span>
                </label>
              </div>
            </div>

            {/* Optional Details */}
            <div>
              <h3 className="font-medium mb-3">Optional Details</h3>
              <p className="text-xs text-neutral-500 dark:text-neutral-400 mb-3">
                Show additional technical information on media cards
              </p>
              <div className="space-y-2">
                <label className="flex items-center gap-2 cursor-pointer">
                  <input
                    type="checkbox"
                    checked={preferences.optional.resolution}
                    onChange={() => toggleOptionalBadge('resolution')}
                  />
                  <span>Video resolution (4K, 1080p, etc.)</span>
                </label>
                <label className="flex items-center gap-2 cursor-pointer">
                  <input
                    type="checkbox"
                    checked={preferences.optional.contentRating}
                    onChange={() => toggleOptionalBadge('contentRating')}
                  />
                  <span>Content rating (PG-13, R, TV-MA)</span>
                </label>
                <label className="flex items-center gap-2 cursor-pointer">
                  <input
                    type="checkbox"
                    checked={preferences.optional.codec}
                    onChange={() => toggleOptionalBadge('codec')}
                  />
                  <span>Video codec (H.264, H.265, AV1)</span>
                </label>
                <label className="flex items-center gap-2 cursor-pointer">
                  <input
                    type="checkbox"
                    checked={preferences.optional.extra}
                    onChange={() => toggleOptionalBadge('extra')}
                  />
                  <span>Extra content indicator</span>
                </label>
              </div>
            </div>
          </div>
        </CardContent>
      </Card>
    </div>
  )
}

export const Route = createFileRoute('/_layout/settings/display')({
  component: SettingsDisplay,
})
```

**Dependencies**: Task 1.1
**Testing**: Integration tests
**Estimated Time**: 1.5 hours

---

### Task 4.2: Add Settings Navigation Link (30 minutes)

**Files to Check/Modify**:
- Settings navigation (check existing settings structure)
- Add "Display" link to settings menu

**Dependencies**: Task 4.1
**Estimated Time**: 30 minutes

---

## Phase 5: Backend Integration (2-3 hours)

### Task 5.1: Identify TV Show Metadata Gaps (30 minutes)

**Action**: Audit backend to determine which fields are missing:
- year
- genre
- plot
- imdb_id
- tmdb_id
- content_rating
- created_at

**Files to Review**:
```
/internal/infrastructure/persistence/tvshow/types.go
/internal/api/handlers/tv.go
Database schema for tv_shows table
```

**Deliverable**: Document listing which fields exist vs. need to be added
**Estimated Time**: 30 minutes

---

### Task 5.2: Update Backend TV Show DTO (1-1.5 hours)

**Files to Modify** (if fields missing):
```
/internal/application/tv/dto.go
/internal/infrastructure/persistence/tvshow/repository.go
Database migration (if needed)
```

**Add these fields to TVShowSummary**:
```go
Year         int    `json:"year,omitempty"`
Genre        string `json:"genre,omitempty"`
Plot         string `json:"plot,omitempty"`
IMDbID       string `json:"imdb_id,omitempty"`
TMDbID       int    `json:"tmdb_id,omitempty"`
ContentRating string `json:"content_rating,omitempty"`
CreatedAt    string `json:"created_at,omitempty"`
```

**Dependencies**: Task 5.1
**Testing**: API integration tests
**Estimated Time**: 1-1.5 hours

---

### Task 5.3: Update Frontend Types (30 minutes)

**Files to Modify**:
```
/web/src/lib/types/tv.ts
```

**OR** run OpenAPI generator:
```bash
make api-client-gen
```

**Add to TVShowSummary interface**:
```typescript
year?: number
genre?: string
plot?: string
imdb_id?: string
tmdb_id?: number
content_rating?: string
created_at?: string
```

**Dependencies**: Task 5.2
**Testing**: TypeScript compilation
**Estimated Time**: 30 minutes

---

## Phase 6: Testing & Documentation (2 hours)

### Task 6.1: Component Unit Tests (1 hour)

**Test Files to Create**:
```
/web/src/components/media/MediaBadges/MediaBadges.test.tsx
/web/src/components/media/MediaMetadata/MediaMetadata.test.tsx
/web/src/lib/hooks/useBadgePreferences.test.ts
```

**Test Coverage**:
- MediaBadges: Essential badges always shown, optional based on preference
- MediaMetadata: Duration formatting, plot truncation, genre selection
- useBadgePreferences: localStorage save/load, defaults

**Target**: 80%+ coverage
**Estimated Time**: 1 hour

---

### Task 6.2: Integration Testing (30 minutes)

**Test Scenarios**:
1. Movie browsing with badge preferences
2. TV show browsing with new metadata
3. Settings toggle persistence
4. Cross-media consistency (movies vs TV visual weight)
5. Dark mode rendering

**Estimated Time**: 30 minutes

---

### Task 6.3: Documentation Updates (30 minutes)

**Files to Update**:
```
/docs/CHANGELOG.md
/docs/adr/0002-library-ui-consistency.md (status: Implemented)
```

**Content**:
- Breaking change notice
- New badge preference system
- Settings location
- Migration guide (clean break, no migration needed)

**Estimated Time**: 30 minutes

---

## File Inventory

### New Files (13 files)
```
web/src/lib/types/preferences.ts
web/src/lib/hooks/useBadgePreferences.ts
web/src/components/media/MediaBadges/MediaBadges.tsx
web/src/components/media/MediaBadges/MediaBadges.types.ts
web/src/components/media/MediaBadges/index.ts
web/src/components/media/MediaMetadata/MediaMetadata.tsx
web/src/components/media/MediaMetadata/MediaMetadata.types.ts
web/src/components/media/MediaMetadata/index.ts
web/src/components/media/MediaProgress/MediaProgress.tsx
web/src/components/media/MediaProgress/MediaProgress.types.ts
web/src/components/media/MediaProgress/index.ts
web/src/routes/_layout/settings.display.tsx
docs/PROJECT_PLAN_ADR_0002.md (this file)
```

### Modified Files (6 files)
```
web/src/components/movies/MovieCard/MovieCard.tsx (176 → ~60 lines)
web/src/components/tv/TVShowCard/TVShowCard.tsx (38 → ~70 lines)
web/src/components/media/MediaPoster/MediaPoster.tsx
web/src/lib/types/tv.ts
docs/adr/0002-library-ui-consistency.md
docs/CHANGELOG.md
```

### Backend Files (if needed)
```
internal/application/tv/dto.go
internal/infrastructure/persistence/tvshow/repository.go
Database migration
```

---

## Dependencies & Critical Path

```
Phase 1: Core Architecture
├─ Task 1.1: Badge Preferences ────┐
├─ Task 1.2: MediaBadges ──────────┼──► Phase 2
├─ Task 1.3: MediaMetadata ────────┤
└─ Task 1.4: MediaProgress ────────┘

Phase 2: Rebuild Cards
├─ Task 2.1: MovieCard ────────────┐
├─ Task 2.2: TVShowCard ───────────┼──► Phase 6
└─ Task 2.3: ArtistCard ───────────┘

Phase 3: Progressive Loading
└─ Independent (can run parallel)

Phase 4: Settings UI
└─ Depends on Task 1.1

Phase 5: Backend Integration
└─ Task 5.1 → 5.2 → 5.3 → Phase 2 Task 2.2

Phase 6: Testing & Docs
└─ Depends on all phases
```

---

## Execution Schedule (Recommended)

### Day 1 (6-7 hours)
- ✅ Phase 1: Core Architecture (3-4 hours)
- ✅ Phase 4: Settings UI (2 hours)
- ✅ Checkpoint 1 testing (30 min)

### Day 2 (5-6 hours)
- ✅ Phase 5 Tasks 5.1-5.2: Backend (2 hours)
- ✅ Phase 2: Rebuild Cards (3-4 hours)
- ✅ Checkpoint 2 testing (30 min)

### Day 3 (3-4 hours)
- ✅ Phase 3: Progressive Loading (2 hours)
- ✅ Phase 5 Task 5.3: Frontend types (30 min)
- ✅ Phase 6: Testing & Docs (2 hours)
- ✅ Final checkpoint (1 hour)

---

## Success Criteria

- [ ] MovieCard reduced from 176 to ~60 lines
- [ ] TVShowCard enhanced from 38 to ~70 lines with full feature parity
- [ ] All optional badges hidden by default
- [ ] User can control badges via Settings → Display
- [ ] No layout shifts during poster loading (CLS < 0.1)
- [ ] MediaCard base component unchanged (hover scale preserved)
- [ ] All tests passing (80%+ coverage)
- [ ] Documentation updated
- [ ] Dark mode works correctly

---

## Risk Mitigation

| Risk | Mitigation |
|------|------------|
| Backend TV metadata missing | Phase 5.1 identifies early; can use mock data temporarily |
| Breaking changes disrupt users | Intentional; document in changelog with clear notice |
| Performance regression | Phase 3 addresses loading; measure bundle size before/after |
| Badge preferences don't persist | Comprehensive testing in Phase 1 before dependent work |

---

**Status**: Ready for Execution
**Next Step**: Begin Phase 1, Task 1.1 (Create Badge Preference System)
