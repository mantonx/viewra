import { text } from '@/styles/semantic'
import { cn } from '@/lib/utils'
import { X, Keyboard } from 'lucide-react'

interface KeyboardShortcutsProps {
  isOpen: boolean
  onClose: () => void
}

const shortcuts = [
  { key: 'Space', description: 'Play / Pause' },
  { key: '←', description: 'Seek backward 5s' },
  { key: '→', description: 'Seek forward 5s' },
  { key: '↑', description: 'Increase volume' },
  { key: '↓', description: 'Decrease volume' },
  { key: 'M', description: 'Mute / Unmute' },
  { key: '?', description: 'Show keyboard shortcuts' },
]

export const KeyboardShortcuts = ({ isOpen, onClose }: KeyboardShortcutsProps) => {
  if (!isOpen) {return null}

  return (
    <div className="fixed inset-0 z-[70] flex items-center justify-center pointer-events-none">
      {/* Backdrop */}
      <div
        className="absolute inset-0 bg-black/50 pointer-events-auto"
        onClick={onClose}
      />

      {/* Modal */}
      <div className="relative w-full max-w-md bg-neutral-900 rounded-lg shadow-2xl pointer-events-auto overflow-hidden">
        {/* Header */}
        <div className="flex items-center justify-between p-4 border-b border-neutral-800">
          <div className="flex items-center gap-2">
            <Keyboard size={20} className="text-rose-500" />
            <h2 className="text-lg font-semibold text-white">Keyboard Shortcuts</h2>
          </div>
          <button
            onClick={onClose}
            className={cn(
              'p-2 rounded hover:bg-neutral-800 transition-colors',
              text.tertiary
            )}
            aria-label="Close keyboard shortcuts"
          >
            <X size={20} />
          </button>
        </div>

        {/* Shortcuts list */}
        <div className="p-4">
          <div className="space-y-3">
            {shortcuts.map((shortcut, index) => (
              <div
                key={index}
                className="flex items-center justify-between py-2"
              >
                <span className={cn('text-sm', text.secondary)}>{shortcut.description}</span>
                <kbd
                  className={cn(
                    'px-3 py-1.5 text-xs font-mono font-semibold',
                    'bg-neutral-800 text-white rounded border border-neutral-700',
                    'shadow-sm min-w-[2.5rem] text-center'
                  )}
                >
                  {shortcut.key}
                </kbd>
              </div>
            ))}
          </div>

          <div className={cn('mt-4 pt-4 border-t border-neutral-800 text-xs', text.tertiary)}>
            <p>Note: Keyboard shortcuts work when not typing in an input field.</p>
          </div>
        </div>
      </div>
    </div>
  )
}
