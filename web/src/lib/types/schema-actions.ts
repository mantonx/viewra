/**
 * Schema-driven action types for settings UI.
 * Actions are defined through `x-viewra-actions` in JSON Schema.
 * Metadata is defined through `x-viewra-meta`.
 */

/** Metadata from x-viewra-meta */
export type PluginMeta = {
  displayName: string
  description: string
  tip?: string
  isLocal: boolean
  icon: string
}

/** System resource information */
export type SystemInfo = {
  ramBytes: number
  ramFormatted: string
  vramBytes: number
  vramFormatted: string
  hasGpu: boolean
}

/** Condition for showing/hiding item actions */
export type ShowWhen = {
  field: string
  value: unknown
}

/** Badge display configuration */
export type ActionBadge = {
  field: string
  value?: unknown
  label: string
  color: 'blue' | 'green' | 'yellow' | 'red' | 'purple' | 'gray' | 'emerald'
}

/** Display configuration for list items */
export type ActionDisplay = {
  primaryField: string
  secondaryField?: string
  badges?: ActionBadge[]
  metadata?: string[]
}

/** Confirmation dialog configuration */
export type ActionConfirm = {
  title: string
  message: string
}

/** Item-level action (e.g., delete button on each list item) */
export type ItemAction = {
  id: string
  type: 'delete' | 'action' | 'custom'
  label: string
  endpoint: string
  confirm?: ActionConfirm
  showWhen?: ShowWhen
  streaming?: boolean // If true, endpoint returns SSE progress events
}

/** SSE progress event for streaming actions */
export type StreamingProgress = {
  status?: string
  total?: number
  completed?: number
  percent?: number
  done?: boolean
  error?: string
}

/** Empty state configuration for lists */
export type ActionEmptyState = {
  title: string
  description?: string
  showCreate?: string // ID of create action to show
}

/** Success handler configuration */
export type ActionOnSuccess = {
  refresh?: string | string[] // Action IDs to refresh
  message?: string
}

/** Source configuration for data fetching */
export type ActionSource = {
  endpoint: string
  /** Query parameters to pass to the endpoint */
  params?: Record<string, string>
}

/** JSON Schema for form fields */
export type ActionSchema = {
  type: 'object'
  required?: string[]
  properties: Record<
    string,
    {
      type: string
      title?: string
      description?: string
      default?: unknown
      enum?: unknown[]
    }
  >
}

/** List action - displays a list of items from an endpoint */
export type ListAction = {
  id: string
  type: 'list'
  title: string
  tabTitle?: string
  source: ActionSource
  display: ActionDisplay
  itemActions?: ItemAction[]
  emptyState?: ActionEmptyState
  showSystemInfo?: boolean
}

/** Create action - form to create new items (can be streaming) */
export type CreateAction = {
  id: string
  type: 'create'
  title: string
  buttonLabel?: string
  endpoint: string
  streaming?: boolean
  schema?: ActionSchema
  onSuccess?: ActionOnSuccess
}

/** Test action - connectivity/health check */
export type TestAction = {
  id: string
  type: 'test'
  title: string
  endpoint: string
}

/** Delete action - remove an item */
export type DeleteAction = {
  id: string
  type: 'delete'
  title?: string
  endpoint: string
  confirm?: ActionConfirm
}

/** Union of all action types */
export type SchemaAction = ListAction | CreateAction | TestAction | DeleteAction

/** Type guard for list actions */
export const isListAction = (action: SchemaAction): action is ListAction =>
  action.type === 'list'

/** Type guard for create actions */
export const isCreateAction = (action: SchemaAction): action is CreateAction =>
  action.type === 'create'

/** Type guard for test actions */
export const isTestAction = (action: SchemaAction): action is TestAction =>
  action.type === 'test'

/** Type guard for delete actions */
export const isDeleteAction = (action: SchemaAction): action is DeleteAction =>
  action.type === 'delete'

/** Parse x-viewra-actions from a JSON Schema */
export const parseSchemaActions = (schema: unknown): SchemaAction[] => {
  if (!schema || typeof schema !== 'object') {return []}
  const s = schema as Record<string, unknown>
  const actions = s['x-viewra-actions']
  if (!Array.isArray(actions)) {return []}
  return actions as SchemaAction[]
}

/** Parse x-viewra-meta from a JSON Schema */
export const parsePluginMeta = (schema: unknown): PluginMeta | null => {
  if (!schema || typeof schema !== 'object') {return null}
  const s = schema as Record<string, unknown>
  const meta = s['x-viewra-meta']
  if (!meta || typeof meta !== 'object') {return null}
  return meta as PluginMeta
}

/** Get actions that should appear as tabs */
export const getTabActions = (actions: SchemaAction[]): ListAction[] =>
  actions.filter(isListAction).filter((a) => a.tabTitle)

/** Get list actions that should appear inline (no tabTitle) */
export const getInlineListActions = (actions: SchemaAction[]): ListAction[] =>
  actions.filter(isListAction).filter((a) => !a.tabTitle)

/** Find a create action by ID */
export const findCreateAction = (
  actions: SchemaAction[],
  id: string,
): CreateAction | undefined => actions.filter(isCreateAction).find((a) => a.id === id)

/** Find a test action */
export const findTestAction = (actions: SchemaAction[]): TestAction | undefined =>
  actions.find(isTestAction)

// ============================================================================
// Section Types (x-viewra-sections)
// ============================================================================

/** Capability types for section filtering */
export type Capability = 'embedding' | 'chat'

/** Section configuration from x-viewra-sections */
export type SchemaSection = {
  id: string
  title?: string
  properties?: string[]
  actions?: string[]
  capabilities: Capability[]
}

/** Parse x-viewra-sections from a JSON Schema */
export const parseSchemaSections = (schema: unknown): SchemaSection[] => {
  if (!schema || typeof schema !== 'object') {
    return []
  }
  const s = schema as Record<string, unknown>
  const sections = s['x-viewra-sections']
  if (!Array.isArray(sections)) {
    return []
  }
  return sections as SchemaSection[]
}

/** Filter sections by capability */
export const getSectionsForCapability = (
  sections: SchemaSection[],
  capability: Capability
): SchemaSection[] => sections.filter((s) => s.capabilities.includes(capability))
