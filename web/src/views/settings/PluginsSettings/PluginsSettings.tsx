import { SettingsPage } from '@/components/common'
import { Alert, Loading } from '@/components/ui'
import { AlertTriangle } from 'lucide-react'
import { usePluginsData } from './hooks'
import { PluginCard } from './PluginCard'

/**
 * Plugins settings page component.
 * Manages installed plugins with enable/disable and configuration.
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

  // Check if any plugins have unmet dependencies
  const hasMissingDependencies = plugins.some(
    (p) => (p.missing_dependencies?.length ?? 0) > 0
  )

  if (isLoading) {
    return (
      <SettingsPage>
        <SettingsPage.Header
          title="Plugins"
          description="Manage plugins and their settings"
        />
        <SettingsPage.Card>
          <Loading text="Loading plugins..." />
        </SettingsPage.Card>
      </SettingsPage>
    )
  }

  if (error) {
    return (
      <SettingsPage>
        <SettingsPage.Header
          title="Plugins"
          description="Manage plugins and their settings"
        />
        <SettingsPage.Card>
          <Alert variant="error">Failed to load plugins.</Alert>
        </SettingsPage.Card>
      </SettingsPage>
    )
  }

  if (plugins.length === 0) {
    return (
      <SettingsPage>
        <SettingsPage.Header
          title="Plugins"
          description="Manage plugins and their settings"
        />
        <SettingsPage.Card>
          <Alert variant="info">
            No plugins installed. Place plugin binaries in the plugins directory and restart the
            server.
          </Alert>
        </SettingsPage.Card>
      </SettingsPage>
    )
  }

  return (
    <SettingsPage>
      <SettingsPage.Header
        title="Plugins"
        description="Manage plugins and their settings"
      />

      {/* Warning if any plugins have unmet dependencies */}
      {hasMissingDependencies && (
        <Alert variant="warning" className="mb-6">
          <div className="flex items-center gap-2">
            <AlertTriangle className="w-4 h-4" />
            <span>
              Some plugins have unmet dependencies and cannot be enabled until the required
              capabilities are available.
            </span>
          </div>
        </Alert>
      )}

      {/* Plugin cards */}
      {plugins.map((plugin) => (
        <PluginCard
          key={plugin.id}
          plugin={plugin}
          onEnable={() => enablePlugin(plugin.id ?? '')}
          onDisable={() => disablePlugin(plugin.id ?? '')}
          isLoading={isEnabling || isDisabling}
        />
      ))}
    </SettingsPage>
  )
}
