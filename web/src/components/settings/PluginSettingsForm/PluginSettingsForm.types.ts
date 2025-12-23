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
  /** Filter to only show specific fields (e.g., ['embedding_model'] or ['chat_model']) */
  fieldFilter?: string[]
  /** Hide the settings tab entirely (useful when only showing filtered fields inline) */
  hideSettingsTab?: boolean
  /** Hide action tabs (Models, etc.) */
  hideActionTabs?: boolean
}
