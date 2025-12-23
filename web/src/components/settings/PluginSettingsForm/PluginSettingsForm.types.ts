/**
 * Types for PluginSettingsForm component.
 */

export interface PluginSettingsFormProps {
  /** Plugin ID */
  pluginId: string
  /** Called when settings have unsaved changes */
  onSettingsChange?: (hasChanges: boolean) => void
  /** Additional class names */
  className?: string
  /** Filter to only show specific schema fields by name */
  fieldFilter?: string[]
  /** Hide the settings tab entirely */
  hideSettingsTab?: boolean
  /** Hide all action tabs */
  hideActionTabs?: boolean
  /** Filter to only show specific action tabs by ID */
  tabFilter?: string[]
}
