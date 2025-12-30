import { useMemo, lazy, Suspense } from 'react'
import type { FieldProps } from '@rjsf/utils'
import { cn } from '@/lib/utils'
import { text } from '@/styles/semantic'
import { useGetApiPlugins } from '@/lib/api/generated/plugins/plugins'
import type { PluginRefConfig, Capability } from '@/lib/types/schema-actions'
import type { GithubComMantonxViewraInternalApplicationPluginsPluginSummary } from '@/lib/api/generated/models'

// Lazy import to break circular dependency:
// JsonSchemaForm -> PluginRefField -> PluginSettingsForm -> JsonSchemaForm
const PluginSettingsForm = lazy(() =>
  import('@/components/settings/forms/PluginSettingsForm').then((mod) => ({
    default: mod.PluginSettingsForm,
  }))
)

/**
 * PluginRefField renders a plugin selector dropdown followed by the selected
 * plugin's settings form inline. Used for configuration plugins (like ai-local)
 * to let users select and configure provider plugins.
 *
 * The field reads x-viewra-plugin-ref from the schema to determine:
 * - capability: Which capability to filter plugins by (e.g., "embedding")
 * - settingsKey: Where to store the referenced plugin's settings
 *
 * When a plugin is selected, only settings relevant to the capability are shown
 * (e.g., when selecting an embedding provider, only embedding-related settings appear).
 */
export const PluginRefField = (props: FieldProps) => {
  const { schema, formData, onChange, idSchema, disabled, readonly, name } = props

  // Extract plugin ref configuration from schema extension
  const pluginRef = schema['x-viewra-plugin-ref'] as PluginRefConfig | undefined
  const capability = pluginRef?.capability ?? ''

  // Fetch all plugins to filter by capability
  const { data: pluginsResponse, isLoading: pluginsLoading } = useGetApiPlugins()

  // Filter plugins that provide the required capability
  const availablePlugins = useMemo(() => {
    if (pluginsResponse?.status !== 200 || !capability) {
      return []
    }

    const plugins = pluginsResponse.data.plugins ?? []
    return plugins.filter((plugin: GithubComMantonxViewraInternalApplicationPluginsPluginSummary) => {
      const capabilities = plugin.capabilities ?? []
      return capabilities.includes(capability)
    })
  }, [pluginsResponse, capability])

  // Current selected plugin ID (the field value)
  const selectedPluginId = (formData as string) ?? ''

  // Handle plugin selection change
  const handlePluginChange = (e: React.ChangeEvent<HTMLSelectElement>) => {
    const newPluginId = e.target.value
    onChange(newPluginId, [name])
  }

  const selectedPlugin = availablePlugins.find(
    (p: GithubComMantonxViewraInternalApplicationPluginsPluginSummary) => p.id === selectedPluginId
  )

  const fieldId = idSchema?.$id ?? `plugin-ref-${name ?? 'field'}`

  return (
    <div className="space-y-4">
      {/* Plugin selector dropdown */}
      <div>
        <select
          id={fieldId}
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
              {String(plugin.meta?.providerName ?? plugin.meta?.displayName ?? plugin.name ?? plugin.id)}
            </option>
          ))}
        </select>
      </div>

      {/* Inline plugin settings - filtered by capability */}
      {selectedPluginId && selectedPlugin && (
        <div
          className={cn(
            'ml-4 pl-4 border-l-2',
            'border-neutral-200 dark:border-neutral-700'
          )}
        >
          <Suspense fallback={<p className={cn('text-sm', text.secondary)}>Loading settings...</p>}>
            <PluginSettingsForm
              pluginId={selectedPluginId}
              capability={capability as Capability}
              hideSubmit
            />
          </Suspense>
        </div>
      )}
    </div>
  )
}
