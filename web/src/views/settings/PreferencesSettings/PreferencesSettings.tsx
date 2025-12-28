import { Loading, Alert } from '@/components/ui'
import { SettingsPage } from '@/components/common'
import { SettingRow } from '@/components/settings/ui'
import { useToast } from '@/lib/hooks/useToast'
import { usePreferencesData } from './hooks'
import { PREFERENCES_CATEGORIES } from './PreferencesSettings.constants'
import type { InternalApiHandlersSettingDefinitionResponse as SettingDefinition } from '@/lib/api/generated/models'

/**
 * Preferences Settings page component.
 * Manages user-specific preferences like playback and UI settings.
 * Auto-saves on change.
 */
export const PreferencesSettings = () => {
  const toast = useToast()

  const {
    isLoading,
    error,
    initialValues,
    definitionsByCategory,
    saveValues,
  } = usePreferencesData()

  // Handle saving a single value
  const handleSave = async (key: string, value: unknown) => {
    try {
      await saveValues({ [key]: value })
      toast.success('Setting saved')
    } catch {
      toast.error('Failed to save setting')
    }
  }

  // Render a setting based on its definition
  const renderSetting = (definition: SettingDefinition) => {
    const key = definition.key ?? ''
    if (!key) {
      return null
    }

    const currentValue = initialValues[key]
    const hasOptions = definition.options && definition.options.length > 0

    // Boolean toggle
    if (definition.type === 'bool') {
      return (
        <SettingRow
          key={key}
          type="toggle"
          label={definition.label ?? key}
          description={definition.description}
          value={currentValue === true || currentValue === 'true'}
          onChange={(newValue) => handleSave(key, newValue)}
        />
      )
    }

    // Select with options
    if (hasOptions) {
      const options = (definition.options ?? []).map((opt) => ({
        value: opt.value ?? '',
        label: opt.label ?? opt.value ?? '',
      }))
      return (
        <SettingRow
          key={key}
          type="select"
          label={definition.label ?? key}
          description={definition.description}
          value={String(currentValue ?? '')}
          onChange={(newValue) => handleSave(key, newValue)}
          options={options}
        />
      )
    }

    // Text input (readonly for now - we don't have text settings in preferences)
    return (
      <SettingRow
        key={key}
        type="readonly"
        label={definition.label ?? key}
        description={definition.description}
        value={String(currentValue ?? '')}
      />
    )
  }

  if (isLoading) {
    return (
      <SettingsPage>
        <SettingsPage.Header
          title="Preferences"
          description="Customize your viewing experience"
        />
        <SettingsPage.Card>
          <Loading text="Loading preferences..." />
        </SettingsPage.Card>
      </SettingsPage>
    )
  }

  if (error) {
    return (
      <SettingsPage>
        <SettingsPage.Header
          title="Preferences"
          description="Customize your viewing experience"
        />
        <SettingsPage.Card>
          <Alert variant="error">Failed to load settings</Alert>
        </SettingsPage.Card>
      </SettingsPage>
    )
  }

  return (
    <SettingsPage>
      <SettingsPage.Header
        title="Preferences"
        description="Customize your viewing experience"
      />

      {PREFERENCES_CATEGORIES.map((config) => {
        const definitions = definitionsByCategory[config.key]
        if (!definitions || definitions.length === 0) {
          return null
        }

        return (
          <SettingsPage.Card
            key={config.key}
            title={config.label}
            description={config.description}
            className="mb-6"
          >
            <div className="space-y-4">
              {definitions.map(renderSetting)}
            </div>
          </SettingsPage.Card>
        )
      })}
    </SettingsPage>
  )
}
