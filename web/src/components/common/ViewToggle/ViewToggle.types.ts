/**
 * Types for ViewToggle component
 */

export type ViewMode = 'grid' | 'list'

export interface ViewToggleProps {
  /** Current view mode */
  value: ViewMode

  /** Callback when view mode changes */
  onChange: (mode: ViewMode) => void
}
