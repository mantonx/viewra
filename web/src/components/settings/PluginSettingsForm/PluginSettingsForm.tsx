import { useState, useCallback, useMemo, useEffect, useRef } from 'react'
import { Alert, Loading, Tabs, TabPanel } from '@/components/ui'
import { useGetApiPluginsIdSettings } from '@/lib/api/generated/plugins/plugins'
import { useToast } from '@/lib/hooks/useToast'
import { cn } from '@/lib/utils'
import type { RJSFSchema, UiSchema } from '@rjsf/utils'
import type { Tab } from '@/components/ui'
import { JsonSchemaForm } from '../JsonSchemaForm'
import { ActionList, ActionCreate, ActionTest } from '../SchemaActions'
import {
  parseSchemaActions,
  getTabActions,
  getInlineListActions,
  findCreateAction,
  findTestAction,
  isCreateAction,
} from '@/lib/types/schema-actions'
import type { SchemaAction } from '@/lib/types/schema-actions'

export type PluginSettingsFormProps = {
  pluginId: string
  onSettingsChange?: (hasChanges: boolean) => void
  className?: string
  /** Filter to only show specific fields (e.g., ['embedding_model'] or ['chat_model']) */
  fieldFilter?: string[]
  /** Hide the settings tab entirely (useful when only showing filtered fields inline) */
  hideSettingsTab?: boolean
  /** Hide action tabs (Models, etc.) */
  hideActionTabs?: boolean
}

export const PluginSettingsForm = ({
  pluginId,
  onSettingsChange,
  className,
  fieldFilter,
  hideSettingsTab = false,
  hideActionTabs = false,
}: PluginSettingsFormProps) => {
  const toast = useToast()
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

  // Parse schema and actions (strip title to avoid duplication in UI)
  const schema: RJSFSchema | null = useMemo(() => {
    if (settingsResponse?.status !== 200) {
      return null
    }
    const s = settingsResponse.data.schema
    if (!s || typeof s !== 'object') {
      return null
    }
    // Remove title to avoid duplication when embedded in other components
    const { title: _title, ...schemaWithoutTitle } = s as RJSFSchema & { title?: string }

    // Apply field filter if provided
    if (fieldFilter && fieldFilter.length > 0 && schemaWithoutTitle.properties) {
      const filteredProperties: Record<string, unknown> = {}
      for (const field of fieldFilter) {
        if (schemaWithoutTitle.properties[field]) {
          filteredProperties[field] = schemaWithoutTitle.properties[field]
        }
      }
      return {
        ...schemaWithoutTitle,
        properties: filteredProperties,
      } as RJSFSchema
    }

    return schemaWithoutTitle as RJSFSchema
  }, [settingsResponse, fieldFilter])

  const actions: SchemaAction[] = useMemo(() => {
    if (!schema) {
      return []
    }
    return parseSchemaActions(schema)
  }, [schema])

  const tabActions = useMemo(() => getTabActions(actions), [actions])
  const inlineListActions = useMemo(() => getInlineListActions(actions), [actions])
  const testAction = useMemo(() => findTestAction(actions), [actions])

  // Check if schema has any properties to show
  const hasProperties = useMemo(
    () => schema?.properties && Object.keys(schema.properties).length > 0,
    [schema]
  )

  // Determine which tabs to show
  const showSettingsTab = !hideSettingsTab && hasProperties
  const showActionTabs = !hideActionTabs && tabActions.length > 0

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

  const handleSubmit = useCallback(
    async (data: Record<string, unknown>) => {
      try {
        const response = await fetch(`/api/plugins/${pluginId}/settings`, {
          method: 'PUT',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ settings: data }),
          credentials: 'include',
        })

        const result = await response.json()
        if (!response.ok || !result.success) {
          toast.error(result.error || 'Failed to save settings')
          return
        }

        toast.success('Settings saved')
        setInitialData(data)
      } catch {
        toast.error('Failed to save settings')
      }
    },
    [pluginId, toast]
  )

  const handleShowCreate = useCallback(
    (actionId: string) => {
      const createAction = findCreateAction(actions, actionId)
      if (createAction) {
        // Switch to the tab that contains this action
        setActiveTab(actionId)
      }
    },
    [actions]
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

  if (!schema) {
    return <Alert variant="info">This plugin does not have configurable settings.</Alert>
  }

  // If field filter is set, show a simplified inline form (no tabs, no actions)
  if (fieldFilter && fieldFilter.length > 0) {
    if (!hasProperties) {
      return null // No matching fields to show
    }
    return (
      <div className={cn('space-y-4', className)}>
        <JsonSchemaForm
          schema={schema}
          uiSchema={uiSchema}
          formData={formData}
          onChange={handleChange}
          onSubmit={handleSubmit}
        />
      </div>
    )
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
              <JsonSchemaForm
                schema={schema}
                uiSchema={uiSchema}
                formData={formData}
                onChange={handleChange}
                onSubmit={handleSubmit}
              />

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
              ? actions.find((a) => a.id === createActionId && isCreateAction(a))
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
          <JsonSchemaForm
            schema={schema}
            uiSchema={uiSchema}
            formData={formData}
            onChange={handleChange}
            onSubmit={handleSubmit}
          />
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
}
