import { cn } from '@/lib/utils'
import { Button } from '@/components/ui'
import { AlertTriangle, Settings } from 'lucide-react'
import type { GithubComMantonxViewraInternalApplicationPluginsPluginSummary as PluginSummary } from '@/lib/api/generated/models'

interface PluginRowProps {
  plugin: PluginSummary
  onConfigure: () => void
  onEnable: () => void
  onDisable: () => void
  isEnabling: boolean
  isDisabling: boolean
}

const HealthDot = ({ health, enabled }: { health?: string; enabled: boolean }) => {
  if (!enabled) {
    return (
      <span
        className="h-2 w-2 rounded-full bg-neutral-300 dark:bg-neutral-600"
        title="Disabled"
      />
    )
  }

  const status = health?.toLowerCase() ?? 'unknown'

  if (status === 'healthy') {
    return <span className="h-2 w-2 rounded-full bg-green-500" title="Healthy" />
  }
  if (status === 'unhealthy') {
    return <span className="h-2 w-2 rounded-full bg-red-500" title="Unhealthy" />
  }
  if (status === 'degraded') {
    return <span className="h-2 w-2 rounded-full bg-amber-500" title="Degraded" />
  }
  return <span className="h-2 w-2 rounded-full bg-neutral-400" title="Unknown" />
}

export const PluginRow = ({
  plugin,
  onConfigure,
  onEnable,
  onDisable,
  isEnabling,
  isDisabling,
}: PluginRowProps) => {
  const displayName = String(plugin.meta?.displayName ?? plugin.name ?? plugin.id ?? 'Unknown')
  const description = plugin.description ?? ''
  const version = plugin.version ?? ''
  const hasMissingDeps = (plugin.missing_dependencies?.length ?? 0) > 0
  const hasSettings = plugin.has_settings ?? false
  const isEnabled = plugin.enabled ?? false

  return (
    <div
      className={cn(
        'flex items-center gap-3 px-4 py-3 border-b border-neutral-100 dark:border-neutral-800 last:border-b-0',
        !isEnabled && 'opacity-60'
      )}
    >
      {/* Health indicator */}
      <HealthDot health={plugin.health} enabled={isEnabled} />

      {/* Plugin name and description */}
      <div className="flex-1 min-w-0">
        <div className="flex items-center gap-2">
          <span className="font-medium text-neutral-900 dark:text-neutral-100">{displayName}</span>
          {version && (
            <span className="text-xs text-neutral-400 dark:text-neutral-500">v{version}</span>
          )}
          {hasMissingDeps && (
            <span title={`Missing: ${plugin.missing_dependencies?.join(', ')}`}>
              <AlertTriangle className="w-4 h-4 text-amber-500" />
            </span>
          )}
        </div>
        {description && (
          <p className="text-sm text-neutral-500 dark:text-neutral-400 truncate">{description}</p>
        )}
      </div>

      {/* Actions */}
      <div className="flex items-center gap-2 shrink-0">
        {/* Configure button - only if has settings and is enabled */}
        {hasSettings && isEnabled && (
          <Button
            variant="ghost"
            size="sm"
            onClick={onConfigure}
            title="Configure plugin"
          >
            <Settings className="w-4 h-4" />
          </Button>
        )}

        {/* Enable/Disable button */}
        <Button
          variant={isEnabled ? 'secondary' : 'primary'}
          size="sm"
          onClick={isEnabled ? onDisable : onEnable}
          disabled={hasMissingDeps && !isEnabled}
          isLoading={isEnabling || isDisabling}
        >
          {isEnabled ? 'Disable' : 'Enable'}
        </Button>
      </div>
    </div>
  )
}
