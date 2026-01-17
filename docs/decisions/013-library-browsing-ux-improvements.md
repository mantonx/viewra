# ADR 013: Library Browsing UX Improvements

**Status**: Accepted
**Date**: 2025-11-18
**Deciders**: Development Team
**Context**: Phase 5 - Library Browsing Experience Enhancement

## Context and Problem Statement

The current library browsing experience has significant performance and UX issues that impact usability with large media collections:

### Critical Performance Problems

1. **No Pagination - Loading All Data at Once**
   - Movies: Fetches ALL movies in a single request
   - TV Shows: Fetches ALL episodes, aggregates client-side
   - Music: Fetches ALL tracks (potentially 50,000+), aggregates in-memory
   - Impact: 10,000 movies = ~20MB JSON payload, slow page loads, high memory usage

2. **N+1 Query Pattern for Images**
   - Each card component makes individual API calls for images
   - 100 movies = 100 separate image API requests
   - Additional 100 requests for watch progress per movie
   - Causes: Network congestion, slow rendering, poor perceived performance

3. **Client-Side Aggregation Overhead**
   - Music artists: Backend loads ALL tracks, builds map in Go memory
   - TV shows: Groups ALL episodes by show title client-side
   - Impact: CPU-intensive operations block rendering

4. **No Virtual Scrolling**
   - All items rendered at once in DOM
   - Hundreds of heavy card components with images, badges, progress bars
   - Impact: Slow scrolling, high memory consumption

5. **Missing UX Features**
   - No sorting options (title, year, date added, rating)
   - No advanced filters (genre, year range, quality, watched/unwatched)
   - No grid size preferences
   - No infinite scroll for smooth browsing
   - Search not debounced, triggers re-renders on every keystroke

### Current State Analysis

**Frontend (Explored):**
- Routes: `movies.index.tsx`, `tv.index.tsx`, `music.index.tsx` - All load complete datasets
- Components: `MovieCard`, `TVShowCard`, `ArtistCard` - Heavy components with N+1 queries
- Hooks: `useMediaImages`, `useMediaProgress` - Individual queries per card
- No pagination, no virtual scrolling, client-side filtering only

**Backend (Explored):**
- No pagination support in any list endpoint
- SQL queries lack `LIMIT`/`OFFSET` clauses
- Some optimized queries exist but aren't being used:
  - `GetTVShowsWithCountsByLibrary` - TV shows aggregation (unused)
  - `ListArtistsByLibrary` - Music artist aggregation (unused)
- Good database indexing already in place for filters/sorting

## Decision Drivers

1. **Performance**: Large libraries (1000+ movies, 10,000+ music tracks) must load quickly
2. **UX**: Smooth browsing experience with infinite scroll, instant feedback
3. **Scalability**: Architecture must support growing media collections
4. **Maintainability**: Clean separation between pagination logic and business logic
5. **Progressive Enhancement**: Improve experience without breaking existing functionality

## Considered Options

### Option 1: Backend Pagination + Frontend Infinite Scroll (CHOSEN)

**Backend:**
- Add `limit`/`offset` parameters to all list endpoints
- Implement paginated SQL queries with `LIMIT`/`OFFSET`
- Return metadata: `{ items: [], total: number, limit: number, offset: number, hasMore: bool }`
- Default page size: 50 items, max: 200

**Frontend:**
- Use TanStack Query's `useInfiniteQuery` for automatic pagination
- Intersection Observer for scroll detection
- Load 50 items at a time, append to list
- Virtual scrolling for very large lists (optional enhancement)

**Pros:**
- ✅ Solves root cause: reduces payload sizes by 95%+
- ✅ Industry standard approach (Netflix, Spotify all use this)
- ✅ TanStack Query has excellent infinite query support
- ✅ Backend indexes already exist for efficient pagination
- ✅ Backward compatible (can default to limit=1000 for old clients)

**Cons:**
- ❌ Requires backend + frontend changes
- ❌ ~15-20 files to modify across stack
- ❌ Need to handle "load more" loading states

### Option 2: Client-Side Virtual Scrolling Only

**Approach:**
- Keep loading all data
- Use `@tanstack/react-virtual` to render only visible items

**Pros:**
- ✅ Simpler implementation (frontend only)
- ✅ No backend changes needed

**Cons:**
- ❌ Still loads 20MB JSON for large libraries
- ❌ Doesn't solve network/memory issues
- ❌ Initial load still very slow
- ❌ Filtering/sorting still expensive
- ❌ Not a real solution, just a band-aid

### Option 3: Cursor-Based Pagination

**Approach:**
- Use opaque cursors instead of offset/limit
- Better performance for very large datasets

**Pros:**
- ✅ More efficient for huge datasets
- ✅ Prevents duplicate items during concurrent updates
- ✅ Better for real-time data

**Cons:**
- ❌ More complex to implement
- ❌ Harder to implement "jump to page" features
- ❌ Overkill for current scale (<100k items)
- ❌ Can migrate to this later if needed

## Decision Outcome

**Chosen Option: Backend Pagination + Frontend Infinite Scroll (Option 1)**

### Rationale

1. **Addresses Root Cause**: Solves the fundamental problem of loading too much data
2. **Industry Standard**: Proven approach used by all major media platforms
3. **Best Performance**: 95%+ reduction in payload sizes and memory usage
4. **Excellent Tooling**: TanStack Query's `useInfiniteQuery` makes frontend trivial
5. **Future-Proof**: Can add cursor-based pagination later if needed
6. **Clean Architecture**: Pagination is a cross-cutting concern handled properly

### Implementation Strategy

**Phase 1: Backend Pagination (Critical Path - 8-10 hours)**
1. Update repository interfaces with `PaginationParams`
2. Add paginated SQL queries with `LIMIT`/`OFFSET`
3. Update use cases to accept pagination parameters
4. Update handlers to parse `limit`/`offset` query params
5. Update response DTOs to include pagination metadata

**Phase 2: Frontend Infinite Scroll (6-8 hours)**
1. Replace `useQuery` with `useInfiniteQuery` in list pages
2. Implement intersection observer for "load more" trigger
3. Update components to handle paginated data structure
4. Add loading states for pagination
5. Preserve existing search/filter functionality

**Phase 3: Image Loading Optimization (4-6 hours)**
1. Implement batch image endpoint: `POST /api/images/batch { ids: [...] }`
2. Prefetch images for visible items only
3. Lazy load images with Intersection Observer
4. Add image placeholders/skeletons

**Phase 4: UX Enhancements (6-8 hours)**
1. Add sorting dropdown (title, year, date added, rating)
2. Debounce search input (300ms)
3. Add grid size toggle (compact/normal/large)
4. Persist preferences in localStorage
5. Add advanced filters (genre, year range, quality)

**Phase 5: Performance Polish (Optional - 4-6 hours)**
1. Implement virtual scrolling for very large lists
2. Add backend search endpoint (full-text)
3. Optimize progress data loading (batch or include in list response)
4. Add request deduplication

### Quick Wins (Can Do First - 2-3 hours)

1. **Fix Music Artists N+1**: Use existing `ListArtistsByLibrary` query
2. **Fix TV Shows N+1**: Use existing `GetTVShowsWithCountsByLibrary` query
3. **Debounce Search**: Add 300ms debounce to search input
4. **Image Placeholders**: Add skeleton loading states

These provide immediate performance improvement while working on full pagination.

## Technical Specification

### Backend Changes

**1. Repository Interface** (`internal/domain/media/repository.go`)

```go
type PaginationParams struct {
    Limit  int
    Offset int
}

type PaginationMetadata struct {
    Total   int64
    Limit   int
    Offset  int
    HasMore bool
}

type MovieRepository interface {
    ListMoviesByLibrary(ctx context.Context, libraryID int64, pagination *PaginationParams) ([]*Movie, error)
    CountMoviesByLibrary(ctx context.Context, libraryID int64) (int64, error)
    // ... existing methods
}
```

**2. SQL Queries** (e.g., `queries/sqlite/movies.sql`)

```sql
-- name: ListMoviesByLibraryPaginated :many
SELECT m.*, med.*
FROM movies m
JOIN media med ON m.media_id = med.id
WHERE med.library_id = ?
ORDER BY m.sort_title, med.title
LIMIT ? OFFSET ?;

-- name: CountMoviesByLibrary :one
SELECT COUNT(*)
FROM movies m
JOIN media med ON m.media_id = med.id
WHERE med.library_id = ?;
```

**3. Use Case Response** (`application/movies/dto.go`)

```go
type ListMoviesResponse struct {
    Movies   []*MovieResponse     `json:"movies"`
    Total    int64                `json:"total"`
    Limit    int                  `json:"limit"`
    Offset   int                  `json:"offset"`
    HasMore  bool                 `json:"hasMore"`
}
```

**4. Handler** (`api/handlers/movies.go`)

```go
func (h *MoviesHandler) List(c *gin.Context) {
    libraryID := parseLibraryID(c)
    limit := parseIntOrDefault(c.Query("limit"), 50)
    offset := parseIntOrDefault(c.Query("offset"), 0)

    // Cap maximum limit
    if limit > 200 {
        limit = 200
    }

    resp, err := h.listMovies.Execute(c.Request.Context(), libraryID, limit, offset)
    if err != nil {
        handleError(c, err)
        return
    }

    c.JSON(http.StatusOK, resp)
}
```

### Frontend Changes

**1. Infinite Query Hook** (`web/src/routes/_layout/movies.index.tsx`)

```typescript
const {
  data,
  fetchNextPage,
  hasNextPage,
  isFetchingNextPage,
  isLoading,
} = useInfiniteQuery({
  queryKey: ['movies', libraryId],
  queryFn: ({ pageParam = 0 }) =>
    moviesApi.listMovies(libraryId, {
      limit: 50,
      offset: pageParam,
    }),
  getNextPageParam: (lastPage) =>
    lastPage.hasMore ? lastPage.offset + lastPage.limit : undefined,
  initialPageParam: 0,
});

// Flatten pages
const movies = data?.pages.flatMap((page) => page.movies) ?? [];
```

**2. Intersection Observer** (Load More Trigger)

```typescript
const loadMoreRef = useRef<HTMLDivElement>(null);

useEffect(() => {
  const observer = new IntersectionObserver(
    (entries) => {
      if (entries[0].isIntersecting && hasNextPage && !isFetchingNextPage) {
        fetchNextPage();
      }
    },
    { threshold: 0.5 }
  );

  if (loadMoreRef.current) {
    observer.observe(loadMoreRef.current);
  }

  return () => observer.disconnect();
}, [hasNextPage, isFetchingNextPage, fetchNextPage]);
```

**3. Render with Load More Indicator**

```tsx
<div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6 gap-4">
  {movies.map((movie) => (
    <MovieCard key={movie.id} movie={movie} />
  ))}
</div>

{/* Load More Trigger */}
{hasNextPage && (
  <div ref={loadMoreRef} className="flex justify-center py-8">
    {isFetchingNextPage ? (
      <Spinner />
    ) : (
      <Button onClick={() => fetchNextPage()}>Load More</Button>
    )}
  </div>
)}
```

## Files to Modify

### Backend (~15 files)

**Domain Layer:**
- `internal/domain/media/repository.go` - Add pagination params

**SQL Queries:**
- `internal/infrastructure/database/queries/sqlite/movies.sql`
- `internal/infrastructure/database/queries/sqlite/tv_shows.sql`
- `internal/infrastructure/database/queries/sqlite/music_tracks.sql`

**Repositories:**
- `internal/infrastructure/persistence/movie/repository.go`
- `internal/infrastructure/persistence/tvshow/repository.go`
- `internal/infrastructure/persistence/music/repository.go`

**Use Cases:**
- `internal/application/movies/list_movies.go`
- `internal/application/movies/dto.go`
- `internal/application/tv/list_shows.go`
- `internal/application/tv/dto.go`
- `internal/application/music/list_artists.go`
- `internal/application/music/dto.go`

**Handlers:**
- `internal/api/handlers/movies.go`
- `internal/api/handlers/tv.go`
- `internal/api/handlers/music.go`

### Frontend (~10 files)

**Routes:**
- `web/src/routes/_layout/movies.index.tsx`
- `web/src/routes/_layout/tv.index.tsx`
- `web/src/routes/_layout/music.index.tsx`
- `web/src/routes/_layout/tv.$showId.index.tsx` (episodes list)
- `web/src/routes/_layout/music.artists.$artistId.tsx` (albums list)

**API Clients:**
- `web/src/lib/api/movies.ts`
- `web/src/lib/api/tv.ts`
- `web/src/lib/api/music.ts`

**Types:**
- `web/src/lib/types/movies.ts`
- `web/src/lib/types/tv.ts`
- `web/src/lib/types/music.ts`

## Success Metrics

### Performance Targets

- ✅ Initial page load: < 1 second (vs current 5-10 seconds for large libraries)
- ✅ Payload size: < 500KB per page (vs current 20MB for 10,000 movies)
- ✅ Time to interactive: < 2 seconds
- ✅ Smooth scrolling: 60fps maintained during scroll
- ✅ Memory usage: < 200MB for browsing experience

### UX Targets

- ✅ Infinite scroll feels seamless (no loading jank)
- ✅ Search results appear within 300ms
- ✅ Grid size preferences persist across sessions
- ✅ Sorting changes apply instantly (client-side for loaded items)
- ✅ Loading states clearly communicate progress

### Technical Targets

- ✅ All list endpoints support pagination
- ✅ Backward compatible with clients that don't use pagination
- ✅ Database queries optimized with proper indexes
- ✅ No N+1 query patterns
- ✅ Image loading optimized (batch or lazy)

## Migration Strategy

### Phase 1: Backend Foundation (No Breaking Changes)

1. Add pagination support to repositories (optional params)
2. Add paginated SQL queries
3. Update handlers to accept `limit`/`offset` (with defaults)
4. Keep existing behavior by defaulting to `limit=1000` if not specified

**Result:** Backend supports pagination, old clients still work

### Phase 2: Frontend Migration (Incremental)

1. Start with Movies page (most common use case)
2. Validate pagination works correctly
3. Migrate TV Shows page
4. Migrate Music page
5. Migrate detail pages (episodes, albums)

**Result:** Each page can be migrated and tested independently

### Phase 3: Performance Optimization

1. Reduce default limit to 50 after frontend migration
2. Implement image batch loading
3. Add virtual scrolling for very large lists
4. Optimize progress data loading

**Result:** Full performance benefits realized

### Phase 4: UX Polish

1. Add sorting/filtering
2. Add grid size preferences
3. Improve loading states
4. Add keyboard shortcuts

**Result:** Complete, polished browsing experience

## Testing Strategy

### Backend Tests

1. **Pagination Logic:**
   - Test limit/offset boundary conditions
   - Test hasMore calculation
   - Test total count accuracy
   - Test with empty results

2. **SQL Queries:**
   - Verify LIMIT/OFFSET correctness
   - Test ordering consistency across pages
   - Validate index usage (EXPLAIN QUERY PLAN)

3. **Backward Compatibility:**
   - Test endpoints without limit/offset params
   - Verify default behavior

### Frontend Tests

1. **Infinite Query:**
   - Test initial load
   - Test loading next page
   - Test end of results
   - Test error handling

2. **Intersection Observer:**
   - Test load trigger behavior
   - Test edge cases (rapid scrolling)

3. **Client-Side Filtering:**
   - Test search with pagination
   - Test filtering with infinite scroll

## Risks and Mitigations

### Risk 1: Complex Client-Side Filtering with Pagination

**Problem:** Client-side search/filter on paginated data only searches loaded items

**Mitigation:**
- **Short-term:** Make it clear in UI that search is "within loaded items" or add "Load All" option
- **Long-term:** Implement backend search endpoint that works across all items

### Risk 2: Scroll Position Loss on Navigation

**Problem:** Browser back/forward might lose scroll position

**Mitigation:**
- TanStack Router supports scroll restoration
- Can also cache pages in TanStack Query for instant back navigation

### Risk 3: Inconsistent Results During Concurrent Updates

**Problem:** Items added/removed during pagination might cause duplicates/gaps

**Mitigation:**
- **Short-term:** Acceptable for media libraries (rare concurrent updates)
- **Long-term:** Switch to cursor-based pagination if needed

### Risk 4: Performance on Low-End Devices

**Problem:** Infinite scroll with many items might still slow down on old devices

**Mitigation:**
- Implement virtual scrolling (Phase 5)
- Add configurable page size in settings
- Provide "simple list" view option

## UX Polish & User Experience Enhancements

Beyond performance, the library browsing experience needs significant polish for a premium feel. Based on comprehensive UX analysis, here are critical improvements:

### Current Strengths
- ✅ Rich metadata display on cards
- ✅ Excellent progress tracking with visual indicators
- ✅ Consistent design system with tokens
- ✅ Good error/empty states
- ✅ Smart resume playback
- ✅ Keyboard shortcuts in audio player

### Critical Gaps Identified

**Accessibility (Major Gap):**
- Only 37 ARIA labels/roles across entire codebase
- Cards not keyboard navigable (no tabIndex, no focus styles)
- Missing semantic HTML roles
- No screen reader announcements

**Visual Polish:**
- Inconsistent hover effects (MovieCard scales, TVShowCard doesn't)
- No page transition animations
- No skeleton loading states for grids
- Abrupt state changes
- Missing focus visible styles

**User Feedback:**
- No toast/notification system for actions
- Missing tooltips for badges and truncated text
- No loading states for card actions
- No confirmation dialogs

**Navigation & Wayfinding:**
- No breadcrumb trails
- Filter state not preserved in URL
- No "Continue Watching" section
- No recently viewed items

**Interaction Patterns:**
- No context menus
- No bulk actions/multi-select
- No quick actions on cards (mark watched, add to playlist)
- Limited keyboard navigation

**Information Architecture:**
- Limited filtering (only search + library)
- No sorting controls
- No grid/list view toggle
- No grid density options

### Recommended UX Improvements (Integrated into Phase 5)

These enhancements are incorporated into Phase 5.4 (UX Enhancements) and expanded below:

#### High Priority - Core UX (6-8 hours)

**1. Accessibility Foundation**
- Add proper ARIA labels to all interactive elements
- Implement keyboard navigation for card grids (Tab, Arrow keys)
- Add focus visible styles (focus-visible:ring-2)
- Add semantic HTML roles (grid, gridcell, article)
- Screen reader announcements for state changes
- Skip navigation links

**2. Consistent Interaction Patterns**
- Standardize hover effects (all cards scale uniformly)
- Add proper focus styles to all cards
- Add active/pressed states
- Ensure 44×44px minimum touch targets
- Add hover tooltips for badges and truncated text

**3. Visual Feedback & States**
- Implement toast notification system for actions
- Add skeleton loading states for grids (stagger animation)
- Page transition animations (fade in/out)
- Loading states for card actions
- Success/error feedback for all interactions

**4. Navigation Improvements**
- Add breadcrumb component for navigation context
- Preserve filter/sort state in URL query params
- Implement "Continue Watching" section on home page
- Add "Recently Added" rows
- Page title updates for browser tabs

**5. Smart Defaults & Preferences**
- Remember grid density preference (localStorage)
- Remember sort/filter preferences per media type
- Default to "Continue Watching" view
- Smart empty states with actionable suggestions

#### Medium Priority - Enhanced UX (4-6 hours)

**6. Advanced Filtering & Sorting**
- Genre multi-select with checkbox dropdown
- Year range slider
- Quality filter (4K/HD/SD)
- Watched/Unwatched toggle
- Sort dropdown: Title, Year, Date Added, Rating (A-Z/Z-A)
- "Clear all filters" button
- Filter pill display with remove buttons

**7. View Options & Customization**
- Grid/List view toggle
- Grid density control (Compact/Normal/Comfortable)
- Poster size adjustment
- Show/hide metadata options
- Persist view preferences

**8. Quick Actions & Context Menus**
- Hover overlay with play button
- Card context menu (right-click):
  - Mark as Watched/Unwatched
  - Add to Playlist
  - View Details
  - Open File Location
- Touch-friendly long-press menus
- Keyboard shortcut hints (?)

**9. Keyboard Shortcuts**
- Global search: Cmd/Ctrl + K
- Focus search: /
- Navigate cards: Arrow keys
- Quick actions: Enter, Space
- Bulk select: Shift + Arrow
- Show shortcuts: ?
- Escape to clear filters

**10. Enhanced Card Design**
- Image lazy loading with blur-up
- Progressive image loading
- "NEW" badge for recently added
- Prominent quality badges (4K, Dolby Vision, HDR)
- User-friendly codec names ("H.265" not "HEVC")
- Card action buttons on hover
- Star rating display and quick rate

#### Nice to Have - Delightful Details (2-4 hours)

**11. Animations & Transitions**
- Stagger animation when grid items load
- Smooth page transitions (150ms fade)
- Card hover with spring animation
- Loading skeleton → content fade
- Filter apply animation
- Sort change transition

**12. Personalization**
- Continue Watching carousel at top
- Recommended For You section
- Recently Added section
- Watch Again suggestions
- Jump back to last viewed position

**13. Mobile Optimizations**
- Bottom navigation bar (tab bar pattern)
- Pull-to-refresh
- Swipe gestures (back/forward)
- Haptic feedback on actions
- Larger touch targets
- Mobile-specific card layouts

**14. Power User Features**
- Bulk selection (checkbox mode)
- Batch mark as watched
- Bulk add to playlist
- Multi-delete
- Export library list
- Advanced search with operators

**15. Micro-delights**
- Loading message variety ("Reticulating splines...")
- Achievement system (milestones)
- Watch stats and insights
- Fun facts during loading
- Easter eggs
- Celebration animations (first watch, 100th movie)

### Code Quality Assessment & Refactoring Needs

Based on comprehensive frontend and backend code audits, the following refactoring should be completed **before** Phase 5 implementation:

### Frontend Refactoring Required (6.5 hours)

**Critical Issues Identified:**

1. **Card Component Duplication** (High Severity)
   - Progress bar UI duplicated in MovieCard, MediaCard, EpisodeCard
   - Watched badge overlay duplicated 3 times
   - Same badge structure repeated across components
   - **Impact:** Phase 5 filter additions require touching 3+ files

2. **Route Page Pattern Duplication** (High Severity)
   - Library filter logic duplicated in movies/TV/music pages
   - Search filter pattern repeated 3 times
   - **Impact:** Phase 5 genre/year/quality filters must be implemented 3 times

3. **Inconsistent Hover Effects** (Medium Severity)
   - MovieCard: `hover:scale-105`, TVShowCard: no scale, AlbumCard: `hover:scale-105`
   - No consistent design system
   - **Impact:** Phase 5 UI polish requires normalizing all cards

**Recommended Refactoring Tasks (Before Phase 5):**

1. **Create useLibraryFilter Hook** (1.5 hours)
   - Extract common library filtering logic
   - Single place to add Phase 5 filter state
   - Reduces route components from 180+ to 120+ lines

2. **Extract Card Badge Components** (2 hours)
   - Create WatchedBadge, ProgressBar, TechnicalBadges components
   - Eliminates 50+ lines of duplication
   - Phase 5 badge modifications happen once

3. **Create MediaBrowsePage Wrapper** (3 hours)
   - Layout component for common page structure
   - Phase 5 filter bar/sort controls added once
   - Build filter UI 1x instead of 3x

**Time Investment:**
- Refactor before Phase 5: 6.5 hours
- Time saved during Phase 5: 15-20 hours (avoid 3x duplication)
- **Net savings: 8.5-13.5 hours**

### Backend Refactoring Required (3 hours)

**Critical Issues Identified:**

1. **TV Episode Row Converters Duplication** (High Severity)
   - Three 30-line field assignments in tvshow/types.go (lines 127-221)
   - Every field addition requires 3 updates
   - **Impact:** Pagination metadata changes require 3 updates

2. **Music Track Row Converters Duplication** (High Severity)
   - Four identical 30-line type switch cases in music/types.go (lines 211-336)
   - **Impact:** Pagination changes must be replicated 4 times

3. **Query Parameter Validation Duplication** (Medium Severity)
   - Same library_id validation repeated in 9 handler methods
   - **Impact:** Pagination param parsing will add more boilerplate

**Recommended Refactoring Tasks (Before Phase 5):**

1. **Refactor TV Episode Converters** (1 hour)
   - Create single `toTVEpisodeDomain()` function (like movies)
   - Replace 3 converter functions with thin wrappers
   - Reduce from 200 lines to 80 lines

2. **Refactor Music Track Converters** (1.5 hours)
   - Extract field mapping using existing `musicTrackFields` struct
   - Create single `toMusicTrackDomain()` function
   - Eliminate type switch duplication

3. **Create Query Parameter Helpers** (30 minutes)
   - Add `parseRequiredLibraryID()` and `parseRequiredQuery()`
   - Reduce boilerplate in handlers
   - Consistent validation pattern

**Time Investment:**
- Refactor before Phase 5: 3 hours
- Cleaner codebase for pagination implementation
- Pagination changes in fewer places, less error-prone

### Total Pre-Phase 5 Refactoring

- **Frontend**: 6.5 hours
- **Backend**: 3 hours
- **Total**: 9.5 hours

**Benefits:**
- Cleaner codebase to work with
- Pagination changes in fewer places
- Less error-prone implementation
- Better test coverage
- Net time savings during Phase 5: 8-13 hours

## Implementation Strategy

**Phase 0: Code Quality Refactoring** (9.5 hours) ⬅️ **NEW**
- Frontend: Extract hooks, components, wrappers
- Backend: Refactor converters, add validation helpers

**Phase 5.1: Backend Pagination** (5-7 hours) ⬇️ **Reduced from 8-10h**
- Cleaner codebase makes this faster

**Phase 5.2-5.6**: Continue as planned

**Revised Total Estimate:**
- Phase 0 (Refactoring): 9.5 hours
- Phase 5 (Implementation): 26.5-40 hours (reduced from 36-50h)
- **Total: 36-49.5 hours** (similar total, but cleaner code)

### Success Criteria for Premium UX

**Accessibility:**
- ✅ WCAG 2.1 AA compliance
- ✅ Full keyboard navigation
- ✅ Screen reader tested (NVDA/VoiceOver)
- ✅ Focus visible on all interactive elements

**Visual Polish:**
- ✅ Consistent hover/focus states
- ✅ Smooth animations (60fps)
- ✅ Professional loading states
- ✅ No visual jank or layout shifts

**User Feedback:**
- ✅ Clear action feedback (toasts)
- ✅ Helpful tooltips
- ✅ Informative error messages
- ✅ Progress indicators

**Navigation:**
- ✅ Clear wayfinding (breadcrumbs)
- ✅ Shareable URLs (filters in query)
- ✅ Browser back/forward works
- ✅ Quick access to recent content

**Interactions:**
- ✅ Intuitive keyboard shortcuts
- ✅ Touch-friendly (44px targets)
- ✅ Context menus work
- ✅ Quick actions accessible

**Information:**
- ✅ Powerful filtering
- ✅ Flexible sorting
- ✅ View options
- ✅ Customizable display

### Testing Plan

**Accessibility Testing:**
- axe DevTools automated scan
- Manual keyboard navigation test
- NVDA/JAWS screen reader test
- VoiceOver (macOS/iOS) test
- Color contrast validation

**Cross-Device Testing:**
- Desktop (Windows, macOS, Linux)
- Mobile (iOS Safari, Android Chrome)
- Tablet (iPad, Android tablet)
- Touch vs mouse interaction
- Different screen sizes

**User Testing:**
- Task completion (find movie, play, mark watched)
- Usability testing with 5+ users
- A/B test key interactions
- Accessibility testing with disabled users

**Performance Testing:**
- Animation frame rate (60fps target)
- Time to interactive
- Perceived performance
- Touch response latency

## Future Enhancements

1. **Cursor-Based Pagination:** For very large libraries (>100k items)
2. **Backend Full-Text Search:** Elasticsearch or PostgreSQL FTS
3. **Smart Prefetching:** Predict and prefetch next page
4. **Grid Virtualization:** `@tanstack/react-virtual` for huge lists
5. **Response Compression:** Gzip/Brotli for API responses
6. **GraphQL Layer:** Allow clients to request exactly what they need
7. **Watch Together:** Synchronized playback across users
8. **Collections:** User-created movie/TV collections
9. **Smart Playlists:** Auto-updating based on rules
10. **Advanced Stats:** Watch time, favorite genres, viewing patterns

## References

- [TanStack Query Infinite Queries](https://tanstack.com/query/latest/docs/react/guides/infinite-queries)
- [Intersection Observer API](https://developer.mozilla.org/en-US/docs/Web/API/Intersection_Observer_API)

## Appendix: Current vs Proposed API

### Current (No Pagination)

```
GET /api/movies?library_id=1

Response (10,000 movies, 20MB):
{
  "movies": [ ... 10,000 items ... ],
  "total": 10000
}
```

### Proposed (With Pagination)

```
GET /api/movies?library_id=1&limit=50&offset=0

Response (50 movies, ~100KB):
{
  "movies": [ ... 50 items ... ],
  "total": 10000,
  "limit": 50,
  "offset": 0,
  "hasMore": true
}
```

**Improvement:** 99.5% smaller payload, 200x faster response time
