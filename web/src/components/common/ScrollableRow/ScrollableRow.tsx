import { useRef, useState, useEffect, useCallback, type ReactNode } from 'react'
import { ChevronLeft, ChevronRight } from 'lucide-react'
import { cn } from '@/lib/utils'

interface ScrollableRowProps {
  children: ReactNode
  className?: string
  /** Gap between items (Tailwind spacing scale, e.g., 4 = gap-4) */
  gap?: number
}

/**
 * ScrollableRow - Horizontal scrolling container with navigation affordances
 *
 * Design approach (Netflix-style):
 * - Gradient fades on edges indicate scrollable content
 * - Large navigation buttons appear on hover, always on top of content
 * - Smooth transitions for polish
 */
export const ScrollableRow = ({ children, className, gap = 4 }: ScrollableRowProps) => {
  const scrollRef = useRef<HTMLDivElement>(null)
  const [canScrollLeft, setCanScrollLeft] = useState(false)
  const [canScrollRight, setCanScrollRight] = useState(false)
  const [isHovered, setIsHovered] = useState(false)

  const updateScrollState = useCallback(() => {
    const el = scrollRef.current
    if (!el) return

    const { scrollLeft, scrollWidth, clientWidth } = el
    setCanScrollLeft(scrollLeft > 4)
    setCanScrollRight(scrollLeft + clientWidth < scrollWidth - 4)
  }, [])

  useEffect(() => {
    const el = scrollRef.current
    if (!el) return

    const timeoutId = setTimeout(updateScrollState, 100)
    const resizeObserver = new ResizeObserver(updateScrollState)
    resizeObserver.observe(el)

    el.addEventListener('scroll', updateScrollState, { passive: true })
    window.addEventListener('resize', updateScrollState)

    return () => {
      clearTimeout(timeoutId)
      resizeObserver.disconnect()
      el.removeEventListener('scroll', updateScrollState)
      window.removeEventListener('resize', updateScrollState)
    }
  }, [updateScrollState, children])

  const scroll = (direction: 'left' | 'right') => {
    const el = scrollRef.current
    if (!el) return

    const scrollAmount = el.clientWidth * 0.8
    el.scrollBy({
      left: direction === 'left' ? -scrollAmount : scrollAmount,
      behavior: 'smooth',
    })
  }

  return (
    <div
      className={cn('relative', className)}
      onMouseEnter={() => setIsHovered(true)}
      onMouseLeave={() => setIsHovered(false)}
    >
      {/* Left edge - gradient fade + button */}
      <div
        className={cn(
          'absolute left-0 top-0 bottom-0 w-20 z-[100]',
          'flex items-center justify-start pl-2',
          'transition-opacity duration-300',
          canScrollLeft ? 'opacity-100' : 'opacity-0 pointer-events-none'
        )}
        style={{
          background: 'linear-gradient(to right, rgb(var(--color-bg-primary)) 0%, rgb(var(--color-bg-primary)) 30%, transparent 100%)',
        }}
      >
        <button
          onClick={() => scroll('left')}
          className={cn(
            'w-12 h-12 rounded-full cursor-pointer',
            'bg-neutral-800/80 dark:bg-neutral-200/90',
            'flex items-center justify-center',
            'text-white dark:text-neutral-900',
            'hover:bg-neutral-900 dark:hover:bg-white',
            'hover:scale-110 hover:shadow-2xl',
            'active:scale-95',
            'transition-all duration-200 ease-out',
            'shadow-xl shadow-black/30',
            'border border-white/20 dark:border-black/10',
            'backdrop-blur-sm',
            isHovered ? 'opacity-100' : 'opacity-0'
          )}
          aria-label="Scroll left"
        >
          <ChevronLeft className="w-7 h-7" />
        </button>
      </div>

      {/* Right edge - gradient fade + button */}
      <div
        className={cn(
          'absolute right-0 top-0 bottom-0 w-20 z-[100]',
          'flex items-center justify-end pr-2',
          'transition-opacity duration-300',
          canScrollRight ? 'opacity-100' : 'opacity-0 pointer-events-none'
        )}
        style={{
          background: 'linear-gradient(to left, rgb(var(--color-bg-primary)) 0%, rgb(var(--color-bg-primary)) 30%, transparent 100%)',
        }}
      >
        <button
          onClick={() => scroll('right')}
          className={cn(
            'w-12 h-12 rounded-full cursor-pointer',
            'bg-neutral-800/80 dark:bg-neutral-200/90',
            'flex items-center justify-center',
            'text-white dark:text-neutral-900',
            'hover:bg-neutral-900 dark:hover:bg-white',
            'hover:scale-110 hover:shadow-2xl',
            'active:scale-95',
            'transition-all duration-200 ease-out',
            'shadow-xl shadow-black/30',
            'border border-white/20 dark:border-black/10',
            'backdrop-blur-sm',
            isHovered ? 'opacity-100' : 'opacity-0'
          )}
          aria-label="Scroll right"
        >
          <ChevronRight className="w-7 h-7" />
        </button>
      </div>

      {/* Scrollable content */}
      <div
        ref={scrollRef}
        className="overflow-x-auto pb-2 scrollbar-hide"
      >
        <div className={cn('flex', `gap-${gap}`)} style={{ minWidth: 'max-content' }}>
          {children}
        </div>
      </div>
    </div>
  )
}
