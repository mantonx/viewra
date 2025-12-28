import { useMemo } from 'react'
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
import type { PreferencesCategory } from '../PreferencesSettings.types'

/**
 * Hook for fetching and managing user preferences data.
 * Handles settings, schema, and value transformations.
 */
export const usePreferencesData = () => {
  const queryClient = useQueryClient()

  const {
    data: settingsData,
    isLoading: settingsLoading,
    error: settingsError,
  } = useGetApiSettingsUser()

  const { data: schemaData, isLoading: schemaLoading } = useGetApiSettingsSchema()

  const saveMutation = usePutApiSettingsUserKey()

  // Get user setting definitions from schema
  const userDefinitions = useMemo(() => {
    if (schemaData?.status !== 200) {
      return []
    }
    return schemaData.data.user || []
  }, [schemaData])

  // Build default values map from schema
  const defaultValuesFromSchema = useMemo(() => {
    const values: Record<string, unknown> = {}
    userDefinitions.forEach((def: SettingDefinition) => {
      if (def.key) {
        if (def.type === 'bool') {
          values[def.key] = def.default === true || def.default === 'true'
        } else {
          values[def.key] = def.default ?? ''
        }
      }
    })
    return values
  }, [userDefinitions])

  // Build current values map from API response
  const savedValues = useMemo(() => {
    const values: Record<string, unknown> = {}
    const settings = settingsData?.status === 200 ? settingsData.data.settings : []
    settings?.forEach((s: UserSetting) => {
      if (s.key && s.value !== undefined) {
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

  // Merge defaults with saved values for initial form state
  const initialValues = useMemo(() => {
    return { ...defaultValuesFromSchema, ...savedValues }
  }, [defaultValuesFromSchema, savedValues])

  // Group definitions by category
  const definitionsByCategory = useMemo(() => {
    const grouped: Record<PreferencesCategory, SettingDefinition[]> = {
      playback: [],
      ui: [],
    }
    userDefinitions.forEach((def: SettingDefinition) => {
      const category = def.category as PreferencesCategory
      if (grouped[category]) {
        grouped[category].push(def)
      }
    })
    return grouped
  }, [userDefinitions])

  // Helper to check if a field is at its default value
  const isFieldDefault = (key: string) => {
    const savedValue = savedValues[key]
    const defaultValue = defaultValuesFromSchema[key]
    return savedValue === undefined || String(savedValue) === String(defaultValue)
  }

  // Save function for a category
  const saveValues = async (values: Record<string, unknown>) => {
    for (const [key, value] of Object.entries(values)) {
      await saveMutation.mutateAsync({
        key,
        data: { value: String(value) },
      })
    }
    await queryClient.invalidateQueries({
      queryKey: getGetApiSettingsUserQueryKey(),
    })
  }

  return {
    isLoading: settingsLoading || schemaLoading,
    error: settingsError,
    initialValues,
    definitionsByCategory,
    defaultValuesFromSchema,
    isFieldDefault,
    saveValues,
  }
}
