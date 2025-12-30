import { useMemo } from 'react'
import type { FieldProps } from '@rjsf/utils'
import { cn } from '@/lib/utils'
import { text } from '@/styles/semantic'
import { useGetApiPlugins, useGetApiPluginsIdSettings } from '@/lib/api/generated/plugins/plugins'
import type { PluginRefConfig } from '@/lib/types/schema-actions'
import type { GithubComMantonxViewraInternalApplicationPluginsPluginSummary } from '@/lib/api/generated/models'
import { JsonSchemaForm } from '../../JsonSchemaForm'

/**
 * Parse a byte array (number[]) as JSON.
 * The API returns schema and values as byte arrays that need to be decoded.
 */
const parseByteArray = (bytes: number[] | undefined): Record<string, unknown> | undefined => {
  if (!bytes || bytes.length === 0) {
    return undefined
  }
  try {
    const str = String.fromCharCode(...bytes)
    return JSON.parse(str) as Record<string, unknown>
  } catch {
    return undefined
  }
}

/**
 * PluginRefField renders a plugin selector dropdown followed by the selected
 * plugin's settings form inline. Used for configuration plugins (like ai-local)
 * to let users select and configure provider plugins.
 *
 * The field reads x-viewra-plugin-ref from the schema to determine:
 * - capability: Which capability to filter plugins by (e.g., "embedding")
 * - settingsKey: Where to store the referenced plugin's settings
 */
export const PluginRefField = (props: FieldProps) => {
  const { schema, formData, onChange, idSchema, disabled, readonly, name } = props

  // Extract plugin ref configuration from schema extension
  const pluginRef = schema['x-viewra-plugin-ref'] as PluginRefConfig | undefined
  const capability = pluginRef?.capability ?? ''
  // settingsKey will be used when we implement settings propagation
  const _settingsKey = pluginRef?.settingsKey ?? `${capability}_provider_settings`
  void _settingsKey // Suppress unused warning until implemented

  // Fetch all plugins to filter by capability
  const { data: pluginsResponse, isLoading: pluginsLoading } = useGetApiPlugins()

  // Filter plugins that provide the required capability
  const availablePlugins = useMemo(() => {
    if (pluginsResponse?.status !== 200 || !capability) {
      return []
    }

    const plugins = pluginsResponse.data.plugins ?? []
    return plugins.filter((plugin: GithubComMantonxViewraInternalApplicationPluginsPluginSummary) => {
      // Check if plugin provides the capability
      // Plugins declare capabilities via the "provides" field in manifest
      // which populates categories or a provider_id
      // For now, filter by category matching the capability
      const categories = plugin.categories ?? []
      return (
        categories.includes(capability) ||
        categories.includes('provider') ||
        plugin.provider_id
      )
    })
  }, [pluginsResponse, capability])

  // Current selected plugin ID (the field value)
  const selectedPluginId = (formData as string) ?? ''

  // Fetch settings for the selected plugin - only when we have a valid ID
  const { data: settingsResponse, isLoading: settingsLoading } = useGetApiPluginsIdSettings(
    selectedPluginId || '__placeholder__',
    {
      query: {
        enabled: !!selectedPluginId,
      },
    }
  )

  // Handle plugin selection change
  const handlePluginChange = (e: React.ChangeEvent<HTMLSelectElement>) => {
    const newPluginId = e.target.value
    // FieldProps onChange signature: (newValue, path, errorSchema?, id?)
    onChange(newPluginId, [name])
  }

  // Handle inline settings change
  const handleSettingsChange = (_newSettings: Record<string, unknown>) => {
    // Settings are stored separately under settingsKey in the parent form
    // This is handled by the parent component watching for changes
    // TODO: Implement proper settings propagation via form context
  }

  const selectedPlugin = availablePlugins.find(
    (p: GithubComMantonxViewraInternalApplicationPluginsPluginSummary) => p.id === selectedPluginId
  )

  // Parse the byte arrays into JSON objects
  const pluginSchema = useMemo(() => {
    if (settingsResponse?.status !== 200) {
      return undefined
    }
    return parseByteArray(settingsResponse.data.schema)
  }, [settingsResponse])

  const pluginSettings = useMemo(() => {
    if (settingsResponse?.status !== 200) {
      return undefined
    }
    return parseByteArray(settingsResponse.data.values)
  }, [settingsResponse])

  const schemaTitle = typeof schema.title === 'string' ? schema.title : 'Provider'
  const schemaDescription = typeof schema.description === 'string' ? schema.description : undefined
  const selectedPluginName = String(selectedPlugin?.meta?.displayName ?? selectedPlugin?.name ?? '')

  return (
    <div className="space-y-4">
      {/* Plugin selector dropdown */}
      <div>
        <label
          htmlFor={idSchema.$id}
          className={cn('block text-sm font-medium mb-2', text.primary)}
        >
          {schemaTitle}
        </label>
        <select
          id={idSchema.$id}
          value={selectedPluginId}
          onChange={handlePluginChange}
          disabled={disabled || readonly || pluginsLoading}
          className={cn(
            'w-full px-3 py-2 rounded-lg transition-all duration-150',
            'bg-white text-neutral-900',
            'border border-neutral-200/50',
            'hover:border-neutral-300',
            'dark:bg-white/5 dark:text-neutral-50',
            'dark:border-white/10',
            'dark:hover:border-white/20',
            'focus:outline-none focus:ring-2 focus:ring-primary-500/30 focus:border-primary-500',
            'disabled:bg-neutral-50 dark:disabled:bg-neutral-900/50',
            'disabled:text-neutral-500 dark:disabled:text-neutral-500',
            'disabled:cursor-not-allowed'
          )}
        >
          <option value="" className="bg-white dark:bg-neutral-800">
            {pluginsLoading ? 'Loading...' : 'Select a provider'}
          </option>
          {availablePlugins.map((plugin: GithubComMantonxViewraInternalApplicationPluginsPluginSummary) => (
            <option
              key={plugin.id}
              value={plugin.id}
              className="bg-white dark:bg-neutral-800 text-neutral-900 dark:text-neutral-50"
            >
              {String(plugin.meta?.displayName ?? plugin.name ?? plugin.id)}
            </option>
          ))}
        </select>
        {schemaDescription && (
          <p className={cn('text-xs mt-1.5', text.tertiary)}>{schemaDescription}</p>
        )}
      </div>

      {/* Inline plugin settings */}
      {selectedPluginId && selectedPlugin && (
        <div
          className={cn(
            'ml-4 pl-4 border-l-2',
            'border-neutral-200 dark:border-neutral-700'
          )}
        >
          {settingsLoading ? (
            <p className={cn('text-sm', text.secondary)}>Loading settings...</p>
          ) : pluginSchema ? (
            <div>
              <h4 className={cn('text-sm font-medium mb-3', text.primary)}>
                {selectedPluginName} Settings
              </h4>
              <JsonSchemaForm
                schema={pluginSchema}
                formData={pluginSettings}
                onChange={handleSettingsChange}
                hideSubmit
              />
            </div>
          ) : (
            <p className={cn('text-sm', text.tertiary)}>
              No additional configuration required.
            </p>
          )}
        </div>
      )}
    </div>
  )
}
