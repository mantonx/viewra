import { Spinner } from '@/components/ui'
import type { PluginGroupInfo } from '../PluginsSettings.types'
import type { GithubComMantonxViewraInternalApplicationPluginsPluginSummary as PluginSummary } from '@/lib/api/generated/models'
import { PluginRow } from './PluginRow'

interface PluginListProps {
  groups: PluginGroupInfo[]
  isLoading: boolean
  error: unknown
  onConfigurePlugin: (plugin: PluginSummary) => void
  onEnablePlugin: (pluginId: string) => void
  onDisablePlugin: (pluginId: string) => void
  isEnabling: boolean
  isDisabling: boolean
}

export const PluginList = ({
  groups,
  isLoading,
  error,
  onConfigurePlugin,
  onEnablePlugin,
  onDisablePlugin,
  isEnabling,
  isDisabling,
}: PluginListProps) => {
  if (isLoading) {
    return (
      <div className="flex items-center justify-center py-12">
        <Spinner size="lg" />
      </div>
    )
  }

  if (error) {
    const errorMessage = error instanceof Error ? error.message : 'An unknown error occurred'
    return (
      <div className="text-center py-12">
        <p className="text-red-600 dark:text-red-400 mb-4">Failed to load plugins</p>
        <p className="text-sm text-neutral-500 dark:text-neutral-400">{errorMessage}</p>
      </div>
    )
  }

  if (groups.length === 0) {
    return (
      <div className="text-center py-12 text-neutral-500 dark:text-neutral-400">
        No plugins found matching your filters
      </div>
    )
  }

  return (
    <div className="divide-y divide-neutral-200 dark:divide-neutral-700">
      {groups.map((group) => (
        <div key={group.id}>
          {/* Group header */}
          <div className="flex items-center gap-2 px-4 py-2 bg-neutral-50 dark:bg-neutral-800/50">
            <span className="text-xs font-medium text-neutral-500 dark:text-neutral-400 uppercase tracking-wide">
              {group.name}
            </span>
          </div>

          {/* Plugins in this group */}
          {group.plugins.map((plugin: PluginSummary) => (
            <PluginRow
              key={plugin.id}
              plugin={plugin}
              onConfigure={() => onConfigurePlugin(plugin)}
              onEnable={() => onEnablePlugin(plugin.id ?? '')}
              onDisable={() => onDisablePlugin(plugin.id ?? '')}
              isEnabling={isEnabling}
              isDisabling={isDisabling}
            />
          ))}
        </div>
      ))}
    </div>
  )
}
