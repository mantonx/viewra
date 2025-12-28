import type { ReactNode } from 'react'
import type { InternalApiHandlersSettingDefinitionResponse as SettingDefinition } from '@/lib/api/generated/models'

export type PreferencesCategory = 'playback' | 'ui'

export type PreferencesCategoryConfig = {
  key: PreferencesCategory
  label: string
  description: string
  icon: ReactNode
}

export type PreferenceSettingRowProps = {
  definition: SettingDefinition
  isChanged: boolean
  isDefault: boolean
  /** Pre-rendered form field */
  children: ReactNode
}

export type PreferencesCategoryCardProps = {
  config: PreferencesCategoryConfig
  definitions: SettingDefinition[]
  hasChanges: boolean
  changeCount: number
  hasFieldChanged: (key: string) => boolean
  isFieldDefault: (key: string) => boolean
  onSave: () => void
  onDiscard: () => void
  isSaving: boolean
  /** Render function for form fields */
  renderField: (definition: SettingDefinition) => ReactNode
}
