import { useEffect, useMemo } from 'react'
import { Loading, Alert } from '@/components/ui'
import { FormInput, FormSelect, FormToggle } from '@/components/ui/Form'
import { PageHeader } from '@/components/common'
import { useToast } from '@/lib/hooks/useToast'
import { useSettingsForm } from '@/lib/hooks'
import { PreferencesCategoryCard } from './components'
import { usePreferencesData } from './hooks'
import { PREFERENCES_CATEGORIES } from './PreferencesSettings.constants'
import type { InternalApiHandlersSettingDefinitionResponse as SettingDefinition } from '@/lib/api/generated/models'

/**
 * Preferences Settings page component.
 * Manages user-specific preferences like playback and UI settings.
 */
export const PreferencesSettings = () => {
  const toast = useToast()

  const {
    isLoading,
    error,
    initialValues,
    definitionsByCategory,
    isFieldDefault,
    saveValues,
  } = usePreferencesData()

  // Build category configurations for useSettingsForm
  const categoryConfigs = useMemo(() => {
    return PREFERENCES_CATEGORIES.map((cat) => ({
      id: cat.key,
      fields: definitionsByCategory[cat.key].map((def) => def.key ?? '').filter(Boolean),
      onSave: async (values: Record<string, unknown>) => {
        await saveValues(values)
        toast.success('Settings saved')
      },
    }))
  }, [definitionsByCategory, saveValues, toast])

  const { form, hasChanges, getChangeCount, saveCategory, discardCategory, isSaving, resetOriginalValues } =
    useSettingsForm({
      defaultValues: initialValues,
      categories: categoryConfigs,
    })

  // Reset form when API data changes
  useEffect(() => {
    if (Object.keys(initialValues).length > 0) {
      resetOriginalValues(initialValues)
    }
  }, [initialValues, resetOriginalValues])

  // Helper to check if a specific field has been changed
  const hasFieldChanged = (key: string) => {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const currentValue = form.getFieldValue(key as any)
    const originalValue = initialValues[key]
    return currentValue !== originalValue
  }

  // Render the appropriate form field for a setting definition
  const renderField = (definition: SettingDefinition) => {
    const key = definition.key ?? ''
    const hasOptions = definition.options && definition.options.length > 0
    const options = (definition.options ?? []).map((opt) => ({
      value: opt.value ?? '',
      label: opt.label ?? opt.value ?? '',
    }))

    if (definition.type === 'bool') {
      return (
        <form.Field name={key}>
          {(field) => <FormToggle field={field} label={definition.label ?? key} />}
        </form.Field>
      )
    }

    if (hasOptions) {
      return (
        <form.Field name={key}>
          {(field) => <FormSelect field={field} options={options} className="w-48" />}
        </form.Field>
      )
    }

    return (
      <form.Field name={key}>
        {(field) => <FormInput field={field} className="w-48" />}
      </form.Field>
    )
  }

  if (isLoading) {
    return (
      <div className="p-8 page-enter">
        <PageHeader title="Preferences" description="Customize your viewing experience" />
        <div className="mt-8">
          <Loading text="Loading preferences..." />
        </div>
      </div>
    )
  }

  if (error) {
    return (
      <div className="p-8 page-enter">
        <PageHeader title="Preferences" description="Customize your viewing experience" />
        <Alert variant="error" className="mt-6">
          Failed to load settings
        </Alert>
      </div>
    )
  }

  return (
    <div className="p-8 page-enter">
      <PageHeader title="Preferences" description="Customize your viewing experience" />

      <div className="mt-6 space-y-6 max-w-3xl">
        {PREFERENCES_CATEGORIES.map((config) => (
          <PreferencesCategoryCard
            key={config.key}
            config={config}
            definitions={definitionsByCategory[config.key]}
            hasChanges={hasChanges(config.key)}
            changeCount={getChangeCount(config.key)}
            hasFieldChanged={hasFieldChanged}
            isFieldDefault={isFieldDefault}
            onSave={() => saveCategory(config.key)}
            onDiscard={() => {
              discardCategory(config.key)
              toast.info('Changes discarded')
            }}
            isSaving={isSaving(config.key)}
            renderField={renderField}
          />
        ))}
      </div>
    </div>
  )
}
