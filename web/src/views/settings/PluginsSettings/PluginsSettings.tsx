import { useState } from 'react'
import { SettingsPage } from '@/components/common'
import { Tabs } from '@/components/ui'
import { AlertTriangle } from 'lucide-react'
import { usePluginsData, usePluginsFilters } from './hooks'
import { PluginFilters, PluginList, PluginSettingsModal, MarketplaceTab } from './components'
import type { GithubComMantonxViewraInternalApplicationPluginsPluginSummary as PluginSummary } from '@/lib/api/generated/models'

type MainTab = 'installed' | 'marketplace'

/**
 * Plugins settings page component.
 * Manages installed plugins with enable/disable and configuration.
 * Includes marketplace for installing new plugins.
 */
export const PluginsSettings = () => {
  const [mainTab, setMainTab] = useState<MainTab>('installed')

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
        description="Manage installed plugins or browse the marketplace"
      />

      {/* Main tab switcher */}
      <div className="mb-6">
        <Tabs
          tabs={[
            { id: 'installed', label: 'Installed' },
            { id: 'marketplace', label: 'Marketplace' },
          ]}
          activeTab={mainTab}
          onTabChange={(id) => setMainTab(id as MainTab)}
          variant="underline"
        />
      </div>

      {mainTab === 'installed' && (
        <>
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
        </>
      )}

      {mainTab === 'marketplace' && (
        <SettingsPage.Card className="overflow-hidden">
          <MarketplaceTab />
        </SettingsPage.Card>
      )}
    </SettingsPage>
  )
}
