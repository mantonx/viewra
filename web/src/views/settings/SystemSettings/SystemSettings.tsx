import { useMemo } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { Alert, Loading } from '@/components/ui'
import { SettingsPage } from '@/components/common'
import {
  SettingRow,
  SystemInfoCard,
  ServerRestartCard,
  MaintenanceCard,
  DatabaseWarningBanner,
  DatabaseCard,
  getCategoryConfig,
} from '@/components/settings'
import { useToast } from '@/lib/hooks/useToast'
import {
  useGetApiSettingsSchema,
  useGetApiSettingsSystemEffective,
  putApiSettingsSystemKey,
  getGetApiSettingsSystemEffectiveQueryKey,
} from '@/lib/api/generated/settings/settings'
import type { InternalApiHandlersEffectiveSettingResponse as EffectiveSetting } from '@/lib/api/generated/models'
import { cn } from '@/lib/utils'
import { Info, AlertTriangle } from 'lucide-react'

type SelectOption = { value: string; label: string }

/**
 * System Settings page component.
 * Manages server-wide system settings with environment variable overrides.
 * Auto-saves on field change.
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

  // Check if any category has restartable settings
  const hasRestartableSettings = useMemo(() => {
    return settings.some((s) => s.key && schemaRestartable[s.key])
  }, [settings, schemaRestartable])

  // Save a single setting value
  const saveSetting = async (key: string, value: unknown) => {
    try {
      const response = await putApiSettingsSystemKey(key, { value })
      if (response.status !== 200) {
        throw new Error(`Failed to save ${key}`)
      }
      await queryClient.invalidateQueries({
        queryKey: getGetApiSettingsSystemEffectiveQueryKey(),
      })
      toast.success('Setting saved')
    } catch {
      toast.error('Failed to save setting')
    }
  }

  // Build description with env var info for locked fields
  const getDescription = (setting: EffectiveSetting): string | undefined => {
    const baseDesc = setting.description || ''
    if (setting.source === 'env_var' && setting.envVar) {
      const envNote = `Controlled by \`${setting.envVar}\``
      return baseDesc ? `${baseDesc}. ${envNote}` : envNote
    }
    return baseDesc || undefined
  }

  // Format value for display
  const formatValue = (value: unknown): string => {
    if (typeof value === 'boolean') {
      return value ? 'Enabled' : 'Disabled'
    }
    return String(value ?? '—')
  }

  if (effectiveLoading) {
    return (
      <SettingsPage>
        <SettingsPage.Header
          title="System Settings"
          description="Configure server-wide settings and view system information"
        />
        <SettingsPage.Card>
          <Loading text="Loading system settings..." />
        </SettingsPage.Card>
      </SettingsPage>
    )
  }

  if (effectiveError) {
    return (
      <SettingsPage>
        <SettingsPage.Header
          title="System Settings"
          description="Configure server-wide settings and view system information"
        />
        <SettingsPage.Card>
          <Alert variant="error">Failed to load system settings.</Alert>
        </SettingsPage.Card>
      </SettingsPage>
    )
  }

  return (
    <SettingsPage>
      <SettingsPage.Header
        title="System Settings"
        description="Configure server-wide settings and view system information"
      />

      {/* Restart required notice */}
      {hasRestartableSettings && (
        <div
          className={cn(
            'flex items-center gap-2 px-4 py-3 rounded-xl mb-6',
            'bg-amber-50 dark:bg-amber-500/10',
            'border border-amber-200 dark:border-amber-500/20',
            'text-amber-700 dark:text-amber-400'
          )}
        >
          <AlertTriangle className="w-4 h-4 shrink-0" />
          <span className="text-sm">
            Some changes require a server restart to take effect
          </span>
        </div>
      )}

      {/* Env var info banner */}
      {hasEnvOverrides && (
        <div
          className={cn(
            'flex items-start gap-3 p-4 rounded-xl mb-6',
            'bg-blue-50 dark:bg-blue-950/30',
            'border border-blue-200 dark:border-blue-900'
          )}
        >
          <Info className="w-5 h-5 text-blue-600 dark:text-blue-400 shrink-0 mt-0.5" />
          <div>
            <p className="text-sm font-medium text-blue-800 dark:text-blue-300">
              Some settings are controlled by environment variables
            </p>
            <p className="text-xs mt-1 text-blue-700 dark:text-blue-400">
              These settings are shown as read-only. Update the environment variable and restart
              the server to change them.
            </p>
          </div>
        </div>
      )}

      {/* Database warnings (SQLite in K8s, etc.) */}
      <DatabaseWarningBanner />

      {/* Hardware/System Info Card */}
      <SystemInfoCard />

      {/* Database Card */}
      <div className="mt-6">
        <DatabaseCard />
      </div>

      {/* Server Control Cards */}
      <div className="mt-6 grid grid-cols-1 lg:grid-cols-2 gap-6">
        <ServerRestartCard />
        <MaintenanceCard />
      </div>

      {/* Category Groups */}
      {Object.entries(groupedSettings).map(([category, categorySettings]) => {
        const config = getCategoryConfig(category)

        return (
          <SettingsPage.Card
            key={category}
            title={config.label}
            description={config.description}
            className="mt-6"
          >
            <div className="space-y-4">
              {categorySettings.map((setting) => {
                const key = setting.key || ''
                if (!key) {
                  return null
                }

                const isLocked = setting.locked || setting.readOnly
                const options = schemaOptions[key]
                const description = getDescription(setting)

                // Read-only / env-var locked field
                if (isLocked) {
                  return (
                    <SettingRow
                      key={key}
                      type="readonly"
                      label={setting.label || key}
                      description={description}
                      value={formatValue(setting.value)}
                    />
                  )
                }

                // Boolean toggle
                if (setting.type === 'bool') {
                  return (
                    <SettingRow
                      key={key}
                      type="toggle"
                      label={setting.label || key}
                      description={description}
                      value={setting.value === true || setting.value === 'true'}
                      onChange={(newValue) => saveSetting(key, newValue)}
                    />
                  )
                }

                // Select with options
                if (options?.length) {
                  return (
                    <SettingRow
                      key={key}
                      type="select"
                      label={setting.label || key}
                      description={description}
                      value={String(setting.value ?? '')}
                      onChange={(newValue) => saveSetting(key, newValue)}
                      options={options}
                    />
                  )
                }

                // Fallback: show as readonly for non-standard types
                return (
                  <SettingRow
                    key={key}
                    type="readonly"
                    label={setting.label || key}
                    description={description}
                    value={formatValue(setting.value)}
                  />
                )
              })}
            </div>
          </SettingsPage.Card>
        )
      })}
    </SettingsPage>
  )
}
