import { useState, useCallback } from 'react'
import { SettingsPage } from '@/components/common'
import { SettingRow } from '@/components/settings'
import { Alert } from '@/components/ui'
import { PluginSettingsForm } from '@/components/settings/forms/PluginSettingsForm'
import { AlertTriangle, ChevronDown, ChevronUp } from 'lucide-react'
import { cn } from '@/lib/utils'
import type { GithubComMantonxViewraInternalApplicationPluginsPluginSummary as PluginSummary } from '@/lib/api/generated/models'

interface PluginCardProps {
  plugin: PluginSummary
  onEnable: () => void
  onDisable: () => void
  isLoading?: boolean
}

/**
 * Card component for displaying and managing a single plugin.
 * Shows plugin info, enable/disable toggle, and settings form.
 */
export const PluginCard = ({ plugin, onEnable, onDisable, isLoading }: PluginCardProps) => {
  const [isExpanded, setIsExpanded] = useState(false)
  const [hasUnsavedChanges, setHasUnsavedChanges] = useState(false)

  const hasMissingDeps = (plugin.missing_dependencies?.length ?? 0) > 0
  const isEnabled = plugin.enabled ?? false
  const hasSettings = plugin.has_settings ?? false
  const capabilities = plugin.capabilities ?? []

  const handleToggle = useCallback(
    (enabled: boolean) => {
      if (enabled) {
        onEnable()
      } else {
        onDisable()
      }
    },
    [onEnable, onDisable]
  )

  const handleSettingsChange = useCallback((hasChanges: boolean) => {
    setHasUnsavedChanges(hasChanges)
  }, [])

  // Get display name from meta or fall back to name/id
  const displayName = String(plugin.meta?.displayName ?? plugin.name ?? plugin.id ?? 'Unknown Plugin')
  const description = String(plugin.meta?.description ?? plugin.description ?? '')
  const version = plugin.version ?? ''

  return (
    <SettingsPage.Card
      title={
        <div className="flex items-center gap-3">
          <span>{displayName}</span>
          {version && (
            <span className="text-xs text-neutral-500 dark:text-neutral-400 font-normal">
              v{version}
            </span>
          )}
          {capabilities.length > 0 && (
            <div className="flex gap-1">
              {capabilities.map((cap) => (
                <span
                  key={cap}
                  className="px-1.5 py-0.5 text-[10px] rounded bg-neutral-100 dark:bg-white/10 text-neutral-600 dark:text-neutral-400"
                >
                  {cap}
                </span>
              ))}
            </div>
          )}
        </div>
      }
      description={description}
      className="mt-6"
    >
      {/* Dependency error banner */}
      {hasMissingDeps && (
        <Alert variant="warning" className="mb-4">
          <div className="flex items-start gap-2">
            <AlertTriangle className="w-4 h-4 shrink-0 mt-0.5" />
            <div>
              <span className="font-medium">Missing dependencies:</span>
              <div className="mt-1 flex flex-wrap gap-1">
                {plugin.missing_dependencies?.map((dep) => (
                  <code
                    key={dep}
                    className="px-1.5 py-0.5 text-xs rounded bg-amber-100 dark:bg-amber-900/30"
                  >
                    {dep}
                  </code>
                ))}
              </div>
              <p className="mt-1 text-sm opacity-80">
                Enable and configure a plugin that provides these capabilities.
              </p>
            </div>
          </div>
        </Alert>
      )}

      {/* Enable/disable toggle */}
      <SettingRow
        type="toggle"
        label="Enabled"
        description="Enable or disable this plugin"
        value={isEnabled}
        onChange={handleToggle}
        disabled={isLoading || (hasMissingDeps && !isEnabled)}
      />

      {/* Plugin settings (expandable) */}
      {isEnabled && hasSettings && (
        <div className="mt-4 pt-4 border-t border-neutral-200/50 dark:border-white/10">
          <button
            onClick={() => setIsExpanded(!isExpanded)}
            className={cn(
              'w-full flex items-center justify-between py-2 px-1',
              'text-sm font-medium text-neutral-700 dark:text-neutral-300',
              'hover:text-neutral-900 dark:hover:text-neutral-100 transition-colors'
            )}
          >
            <span className="flex items-center gap-2">
              Settings
              {hasUnsavedChanges && (
                <span className="text-xs text-amber-600 dark:text-amber-400">(unsaved changes)</span>
              )}
            </span>
            {isExpanded ? (
              <ChevronUp className="w-4 h-4" />
            ) : (
              <ChevronDown className="w-4 h-4" />
            )}
          </button>

          {isExpanded && (
            <div className="mt-3">
              <PluginSettingsForm
                pluginId={plugin.id ?? ''}
                onSettingsChange={handleSettingsChange}
              />
            </div>
          )}
        </div>
      )}

      {/* Health status indicator */}
      {isEnabled && plugin.health && (
        <div className="mt-3 pt-3 border-t border-neutral-200/50 dark:border-white/10">
          <div className="flex items-center gap-2 text-xs">
            <span
              className={cn(
                'w-2 h-2 rounded-full',
                plugin.health === 'healthy' && 'bg-green-500',
                plugin.health === 'unhealthy' && 'bg-red-500',
                plugin.health === 'degraded' && 'bg-amber-500',
                plugin.health === 'unknown' && 'bg-neutral-400'
              )}
            />
            <span className="text-neutral-500 dark:text-neutral-400 capitalize">
              {plugin.health}
            </span>
          </div>
        </div>
      )}
    </SettingsPage.Card>
  )
}
