import { useState, useRef, useEffect } from 'react'
import { cn } from '@/lib/utils'
import { videoOverlay } from '@/styles/semantic'
import type { DropdownBaseProps } from './DropdownBase.types'

export const DropdownBase = ({
  buttonContent,
  icon,
  minButtonWidth = '80px',
  ariaLabel,
  children,
  panelWidth = 'w-56',
}: DropdownBaseProps) => {
  const [showPanel, setShowPanel] = useState(false)
  const panelRef = useRef<HTMLDivElement>(null)
  const buttonRef = useRef<HTMLButtonElement>(null)

  // Close panel when clicking outside
  useEffect(() => {
    const handleClickOutside = (e: MouseEvent) => {
      if (
        panelRef.current &&
        buttonRef.current &&
        !panelRef.current.contains(e.target as Node) &&
        !buttonRef.current.contains(e.target as Node)
      ) {
        setShowPanel(false)
      }
    }

    if (showPanel) {
      document.addEventListener('mousedown', handleClickOutside)
    }

    return () => {
      document.removeEventListener('mousedown', handleClickOutside)
    }
  }, [showPanel])

  const close = () => setShowPanel(false)

  return (
    <div className="relative">
      {/* Dropdown button */}
      <button
        ref={buttonRef}
        onClick={() => setShowPanel(!showPanel)}
        className={cn(
          'text-xs sm:text-sm rounded-md px-2 sm:px-3 py-1.5 cursor-pointer flex items-center gap-1',
          videoOverlay.patterns.button,
          videoOverlay.ring.focus
        )}
        style={{ minWidth: minButtonWidth }}
        aria-label={ariaLabel}
        aria-expanded={showPanel}
        aria-haspopup="listbox"
      >
        {icon}
        <span>{buttonContent}</span>
        {/* Chevron */}
        <svg
          className={cn('w-3 h-3 transition-transform', showPanel && 'rotate-180')}
          fill="currentColor"
          viewBox="0 0 20 20"
        >
          <path
            fillRule="evenodd"
            d="M5.293 7.293a1 1 0 011.414 0L10 10.586l3.293-3.293a1 1 0 111.414 1.414l-4 4a1 1 0 01-1.414 0l-4-4a1 1 0 010-1.414z"
            clipRule="evenodd"
          />
        </svg>
      </button>

      {/* Dropdown panel */}
      {showPanel && (
        <div
          ref={panelRef}
          className={cn(
            'absolute bottom-full right-0 mb-2 rounded-lg overflow-hidden z-50',
            panelWidth,
            videoOverlay.patterns.panel
          )}
          role="listbox"
        >
          {children({ close })}
        </div>
      )}
    </div>
  )
}
