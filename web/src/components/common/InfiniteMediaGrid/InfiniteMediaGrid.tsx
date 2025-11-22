/**
 * InfiniteMediaGrid Component
 * Unified component for infinite scroll + coordinated image loading
 * Eliminates race conditions and duplicate code across movies/tv/music pages
 */

import { useMemo } from 'react'
import { useWatchedList } from '@/lib/hooks'
import { BatchImagesProvider } from '@/lib/hooks/useBatchImages'
import { useBatchImagesPrefetch } from '@/lib/hooks/useBatchImagesPrefetch'
import { useInfiniteArtists, flattenArtists } from '@/lib/hooks/useInfiniteArtists'
import { useInfiniteMovies, flattenMovies } from '@/lib/hooks/useInfiniteMovies'
import { useInfiniteScrollObserver } from '@/lib/hooks/useInfiniteScrollObserver'
import { useInfiniteTVShows, flattenTVShows } from '@/lib/hooks/useInfiniteTVShows'
import type { InfiniteMediaGridProps, MediaItem } from './InfiniteMediaGrid.types'

export const InfiniteMediaGrid = <T extends MediaItem>({
  type,
  libraryId,
  sort,
  renderCard,
  renderListItem,
  searchQuery = '',
  genres = [],
  yearMin,
  yearMax,
  qualities = [],
  watchedFilter = 'all',
  viewMode = 'grid',
  gridClassName = 'grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6 gap-4',
  onItemSelect: _onItemSelect,
  emptyState,
  loadingIndicator,
  getItemSearchText,
  getItemGenres,
  getItemYear,
  getItemQuality,
}: InfiniteMediaGridProps<T>) => {
  // Select appropriate infinite query hook based on type
  const useInfiniteHook = useMemo(() => {
    switch (type) {
      case 'movies':
        return { hook: useInfiniteMovies, flatten: flattenMovies }
      case 'tv':
        return { hook: useInfiniteTVShows, flatten: flattenTVShows }
      case 'music':
        return { hook: useInfiniteArtists, flatten: flattenArtists }
    }
  }, [type])

  // Fetch data with infinite scroll
  const {
    data,
    isLoading,
    error,
    fetchNextPage,
    hasNextPage,
    isFetchingNextPage,
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
  } = useInfiniteHook.hook({ libraryId, sort, enabled: libraryId > 0 }) as any

  // Flatten pages into single array - memoized to prevent re-creation
  const allItems = useMemo(
    () => (data ? useInfiniteHook.flatten(data.pages) : []) as T[],
    [data, useInfiniteHook],
  )

  // Fetch watched items for filtering
  const { data: watchedData } = useWatchedList({ limit: 10000 })
  const watchedMediaIds = useMemo(
    () => new Set(watchedData?.progress?.map((p) => p.media_id) || []),
    [watchedData],
  )

  // Client-side filtering
  const filteredItems = useMemo(() => {
    return allItems.filter((item) => {
      // Text search
      if (searchQuery) {
        const searchText = getItemSearchText ? getItemSearchText(item) : (item.title || item.name || '')
        if (!searchText.toLowerCase().includes(searchQuery.toLowerCase())) {
          return false
        }
      }

      // Genre filter
      if (genres.length > 0 && getItemGenres) {
        const itemGenres = getItemGenres(item) || []
        const hasMatchingGenre = genres.some((filterGenre) =>
          itemGenres.some((itemGenre) => itemGenre.toLowerCase() === filterGenre.toLowerCase())
        )
        if (!hasMatchingGenre) {
          return false
        }
      }

      // Year range filter
      if ((yearMin !== undefined || yearMax !== undefined) && getItemYear) {
        const itemYear = getItemYear(item)
        if (itemYear !== undefined) {
          if (yearMin !== undefined && itemYear < yearMin) {
            return false
          }
          if (yearMax !== undefined && itemYear > yearMax) {
            return false
          }
        }
      }

      // Quality filter
      if (qualities.length > 0 && getItemQuality) {
        const itemQuality = getItemQuality(item)
        if (!itemQuality || !qualities.includes(itemQuality)) {
          return false
        }
      }

      // Watched/unwatched filter
      if (watchedFilter !== 'all') {
        const isWatched = watchedMediaIds.has(item.id)
        if (watchedFilter === 'watched' && !isWatched) {
          return false
        }
        if (watchedFilter === 'unwatched' && isWatched) {
          return false
        }
      }

      return true
    })
  }, [allItems, searchQuery, genres, yearMin, yearMax, qualities, watchedFilter, getItemSearchText, getItemGenres, getItemYear, getItemQuality, watchedMediaIds])

  // Extract IDs for batch image loading
  const itemIds = filteredItems.map((item) => item.id)

  // Coordinated image prefetching with predictive next page loading
  const { images: _images } = useBatchImagesPrefetch({
    currentItemIds: itemIds,
    mediaType: type === 'tv' ? 'tv_show' : type === 'music' ? 'music_artist' : undefined,
    isEntityBased: type === 'tv' || type === 'music',
    hasNextPage,
    isFetchingNextPage,
    libraryId,
    sort,
    contentType: type,
  })

  // Infinite scroll observer
  const observerRef = useInfiniteScrollObserver({
    onLoadMore: fetchNextPage,
    enabled: hasNextPage && !isFetchingNextPage,
    rootMargin: '3000px', // Very aggressive prefetch - load ~3 viewport heights early
  })

  // Loading state
  if (isLoading) {
    return (
      <div className="flex items-center justify-center py-20">
        <div className="text-gray-400">Loading {type}...</div>
      </div>
    )
  }

  // Error state
  if (error) {
    return (
      <div className="flex items-center justify-center py-20">
        <div className="text-red-500">Error loading {type}: {(error as Error).message}</div>
      </div>
    )
  }

  // Empty state
  if (filteredItems.length === 0) {
    return emptyState || (
      <div className="flex items-center justify-center py-20">
        <div className="text-gray-400">No {type} found</div>
      </div>
    )
  }

  // Render grid with BatchImagesProvider for coordinated image loading
  return (
    <BatchImagesProvider
      mediaIds={type === 'movies' ? itemIds : undefined}
      entityIds={type === 'tv' || type === 'music' ? itemIds : undefined}
      mediaType={type === 'tv' ? 'tv_show' : type === 'music' ? 'music_artist' : undefined}
    >
      {viewMode === 'list' && renderListItem ? (
        <div className="space-y-2" role="list" aria-label={`${type} list`}>
          {filteredItems.map((item) => renderListItem(item))}
        </div>
      ) : (
        <div className={gridClassName} role="grid" aria-label={`${type} grid`}>
          {filteredItems.map((item) => renderCard(item))}
        </div>
      )}

      {/* Infinite scroll trigger */}
      <div ref={observerRef} className="h-20 flex items-center justify-center">
        {isFetchingNextPage && (
          loadingIndicator || <div className="text-gray-400">Loading more {type}...</div>
        )}
      </div>

      {/* Item count */}
      <div className="mt-4 text-sm text-gray-500 text-center">
        Showing {filteredItems.length} of {allItems.length} {type}
      </div>
    </BatchImagesProvider>
  )
}
