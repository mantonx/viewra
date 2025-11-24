# ADR 0002: Library UI Consistency and Browsing Experience

## Status
**Proposed** - 2025-11-23

**Breaking Change**: Yes - complete rewrite of media card components with no backward compatibility.

## Context

After analyzing the TV, movie, and music library interfaces, significant inconsistencies exist that negatively impact the user experience. While all three media types use the shared `MediaCard` component as a foundation, the implementation details vary dramatically, creating an inconsistent browsing experience.

### Current State Analysis

#### Movie Cards ([MovieCard.tsx](web/src/components/movies/MovieCard/MovieCard.tsx))
**Strengths:**
- Rich metadata display (year, duration, genre, plot preview, director)
- Multiple badge types (NEW, EXTRA, resolution, content rating, codec)
- Progress tracking with visual indicator and percentage
- IMDb/TMDb integration with clickable links
- File size display
- Watched indicator (checkmark)
- 2/3 aspect ratio (poster format)

**Issues:**
- 9 different badge types create visual clutter
- Badges always visible, no user control
- Extensive metadata in card (80+ lines of code)

#### TV Show Cards ([TVShowCard.tsx](web/src/components/tv/TVShowCard/TVShowCard.tsx))
**Strengths:**
- Clean, minimal design
- Clear season/episode counts
- 2/3 aspect ratio (poster format)
- Consistent with poster-based media

**Issues:**
- Only shows "TV SHOW" badge (minimal metadata)
- No progress tracking
- No genre, year, or plot information
- Missing rich details that movies have
- Only 38 lines of code vs 176 for movies

#### Music/Artist Cards ([ArtistCard.tsx](web/src/components/music/ArtistCard/ArtistCard.tsx))
**Strengths:**
- Square aspect ratio (appropriate for artist photos/album art)
- Clean album/track counts
- Minimal, focused design

**Issues:**
- Different aspect ratio creates visual discontinuity
- "ARTIST" badge is redundant (users know they're on music page)

### Key Problems Identified

1. **Massive Feature Disparity**: Movies have 9 badge types, rich metadata, and progress tracking. TV shows have essentially none of this despite being similarly structured (episodic content with seasons, metadata, etc.).

2. **Badge Overload**: Movies display up to 6 badges simultaneously (NEW, EXTRA, resolution, rating, codec, plus watched indicator). This creates visual noise and makes cards feel cluttered.

3. **No User Control**: Users cannot hide superfluous badges, even though many (codec, resolution, rating) are "nice to have" but not essential for browsing.

4. **Inconsistent Information Density**:
   - Movies: Year, duration, genre, plot (100 chars), director, file size, IMDb/TMDb
   - TV Shows: Season count, episode count only
   - Music: Album count, track count only

5. **DRY Violation**: Each card reimplements similar logic (formatting, badge rendering, metadata display) in different ways.

6. **Loading Experience**: No progressive loading strategy for posters, which can create jarring gaps when scrolling through large libraries.

## Decision

We will implement a **unified library browsing experience** with the following architectural decisions:

### 1. Unified Visual Language for Video Content

**Decision**: Movies and TV shows will share the same visual design, information density, and feature set.

**Rationale**:
- Both are video content consumed in similar ways
- Users expect similar metadata (year, genre, runtime, ratings)
- Both benefit from progress tracking and watched indicators
- Maintains professional consistency across the application

**Implementation**:
- TV show cards will gain: year, genre, IMDb/TMDb links, plot preview, progress tracking
- Both will use 2/3 aspect ratio (poster format)
- Shared badge system and metadata rendering logic

### 2. Music Maintains Distinct Identity

**Decision**: Music/artist cards will keep their current square aspect ratio and focused design.

**Rationale**:
- Album art and artist photos are traditionally square
- Music consumption patterns differ from video (browsing artists → albums → tracks)
- Simpler metadata is appropriate (album/track counts, not plot summaries)
- Square grid creates visual distinction that matches user mental model

**Constraints**:
- Still uses shared `MediaCard` component (maintains consistency)
- Follows same dark mode patterns and semantic utilities
- Uses same badge system architecture (even if fewer badges used)

### 3. Badge Visibility Control System

**Decision**: Implement user-configurable badge visibility with smart defaults.

**Design**:

```typescript
// User preferences (stored in localStorage)
interface BadgePreferences {
  // Essential badges (always shown, not configurable)
  essential: {
    new: true,        // NEW indicator (always visible)
    watched: true,    // Watched checkmark (always visible)
    progress: true,   // Progress bar (always visible)
  }

  // Optional badges (user can toggle, hidden by default)
  optional: {
    resolution: boolean,     // 4K, 1080p, 720p, SD
    contentRating: boolean,  // PG-13, R, TV-MA, etc.
    codec: boolean,         // H.264, H.265, AV1
    extra: boolean,         // EXTRA badge for bonus content
    mediaType: boolean,     // "TV SHOW", "MOVIE", "ARTIST" (redundant on category pages)
  }
}
```

**Default State**: All optional badges hidden by default
- Reasoning: Most users browse by thumbnail and title. Technical details (codec, resolution) are "nice to know" but clutter the UI for casual browsing.
- Power users can enable via Settings → Display → Badge Preferences

**Rationale**:
- Reduces visual clutter by default
- Gives users control over information density
- Maintains important signals (NEW, watched status, progress)
- Technical users can still access detailed metadata

### 4. Progressive Poster Loading Strategy

**Decision**: Implement seamless poster loading with no visible gaps while **preserving existing poster size and hover interactions**.

**Preserve Current Design** (DO NOT CHANGE):

- ✅ Current poster dimensions (aspect ratio 2/3 for video, square for music)
- ✅ Hover scale effect (`hover:scale-105` on cards)
- ✅ Play button overlay behavior (appears on hover, clickable separately from card)
- ✅ `HoverPlayButton` component interaction model
- ✅ Smooth transitions on hover

**Enhance with Progressive Loading**:

```typescript
// Poster loading states (enhancement only, keep existing size/hover)
1. Initial render: Show colored placeholder maintaining exact dimensions
2. Image loading: Fade in poster when loaded (300ms transition)
3. Error state: Show fallback icon with same dimensions
4. Skeleton state: Never show empty gaps or broken layout
```

**Implementation**:

- MediaPoster component already has fallback icon support (keep as-is)
- Add CSS `aspect-ratio` to maintain dimensions during load
- Use `loading="lazy"` for off-screen images
- Implement IntersectionObserver for batch image priority
- Add fade-in animation for loaded images
- **Do NOT change**: Card hover scale, play button positioning, transition timing

**Rationale**:

- Eliminates jarring layout shifts during loading
- Improves perceived performance
- Maintains professional polish during scroll
- **Preserves working UX**: Existing hover and play interactions already feel great
- Reduces cumulative layout shift (CLS) metric

### 5. Shared Metadata Rendering Components

**Decision**: Extract shared logic into reusable components.

**New Components**:

```typescript
// Shared badge rendering
<MediaBadges
  media={movie | tvShow}
  preferences={badgePreferences}
  badges={{
    isNew: boolean,
    isExtra: boolean,
    resolution: string,
    contentRating: string,
    codec: string,
  }}
/>

// Shared metadata display
<MediaMetadata
  title={string}
  year={number}
  duration={number}
  genres={string[]}
  plot={string}
  links={{ imdb?: string, tmdb?: string }}
  progress={ProgressData}
/>

// Shared progress indicator
<MediaProgress
  progress={ProgressData}
  showPercentage={boolean}
  showWatchedIndicator={boolean}
/>
```

**Benefits**:
- DRY: Single source of truth for badge logic
- Consistency: Same formatting across all media types
- Maintainability: Changes apply to all cards automatically
- Type safety: Shared interfaces prevent inconsistencies

### 6. Enhanced TV Show Metadata

**Decision**: TV shows will display rich metadata matching movies.

**New TV Show Card Features**:
- **Year**: First aired year (from show metadata)
- **Genre**: Primary genre badge (Drama, Comedy, etc.)
- **Plot**: 100-character preview (from show description)
- **IMDb/TMDb links**: Clickable links in footer
- **Progress tracking**: Shows progress on currently watching season/episode
- **Watched indicator**: Checkmark when all episodes watched
- **Season/episode counts**: Retained (distinguishes from movies)

**Data Requirements**:
- Backend may need to expose additional TV show metadata
- Progress tracking requires "next episode" logic
- Watched status requires aggregating episode watch states

### 7. Settings Integration

**Decision**: Add "Display Preferences" section to Settings page.

**UI Location**: Settings → Display → Library Cards

**Controls**:
```
Library Card Display

Badge Visibility
├─ Always Show
│  ├─ [x] New content indicator
│  ├─ [x] Watched status
│  └─ [x] Progress bars
│
└─ Optional Details (show only when needed)
   ├─ [ ] Video resolution (4K, 1080p, etc.)
   ├─ [ ] Content rating (PG-13, R, TV-MA)
   ├─ [ ] Video codec (H.264, H.265, AV1)
   └─ [ ] Extra content indicator

Poster Loading
├─ [x] Smooth fade-in animations
├─ [x] Maintain aspect ratio during load
└─ [ ] Show placeholder colors (faster, less bandwidth)
```

## Consequences

### Positive

✅ **Consistency**: TV shows and movies have unified visual language
✅ **User Control**: Badge visibility preferences reduce clutter
✅ **Better DRY**: Shared components eliminate duplicate code
✅ **Smooth Browsing**: No poster loading gaps or layout shifts
✅ **Scalability**: New media types can reuse the same components
✅ **Accessibility**: Consistent patterns improve screen reader navigation
✅ **Performance**: Progressive loading reduces initial render time
✅ **Maintainability**: Single source of truth for badge logic

### Negative

⚠️ **Breaking Change**: Complete rewrite of MovieCard and TVShowCard components
⚠️ **Backend Changes**: Will need additional TV show metadata endpoints for full feature parity
⚠️ **User Impact**: Existing users will see immediate visual changes (may require documentation/changelog)

### Neutral

ℹ️ **Bundle Size**: Net reduction (~2KB) due to eliminated duplication despite new settings UI
ℹ️ **No Backward Compatibility**: Old card implementations will be completely replaced

## Implementation Plan

**Approach**: Clean break - complete rewrite with no backward compatibility concerns.

### Phase 1: Core Architecture (3-4 hours)

1. **Delete old implementations**: Remove MovieCard, TVShowCard internals completely
2. **Create shared foundation**:
   - `MediaBadges` component with preference support
   - `MediaMetadata` component for unified info display
   - `MediaProgress` component for watch tracking
   - Badge preference TypeScript interfaces
   - localStorage preference hook (`useBadgePreferences`)

### Phase 2: Rebuild Cards from Scratch (3-4 hours)

1. **MovieCard rewrite**:
   - Use shared components exclusively
   - Clean, minimal implementation (~60 lines vs current 176)
   - Badge preferences integrated from day one
   - **Preserve**: MediaCard base component with hover scale, play button overlay, all current transitions

2. **TVShowCard rewrite**:
   - Full feature parity with movies
   - Year, genre, plot, IMDb/TMDb, progress tracking
   - Same visual language as movies
   - ~70 lines (vs current minimal 38)
   - **Preserve**: Same MediaCard base, hover effects, play button behavior as movies

3. **ArtistCard minimal touch**:
   - Verify compatibility with shared MediaCard
   - Ensure badge system works (even if unused)
   - **Preserve**: Square aspect ratio, all existing hover/click behavior
   - No major changes needed

### Phase 3: Progressive Loading System (2 hours)

1. Enhance MediaPoster component (preserve existing behavior):
   - Add aspect-ratio CSS for loading states
   - Add fade-in transitions for loaded images
   - Add placeholder color support
   - Implement IntersectionObserver batch loading
   - **Keep existing**: All hover effects, scale transitions, play button overlay logic
2. Remove any old loading logic that causes layout shifts (but keep working UX)

### Phase 4: Settings UI (2 hours)

1. Add "Display" section to Settings page
2. Badge preference toggle UI
3. Live preview of changes
4. No migration code needed (fresh start)

### Phase 5: Backend Integration (2-3 hours)

1. Identify TV show metadata gaps
2. Add/update backend endpoints for:
   - Show year, genre, plot
   - IMDb/TMDb IDs
   - Progress tracking for shows
3. Update API types

### Phase 6: Testing & Documentation (2 hours)

1. Remove old test suites for deleted code
2. Write new tests for shared components
3. Integration tests for all three media types
4. Update user documentation
5. Create migration guide for any custom card usage

**Total Estimated Time**: 14-17 hours

**Breaking Changes Accepted**: This is a complete architectural rewrite. Old card implementations are deleted, not refactored.

## Testing Strategy

### Unit Tests
- Badge preference save/load logic
- MediaBadges renders correct badges based on preferences
- MediaMetadata formats data correctly
- MediaProgress calculates percentages accurately

### Integration Tests
- Movie cards respect badge preferences
- TV show cards display all new metadata
- Music cards maintain existing behavior
- Settings changes propagate to all library pages

### Visual Regression Tests
- Capture screenshots before/after for all three media types
- Verify poster loading states (placeholder → loaded → error)
- Test dark mode rendering
- Validate responsive behavior (mobile, tablet, desktop)

### Performance Tests
- Measure scroll performance with 1000+ items
- Verify no layout shifts during poster loading
- Test badge preference toggle responsiveness
- Measure bundle size impact

## Migration Notes

### For Developers

**Breaking Change**: Old MovieCard and TVShowCard implementations are completely deleted.

**Old Code (DO NOT USE)**:
```tsx
// ❌ DELETED - This no longer exists
import { MovieCard } from '@/components/movies/MovieCard'

// All internal badge rendering logic removed
// All metadata formatting removed
// 176 lines deleted
```

**New Architecture** (required for all media cards):
```tsx
import { MediaBadges, MediaMetadata, MediaProgress } from '@/components/media'
import { useBadgePreferences } from '@/lib/hooks'

// Every card now uses shared components
// MediaCard base component is PRESERVED (hover scale, play button overlay, etc.)
const MovieCard = ({ movie }: MovieCardProps) => {
  const { preferences } = useBadgePreferences()

  return (
    <MediaCard
      mediaId={movie.id}
      mediaType="movie"
      aspectRatio="2/3"              // Preserved: existing poster size
      onClick={handleClick}
      playIconType="play"             // Preserved: existing play button
      // PRESERVED: All hover effects, scale transitions from base MediaCard
      badges={
        <MediaBadges
          preferences={preferences}
          isNew={isWithinDays(movie.created_at, 7)}
          resolution={formatResolution(movie.height)}
          contentRating={movie.content_rating}
          codec={movie.video_codec}
        />
      }
      infoContent={
        <MediaMetadata
          title={movie.title}
          year={movie.year}
          duration={movie.duration}
          genres={movie.genre}
          plot={movie.plot}
          progress={useProgress(movie.id)}
        />
      }
    />
  )
}

// ~60 lines instead of 176
// Zero duplicate logic
// Type-safe by default
// Preserves all working hover/play interactions
```

**Badge Preference Management**:
```tsx
const { preferences, updatePreferences } = useBadgePreferences()

// In settings
<Toggle
  checked={preferences.optional.resolution}
  onChange={(enabled) => updatePreferences({
    optional: { ...preferences.optional, resolution: enabled }
  })}
/>
```

### For Users

**Breaking Change**: Completely new card designs for movies and TV shows.

**Before**:

- Movies: 9+ badges visible, cluttered
- TV shows: Minimal info (just season/episode counts)
- Inconsistent experience

**After**:

- Movies & TV: Unified visual language, clean default appearance
- Essential badges only (NEW, watched, progress) by default
- Optional badges hidden but available in Settings
- Rich metadata on both (year, genre, plot, IMDb/TMDb)
- Smooth poster loading with no gaps
- **Preserved**: Same poster sizes, hover scale effect, play button behavior

**What Stays the Same**:

- Poster dimensions and aspect ratios
- Card hover scale effect (grows slightly on hover)
- Play button appears on hover and works separately from card click
- All transition timings and animations

**What Changes**:

- Badge visibility (cleaner by default)
- TV shows gain rich metadata
- No layout shifts during image loading

**Migration**: None - this is a breaking change. Users will immediately see the new design.

## Future Enhancements (Phase 2+)

### Advanced Badge Customization
- Per-media-type preferences (e.g., show resolution for movies, hide for TV)
- Badge position customization (top-left, top-right, bottom)
- Custom badge colors/styles
- Badge grouping (technical vs metadata vs status)

### Enhanced Poster Loading
- Dominant color extraction for placeholders
- Blur-up technique (tiny preview → full image)
- WebP/AVIF format support with fallbacks
- Client-side image caching with Service Worker

### Metadata Expansion
- Cast/crew quick preview on hover
- Ratings aggregation (IMDb + TMDb + local)
- Related content suggestions
- Customizable metadata display fields

### List View Improvements
- Sortable columns with metadata
- Bulk actions (mark watched, add to playlist)
- Inline editing for metadata
- Export to CSV/JSON

## References

### Files to Modify
- **New Files**:
  - `web/src/components/media/MediaBadges/MediaBadges.tsx`
  - `web/src/components/media/MediaMetadata/MediaMetadata.tsx`
  - `web/src/components/media/MediaProgress/MediaProgress.tsx`
  - `web/src/lib/hooks/useBadgePreferences.ts`
  - `web/src/lib/types/preferences.ts`

- **Modified Files**:
  - [MovieCard.tsx](web/src/components/movies/MovieCard/MovieCard.tsx): Migrate to shared components
  - [TVShowCard.tsx](web/src/components/tv/TVShowCard/TVShowCard.tsx): Add rich metadata
  - [ArtistCard.tsx](web/src/components/music/ArtistCard/ArtistCard.tsx): Minimal changes (maintain identity)
  - [MediaPoster.tsx](web/src/components/media/MediaPoster/MediaPoster.tsx): Add progressive loading
  - Settings page: Add Display → Library Cards section

### Design System Alignment
- Follows ADR 0001 semantic utilities and dark mode patterns
- Uses `neutral-*` color scale consistently
- Integrates with existing `MediaCard` component
- Maintains Tailwind v4 CSS-native token system

### Dependencies
- None required (uses existing React, Tailwind, Lucide icons)
- Optional: `blurhash` for advanced placeholder images (future enhancement)

## Decision Makers
- Architecture: Claude Code (AI Assistant)
- Review: Development Team
- Approval: Project Lead

## Date
2025-11-23

---

## Appendix: Badge Taxonomy

### Essential Badges (Always Visible)
| Badge | Purpose | Visual | Shown On |
|-------|---------|--------|----------|
| NEW | Content added within 7 days | Green gradient | All media types |
| Watched | 95%+ completion | Green checkmark | Movies, TV episodes |
| Progress | Current watch position | Blue progress bar | Movies, TV episodes |

### Optional Badges (Hidden by Default)
| Badge | Purpose | Visual | Shown On | User Value |
|-------|---------|--------|----------|------------|
| Resolution | Video quality | Black badge (4K, 1080p, 720p, SD) | Movies, TV | Quality-conscious users |
| Content Rating | Age appropriateness | Gray badge (PG-13, R, TV-MA) | Movies, TV | Parents, content filtering |
| Codec | Video encoding | Colored badge (H.264, H.265, AV1) | Movies, TV | Tech enthusiasts |
| EXTRA | Bonus content | Yellow badge | Movies, TV | Completionists |
| Media Type | Category indicator | Black badge (MOVIE, TV SHOW, ARTIST) | All | Redundant on category pages |

### Badge Visibility Rules
```typescript
function shouldShowBadge(badge: BadgeType, preferences: BadgePreferences, context: Context): boolean {
  // Essential badges always shown
  if (badge in preferences.essential) {
    return true
  }

  // Media type badge only shown in search/mixed contexts
  if (badge === 'mediaType') {
    return context.isMixedMediaView
  }

  // Optional badges respect user preference
  return preferences.optional[badge] ?? false // Default: hidden
}
```

---

**Status**: Ready for implementation
**Estimated Impact**: High (affects all library browsing experiences)
**Risk Level**: Medium (significant refactor, but incremental rollout possible)
