import { useState, useCallback, useRef } from 'react'
import { useForm } from '@tanstack/react-form'

/**
 * Configuration for a settings category
 */
export type SettingsCategoryConfig = {
  /** Category identifier */
  id: string
  /** Field names that belong to this category */
  fields: string[]
  /** Called when this category is saved */
  onSave: (values: Record<string, unknown>) => Promise<void>
}

/**
 * Options for useSettingsForm hook
 */
export type UseSettingsFormOptions = {
  /** Initial/default values for all settings */
  defaultValues: Record<string, unknown>
  /** Category configurations */
  categories: SettingsCategoryConfig[]
}

/**
 * Hook for managing settings forms with per-category save/discard functionality.
 * 
 * This hook creates a single TanStack Form instance that manages all settings,
 * but allows saving/discarding changes on a per-category basis.
 * 
 * @example
 * ```tsx
 * const { form, hasChanges, saveCategory, discardCategory, isSaving } = useSettingsForm({
 *   defaultValues: { enabled: false, provider: '' },
 *   categories: [
 *     {
 *       id: 'ai',
 *       fields: ['enabled', 'provider'],
 *       onSave: async (values) => { await saveToApi(values) },
 *     },
 *   ],
 * })
 * ```
 */
export const useSettingsForm = ({
  defaultValues,
  categories,
}: UseSettingsFormOptions) => {
  // Track which category is currently saving
  const [savingCategory, setSavingCategory] = useState<string | null>(null)
  
  // Store original values to track changes (using ref to avoid re-renders)
  const originalValuesRef = useRef<Record<string, unknown>>(defaultValues)

  const form = useForm({
    defaultValues,
  })

  // Get category config by ID
  const getCategoryConfig = useCallback(
    (categoryId: string) => categories.find((c) => c.id === categoryId),
    [categories]
  )

  // Check if a specific field has changed from original
  const hasFieldChanged = useCallback(
    (fieldName: string) => {
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const currentValue = form.getFieldValue(fieldName as any)
      const originalValue = originalValuesRef.current[fieldName]
      return currentValue !== originalValue
    },
    [form]
  )

  // Check if a category has any unsaved changes
  const hasChanges = useCallback(
    (categoryId: string) => {
      const config = getCategoryConfig(categoryId)
      if (!config) {
        return false
      }
      return config.fields.some((field) => hasFieldChanged(field))
    },
    [getCategoryConfig, hasFieldChanged]
  )

  // Get count of changed fields in a category
  const getChangeCount = useCallback(
    (categoryId: string) => {
      const config = getCategoryConfig(categoryId)
      if (!config) {
        return 0
      }
      return config.fields.filter((field) => hasFieldChanged(field)).length
    },
    [getCategoryConfig, hasFieldChanged]
  )

  // Save a specific category
  const saveCategory = useCallback(
    async (categoryId: string) => {
      const config = getCategoryConfig(categoryId)
      if (!config) {
        return
      }

      // Get only the changed values for this category
      const changedValues: Record<string, unknown> = {}
      for (const field of config.fields) {
        if (hasFieldChanged(field)) {
          // eslint-disable-next-line @typescript-eslint/no-explicit-any
          changedValues[field] = form.getFieldValue(field as any)
        }
      }

      if (Object.keys(changedValues).length === 0) {
        return
      }

      setSavingCategory(categoryId)
      try {
        await config.onSave(changedValues)
        
        // Update original values for saved fields
        originalValuesRef.current = {
          ...originalValuesRef.current,
          ...changedValues,
        }
      } finally {
        setSavingCategory(null)
      }
    },
    [getCategoryConfig, hasFieldChanged, form]
  )

  // Discard changes for a specific category
  const discardCategory = useCallback(
    (categoryId: string) => {
      const config = getCategoryConfig(categoryId)
      if (!config) {
        return
      }

      // Reset each field in the category to its original value
      for (const field of config.fields) {
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        form.setFieldValue(field as any, originalValuesRef.current[field] as any)
      }
    },
    [getCategoryConfig, form]
  )

  // Check if a category is currently saving
  const isSaving = useCallback(
    (categoryId: string) => savingCategory === categoryId,
    [savingCategory]
  )

  // Method to update original values (called after fetching from API)
  const resetOriginalValues = useCallback(
    (values: Record<string, unknown>) => {
      originalValuesRef.current = values
      form.reset(values)
    },
    [form]
  )

  return {
    form,
    hasChanges,
    getChangeCount,
    saveCategory,
    discardCategory,
    isSaving,
    savingCategory,
    resetOriginalValues,
  }
}
