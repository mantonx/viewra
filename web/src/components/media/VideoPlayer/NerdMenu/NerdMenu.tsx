import { memo, useState, useRef, useEffect } from 'react'
import { Settings, Check } from 'lucide-react'
import type { NerdMenuProps } from './NerdMenu.types'

/**
 * NerdMenu - Advanced settings menu for video player
 *
 * Accessible via gear icon in video controls. Contains options for:
 * - Network stats overlay
 * - (Future: playback stats, debug info, etc.)
 */
export const NerdMenu = memo(({ showStats, onToggleStats }: NerdMenuProps) => {
  const [isOpen, setIsOpen] = useState(false)
  const menuRef = useRef<HTMLDivElement>(null)

  // Close menu when clicking outside
  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      if (menuRef.current && !menuRef.current.contains(event.target as Node)) {
        setIsOpen(false)
      }
    }

    if (isOpen) {
      document.addEventListener('mousedown', handleClickOutside)
      return () => document.removeEventListener('mousedown', handleClickOutside)
    }
  }, [isOpen])

  // Close menu on escape key
  useEffect(() => {
    const handleEscape = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        setIsOpen(false)
      }
    }

    if (isOpen) {
      document.addEventListener('keydown', handleEscape)
      return () => document.removeEventListener('keydown', handleEscape)
    }
  }, [isOpen])

  return (
    <div ref={menuRef} className="relative">
      {/* Gear Button */}
      <button
        onClick={() => setIsOpen(!isOpen)}
        className={`
          flex items-center justify-center w-8 h-8 rounded-md
          transition-all duration-150 cursor-pointer
          ${isOpen
            ? 'bg-white/20 text-white'
            : 'text-white/70 hover:text-white hover:bg-white/10'
          }
        `}
        aria-label="Advanced settings"
        aria-expanded={isOpen}
        aria-haspopup="menu"
        title="Advanced settings"
      >
        <Settings className="w-4 h-4" />
      </button>

      {/* Dropdown Menu */}
      {isOpen && (
        <div
          className="absolute bottom-full right-0 mb-2 w-56 bg-black/90 backdrop-blur-md rounded-lg shadow-xl border border-white/10 overflow-hidden"
          role="menu"
        >
          {/* Header */}
          <div className="px-3 py-2 border-b border-white/10">
            <span className="text-white/50 text-[10px] uppercase tracking-wider">
              Advanced
            </span>
          </div>

          {/* Menu Items */}
          <div className="py-1">
            {/* Stats for Nerds */}
            <button
              onClick={() => {
                onToggleStats()
                setIsOpen(false)
              }}
              className="w-full flex items-center justify-between px-3 py-2 text-sm text-white/90 hover:bg-white/10 transition-colors"
              role="menuitem"
            >
              <div className="flex flex-col items-start">
                <span>Stats for nerds</span>
                <span className="text-[10px] text-white/40">
                  Network speed, buffer health, quality
                </span>
              </div>
              <div className={`w-5 h-5 flex items-center justify-center ${showStats ? 'text-blue-400' : 'text-transparent'}`}>
                <Check className="w-3.5 h-3.5" strokeWidth={2.5} />
              </div>
            </button>

            {/* Keyboard shortcut hint */}
            <div className="px-3 py-1.5 text-[10px] text-white/30 border-t border-white/5 mt-1">
              Tip: Press <kbd className="px-1 py-0.5 bg-white/10 rounded text-white/50">D</kbd> to toggle stats
            </div>
          </div>
        </div>
      )}
    </div>
  )
})

NerdMenu.displayName = 'NerdMenu'

export default NerdMenu
