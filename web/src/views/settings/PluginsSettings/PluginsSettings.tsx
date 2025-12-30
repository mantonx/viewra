import { useState } from 'react'
import { SettingsPage } from '@/components/common'
import { AlertTriangle } from 'lucide-react'
import { usePluginsData, usePluginsFilters } from './hooks'
import { PluginFilters, PluginList, PluginSettingsModal } from './components'
import type { GithubComMantonxViewraInternalApplicationPluginsPluginSummary as PluginSummary } from '@/lib/api/generated/models'

/**
 * Plugins settings page component.
 * Manages installed plugins with enable/disable and configuration.
 * Uses a compact list layout similar to the Scheduler page.
 */
export const PluginsSettings = () => {
  const {
    plugins,
    isLoading,
    error,
    enablePlugin,
    disablePlugin,
    isEnabling,
    isDisabling,
  } = usePluginsData()

  const {
    activeTab,
    setActiveTab,
    tabCounts,
    searchQuery,
    setSearchQuery,
    groupedPlugins,
  } = usePluginsFilters(plugins)

  const [configuringPlugin, setConfiguringPlugin] = useState<PluginSummary | null>(null)

  // Check if any plugins have unmet dependencies
  const hasMissingDependencies = plugins.some(
    (p) => (p.missing_dependencies?.length ?? 0) > 0
  )

  return (
    <SettingsPage>
      <SettingsPage.Header
        title="Plugins"
        description="Manage plugins and their settings"
      />

      {/* Warning if any plugins have unmet dependencies */}
      {hasMissingDependencies && !isLoading && (
        <div className="flex items-center gap-2 px-4 py-3 rounded-xl mb-6 bg-amber-50 dark:bg-amber-500/10 border border-amber-200 dark:border-amber-500/20 text-amber-700 dark:text-amber-400">
          <AlertTriangle className="w-4 h-4 shrink-0" />
          <span className="text-sm">
            Some plugins have unmet dependencies and cannot be enabled
          </span>
        </div>
      )}

      <SettingsPage.Card>
        <PluginFilters
          activeTab={activeTab}
          onTabChange={setActiveTab}
          tabCounts={tabCounts}
          searchQuery={searchQuery}
          onSearchChange={setSearchQuery}
        />

        <div className="mt-4 -mx-5 -mb-5">
          <PluginList
            groups={groupedPlugins}
            isLoading={isLoading}
            error={error}
            onConfigurePlugin={setConfiguringPlugin}
            onEnablePlugin={enablePlugin}
            onDisablePlugin={disablePlugin}
            isEnabling={isEnabling}
            isDisabling={isDisabling}
          />
        </div>
      </SettingsPage.Card>

      <PluginSettingsModal
        isOpen={!!configuringPlugin}
        onClose={() => setConfiguringPlugin(null)}
        plugin={configuringPlugin}
      />
    </SettingsPage>
  )
}
