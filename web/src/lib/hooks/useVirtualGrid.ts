/**
 * useVirtualGrid Hook
 *
 * TanStack Virtual-powered grid with automatic infinite scroll and prefetching
 * Replaces CSS Grid with virtualized rendering for better performance
 */

import { useEffect, useRef } from 'react'
import { useVirtualizer } from '@tanstack/react-virtual'

interface UseVirtualGridOptions {
  /**
   * Total number of items (including not-yet-loaded pages)
   */
  itemCount: number

  /**
   * Number of columns in the grid (responsive breakpoint aware)
   */
  columns: number

  /**
   * Estimated height of each row in pixels
   */
  estimatedRowHeight: number

  /**
   * Gap between items in pixels
   */
  gap: number

  /**
   * Number of items to overscan (prefetch) above/below viewport
   */
  overscan?: number

  /**
   * Fetch next page callback
   */
  fetchNextPage: () => void

  /**
   * Whether there's a next page to fetch
   */
  hasNextPage: boolean

  /**
   * Whether currently fetching next page
   */
  isFetchingNextPage: boolean
}

/**
 * Hook for virtualized grid with infinite scroll
 *
 * @example
 * ```tsx
 * const { parentRef, virtualRows, totalHeight } = useVirtualGrid({
 *   itemCount: allMovies.length,
 *   columns: 6,
 *   estimatedRowHeight: 400,
 *   gap: 16,
 *   overscan: 3,
 *   fetchNextPage,
 *   hasNextPage,
 *   isFetchingNextPage,
 * })
 *
 * return (
 *   <div ref={parentRef} style={{ height: '100vh', overflow: 'auto' }}>
 *     <div style={{ height: totalHeight }}>
 *       {virtualRows.map(virtualRow => (
 *         <div key={virtualRow.index} style={{ transform: `translateY(${virtualRow.start}px)` }}>
 *           {/* Render row items */}
 *         </div>
 *       ))}
 *     </div>
 *   </div>
 * )
 * ```
 */
export const useVirtualGrid = ({
  itemCount,
  columns,
  estimatedRowHeight,
  gap,
  overscan = 5,
  fetchNextPage,
  hasNextPage,
  isFetchingNextPage,
}: UseVirtualGridOptions) => {
  const parentRef = useRef<HTMLDivElement>(null)

  // Calculate number of rows
  const rowCount = Math.ceil(itemCount / columns)

  // Create virtualizer for rows
  const rowVirtualizer = useVirtualizer({
    count: hasNextPage ? rowCount + 1 : rowCount, // +1 for loading row
    getScrollElement: () => parentRef.current,
    estimateSize: () => estimatedRowHeight + gap,
    overscan,
  })

  // Auto-fetch when reaching end
  useEffect(() => {
    const [lastItem] = [...rowVirtualizer.getVirtualItems()].reverse()

    if (!lastItem) return

    // Fetch when last visible row is close to the end
    if (
      lastItem.index >= rowCount - 1 &&
      hasNextPage &&
      !isFetchingNextPage
    ) {
      fetchNextPage()
    }
  }, [
    hasNextPage,
    fetchNextPage,
    rowCount,
    isFetchingNextPage,
    rowVirtualizer.getVirtualItems(),
  ])

  return {
    parentRef,
    virtualRows: rowVirtualizer.getVirtualItems(),
    totalHeight: rowVirtualizer.getTotalSize(),
    rowVirtualizer,
  }
}
