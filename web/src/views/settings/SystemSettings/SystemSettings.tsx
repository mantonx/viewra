import { useEffect, useMemo } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { Card, CardHeader, CardContent, CardFooter, Button, Alert, Loading } from '@/components/ui'
import { FormInput, FormSelect, FormToggle, FormNumberInput } from '@/components/ui/Form'
import { PageHeader } from '@/components/common'
import {
  SourceBadge,
  RestartBadge,
  SystemInfoCard,
  ServerRestartCard,
  getCategoryConfig,
} from '@/components/settings'
import { useToast } from '@/lib/hooks/useToast'
import { useSettingsForm } from '@/lib/hooks'
import {
  useGetApiSettingsSchema,
  useGetApiSettingsSystemEffective,
  putApiSettingsSystemKey,
  getGetApiSettingsSystemEffectiveQueryKey,
} from '@/lib/api/generated/settings/settings'
import type { InternalApiHandlersEffectiveSettingResponse as EffectiveSetting } from '@/lib/api/generated/models'
import { text } from '@/styles/semantic'
import { cn } from '@/lib/utils'
import { RotateCcw, Info } from 'lucide-react'

type SelectOption = { value: string; label: string }

/**
 * System Settings page component.
 * Manages server-wide system settings with environment variable overrides.
 * Note: Admin access is enforced by AdminRoute wrapper in the route file.
 */
export const SystemSettings = () => {
  const toast = useToast()
  const queryClient = useQueryClient()

  // Fetch effective settings and schema
  const {
    data: effectiveData,
    isLoading: effectiveLoading,
    error: effectiveError,
  } = useGetApiSettingsSystemEffective()

  const { data: schemaData } = useGetApiSettingsSchema()

  // Extract settings
  const settings: EffectiveSetting[] = useMemo(() => {
    return effectiveData?.status === 200 && 'settings' in effectiveData.data
      ? effectiveData.data.settings || []
      : []
  }, [effectiveData])

  // Build schema maps
  const { schemaOptions, schemaRestartable } = useMemo(() => {
    const options: Record<string, SelectOption[]> = {}
    const restartable: Record<string, boolean> = {}

    if (schemaData?.status === 200 && 'system' in schemaData.data) {
      for (const def of schemaData.data.system || []) {
        const key = def.key || ''
        if (def.options?.length) {
          options[key] = def.options.map((o) => ({
            value: o.value || '',
            label: o.label || o.value || '',
          }))
        }
        if (def.restartable) {
          restartable[key] = true
        }
      }
    }
    return { schemaOptions: options, schemaRestartable: restartable }
  }, [schemaData])

  // Build original values
  const originalValues = useMemo(() => {
    const values: Record<string, unknown> = {}
    for (const setting of settings) {
      if (setting.key) {
        values[setting.key] = setting.value
      }
    }
    return values
  }, [settings])

  // Group settings by category (exclude 'system' category - shown in hardware card)
  const groupedSettings = useMemo(() => {
    return settings
      .filter((s) => s.category !== 'system')
      .reduce<Record<string, EffectiveSetting[]>>((acc, setting) => {
        const category = setting.category || 'other'
        if (!acc[category]) {
          acc[category] = []
        }
        acc[category].push(setting)
        return acc
      }, {})
  }, [settings])

  // Check if any settings are env-var controlled
  const hasEnvOverrides = settings.some((s) => s.source === 'env_var')

  // Build category configs for form
  const categoryConfigs = useMemo(() => {
    return Object.keys(groupedSettings).map((category) => ({
      id: category,
      fields: groupedSettings[category]
        .filter((s) => !s.locked && !s.readOnly)
        .map((s) => s.key || '')
        .filter(Boolean),
      onSave: async (values: Record<string, unknown>) => {
        for (const [key, value] of Object.entries(values)) {
          const response = await putApiSettingsSystemKey(key, { value })
          if (response.status !== 200) {
            throw new Error(`Failed to save ${key}`)
          }
        }
        await queryClient.invalidateQueries({
          queryKey: getGetApiSettingsSystemEffectiveQueryKey(),
        })
        toast.success('Settings saved')
      },
    }))
  }, [groupedSettings, queryClient, toast])

  // Initialize form
  const { form, hasChanges, getChangeCount, saveCategory, discardCategory, isSaving, resetOriginalValues } =
    useSettingsForm({
      defaultValues: originalValues,
      categories: categoryConfigs,
    })

  // Reset form when data changes
  useEffect(() => {
    if (Object.keys(originalValues).length > 0) {
      resetOriginalValues(originalValues)
    }
  }, [originalValues, resetOriginalValues])

  // Helper to check if field changed
  const hasFieldChanged = (key: string) => {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const currentValue = form.getFieldValue(key as any)
    return currentValue !== originalValues[key]
  }

  // Render form field based on setting type
  const renderField = (setting: EffectiveSetting) => {
    const key = setting.key || ''
    const options = schemaOptions[key]
    const isDisabled = setting.locked || setting.readOnly

    if (setting.type === 'bool') {
      return (
        <form.Field name={key}>
          {(field) => <FormToggle field={field} label={setting.label || key} disabled={isDisabled} />}
        </form.Field>
      )
    }

    if (options?.length) {
      return (
        <form.Field name={key}>
          {(field) => <FormSelect field={field} options={options} disabled={isDisabled} />}
        </form.Field>
      )
    }

    if (setting.type === 'int') {
      return (
        <form.Field name={key}>
          {(field) => <FormNumberInput field={field} disabled={isDisabled} className="max-w-xs" />}
        </form.Field>
      )
    }

    return (
      <form.Field name={key}>
        {(field) => <FormInput field={field} disabled={isDisabled} />}
      </form.Field>
    )
  }

  if (effectiveLoading) {
    return (
      <div className="h-full overflow-auto">
        <div className="p-8 page-enter">
          <Loading text="Loading system settings..." />
        </div>
      </div>
    )
  }

  if (effectiveError) {
    return (
      <div className="h-full overflow-auto">
        <div className="p-8 page-enter">
          <Alert variant="error">Failed to load system settings.</Alert>
        </div>
      </div>
    )
  }

  return (
    <div className="h-full overflow-auto">
      <div className="p-8 page-enter">
        <PageHeader
          title="System Settings"
          description="Configure server-wide settings and view system information"
        />

        <div className="mt-6 space-y-6 max-w-3xl">
          {/* Env var info banner */}
          {hasEnvOverrides && (
            <div
              className={cn(
                'flex items-start gap-3 p-4 rounded-lg',
                'bg-blue-50 dark:bg-blue-950/30 border border-blue-200 dark:border-blue-900'
              )}
            >
              <Info className="w-5 h-5 text-blue-600 dark:text-blue-400 shrink-0 mt-0.5" />
              <div>
                <p className={cn('text-sm font-medium', 'text-blue-800 dark:text-blue-300')}>
                  Some settings are controlled by environment variables
                </p>
                <p className={cn('text-xs mt-1', 'text-blue-700 dark:text-blue-400')}>
                  Settings with the &quot;Environment&quot; badge cannot be changed here. Update the
                  environment variable and restart the server to change them.
                </p>
              </div>
            </div>
          )}

          {/* Hardware/System Info Card */}
          <SystemInfoCard />

          {/* Server Restart Card */}
          <ServerRestartCard />

          {/* Category Cards */}
          {Object.entries(groupedSettings).map(([category, categorySettings]) => {
            const config = getCategoryConfig(category)
            const Icon = config.icon
            const catHasChanges = hasChanges(category)
            const changeCount = getChangeCount(category)

            // Split into read-only and editable
            const readOnlySettings = categorySettings.filter((s) => s.locked || s.readOnly)
            const editableSettings = categorySettings.filter((s) => !s.locked && !s.readOnly)

            return (
              <Card key={category}>
                <CardHeader className="border-b border-neutral-100 dark:border-neutral-800">
                  <div className="flex items-center gap-3">
                    <div
                      className={cn(
                        'p-2 rounded-lg',
                        'bg-neutral-100 dark:bg-neutral-800',
                        'text-neutral-600 dark:text-neutral-400'
                      )}
                    >
                      <Icon className="w-5 h-5" />
                    </div>
                    <div>
                      <h2 className={cn('text-lg font-semibold', text.primary)}>{config.label}</h2>
                      {config.description && (
                        <p className={cn('text-sm mt-0.5', text.secondary)}>{config.description}</p>
                      )}
                    </div>
                  </div>
                </CardHeader>

                <CardContent className="divide-y divide-neutral-100 dark:divide-neutral-800">
                  {/* Read-only settings */}
                  {readOnlySettings.map((setting) => (
                    <div key={setting.key} className="flex items-start justify-between gap-4 py-3">
                      <div className="flex-1 min-w-0">
                        <div className="flex items-center gap-2 flex-wrap">
                          <span className={cn('text-sm font-medium', text.primary)}>
                            {setting.label || setting.key}
                          </span>
                          <SourceBadge
                            source={setting.source}
                            envVar={setting.envVar}
                            locked={setting.locked}
                            readOnly={setting.readOnly}
                          />
                        </div>
                        {setting.description && (
                          <p className={cn('text-xs mt-1', text.tertiary)}>{setting.description}</p>
                        )}
                      </div>
                      <div
                        className={cn(
                          'px-3 py-1.5 rounded-md text-sm font-mono shrink-0',
                          'bg-neutral-100 dark:bg-neutral-800',
                          text.secondary
                        )}
                      >
                        {typeof setting.value === 'boolean'
                          ? setting.value
                            ? 'Enabled'
                            : 'Disabled'
                          : String(setting.value ?? '—')}
                      </div>
                    </div>
                  ))}

                  {/* Editable settings */}
                  {editableSettings.map((setting) => {
                    const key = setting.key || ''
                    const isChanged = hasFieldChanged(key)
                    const restartable = schemaRestartable[key]

                    return (
                      <div
                        key={key}
                        className={cn(
                          'py-3 transition-all duration-150',
                          isChanged &&
                            'bg-amber-50/50 dark:bg-amber-950/20 -mx-4 px-4 rounded-lg border-l-2 border-amber-500'
                        )}
                      >
                        <div className="flex items-center gap-2 flex-wrap mb-2">
                          <span className={cn('text-sm font-medium', text.primary)}>
                            {setting.label || key}
                          </span>
                          <SourceBadge
                            source={setting.source}
                            envVar={setting.envVar}
                            locked={setting.locked}
                            readOnly={setting.readOnly}
                          />
                          {restartable && <RestartBadge />}
                        </div>
                        {renderField(setting)}
                        {setting.description && (
                          <p className={cn('text-xs mt-1.5', text.tertiary)}>{setting.description}</p>
                        )}
                      </div>
                    )
                  })}
                </CardContent>

                {catHasChanges && (
                  <CardFooter className="flex justify-end gap-2 border-t border-neutral-200 dark:border-neutral-700 pt-4">
                    <Button
                      variant="ghost"
                      onClick={() => {
                        discardCategory(category)
                        toast.info('Changes discarded')
                      }}
                      disabled={isSaving(category)}
                    >
                      <RotateCcw className="w-4 h-4 mr-1.5" />
                      Discard
                    </Button>
                    <Button onClick={() => saveCategory(category)} isLoading={isSaving(category)}>
                      Save {changeCount > 1 ? `(${changeCount})` : ''}
                    </Button>
                  </CardFooter>
                )}
              </Card>
            )
          })}
        </div>
      </div>
    </div>
  )
}
