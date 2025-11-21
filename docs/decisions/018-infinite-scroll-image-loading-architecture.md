# ADR 018: Infinite Scroll Image Loading Architecture

**Status**: Proposed
**Date**: 2025-11-21
**Deciders**: Development Team
**Context**: Phase 5 - Library Browsing Experience Enhancement (Follow-up to ADR 013)

## Context and Problem Statement

ADR 013 successfully implemented backend pagination and frontend infinite scroll for library browsing. However, a critical UX issue remains: **visible gaps with missing artwork during infinite scroll**. When users scroll past the initial 50 items, they see 5-10 cards without posters before the next page loads, creating a jarring, unprofessional experience.

### Current Architecture (Implemented from ADR 013)

**What Works:**
- ✅ Backend pagination with `limit`/`offset` (50 items per page)
- ✅ `useInfiniteQuery` from TanStack Query
- ✅ IntersectionObserver for scroll detection
- ✅ `BatchImagesProvider` to eliminate N+1 image queries (batches of 50)

**What's Broken:**
- ❌ **Race condition between data loading and image loading**
- ❌ **Three separate layers of complexity** (page routes, MediaBrowsePage, BatchImagesProvider)
- ❌ **Band-aid fixes accumulating** (skeleton cards, prefetch margins, manual observer management)
- ❌ **Duplicate code across 3 library types** (movies, TV shows, music)

### Root Cause Analysis

The problem stems from **architectural fragmentation**:

1. **movies.index.tsx** (334 lines)
   - Manages IntersectionObserver manually
   - Extracts movie IDs for BatchImagesProvider
   - Wraps MediaBrowsePage in BatchImagesProvider
   - Duplicated 3x across movies/tv/music

2. **MediaBrowsePage.tsx** (410 lines)
   - Generic component trying to handle all media types
   - Performs client-side filtering/sorting (defeating pagination benefits)
   - No awareness of infinite scroll loading states
   - Can't coordinate with image loading

3. **BatchImagesProvider** (separate concern)
   - Batches image requests in groups of 50
   - Recalculates batches when IDs change (causing delay)
   - No prefetching or coordination with infinite scroll

**The Gap:**
- User scrolls to item 48
- IntersectionObserver triggers at 1000px before end
- API call starts for items 51-100
- New items added to `allMovies` array
- `BatchImagesProvider` sees new IDs, creates new batch query
- **500-1000ms delay** while batch query executes
- User sees empty posters during this delay

### Why Band-Aids Failed

1. **rootMargin: '1000px'** - Helps, but doesn't eliminate the gap (network latency varies)
2. **Skeleton cards** - Adds visual clutter, doesn't solve core timing issue
3. **Prefetch images** - Would require complex preflight logic, more band-aids

The fundamental issue: **Three separate concerns (data pagination, image batching, UI rendering) are not coordinated**.

## Decision Drivers

1. **Smooth UX**: Zero visible gaps during scroll, instant perceived loading
2. **DRY**: Eliminate 900+ lines of duplicate code across 3 library types
3. **Maintainability**: Single source of truth for infinite scroll + images
4. **Performance**: Efficient image loading without N+1 queries
5. **Simplicity**: Remove complexity, not add more band-aids

## Considered Options

### Option 1: Unified InfiniteMediaGrid Component (RECOMMENDED)

Create a single, purpose-built component that coordinates all concerns:

```typescript
// Usage (simple, DRY)
<InfiniteMediaGrid
  type="movies"
  libraryId={libraryId}
  sort={sort}
  renderCard={(movie) => <MovieCard movie={movie} onClick={...} />}
/>
```

**Architecture:**

1. **Single Responsibility**: One component owns infinite scroll + image batching
2. **Coordinated Loading**: Prefetch images for next page before rendering cards
3. **Optimistic Rendering**: Show cards immediately with loading state, populate when ready
4. **Built-in Observer**: Managed internally, no manual setup in 3 places
5. **Generic Type-Safe**: Works for movies, TV shows, music with full type safety

**Key Innovation - Predictive Image Prefetching:**

```typescript
// When user is 75% through current page, prefetch next page images
useEffect(() => {
  if (hasNextPage && !isFetchingNextPage) {
    const currentPageItems = pages[pages.length - 1]?.items?.length ?? 0
    const visibleThreshold = Math.floor(currentPageItems * 0.75)

    // Prefetch images for NEXT page BEFORE fetching the page
    if (scrolledPastThreshold(visibleThreshold)) {
      prefetchNextPageImages(nextPageOffset, pageSize)
    }
  }
}, [scrollPosition, pages, hasNextPage])
```

**Benefits:**
- ✅ Eliminates race condition - images ready before cards render
- ✅ Reduces code from ~900 lines (3 routes) to ~200 lines (1 component)
- ✅ No more manual IntersectionObserver setup in routes
- ✅ No more BatchImagesProvider wrapper complexity
- ✅ Proper separation of concerns (component owns its dependencies)
- ✅ Easy to test, maintain, extend

**Cons:**
- ❌ Requires refactoring 3 route files
- ❌ Need to extract filtering logic to separate component/hook
- ❌ ~1-2 days of work

### Option 2: Fix Current Architecture with More Band-Aids

Continue current approach:
- Add skeleton cards (already attempted)
- Increase prefetch margin to 2000px
- Add loading states between cards
- Optimize BatchImagesProvider memoization

**Pros:**
- ✅ Minimal changes to existing code
- ✅ Incremental improvements

**Cons:**
- ❌ Doesn't solve root cause
- ❌ Adds more complexity on top of fragmented architecture
- ❌ Still duplicate code across 3 routes
- ❌ Technical debt accumulates
- ❌ Will need another refactor eventually

### Option 3: Virtual Scrolling with @tanstack/react-virtual

Use virtual scrolling to render only visible items:

**Pros:**
- ✅ Best performance for huge lists (10,000+ items)
- ✅ Minimal DOM elements

**Cons:**
- ❌ Doesn't solve image loading timing issue
- ❌ Complex to implement with dynamic heights
- ❌ Overkill for current scale (< 2000 items typically)
- ❌ Worse UX (items pop in/out as you scroll)

## Decision

**RECOMMENDATION: Option 1 - Unified InfiniteMediaGrid Component**

This is the correct architectural solution that:
1. Eliminates the root cause (coordination failure)
2. Removes technical debt (duplicate code)
3. Provides better UX (predictive prefetching)
4. Simplifies maintenance (single component)

## Implementation Plan

### Phase 1: Create InfiniteMediaGrid Component

**File**: `web/src/components/common/InfiniteMediaGrid/InfiniteMediaGrid.tsx`

```typescript
interface InfiniteMediaGridProps<T> {
  type: 'movies' | 'tv' | 'music'
  libraryId: number
  sort?: string

  // Rendering
  renderCard: (item: T) => ReactNode
  renderListItem?: (item: T) => ReactNode

  // Filtering (optional)
  filters?: MediaFilters
  searchQuery?: string

  // Customization
  gridClassName?: string
  emptyState?: ReactNode
}

export function InfiniteMediaGrid<T extends MediaItem>(props: InfiniteMediaGridProps<T>) {
  // 1. Infinite query for data
  const { data, fetchNextPage, hasNextPage, isFetchingNextPage } = useInfiniteMedia({...})

  // 2. Managed intersection observer
  const observerRef = useInfiniteScrollObserver({
    onLoadMore: fetchNextPage,
    enabled: hasNextPage && !isFetchingNextPage,
    rootMargin: '1000px' // Aggressive prefetch
  })

  // 3. Coordinated image prefetching
  const { images, prefetchNextBatch } = useBatchImagesPrefetch({
    currentItems: allItems,
    nextPageOffset: data?.pages.length * pageSize,
    pageSize,
    type: props.type
  })

  // 4. Render grid with proper loading states
  return (
    <div className={props.gridClassName}>
      {allItems.map(item => props.renderCard(item))}
      {isFetchingNextPage && <LoadingCards count={12} />}
      <div ref={observerRef} />
    </div>
  )
}
```

**Key Features:**
- Encapsulates IntersectionObserver logic
- Coordinates image prefetching with scroll position
- Handles loading states internally
- Generic and type-safe
- ~150-200 lines total

### Phase 2: Enhanced Batch Image Hook

**File**: `web/src/lib/hooks/useBatchImagesPrefetch.ts`

```typescript
export function useBatchImagesPrefetch({ currentItems, nextPageOffset, pageSize, type }) {
  // Current page images (normal batching)
  const currentIds = currentItems.map(i => i.id)
  const currentImages = useBatchImages(currentIds, type)

  // PREFETCH next page (before user scrolls there)
  const nextPageIds = Array.from({ length: pageSize }, (_, i) => nextPageOffset + i)
  const { refetch: prefetchNextBatch } = useBatchImages(nextPageIds, type, {
    enabled: false, // Manual trigger
    staleTime: Infinity // Keep in cache
  })

  // Prefetch when user is 75% through current page
  useEffect(() => {
    if (shouldPrefetch(currentItems.length, scrollPosition)) {
      prefetchNextBatch()
    }
  }, [scrollPosition, currentItems])

  return { images: currentImages, prefetchNextBatch }
}
```

**Innovation**: Images are in cache BEFORE cards render, eliminating the gap.

### Phase 3: Refactor Routes to Use InfiniteMediaGrid

**Before** (movies.index.tsx - 334 lines):
```typescript
const Movies = () => {
  // URL state management (60 lines)
  // useInfiniteMovies hook
  // IntersectionObserver setup (25 lines)
  // Extract movie IDs
  // Wrap in BatchImagesProvider
  // Pass everything to MediaBrowsePage (50 props)

  return (
    <BatchImagesProvider mediaIds={movieIds}>
      <MediaBrowsePage
        type="movies"
        data={allMovies}
        renderItem={...}
        // ... 30+ props
      />
      <div ref={observerTarget} />
    </BatchImagesProvider>
  )
}
```

**After** (movies.index.tsx - ~100 lines):
```typescript
const Movies = () => {
  const { libraryId } = useLibraryFilter('movies')
  const { sort, filters, searchQuery } = useMovieFilters() // Extract to hook

  return (
    <PageHeader title="Movies" description="Browse your movie collection." />
    <MediaFilters
      type="movies"
      onFiltersChange={...}
      onSearchChange={...}
      onSortChange={...}
    />
    <InfiniteMediaGrid
      type="movies"
      libraryId={libraryId}
      sort={sort}
      filters={filters}
      searchQuery={searchQuery}
      renderCard={(movie) => (
        <MovieCard movie={movie} onClick={() => playMovie(movie.id)} />
      )}
    />
  )
}
```

**Reduction**: 334 lines → ~100 lines per route × 3 routes = **~700 lines removed**

### Phase 4: Deprecate Old Components (Optional)

- Keep `MediaBrowsePage` for backward compatibility initially
- Migrate music and TV routes to `InfiniteMediaGrid`
- Remove `MediaBrowsePage` once all routes migrated
- Simplify `BatchImagesProvider` or fold into `InfiniteMediaGrid`

## Files to Create

1. `web/src/components/common/InfiniteMediaGrid/`
   - `InfiniteMediaGrid.tsx` (~150 lines)
   - `InfiniteMediaGrid.types.ts` (~30 lines)
   - `index.ts`

2. `web/src/lib/hooks/useBatchImagesPrefetch.ts` (~80 lines)

3. `web/src/lib/hooks/useInfiniteScrollObserver.ts` (~40 lines)
   - Extract observer logic for reuse

4. `web/src/lib/hooks/useMovieFilters.ts` (extract from route)
5. `web/src/lib/hooks/useTVShowFilters.ts`
6. `web/src/lib/hooks/useMusicFilters.ts`

## Files to Modify

1. `web/src/routes/_layout/movies.index.tsx` (refactor)
2. `web/src/routes/_layout/tv.index.tsx` (refactor)
3. `web/src/routes/_layout/music.index.tsx` (refactor)
4. `web/src/lib/hooks/useBatchImages.tsx` (enhance with prefetch support)

## Files to Remove (Eventually)

1. `web/src/components/common/MediaBrowsePage/` (once migration complete)
2. Skeleton components (no longer needed)

## Consequences

### Positive

1. **Better UX**: Seamless infinite scroll with zero visible gaps
2. **Less Code**: ~700 lines removed, easier to maintain
3. **DRY**: Single source of truth for infinite scroll + images
4. **Better Perf**: Predictive prefetching reduces perceived latency
5. **Testable**: Isolated component easy to test
6. **Extensible**: Easy to add features (virtual scrolling, skeleton states, etc.)

### Negative

1. **Refactoring Required**: ~1-2 days of focused work
2. **Migration Risk**: Need to test all 3 library types thoroughly
3. **Temporary Duplication**: During migration, both old and new code exist

### Neutral

1. **New Component**: Adds one more component, but removes several concerns from routes
2. **Learning Curve**: Team needs to understand new prefetching pattern

## Alternatives Considered and Rejected

### Keep MediaBrowsePage, Enhance It
- **Why Rejected**: Already too complex (410 lines), trying to do too much
- Adding more features would make it unmaintainable
- Generic components that do everything do nothing well

### Move Image Batching into MediaBrowsePage
- **Why Rejected**: Couples image loading to generic browse component
- MediaBrowsePage used in other contexts where batching doesn't apply
- Violates single responsibility principle

### Use a Third-Party Library
- **Why Rejected**: No library combines infinite scroll + coordinated image prefetch
- `@tanstack/react-virtual`: Solves different problem (rendering performance)
- `react-infinite-scroll-component`: Too basic, no image coordination
- Custom solution gives us exact control we need

## Success Metrics

1. **Zero Visible Gaps**: Users should never see cards without posters during scroll
2. **Code Reduction**: Remove ~700 lines of duplicate code
3. **Performance**: Image prefetch hit rate > 95% (images cached before needed)
4. **Maintainability**: Single file to modify for infinite scroll improvements
5. **DX**: New library types use InfiniteMediaGrid in < 50 lines

## Implementation Checklist

- [ ] Create `InfiniteMediaGrid` component
- [ ] Create `useBatchImagesPrefetch` hook
- [ ] Create `useInfiniteScrollObserver` hook
- [ ] Extract filter hooks from routes
- [ ] Refactor movies.index.tsx to use InfiniteMediaGrid
- [ ] Test movies page thoroughly
- [ ] Refactor tv.index.tsx
- [ ] Test TV shows page
- [ ] Refactor music.index.tsx
- [ ] Test music page
- [ ] Remove skeleton components
- [ ] Update documentation
- [ ] Deprecate MediaBrowsePage (optional)

## Related ADRs

- ADR 013: Library Browsing UX Improvements (pagination foundation)
- ADR 006: Image Handling Strategy (batch loading strategy)

## References

- [TanStack Query Infinite Queries](https://tanstack.com/query/latest/docs/react/guides/infinite-queries)
- [Intersection Observer API](https://developer.mozilla.org/en-US/docs/Web/API/Intersection_Observer_API)
- [React Performance Optimization](https://react.dev/learn/render-and-commit)
