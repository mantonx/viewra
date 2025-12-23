/**
 * Types for PluginSettingsForm component.
 */

import type { Capability } from '@/lib/types/schema-actions'

export interface PluginSettingsFormProps {
  /** Plugin ID */
  pluginId: string
  /** Called when settings have unsaved changes */
  onSettingsChange?: (hasChanges: boolean) => void
  /** Additional class names */
  className?: string
  /**
   * Filter to show only sections matching this capability.
   * Uses x-viewra-sections from the plugin schema.
   * If omitted, all properties and actions are rendered.
   */
  capability?: Capability
}
