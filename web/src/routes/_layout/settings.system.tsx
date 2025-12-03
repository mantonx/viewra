import { createFileRoute } from '@tanstack/react-router'
import { useState } from 'react'
import { useAuth } from '@/contexts'
import {
  Card,
  CardHeader,
  CardContent,
  CardFooter,
  Button,
  Input,
  Select,
  Alert,
  Loading,
  SettingToggle,
} from '@/components/ui'
import { PageHeader } from '@/components/common'
import { useToast } from '@/lib/hooks/useToast'
import { text } from '@/styles/semantic'
import { cn } from '@/lib/utils'
import {
  Settings2,
  Server,
  Film,
  Folder,
  Shield,
  RefreshCw,
  Lock,
  Database,
  Cpu,
  HardDrive,
  MemoryStick,
  MonitorPlay,
  Info,
  Eye,
  RotateCcw,
  Check,
} from 'lucide-react'
import {
  useGetApiSettingsSchema,
  useGetApiSettingsSystemEffective,
  putApiSettingsSystemKey,
} from '@/lib/api/generated/settings/settings'
import { useGetApiSystemInfo } from '@/lib/api/generated/system/system'
import type { InternalApiHandlersEffectiveSettingResponse as EffectiveSetting } from '@/lib/api/generated/models'

// Category icons and labels
const categoryConfig: Record<
  string,
  { icon: typeof Server; label: string; description: string }
> = {
  server: {
    icon: Server,
    label: 'Server',
    description: 'Core server configuration',
  },
  transcoding: {
    icon: Film,
    label: 'Transcoding',
    description: 'Video transcoding and encoding options',
  },
  scanning: {
    icon: Folder,
    label: 'Library Scanning',
    description: 'File scanning and indexing behavior',
  },
  security: {
    icon: Shield,
    label: 'Security',
    description: 'Security and access policies',
  },
}

// Compact inline source badge - smaller and next to label
const SourceBadge = ({
  source,
  envVar,
  locked,
  readOnly,
}: {
  source?: string
  envVar?: string
  locked?: boolean
  readOnly?: boolean
}) => {
  if (readOnly || source === 'detected') {
    return (
      <span
        className={cn(
          'inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-[10px] font-medium',
          'bg-slate-100 dark:bg-slate-800 text-slate-500 dark:text-slate-400'
        )}
      >
        <Eye className="w-2.5 h-2.5" />
        detected
      </span>
    )
  }

  if (source === 'env_var' && locked) {
    return (
      <span
        className={cn(
          'inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-[10px] font-medium font-mono',
          'bg-amber-100 dark:bg-amber-900/50 text-amber-700 dark:text-amber-400'
        )}
      >
        <Lock className="w-2.5 h-2.5" />
        {envVar}
      </span>
    )
  }

  if (source === 'database') {
    return (
      <span
        className={cn(
          'inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-[10px] font-medium',
          'bg-emerald-100 dark:bg-emerald-900/50 text-emerald-700 dark:text-emerald-400'
        )}
      >
        <Database className="w-2.5 h-2.5" />
        saved
      </span>
    )
  }

  // Default - very subtle
  return (
    <span
      className={cn(
        'inline-flex items-center px-1.5 py-0.5 rounded text-[10px] font-medium',
        'bg-neutral-100 dark:bg-neutral-800 text-neutral-400 dark:text-neutral-500'
      )}
    >
      default
    </span>
  )
}

// Restart warning badge
const RestartBadge = () => (
  <span
    className={cn(
      'inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-[10px] font-medium',
      'bg-orange-100 dark:bg-orange-900/30 text-orange-600 dark:text-orange-400'
    )}
  >
    <RotateCcw className="w-2.5 h-2.5" />
    restart required
  </span>
)

// System Info Card Component
const SystemInfoCard = () => {
  const { data: systemInfo, isLoading } = useGetApiSystemInfo()

  if (isLoading) {
    return (
      <Card className="relative overflow-hidden">
        <div className="absolute inset-0 bg-gradient-to-br from-primary-500/5 via-transparent to-primary-500/10" />
        <CardContent className="py-8">
          <Loading text="Detecting hardware..." />
        </CardContent>
      </Card>
    )
  }

  const info = systemInfo?.status === 200 ? systemInfo.data : null
  if (!info) return null

  const cpuInfo = info.cpu
  const memoryInfo = info.memory
  const gpuInfo = info.gpu

  return (
    <Card className="relative overflow-hidden">
      <div className="absolute inset-0 bg-gradient-to-br from-primary-500/5 via-transparent to-primary-500/10" />

      <CardHeader className="relative border-b border-neutral-100 dark:border-neutral-800">
        <div className="flex items-center gap-3">
          <div
            className={cn(
              'p-2.5 rounded-xl',
              'bg-gradient-to-br from-primary-500 to-primary-600',
              'text-white shadow-lg shadow-primary-500/25'
            )}
          >
            <MonitorPlay className="w-5 h-5" />
          </div>
          <div>
            <h2 className={cn('text-lg font-semibold', text.primary)}>System Hardware</h2>
            <p className={cn('text-sm mt-0.5', text.secondary)}>
              Detected hardware capabilities
            </p>
          </div>
        </div>
      </CardHeader>

      <CardContent className="relative">
        <div className="grid grid-cols-1 md:grid-cols-3 gap-6 py-2">
          {/* CPU Info */}
          <div className="space-y-3">
            <div className="flex items-center gap-2">
              <Cpu className={cn('w-4 h-4', text.secondary)} />
              <span className={cn('text-sm font-medium', text.secondary)}>CPU</span>
            </div>
            <div className="space-y-1">
              <p className={cn('text-sm font-medium', text.primary)}>
                {cpuInfo?.model || 'Unknown'}
              </p>
              <p className={cn('text-xs', text.tertiary)}>
                {cpuInfo?.physicalCpus || 0} cores / {cpuInfo?.logicalCpus || 0} threads
              </p>
            </div>
          </div>

          {/* Memory Info */}
          <div className="space-y-3">
            <div className="flex items-center gap-2">
              <MemoryStick className={cn('w-4 h-4', text.secondary)} />
              <span className={cn('text-sm font-medium', text.secondary)}>Memory</span>
            </div>
            <div className="space-y-1">
              <p className={cn('text-sm font-medium', text.primary)}>
                {memoryInfo?.totalFormatted || 'Unknown'}
              </p>
              <p className={cn('text-xs', text.tertiary)}>System RAM</p>
            </div>
          </div>

          {/* GPU Info */}
          <div className="space-y-3">
            <div className="flex items-center gap-2">
              <HardDrive className={cn('w-4 h-4', text.secondary)} />
              <span className={cn('text-sm font-medium', text.secondary)}>GPU</span>
            </div>
            <div className="space-y-1">
              {gpuInfo?.available ? (
                <>
                  <p className={cn('text-sm font-medium', text.primary)}>
                    {gpuInfo.deviceNames?.[0] || gpuInfo.type || 'GPU Detected'}
                  </p>
                  <div className="flex items-center gap-2">
                    <span
                      className={cn(
                        'inline-flex items-center px-1.5 py-0.5 rounded text-xs font-medium',
                        'bg-emerald-100 dark:bg-emerald-900/50 text-emerald-700 dark:text-emerald-400'
                      )}
                    >
                      {gpuInfo.hwAccelType || gpuInfo.type}
                    </span>
                    {gpuInfo.hasVaapi && (
                      <span
                        className={cn(
                          'inline-flex items-center px-1.5 py-0.5 rounded text-xs',
                          'bg-blue-100 dark:bg-blue-900/50 text-blue-700 dark:text-blue-400'
                        )}
                      >
                        VAAPI
                      </span>
                    )}
                  </div>
                </>
              ) : (
                <>
                  <p className={cn('text-sm font-medium', text.primary)}>No GPU Detected</p>
                  <p className={cn('text-xs', text.tertiary)}>Software encoding only</p>
                </>
              )}
            </div>
          </div>
        </div>
      </CardContent>
    </Card>
  )
}

// Read-only setting display (for detected/locked values)
const ReadOnlySetting = ({
  setting,
  value,
}: {
  setting: EffectiveSetting
  value: unknown
}) => {
  const { label, description, envVar, source, locked, readOnly } = setting

  return (
    <div className="flex items-start justify-between gap-4 py-3">
      <div className="flex-1 min-w-0">
        <div className="flex items-center gap-2 flex-wrap">
          <span className={cn('text-sm font-medium', text.primary)}>{label || setting.key}</span>
          <SourceBadge source={source} envVar={envVar} locked={locked} readOnly={readOnly} />
        </div>
        {description && <p className={cn('text-xs mt-1', text.tertiary)}>{description}</p>}
      </div>
      <div
        className={cn(
          'px-3 py-1.5 rounded-md text-sm font-mono shrink-0',
          'bg-neutral-100 dark:bg-neutral-800',
          text.secondary
        )}
      >
        {typeof value === 'boolean' ? (value ? 'Enabled' : 'Disabled') : String(value ?? '—')}
      </div>
    </div>
  )
}

// Editable setting input
const EditableSetting = ({
  setting,
  value,
  originalValue,
  onChange,
  disabled,
  options,
  restartable,
}: {
  setting: EffectiveSetting
  value: unknown
  originalValue: unknown
  onChange: (value: unknown) => void
  disabled?: boolean
  options?: { value: string; label: string }[]
  restartable?: boolean
}) => {
  const { type, label, description, envVar, source, locked, readOnly } = setting
  const isChanged = value !== originalValue

  // Boolean toggle
  if (type === 'bool') {
    return (
      <div
        className={cn(
          'py-3 transition-all duration-150',
          isChanged && 'bg-amber-50/50 dark:bg-amber-950/20 -mx-4 px-4 rounded-lg border-l-2 border-amber-500'
        )}
      >
        <div className="flex items-start gap-4">
          <div className="flex-1">
            <div className="flex items-center gap-2 flex-wrap mb-1">
              <span className={cn('text-sm font-medium', text.primary)}>
                {label || setting.key}
              </span>
              <SourceBadge source={source} envVar={envVar} locked={locked} readOnly={readOnly} />
              {restartable && <RestartBadge />}
            </div>
            {description && <p className={cn('text-xs', text.tertiary)}>{description}</p>}
          </div>
          <SettingToggle
            enabled={Boolean(value)}
            onChange={onChange}
            label=""
            description=""
            ariaLabel={label || setting.key || ''}
            disabled={disabled}
          />
        </div>
      </div>
    )
  }

  // Select dropdown
  if (options && options.length > 0) {
    return (
      <div
        className={cn(
          'py-3 transition-all duration-150',
          isChanged && 'bg-amber-50/50 dark:bg-amber-950/20 -mx-4 px-4 rounded-lg border-l-2 border-amber-500'
        )}
      >
        <div className="flex items-center gap-2 flex-wrap mb-2">
          <span className={cn('text-sm font-medium', text.primary)}>{label || setting.key}</span>
          <SourceBadge source={source} envVar={envVar} locked={locked} readOnly={readOnly} />
          {restartable && <RestartBadge />}
        </div>
        <Select
          value={String(value ?? '')}
          onChange={(e) => onChange(e.target.value)}
          options={options}
          disabled={disabled}
        />
        {description && <p className={cn('text-xs mt-1.5', text.tertiary)}>{description}</p>}
      </div>
    )
  }

  // Number input
  if (type === 'int') {
    return (
      <div
        className={cn(
          'py-3 transition-all duration-150',
          isChanged && 'bg-amber-50/50 dark:bg-amber-950/20 -mx-4 px-4 rounded-lg border-l-2 border-amber-500'
        )}
      >
        <div className="flex items-center gap-2 flex-wrap mb-2">
          <span className={cn('text-sm font-medium', text.primary)}>{label || setting.key}</span>
          <SourceBadge source={source} envVar={envVar} locked={locked} readOnly={readOnly} />
          {restartable && <RestartBadge />}
        </div>
        <Input
          type="number"
          value={String(value ?? '')}
          onChange={(e) => onChange(parseInt(e.target.value, 10))}
          disabled={disabled}
          className="max-w-xs"
        />
        {description && <p className={cn('text-xs mt-1.5', text.tertiary)}>{description}</p>}
      </div>
    )
  }

  // Default: string input
  return (
    <div
      className={cn(
        'py-3 transition-all duration-150',
        isChanged && 'bg-amber-50/50 dark:bg-amber-950/20 -mx-4 px-4 rounded-lg border-l-2 border-amber-500'
      )}
    >
      <div className="flex items-center gap-2 flex-wrap mb-2">
        <span className={cn('text-sm font-medium', text.primary)}>{label || setting.key}</span>
        <SourceBadge source={source} envVar={envVar} locked={locked} readOnly={readOnly} />
        {restartable && <RestartBadge />}
      </div>
      <Input
        value={String(value ?? '')}
        onChange={(e) => onChange(e.target.value)}
        disabled={disabled}
      />
      {description && <p className={cn('text-xs mt-1.5', text.tertiary)}>{description}</p>}
    </div>
  )
}

// Category card with grouped read-only and editable settings
const CategoryCard = ({
  category,
  settings,
  editedValues,
  originalValues,
  schemaOptions,
  schemaRestartable,
  onValueChange,
  onSave,
  onDiscard,
  isSaving,
}: {
  category: string
  settings: EffectiveSetting[]
  editedValues: Record<string, unknown>
  originalValues: Record<string, unknown>
  schemaOptions: Record<string, { value: string; label: string }[]>
  schemaRestartable: Record<string, boolean>
  onValueChange: (key: string, value: unknown) => void
  onSave: () => Promise<void>
  onDiscard: () => void
  isSaving: boolean
}) => {
  const config = categoryConfig[category] || {
    icon: Settings2,
    label: category.charAt(0).toUpperCase() + category.slice(1),
    description: '',
  }
  const CategoryIcon = config.icon

  // Separate read-only from editable settings
  const readOnlySettings = settings.filter((s) => s.locked || s.readOnly)
  const editableSettings = settings.filter((s) => !s.locked && !s.readOnly)

  // Count changes in this category
  const changedKeys = Object.keys(editedValues).filter((key) =>
    settings.some((s) => s.key === key)
  )
  const changeCount = changedKeys.length
  const hasChanges = changeCount > 0

  // Check if any changed settings require restart
  const hasRestartRequired = changedKeys.some((key) => schemaRestartable[key])

  return (
    <Card className="overflow-hidden">
      <CardHeader className="border-b border-neutral-100 dark:border-neutral-800">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-3">
            <div
              className={cn(
                'p-2 rounded-lg',
                'bg-primary-50 dark:bg-primary-950/50',
                'text-primary-600 dark:text-primary-400'
              )}
            >
              <CategoryIcon className="w-5 h-5" />
            </div>
            <div>
              <h2 className={cn('text-lg font-semibold', text.primary)}>{config.label}</h2>
              {config.description && (
                <p className={cn('text-sm mt-0.5', text.secondary)}>{config.description}</p>
              )}
            </div>
          </div>
          {hasChanges && (
            <div
              className={cn(
                'flex items-center gap-1.5 px-2 py-1 rounded-md text-xs font-medium',
                'bg-amber-100 dark:bg-amber-900/50 text-amber-700 dark:text-amber-400'
              )}
            >
              {changeCount} unsaved
            </div>
          )}
        </div>
      </CardHeader>

      <CardContent className="space-y-0">
        {/* Read-only section */}
        {readOnlySettings.length > 0 && (
          <div className="mb-4">
            <div
              className={cn(
                'px-3 py-2 -mx-4 mb-2',
                'bg-neutral-50 dark:bg-neutral-900/50',
                'border-y border-neutral-100 dark:border-neutral-800'
              )}
            >
              <span className={cn('text-[10px] font-medium uppercase tracking-wider', text.tertiary)}>
                Read-only Values
              </span>
            </div>
            <div className="divide-y divide-neutral-100 dark:divide-neutral-800">
              {readOnlySettings.map((setting) => (
                <ReadOnlySetting
                  key={setting.key}
                  setting={setting}
                  value={setting.value}
                />
              ))}
            </div>
          </div>
        )}

        {/* Editable section */}
        {editableSettings.length > 0 && (
          <div>
            {readOnlySettings.length > 0 && (
              <div
                className={cn(
                  'px-3 py-2 -mx-4 mb-2',
                  'bg-neutral-50 dark:bg-neutral-900/50',
                  'border-y border-neutral-100 dark:border-neutral-800'
                )}
              >
                <span className={cn('text-[10px] font-medium uppercase tracking-wider', text.tertiary)}>
                  Configurable
                </span>
              </div>
            )}
            <div className="divide-y divide-neutral-100 dark:divide-neutral-800">
              {editableSettings.map((setting) => {
                const key = setting.key || ''
                const editedValue = editedValues[key]
                const displayValue = editedValue !== undefined ? editedValue : setting.value

                return (
                  <EditableSetting
                    key={key}
                    setting={setting}
                    value={displayValue}
                    originalValue={originalValues[key] ?? setting.value}
                    onChange={(value) => onValueChange(key, value)}
                    disabled={isSaving}
                    options={schemaOptions[key]}
                    restartable={schemaRestartable[key]}
                  />
                )
              })}
            </div>
          </div>
        )}

        {settings.length === 0 && (
          <p className={cn('py-4 text-sm', text.secondary)}>
            No settings available in this category.
          </p>
        )}
      </CardContent>

      {/* Category footer with save/discard - only shows when there are changes */}
      {hasChanges && (
        <CardFooter className="flex items-center justify-between bg-neutral-50 dark:bg-neutral-900/50">
          {hasRestartRequired && (
            <div className="flex items-center gap-2 text-xs text-orange-600 dark:text-orange-400">
              <RotateCcw className="w-3.5 h-3.5" />
              Some changes require server restart
            </div>
          )}
          {!hasRestartRequired && <div />}
          <div className="flex items-center gap-2">
            <Button variant="ghost" size="sm" onClick={onDiscard} disabled={isSaving}>
              Discard
            </Button>
            <Button size="sm" onClick={onSave} isLoading={isSaving}>
              <Check className="w-3.5 h-3.5 mr-1" />
              Save {changeCount > 1 ? `(${changeCount})` : ''}
            </Button>
          </div>
        </CardFooter>
      )}
    </Card>
  )
}

const SystemSettings = () => {
  const { user } = useAuth()
  const toast = useToast()

  // Fetch effective settings (includes source tracking)
  const {
    data: effectiveData,
    isLoading: effectiveLoading,
    error: effectiveError,
    refetch: refetchSettings,
  } = useGetApiSettingsSystemEffective()

  // Fetch schema for options and restartable flags
  const { data: schemaData } = useGetApiSettingsSchema()

  // Per-category edited values
  const [editedValues, setEditedValues] = useState<Record<string, unknown>>({})
  const [savingCategory, setSavingCategory] = useState<string | null>(null)

  // Extract settings from response
  const settings: EffectiveSetting[] =
    effectiveData?.status === 200 && 'settings' in effectiveData.data
      ? effectiveData.data.settings || []
      : []

  // Build schema maps
  const schemaOptions: Record<string, { value: string; label: string }[]> = {}
  const schemaRestartable: Record<string, boolean> = {}

  if (schemaData?.status === 200 && 'system' in schemaData.data) {
    for (const def of schemaData.data.system || []) {
      const key = def.key || ''
      if (def.options?.length) {
        schemaOptions[key] = def.options.map((o) => ({
          value: o.value || '',
          label: o.label || o.value || '',
        }))
      }
      if (def.restartable) {
        schemaRestartable[key] = true
      }
    }
  }

  // Build original values map for change detection
  const originalValues: Record<string, unknown> = {}
  for (const setting of settings) {
    if (setting.key) {
      originalValues[setting.key] = setting.value
    }
  }

  // Check if user is admin
  if (!user?.is_admin) {
    return (
      <div className="h-full overflow-auto">
        <div className="p-8 page-enter">
          <Alert variant="error">You must be an administrator to access system settings.</Alert>
        </div>
      </div>
    )
  }

  // Group settings by category, excluding 'system' (shown in hardware card)
  const groupedSettings = settings
    .filter((s) => s.category !== 'system')
    .reduce<Record<string, EffectiveSetting[]>>((acc, setting) => {
      const category = setting.category || 'other'
      if (!acc[category]) acc[category] = []
      acc[category].push(setting)
      return acc
    }, {})

  const handleValueChange = (key: string, value: unknown) => {
    // If value is same as original, remove from edited
    if (value === originalValues[key]) {
      setEditedValues((prev) => {
        const next = { ...prev }
        delete next[key]
        return next
      })
    } else {
      setEditedValues((prev) => ({ ...prev, [key]: value }))
    }
  }

  const handleSaveCategory = async (category: string) => {
    const categoryKeys = (groupedSettings[category] || []).map((s) => s.key || '')
    const changes = Object.entries(editedValues).filter(([key]) => categoryKeys.includes(key))

    if (changes.length === 0) return

    setSavingCategory(category)
    try {
      // Save all changes in this category
      for (const [key, value] of changes) {
        const response = await putApiSettingsSystemKey(key, { value })
        if (response.status !== 200) {
          throw new Error(`Failed to save ${key}`)
        }
      }

      toast.success(
        changes.length === 1
          ? 'Setting saved successfully'
          : `${changes.length} settings saved successfully`
      )

      // Clear edited values for this category
      setEditedValues((prev) => {
        const next = { ...prev }
        for (const [key] of changes) {
          delete next[key]
        }
        return next
      })

      // Refetch settings
      refetchSettings()
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to save settings')
    } finally {
      setSavingCategory(null)
    }
  }

  const handleDiscardCategory = (category: string) => {
    const categoryKeys = (groupedSettings[category] || []).map((s) => s.key || '')
    setEditedValues((prev) => {
      const next = { ...prev }
      for (const key of categoryKeys) {
        delete next[key]
      }
      return next
    })
    toast.info('Changes discarded')
  }

  if (effectiveLoading) {
    return (
      <div className="h-full overflow-auto">
        <div className="p-8 page-enter">
          <Loading text="Loading settings..." />
        </div>
      </div>
    )
  }

  if (effectiveError) {
    return (
      <div className="h-full overflow-auto">
        <div className="p-8 page-enter">
          <Alert variant="error">Failed to load settings. Please try again later.</Alert>
        </div>
      </div>
    )
  }

  return (
    <div className="h-full overflow-auto">
      <div className="p-8 page-enter">
        <PageHeader
          title="System Settings"
          description="Configure server-wide settings that affect all users"
        />

        {/* System Hardware Info Card */}
        <div className="mt-6">
          <SystemInfoCard />
        </div>

        {/* Env var info banner */}
        <div
          className={cn(
            'mt-6 p-4 rounded-xl flex items-start gap-3',
            'bg-blue-50 dark:bg-blue-950/30',
            'border border-blue-200 dark:border-blue-800/50'
          )}
        >
          <Info className="w-5 h-5 text-blue-600 dark:text-blue-400 mt-0.5 shrink-0" />
          <div className="space-y-1">
            <p className={cn('text-sm font-medium', 'text-blue-800 dark:text-blue-200')}>
              Environment Variable Overrides
            </p>
            <p className={cn('text-xs', 'text-blue-700 dark:text-blue-300')}>
              Settings marked with <Lock className="w-3 h-3 inline mx-0.5" /> are controlled by
              environment variables and cannot be changed in the UI. To modify them, update the
              environment variable and restart the server.
            </p>
          </div>
        </div>

        {/* Settings grouped by category */}
        <div className="mt-6 space-y-6">
          {Object.entries(groupedSettings).map(([category, categorySettings]) => (
            <CategoryCard
              key={category}
              category={category}
              settings={categorySettings}
              editedValues={editedValues}
              originalValues={originalValues}
              schemaOptions={schemaOptions}
              schemaRestartable={schemaRestartable}
              onValueChange={handleValueChange}
              onSave={() => handleSaveCategory(category)}
              onDiscard={() => handleDiscardCategory(category)}
              isSaving={savingCategory === category}
            />
          ))}

          {Object.keys(groupedSettings).length === 0 && (
            <Card>
              <CardContent className="py-12 text-center">
                <Settings2 className={cn('w-12 h-12 mx-auto mb-4', text.tertiary)} />
                <p className={cn('text-lg font-medium', text.primary)}>
                  No system settings available
                </p>
                <p className={cn('text-sm mt-1', text.secondary)}>
                  System settings will appear here once configured.
                </p>
              </CardContent>
            </Card>
          )}
        </div>

        {/* Reload settings button */}
        <div className="mt-8 flex justify-end">
          <Button variant="secondary" onClick={() => refetchSettings()} className="gap-2">
            <RefreshCw className="w-4 h-4" />
            Reload Settings
          </Button>
        </div>
      </div>
    </div>
  )
}

export const Route = createFileRoute('/_layout/settings/system')({
  component: SystemSettings,
})
