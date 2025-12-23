import { useState, useRef, useEffect } from 'react'
import { Info } from 'lucide-react'
import { cn } from '@/lib/utils'
import type { TooltipProps } from './Tooltip.types'

export const Tooltip = ({ content, className }: TooltipProps) => {
  const [isVisible, setIsVisible] = useState(false)
  const [position, setPosition] = useState<'top' | 'bottom'>('top')
  const triggerRef = useRef<HTMLButtonElement>(null)
  const tooltipRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (isVisible && triggerRef.current && tooltipRef.current) {
      const triggerRect = triggerRef.current.getBoundingClientRect()
      const tooltipRect = tooltipRef.current.getBoundingClientRect()

      // Check if tooltip would overflow top of viewport
      if (triggerRect.top - tooltipRect.height - 8 < 0) {
        setPosition('bottom')
      } else {
        setPosition('top')
      }
    }
  }, [isVisible])

  return (
    <span className={cn('relative inline-flex', className)}>
      <button
        ref={triggerRef}
        type="button"
        className={cn(
          'inline-flex items-center justify-center',
          'w-4 h-4 rounded-full',
          'text-neutral-400 hover:text-neutral-600',
          'dark:text-neutral-500 dark:hover:text-neutral-300',
          'transition-colors cursor-help'
        )}
        onMouseEnter={() => setIsVisible(true)}
        onMouseLeave={() => setIsVisible(false)}
        onFocus={() => setIsVisible(true)}
        onBlur={() => setIsVisible(false)}
        aria-label="More information"
      >
        <Info className="w-3.5 h-3.5" />
      </button>

      {isVisible && (
        <div
          ref={tooltipRef}
          role="tooltip"
          className={cn(
            'absolute z-50 px-3 py-2 max-w-xs',
            'bg-neutral-900/95 dark:bg-neutral-800/95',
            'backdrop-blur-md',
            'text-white text-xs leading-relaxed',
            'rounded-lg shadow-xl',
            'border border-white/10',
            'pointer-events-none',
            'animate-in fade-in-0 zoom-in-95 duration-150',
            position === 'top' && 'bottom-full mb-2 left-1/2 -translate-x-1/2',
            position === 'bottom' && 'top-full mt-2 left-1/2 -translate-x-1/2'
          )}
        >
          {content}
          {/* Arrow */}
          <span
            className={cn(
              'absolute left-1/2 -translate-x-1/2',
              'w-2 h-2 rotate-45',
              'bg-neutral-900/95 dark:bg-neutral-800/95',
              'border border-white/10',
              position === 'top' && 'top-full -mt-1 border-t-0 border-l-0',
              position === 'bottom' && 'bottom-full -mb-1 border-b-0 border-r-0'
            )}
          />
        </div>
      )}
    </span>
  )
}
