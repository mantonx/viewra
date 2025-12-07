import { useState, useRef, useEffect } from 'react'
import type { DropdownOption, DropdownSelectorProps } from './DropdownSelector.types'

export type { DropdownOption }

export const DropdownSelector = <T,>({
  displayText,
  icon,
  title,
  subtitle,
  options,
  onSelect,
  minButtonWidth = '80px',
  panelWidth = 'w-56',
  ariaLabel,
  footer,
}: DropdownSelectorProps<T>) => {
  const [showPanel, setShowPanel] = useState(false)
  const [hoveredIndex, setHoveredIndex] = useState<number | null>(null)
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

  return (
    <div className="relative">
      {/* Dropdown button */}
      <button
        ref={buttonRef}
        onClick={() => setShowPanel(!showPanel)}
        className="bg-white/10 backdrop-blur-sm text-white text-xs sm:text-sm rounded-md px-2 sm:px-3 py-1.5 hover:bg-white/20 transition-all cursor-pointer border border-white/20 focus:outline-none focus:ring-2 focus:ring-primary-500/50 flex items-center gap-1"
        style={{ minWidth: minButtonWidth }}
        aria-label={ariaLabel}
        aria-expanded={showPanel}
        aria-haspopup="listbox"
      >
        {icon}
        <span>{displayText}</span>
        {/* Chevron */}
        <svg
          className={`w-3 h-3 transition-transform ${showPanel ? 'rotate-180' : ''}`}
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
          className={`absolute bottom-full right-0 mb-2 ${panelWidth} bg-black/95 backdrop-blur-md rounded-lg shadow-xl border border-white/20 overflow-hidden z-50`}
          role="listbox"
          aria-label={title}
        >
          {/* Header */}
          <div className="px-3 py-2 border-b border-white/10">
            <div className="text-white text-sm font-semibold">{title}</div>
            {subtitle && (
              <div className="text-white/60 text-xs mt-0.5">{subtitle}</div>
            )}
          </div>

          {/* Options */}
          <div className="max-h-80 overflow-y-auto">
            {options.map((option, index) => {
              const isHovered = hoveredIndex === index

              return (
                <button
                  key={index}
                  onClick={() => {
                    onSelect(option.value)
                    setShowPanel(false)
                  }}
                  onMouseEnter={() => setHoveredIndex(index)}
                  onMouseLeave={() => setHoveredIndex(null)}
                  className={`w-full px-3 py-2.5 flex items-center justify-between hover:bg-white/10 transition-colors cursor-pointer ${
                    option.isSelected ? 'bg-primary-500/20' : ''
                  } ${isHovered && !option.isSelected ? 'bg-white/5' : ''}`}
                  role="option"
                  aria-selected={option.isSelected}
                >
                  <div className="flex items-center gap-2">
                    {option.icon}
                    <span className="text-white text-sm">{option.label}</span>
                    {option.sublabel && (
                      <span className="text-white/50 text-xs">{option.sublabel}</span>
                    )}
                  </div>
                  {option.isSelected && (
                    <svg className="w-4 h-4 text-primary-400" fill="currentColor" viewBox="0 0 20 20">
                      <path
                        fillRule="evenodd"
                        d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z"
                        clipRule="evenodd"
                      />
                    </svg>
                  )}
                </button>
              )
            })}
          </div>

          {/* Footer */}
          {footer && (
            <div className="px-3 py-2 border-t border-white/10 bg-white/5">
              {footer}
            </div>
          )}
        </div>
      )}
    </div>
  )
}
