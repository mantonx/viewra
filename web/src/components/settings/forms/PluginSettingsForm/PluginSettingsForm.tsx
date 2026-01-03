import { useState, useCallback, useMemo, useEffect, useRef, forwardRef, useImperativeHandle } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { Alert, Loading, Tabs, TabPanel } from '@/components/ui'
import {
  useGetApiPluginsIdSettings,
  usePutApiPluginsIdSettings,
  getGetApiPluginsIdSettingsQueryKey,
} from '@/lib/api/generated/plugins/plugins'
import { useToast } from '@/lib/hooks/useToast'
import { cn } from '@/lib/utils'
import type { RJSFSchema, UiSchema } from '@rjsf/utils'
import type { Tab } from '@/components/ui'
import { JsonSchemaForm } from '../JsonSchemaForm'
import { ActionList, ActionCreate, ActionTest } from '../SchemaActions'
import {
  parseSchemaActions,
  parseSchemaSections,
  getSectionsForCapability,
  getTabActions,
  getVisibleTabActions,
  getInlineListActions,
  findCreateAction,
  findTestAction,
  isCreateAction,
  parseDependsOn,
  shouldShowField,
  parsePropertyOrder,
} from '@/lib/types/schema-actions'
import type { SchemaAction, SchemaSection } from '@/lib/types/schema-actions'
import type { PluginSettingsFormProps, PluginSettingsFormHandle } from './PluginSettingsForm.types'

export const PluginSettingsForm = forwardRef<PluginSettingsFormHandle, PluginSettingsFormProps>(({
  pluginId,
  onSettingsChange,
  className,
  capability,
  hideSubmit = false,
  onSaveComplete,
}, ref) => {
  const toast = useToast()
  const queryClient = useQueryClient()
  const [formData, setFormData] = useState<Record<string, unknown>>({})
  const [initialData, setInitialData] = useState<Record<string, unknown>>({})
  const [activeTab, setActiveTab] = useState('settings')
  const listRefreshRef = useRef<(() => void) | null>(null)

  // Fetch plugin settings (includes schema)
  const {
    data: settingsResponse,
    isLoading,
    error,
  } = useGetApiPluginsIdSettings(pluginId, {
    query: { enabled: !!pluginId },
  })

  // Parse raw schema (strip title to avoid duplication in UI)
  const rawSchema: RJSFSchema | null = useMemo(() => {
    if (settingsResponse?.status !== 200) {
      return null
    }
    const s = settingsResponse.data.schema
    if (!s || typeof s !== 'object') {
      return null
    }
    // Remove title to avoid duplication when embedded in other components
    const { title: _title, ...schemaWithoutTitle } = s as RJSFSchema & { title?: string }
    return schemaWithoutTitle as RJSFSchema
  }, [settingsResponse])

  // Parse sections from schema
  const allSections: SchemaSection[] = useMemo(() => {
    if (!rawSchema) {
      return []
    }
    return parseSchemaSections(rawSchema)
  }, [rawSchema])

  // Filter sections by capability if provided
  const activeSections: SchemaSection[] | null = useMemo(() => {
    if (allSections.length === 0) {
      return null // No sections defined = show all properties
    }

    if (capability) {
      // Filter to sections that have this capability
      return getSectionsForCapability(allSections, capability)
    }

    // No capability filter = show "main" sections (those without capabilities)
    // This allows plugins to define what shows in the main settings view
    const mainSections = allSections.filter(
      (s) => !s.capabilities || s.capabilities.length === 0
    )
    return mainSections.length > 0 ? mainSections : null
  }, [allSections, capability])

  // Derive allowed properties and action IDs from active sections
  const allowedProperties: Set<string> | null = useMemo(() => {
    // null = allow all
    if (!activeSections) {
      return null
    }
    const props = new Set<string>()
    for (const section of activeSections) {
      if (section.properties) {
        for (const prop of section.properties) {
          props.add(prop)
        }
      }
    }
    return props
  }, [activeSections])

  const allowedActionIds: Set<string> | null = useMemo(() => {
    // null = allow all
    if (!activeSections) {
      return null
    }
    const ids = new Set<string>()
    for (const section of activeSections) {
      if (section.actions) {
        for (const actionId of section.actions) {
          ids.add(actionId)
        }
      }
    }
    return ids
  }, [activeSections])

  // Build filtered schema with only allowed properties
  // Also filters by dependsOn and sorts by x-viewra-order
  const schema: RJSFSchema | null = useMemo(() => {
    if (!rawSchema) {
      return null
    }

    // Start with rawSchema properties or empty
    type SchemaProperties = RJSFSchema['properties']
    let baseProperties: SchemaProperties = rawSchema.properties || {}

    // Filter by allowed properties (capability-based sections)
    if (allowedProperties && allowedProperties.size > 0) {
      const filtered: SchemaProperties = {}
      for (const prop of allowedProperties) {
        if (baseProperties[prop]) {
          filtered[prop] = baseProperties[prop]
        }
      }
      baseProperties = filtered
    } else if (activeSections && allowedProperties?.size === 0) {
      // Sections exist but no properties in them
      baseProperties = {}
    }

    // Filter by dependsOn (conditional visibility based on formData)
    const visibleProperties: SchemaProperties = {}
    for (const [propName, propSchema] of Object.entries(baseProperties)) {
      if (typeof propSchema !== 'object' || propSchema === null) {
        visibleProperties[propName] = propSchema
        continue
      }
      const dependsOn = parseDependsOn(propSchema)
      if (shouldShowField(dependsOn, formData)) {
        visibleProperties[propName] = propSchema
      }
    }

    // Sort properties by x-viewra-order
    const propertyOrder = parsePropertyOrder(rawSchema)
    if (propertyOrder.length > 0) {
      const orderMap = new Map(propertyOrder.map((name, index) => [name, index]))
      const sortedEntries = Object.entries(visibleProperties).sort(([a], [b]) => {
        const aOrder = orderMap.has(a) ? orderMap.get(a)! : Infinity
        const bOrder = orderMap.has(b) ? orderMap.get(b)! : Infinity
        return aOrder - bOrder
      })
      const sortedProperties: SchemaProperties = {}
      for (const [name, value] of sortedEntries) {
        sortedProperties[name] = value
      }
      return { ...rawSchema, properties: sortedProperties } as RJSFSchema
    }

    return { ...rawSchema, properties: visibleProperties } as RJSFSchema
  }, [rawSchema, allowedProperties, activeSections, formData])

  // Parse all actions from raw schema
  const allActions: SchemaAction[] = useMemo(() => {
    if (!rawSchema) {
      return []
    }
    return parseSchemaActions(rawSchema)
  }, [rawSchema])

  // Filter actions by allowed IDs
  const actions: SchemaAction[] = useMemo(() => {
    if (!allowedActionIds) {
      return allActions
    }
    return allActions.filter((a) => allowedActionIds.has(a.id))
  }, [allActions, allowedActionIds])

  // Compute tab actions, inline list actions, and test action from filtered actions
  // Tab actions are filtered by their dependsOn config (e.g., show Ollama tabs only when Ollama is selected)
  const tabActions = useMemo(() => getVisibleTabActions(actions, formData), [actions, formData])
  const inlineListActions = useMemo(() => getInlineListActions(actions), [actions])
  const testAction = useMemo(() => findTestAction(actions), [actions])

  // Check if schema has any properties to show
  const hasProperties = useMemo(
    () => schema?.properties && Object.keys(schema.properties).length > 0,
    [schema]
  )

  // Determine which tabs to show
  const showSettingsTab = hasProperties
  // Tab actions are already filtered by their dependsOn config via getVisibleTabActions
  const showActionTabs = tabActions.length > 0

  // Build visible tabs
  const visibleTabs: Tab[] = useMemo(() => {
    const result: Tab[] = []
    if (showSettingsTab) {
      result.push({ id: 'settings', label: 'Settings' })
    }
    if (showActionTabs) {
      for (const action of tabActions) {
        result.push({ id: action.id, label: action.tabTitle || action.title })
      }
    }
    return result
  }, [showSettingsTab, showActionTabs, tabActions])

  // Build uiSchema for model selection dropdowns
  const uiSchema: UiSchema = useMemo(() => {
    const ui: UiSchema = {}
    // Can be enhanced later to auto-detect select fields
    return ui
  }, [])

  // Track changes
  useEffect(() => {
    if (onSettingsChange) {
      const hasChanges = JSON.stringify(formData) !== JSON.stringify(initialData)
      onSettingsChange(hasChanges)
    }
  }, [formData, initialData, onSettingsChange])

  // Initialize form data from saved settings
  useEffect(() => {
    if (settingsResponse?.status === 200 && settingsResponse.data.values) {
      const rawValues = settingsResponse.data.values
      const values =
        typeof rawValues === 'object' && rawValues !== null && !Array.isArray(rawValues)
          ? (rawValues as Record<string, unknown>)
          : {}
      setFormData(values)
      setInitialData(values)
    }
  }, [settingsResponse])

  const handleChange = useCallback((data: Record<string, unknown>) => {
    setFormData(data)
  }, [])

  // Auto-save debounce ref
  const autoSaveTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  // Use generated mutation hook for updating settings
  const updateSettingsMutation = usePutApiPluginsIdSettings()

  // Core save function - returns true on success, false on failure
  const saveSettings = useCallback(
    async (data: Record<string, unknown>, showToast = true): Promise<boolean> => {
      try {
        // Note: The generated types show values as number[] due to a swagger gen issue,
        // but the actual API expects an object. We cast here for type safety.
        const result = await updateSettingsMutation.mutateAsync({
          id: pluginId,
          data: { values: data as unknown as number[] },
        })

        if (result.status !== 200 || !result.data.success) {
          if (showToast) {toast.error('Failed to save settings')}
          return false
        }

        // Invalidate the settings query so reopening the modal shows fresh data
        await queryClient.invalidateQueries({
          queryKey: getGetApiPluginsIdSettingsQueryKey(pluginId),
        })

        if (showToast) {toast.success('Settings saved')}
        setInitialData(data)
        onSaveComplete?.()
        return true
      } catch (err) {
        if (showToast) {
          const message = err instanceof Error ? err.message : 'Failed to save settings'
          toast.error(message)
        }
        return false
      }
    },
    [pluginId, updateSettingsMutation, toast, queryClient, onSaveComplete]
  )

  const handleSubmit = useCallback(
    async (data: Record<string, unknown>) => {
      await saveSettings(data, true)
    },
    [saveSettings]
  )

  // Expose save function via ref for parent forms to trigger
  useImperativeHandle(ref, () => ({
    save: () => saveSettings(formData, false),
  }), [saveSettings, formData])

  // Auto-save for embedded forms (hideSubmit=true)
  // Debounces saves to avoid excessive API calls while typing
  useEffect(() => {
    if (!hideSubmit) {return} // Only auto-save for embedded forms
    if (JSON.stringify(formData) === JSON.stringify(initialData)) {return} // No changes

    // Clear existing timer
    if (autoSaveTimerRef.current) {
      clearTimeout(autoSaveTimerRef.current)
    }

    // Debounce save by 500ms
    autoSaveTimerRef.current = setTimeout(() => {
      saveSettings(formData, false)
    }, 500)

    return () => {
      if (autoSaveTimerRef.current) {
        clearTimeout(autoSaveTimerRef.current)
      }
    }
  }, [hideSubmit, formData, initialData, saveSettings])

  const handleShowCreate = useCallback(
    (actionId: string) => {
      const createAction = findCreateAction(allActions, actionId)
      if (createAction) {
        // Switch to the tab that contains this action
        setActiveTab(actionId)
      }
    },
    [allActions]
  )

  const handleListRefresh = useCallback(() => {
    listRefreshRef.current?.()
  }, [])

  if (isLoading) {
    return <Loading text="Loading plugin settings..." />
  }

  if (error) {
    return <Alert variant="warning">Failed to load plugin settings.</Alert>
  }

  if (!rawSchema) {
    return <Alert variant="info">This plugin does not have configurable settings.</Alert>
  }

  // If capability filtering results in nothing to show, return null
  if (capability && activeSections && activeSections.length === 0) {
    return null
  }

  // If there are tabs to show, use tabbed interface
  if (visibleTabs.length > 1) {
    // Ensure active tab is valid
    const validActiveTab = visibleTabs.some((t) => t.id === activeTab)
      ? activeTab
      : visibleTabs[0]?.id ?? 'settings'

    return (
      <div className={cn('space-y-4', className)}>
        <Tabs
          tabs={visibleTabs}
          activeTab={validActiveTab}
          onTabChange={setActiveTab}
          variant="underline"
        />

        {/* Settings tab */}
        {showSettingsTab && (
          <TabPanel isActive={validActiveTab === 'settings'}>
            <div className="space-y-4 pt-4">
              {/* Test connection button if available */}
              {testAction && <ActionTest action={testAction} pluginId={pluginId} />}

              {/* Main settings form */}
              {schema && (
                <JsonSchemaForm
                  schema={schema}
                  uiSchema={uiSchema}
                  formData={formData}
                  onChange={handleChange}
                  onSubmit={handleSubmit}
                  hideSubmit={hideSubmit}
                  asDiv={hideSubmit}
                />
              )}

              {/* Inline list actions (no tabTitle) */}
              {inlineListActions.map((listAction) => (
                <ActionList
                  key={listAction.id}
                  action={listAction}
                  pluginId={pluginId}
                  onShowCreate={handleShowCreate}
                />
              ))}
            </div>
          </TabPanel>
        )}

        {/* Action tabs */}
        {showActionTabs &&
          tabActions.map((listAction) => {
            // Find associated create action
            const createActionId = listAction.emptyState?.showCreate
            const createAction = createActionId
              ? allActions.find((a) => a.id === createActionId && isCreateAction(a))
              : undefined

            return (
              <TabPanel key={listAction.id} isActive={validActiveTab === listAction.id}>
                <div className="space-y-4 pt-4">
                  {/* Create form if available */}
                  {createAction && isCreateAction(createAction) && (
                    <ActionCreate
                      action={createAction}
                      pluginId={pluginId}
                      onSuccess={handleListRefresh}
                    />
                  )}

                  {/* List */}
                  <ActionList
                    action={listAction}
                    pluginId={pluginId}
                    onShowCreate={handleShowCreate}
                  />
                </div>
              </TabPanel>
            )
          })}
      </div>
    )
  }

  // Simple form without tabs (single tab or no tabs needed)
  return (
    <div className={cn('space-y-4', className)}>
      {showSettingsTab && (
        <>
          {testAction && <ActionTest action={testAction} pluginId={pluginId} />}
          {schema && (
            <JsonSchemaForm
              schema={schema}
              uiSchema={uiSchema}
              formData={formData}
              onChange={handleChange}
              onSubmit={handleSubmit}
              hideSubmit={hideSubmit}
              asDiv={hideSubmit}
            />
          )}
          {/* Inline list actions */}
          {inlineListActions.map((listAction) => (
            <ActionList
              key={listAction.id}
              action={listAction}
              pluginId={pluginId}
              onShowCreate={handleShowCreate}
            />
          ))}
        </>
      )}
      {showActionTabs &&
        tabActions.map((listAction) => (
          <ActionList
            key={listAction.id}
            action={listAction}
            pluginId={pluginId}
            onShowCreate={handleShowCreate}
          />
        ))}
    </div>
  )
})

PluginSettingsForm.displayName = 'PluginSettingsForm'
