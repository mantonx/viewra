import { useCallback, useEffect, useRef } from 'react'

interface UseGridNavigationOptions {
  /** Number of columns in the grid at current viewport */
  columns: number
  /** Total number of items in the grid */
  itemCount: number
  /** Whether navigation is enabled */
  enabled?: boolean
  /** Callback when an item is selected via Enter/Space */
  onItemSelect?: (index: number) => void
}

/**
 * Custom hook for keyboard navigation in media grids
 * Handles arrow key navigation (up, down, left, right) and Enter/Space selection
 *
 * @example
 * const gridRef = useGridNavigation({
 *   columns: 6,
 *   itemCount: movies.length,
 *   onItemSelect: (index) => handlePlayMovie(movies[index].id)
 * })
 *
 * return <div ref={gridRef}>{movies.map(...)}</div>
 */
export const useGridNavigation = ({
  columns,
  itemCount,
  enabled = true,
  onItemSelect,
}: UseGridNavigationOptions) => {
  const gridRef = useRef<HTMLDivElement>(null)
  const focusedIndexRef = useRef<number>(-1)

  const getFocusableElement = useCallback((index: number): HTMLElement | null => {
    if (!gridRef.current) {
      return null
    }

    // Get all focusable children (buttons, links, or elements with tabindex)
    const focusableElements = gridRef.current.querySelectorAll<HTMLElement>(
      'button, a, [tabindex]:not([tabindex="-1"])'
    )

    return focusableElements[index] || null
  }, [])

  const setFocus = useCallback(
    (index: number) => {
      if (index < 0 || index >= itemCount) {
        return
      }

      const element = getFocusableElement(index)
      if (element) {
        element.focus()
        focusedIndexRef.current = index
      }
    },
    [itemCount, getFocusableElement]
  )

  const handleKeyDown = useCallback(
    (event: KeyboardEvent) => {
      if (!enabled || itemCount === 0) {
        return
      }

      let newIndex = focusedIndexRef.current
      let handled = false

      switch (event.key) {
        case 'ArrowRight':
          // Move right, wrap to next row
          if (newIndex < itemCount - 1) {
            newIndex += 1
            handled = true
          }
          break

        case 'ArrowLeft':
          // Move left, wrap to previous row
          if (newIndex > 0) {
            newIndex -= 1
            handled = true
          }
          break

        case 'ArrowDown':
          // Move down one row
          if (newIndex + columns < itemCount) {
            newIndex += columns
            handled = true
          }
          break

        case 'ArrowUp':
          // Move up one row
          if (newIndex - columns >= 0) {
            newIndex -= columns
            handled = true
          }
          break

        case 'Home':
          // Jump to first item
          newIndex = 0
          handled = true
          break

        case 'End':
          // Jump to last item
          newIndex = itemCount - 1
          handled = true
          break

        case 'Enter':
        case ' ':
          // Select current item
          if (focusedIndexRef.current >= 0 && onItemSelect) {
            onItemSelect(focusedIndexRef.current)
            handled = true
          }
          break
      }

      if (handled) {
        event.preventDefault()
        setFocus(newIndex)
      }
    },
    [enabled, itemCount, columns, onItemSelect, setFocus]
  )

  // Track which element has focus to maintain focusedIndexRef
  const handleFocusIn = useCallback((event: FocusEvent) => {
    if (!gridRef.current) {
      return
    }

    const focusableElements = gridRef.current.querySelectorAll<HTMLElement>(
      'button, a, [tabindex]:not([tabindex="-1"])'
    )

    const index = Array.from(focusableElements).indexOf(event.target as HTMLElement)
    if (index >= 0) {
      focusedIndexRef.current = index
    }
  }, [])

  useEffect(() => {
    const grid = gridRef.current
    if (!grid || !enabled) {
      return
    }

    grid.addEventListener('keydown', handleKeyDown)
    grid.addEventListener('focusin', handleFocusIn)

    return () => {
      grid.removeEventListener('keydown', handleKeyDown)
      grid.removeEventListener('focusin', handleFocusIn)
    }
  }, [enabled, handleKeyDown, handleFocusIn])

  return gridRef
}
