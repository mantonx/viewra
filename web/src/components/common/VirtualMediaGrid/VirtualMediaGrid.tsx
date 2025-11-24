/**
 * VirtualMediaGrid Component
 *
 * Row-virtualized grid for media items using TanStack Virtual
 * Provides smooth infinite scroll with smart prefetching
 */

import { type ReactNode } from 'react'
import type { VirtualItem } from '@tanstack/react-virtual'
import { useVirtualGrid } from '@/lib/hooks'
import { MediaCardSkeleton } from '@/components/media/MediaCard'

interface VirtualMediaGridProps<T> {
  /**
   * All items to display (filtered/sorted)
   */
  items: T[]

  /**
   * Number of columns (responsive)
   */
  columns: number

  /**
   * Estimated row height in pixels
   */
  estimatedRowHeight: number

  /**
   * Gap between items in pixels
   */
  gap: number

  /**
   * Render function for each item
   */
  renderItem: (item: T) => ReactNode

  /**
   * Aspect ratio for skeleton cards ('2/3' for movies/TV, 'square' for music)
   */
  skeletonAspectRatio?: '2/3' | 'square'

  /**
   * Infinite scroll props
   */
  fetchNextPage: () => void
  hasNextPage: boolean
  isFetchingNextPage: boolean
}

export const VirtualMediaGrid = <T extends { id: number }>({
  items,
  columns,
  estimatedRowHeight,
  gap,
  renderItem,
  skeletonAspectRatio = '2/3',
  fetchNextPage,
  hasNextPage,
  isFetchingNextPage,
}: VirtualMediaGridProps<T>) => {
  const { virtualRows, totalHeight } = useVirtualGrid({
    itemCount: items.length,
    columns,
    estimatedRowHeight,
    gap,
    overscan: 3, // Prefetch 3 rows ahead
    fetchNextPage,
    hasNextPage,
    isFetchingNextPage,
  })

  const rowCount = Math.ceil(items.length / columns)

  return (
    <div
      style={{
        height: `${totalHeight}px`,
        width: '100%',
        position: 'relative',
      }}
    >
      {virtualRows.map((virtualRow: VirtualItem) => {
          const isLoaderRow = virtualRow.index > rowCount - 1
          const rowStartIndex = virtualRow.index * columns
          const rowItems = items.slice(rowStartIndex, rowStartIndex + columns)

          return (
            <div
              key={virtualRow.key}
              style={{
                position: 'absolute',
                top: 0,
                left: 0,
                width: '100%',
                height: `${virtualRow.size}px`,
                transform: `translateY(${virtualRow.start}px)`,
              }}
            >
              <div
                className="grid gap-4"
                style={{
                  gridTemplateColumns: `repeat(${columns}, minmax(0, 1fr))`,
                }}
              >
                {isLoaderRow ? (
                  // Loading row with skeletons
                  Array.from({ length: columns }).map((_, i) => (
                    <MediaCardSkeleton
                      key={`skeleton-${i}`}
                      aspectRatio={skeletonAspectRatio}
                    />
                  ))
                ) : (
                  // Regular row with items
                  <>
                    {rowItems.map((item) => renderItem(item))}
                    {/* Fill empty slots in last row with empty divs to maintain grid */}
                    {rowItems.length < columns &&
                      Array.from({ length: columns - rowItems.length }).map(
                        (_, i) => <div key={`empty-${i}`} />
                      )}
                  </>
                )}
              </div>
            </div>
          )
        })}
    </div>
  )
}
