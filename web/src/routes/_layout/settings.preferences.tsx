import { createFileRoute } from '@tanstack/react-router'
import { useState, useEffect, useMemo } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import {
  useGetApiSettingsUser,
  useGetApiSettingsSchema,
  usePutApiSettingsUserKey,
  getGetApiSettingsUserQueryKey,
} from '@/lib/api/generated/settings/settings'
import type {
  InternalApiHandlersSettingDefinitionResponse as SettingDefinition,
  InternalApiHandlersUserSettingResponse as UserSetting,
} from '@/lib/api/generated/models'
import {
  Card,
  CardHeader,
  CardContent,
  CardFooter,
  Button,
  Input,
  Loading,
  Alert,
} from '@/components/ui'
import { PageHeader } from '@/components/common'
import { useToast } from '@/lib/hooks/useToast'
import { text } from '@/styles/semantic'
import { cn } from '@/lib/utils'
import { Play, Palette, Check, RotateCcw } from 'lucide-react'

type Category = 'playback' | 'ui'

interface CategoryConfig {
  key: Category
  label: string
  description: string
  icon: React.ReactNode
}

const categories: CategoryConfig[] = [
  {
    key: 'playback',
    label: 'Playback',
    description: 'Control how media plays',
    icon: <Play className="w-5 h-5" />,
  },
  {
    key: 'ui',
    label: 'Appearance',
    description: 'Customize the look and feel',
    icon: <Palette className="w-5 h-5" />,
  },
]

// Source badge component
const SourceBadge = ({ isDefault }: { isDefault: boolean }) => {
  if (isDefault) {
    return (
      <span className="inline-flex items-center gap-1 px-1.5 py-0.5 text-[10px] font-medium rounded bg-neutral-100 text-neutral-500 dark:bg-neutral-800 dark:text-neutral-400">
        Default
      </span>
    )
  }
  return (
    <span className="inline-flex items-center gap-1 px-1.5 py-0.5 text-[10px] font-medium rounded bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400">
      <Check className="w-2.5 h-2.5" />
      Saved
    </span>
  )
}

// Individual setting component
const SettingRow = ({
  definition,
  value,
  isChanged,
  isDefault,
  onChange,
}: {
  definition: SettingDefinition
  value: string | boolean
  isChanged: boolean
  isDefault: boolean
  onChange: (value: string | boolean) => void
}) => {
  const hasOptions = definition.options && definition.options.length > 0

  return (
    <div
      className={cn(
        'py-4 border-l-2 pl-4 -ml-4 transition-colors',
        isChanged
          ? 'border-amber-500 bg-amber-50/50 dark:bg-amber-950/20'
          : 'border-transparent'
      )}
    >
      <div className="flex items-start justify-between gap-4">
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2 flex-wrap">
            <label className={cn('font-medium', text.primary)}>
              {definition.label}
            </label>
            <SourceBadge isDefault={isDefault && !isChanged} />
          </div>
          {definition.description && (
            <p className={cn('text-sm mt-0.5', text.secondary)}>
              {definition.description}
            </p>
          )}
        </div>

        <div className="shrink-0">
          {definition.type === 'bool' ? (
            <button
              type="button"
              role="switch"
              aria-checked={value === true || value === 'true'}
              onClick={() => onChange(!(value === true || value === 'true'))}
              className={cn(
                'relative inline-flex h-6 w-11 items-center rounded-full transition-colors',
                value === true || value === 'true'
                  ? 'bg-blue-600'
                  : 'bg-neutral-300 dark:bg-neutral-600'
              )}
            >
              <span
                className={cn(
                  'inline-block h-4 w-4 transform rounded-full bg-white transition-transform',
                  value === true || value === 'true'
                    ? 'translate-x-6'
                    : 'translate-x-1'
                )}
              />
            </button>
          ) : hasOptions ? (
            <select
              value={String(value)}
              onChange={(e) => onChange(e.target.value)}
              className={cn(
                'w-48 px-3 py-1.5 rounded-lg border text-sm',
                'bg-white dark:bg-neutral-800',
                'border-neutral-300 dark:border-neutral-600',
                'focus:ring-2 focus:ring-blue-500 focus:border-transparent',
                text.primary
              )}
            >
              {definition.options!.map((opt) => (
                <option key={opt.value} value={opt.value}>
                  {opt.label}
                </option>
              ))}
            </select>
          ) : (
            <Input
              value={String(value)}
              onChange={(e) => onChange(e.target.value)}
              className="w-48"
            />
          )}
        </div>
      </div>
    </div>
  )
}

// Category card with settings
const CategoryCard = ({
  config,
  definitions,
  currentValues,
  editedValues,
  defaultValues,
  onValueChange,
  onSave,
  onDiscard,
  isSaving,
}: {
  config: CategoryConfig
  definitions: SettingDefinition[]
  currentValues: Record<string, string | boolean>
  editedValues: Record<string, string | boolean>
  defaultValues: Record<string, unknown>
  onValueChange: (key: string, value: string | boolean) => void
  onSave: () => void
  onDiscard: () => void
  isSaving: boolean
}) => {
  const changedKeys = Object.keys(editedValues).filter((k) =>
    k.startsWith(config.key + '.')
  )
  const hasChanges = changedKeys.length > 0

  return (
    <Card>
      <CardHeader>
        <div className="flex items-center gap-3">
          <div className={cn('p-2 rounded-lg bg-blue-100 text-blue-600 dark:bg-blue-900/30 dark:text-blue-400')}>
            {config.icon}
          </div>
          <div>
            <h2 className={cn('text-lg font-semibold', text.primary)}>
              {config.label}
            </h2>
            <p className={cn('text-sm', text.secondary)}>{config.description}</p>
          </div>
        </div>
      </CardHeader>

      <CardContent className="space-y-1">
        {definitions.map((def) => {
          const key = def.key!
          const editedValue = editedValues[key]
          const currentValue = currentValues[key]
          const defaultValue = defaultValues[key]
          const displayValue = editedValue !== undefined ? editedValue : currentValue
          const isChanged = editedValue !== undefined
          const isDefault = currentValue === undefined || String(currentValue) === String(defaultValue)

          return (
            <SettingRow
              key={key}
              definition={def}
              value={displayValue ?? defaultValue ?? ''}
              isChanged={isChanged}
              isDefault={isDefault}
              onChange={(value) => onValueChange(key, value)}
            />
          )
        })}
      </CardContent>

      {hasChanges && (
        <CardFooter className="flex justify-end gap-2 border-t border-neutral-200 dark:border-neutral-700 pt-4">
          <Button variant="ghost" onClick={onDiscard} disabled={isSaving}>
            <RotateCcw className="w-4 h-4 mr-1.5" />
            Discard
          </Button>
          <Button onClick={onSave} isLoading={isSaving}>
            Save {changedKeys.length > 1 ? `(${changedKeys.length})` : ''}
          </Button>
        </CardFooter>
      )}
    </Card>
  )
}

const PreferencesSettings = () => {
  const toast = useToast()
  const queryClient = useQueryClient()

  // Fetch user settings and schema
  const { data: settingsData, isLoading: settingsLoading, error: settingsError } =
    useGetApiSettingsUser()
  const { data: schemaData, isLoading: schemaLoading } = useGetApiSettingsSchema()

  // Mutation for saving
  const saveMutation = usePutApiSettingsUserKey()

  // Local edited values state
  const [editedValues, setEditedValues] = useState<Record<string, string | boolean>>({})
  const [savingCategory, setSavingCategory] = useState<Category | null>(null)

  // Build current values map from API response
  const currentValues = useMemo(() => {
    const values: Record<string, string | boolean> = {}
    const settings = settingsData?.status === 200 ? settingsData.data.settings : []
    settings?.forEach((s: UserSetting) => {
      if (s.key && s.value !== undefined) {
        // Parse boolean strings
        if (s.value === 'true') {
          values[s.key] = true
        } else if (s.value === 'false') {
          values[s.key] = false
        } else {
          values[s.key] = s.value
        }
      }
    })
    return values
  }, [settingsData])

  // Get user setting definitions from schema
  const userDefinitions = useMemo(() => {
    if (schemaData?.status !== 200) return []
    return schemaData.data.user || []
  }, [schemaData])

  // Build default values map
  const defaultValues = useMemo(() => {
    const values: Record<string, unknown> = {}
    userDefinitions.forEach((def: SettingDefinition) => {
      if (def.key && def.default !== undefined) {
        values[def.key] = def.default
      }
    })
    return values
  }, [userDefinitions])

  // Group definitions by category
  const definitionsByCategory = useMemo(() => {
    const grouped: Record<Category, SettingDefinition[]> = {
      playback: [],
      ui: [],
    }
    userDefinitions.forEach((def: SettingDefinition) => {
      const category = def.category as Category
      if (grouped[category]) {
        grouped[category].push(def)
      }
    })
    return grouped
  }, [userDefinitions])

  // Handle value change
  const handleValueChange = (key: string, value: string | boolean) => {
    const current = currentValues[key]
    const defaultVal = defaultValues[key]

    // Compare with current saved value (or default if not saved)
    const effectiveCurrentValue = current !== undefined ? current : defaultVal

    // If value matches current, remove from edited
    if (String(value) === String(effectiveCurrentValue)) {
      setEditedValues((prev) => {
        const next = { ...prev }
        delete next[key]
        return next
      })
    } else {
      setEditedValues((prev) => ({ ...prev, [key]: value }))
    }
  }

  // Save category
  const handleSaveCategory = async (category: Category) => {
    const keysToSave = Object.keys(editedValues).filter((k) =>
      k.startsWith(category + '.')
    )
    if (keysToSave.length === 0) return

    setSavingCategory(category)

    try {
      for (const key of keysToSave) {
        const value = editedValues[key]
        await saveMutation.mutateAsync({
          key,
          data: { value: String(value) },
        })
      }

      // Clear edited values for this category
      setEditedValues((prev) => {
        const next = { ...prev }
        keysToSave.forEach((k) => delete next[k])
        return next
      })

      // Refetch settings
      await queryClient.invalidateQueries({
        queryKey: getGetApiSettingsUserQueryKey(),
      })

      toast.success('Settings saved')
    } catch (err) {
      toast.error('Failed to save settings')
    } finally {
      setSavingCategory(null)
    }
  }

  // Discard category changes
  const handleDiscardCategory = (category: Category) => {
    setEditedValues((prev) => {
      const next = { ...prev }
      Object.keys(next)
        .filter((k) => k.startsWith(category + '.'))
        .forEach((k) => delete next[k])
      return next
    })
  }

  const isLoading = settingsLoading || schemaLoading

  if (isLoading) {
    return (
      <div className="p-8 page-enter">
        <PageHeader
          title="Preferences"
          description="Customize your viewing experience"
        />
        <div className="mt-8">
          <Loading text="Loading preferences..." />
        </div>
      </div>
    )
  }

  if (settingsError) {
    return (
      <div className="p-8 page-enter">
        <PageHeader
          title="Preferences"
          description="Customize your viewing experience"
        />
        <Alert variant="error" className="mt-6">
          Failed to load settings
        </Alert>
      </div>
    )
  }

  return (
    <div className="p-8 page-enter">
      <PageHeader
        title="Preferences"
        description="Customize your viewing experience"
      />

      <div className="mt-6 space-y-6 max-w-3xl">
        {categories.map((config) => (
          <CategoryCard
            key={config.key}
            config={config}
            definitions={definitionsByCategory[config.key]}
            currentValues={currentValues}
            editedValues={editedValues}
            defaultValues={defaultValues}
            onValueChange={handleValueChange}
            onSave={() => handleSaveCategory(config.key)}
            onDiscard={() => handleDiscardCategory(config.key)}
            isSaving={savingCategory === config.key}
          />
        ))}
      </div>
    </div>
  )
}

export const Route = createFileRoute('/_layout/settings/preferences')({
  component: PreferencesSettings,
})
