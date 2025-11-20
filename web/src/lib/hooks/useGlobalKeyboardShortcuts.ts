import { useEffect } from 'react'

export interface GlobalKeyboardShortcutsOptions {
  onSearch?: () => void
  onHelp?: () => void
  enabled?: boolean
}

/**
 * Hook for global keyboard shortcuts across the application.
 * Supports:
 * - Cmd/Ctrl+K: Global search
 * - /: Focus search input
 * - ?: Show help modal
 */
export const useGlobalKeyboardShortcuts = ({
  onSearch,
  onHelp,
  enabled = true,
}: GlobalKeyboardShortcutsOptions = {}) => {
  useEffect(() => {
    if (!enabled) {
      return
    }

    const handleKeyDown = (e: KeyboardEvent) => {
      // Ignore if user is typing in an input/textarea
      const target = e.target as HTMLElement
      if (
        target.tagName === 'INPUT' ||
        target.tagName === 'TEXTAREA' ||
        target.isContentEditable
      ) {
        // Exception: Allow Cmd/Ctrl+K even when focused on input
        if (!(e.key === 'k' && (e.metaKey || e.ctrlKey))) {
          return
        }
      }

      // Cmd/Ctrl+K: Global search
      if (e.key === 'k' && (e.metaKey || e.ctrlKey)) {
        e.preventDefault()
        if (onSearch) {
          onSearch()
        }
        return
      }

      // /: Focus search (only if not in input already)
      if (e.key === '/' && target.tagName !== 'INPUT' && target.tagName !== 'TEXTAREA') {
        e.preventDefault()
        if (onSearch) {
          onSearch()
        }
        return
      }

      // ?: Show help
      if (e.key === '?' && !e.shiftKey) {
        e.preventDefault()
        if (onHelp) {
          onHelp()
        }
        return
      }
    }

    document.addEventListener('keydown', handleKeyDown)
    return () => document.removeEventListener('keydown', handleKeyDown)
  }, [enabled, onSearch, onHelp])
}
