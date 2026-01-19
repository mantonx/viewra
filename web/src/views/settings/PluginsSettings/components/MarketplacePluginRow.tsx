import { cn } from '@/lib/utils'
import { Button, Progress } from '@/components/ui'
import { Download, Trash2, ArrowUpCircle, CheckCircle, Loader2 } from 'lucide-react'
import type { GithubComMantonxViewraInternalApplicationPluginsMarketplacePlugin as MarketplacePlugin } from '@/lib/api/generated/models'
import type { InstallProgress } from '../hooks/useMarketplaceData'

interface MarketplacePluginRowProps {
  plugin: MarketplacePlugin
  onInstall: () => void
  onUninstall: () => void
  onUpdate: () => void
  installProgress: InstallProgress | null
  isInstalling: boolean
  isUninstalling: boolean
}

const formatDownloads = (count?: number): string => {
  if (!count) {
    return ''
  }
  if (count >= 1000) {
    return `${(count / 1000).toFixed(1)}k`
  }
  return String(count)
}

export const MarketplacePluginRow = ({
  plugin,
  onInstall,
  onUninstall,
  onUpdate,
  installProgress,
  isInstalling,
  isUninstalling,
}: MarketplacePluginRowProps) => {
  const displayName = plugin.name ?? plugin.id ?? 'Unknown'
  const isInstalledPlugin = plugin.is_installed ?? false
  const hasUpdate = plugin.has_update ?? false
  const downloads = formatDownloads(plugin.download_count)

  const isThisPluginInstalling = isInstalling && installProgress?.plugin_id === plugin.id
  const showProgress = isThisPluginInstalling && installProgress

  return (
    <div
      className={cn(
        'flex items-center gap-3 px-4 py-3 border-b border-neutral-100 dark:border-neutral-800 last:border-b-0',
        isThisPluginInstalling && 'bg-blue-50/50 dark:bg-blue-900/10'
      )}
    >
      {/* Status indicator */}
      <div className="w-2 shrink-0">
        {isInstalledPlugin ? (
          <CheckCircle className="w-4 h-4 text-green-500" />
        ) : (
          <span className="block w-2 h-2 rounded-full bg-neutral-300 dark:bg-neutral-600" />
        )}
      </div>

      {/* Plugin info */}
      <div className="flex-1 min-w-0">
        <div className="flex items-center gap-2">
          <span className="font-medium text-neutral-900 dark:text-neutral-100">{displayName}</span>
          <span className="text-xs text-neutral-400 dark:text-neutral-500">
            v{plugin.latest_version}
          </span>
          {hasUpdate && (
            <span className="text-xs px-1.5 py-0.5 rounded bg-amber-100 dark:bg-amber-900/30 text-amber-700 dark:text-amber-400">
              Update available
            </span>
          )}
          {isInstalledPlugin && !hasUpdate && (
            <span className="text-xs text-neutral-400">
              (installed: v{plugin.installed_version})
            </span>
          )}
        </div>

        {/* Description or progress */}
        {showProgress ? (
          <div className="mt-1 space-y-1">
            <div className="flex items-center gap-2">
              <Loader2 className="w-3 h-3 animate-spin text-blue-500" />
              <span className="text-sm text-blue-600 dark:text-blue-400">
                {installProgress.message || capitalizeStatus(installProgress.status)}
              </span>
            </div>
            <Progress value={installProgress.progress} size="xs" className="max-w-xs" />
          </div>
        ) : (
          <>
            {plugin.description && (
              <p className="text-sm text-neutral-500 dark:text-neutral-400 truncate">
                {plugin.description}
              </p>
            )}
            {(downloads || plugin.author) && (
              <p className="text-xs text-neutral-400 dark:text-neutral-500 mt-0.5">
                {plugin.author && <span>by {plugin.author}</span>}
                {plugin.author && downloads && <span className="mx-1">·</span>}
                {downloads && <span>{downloads} downloads</span>}
              </p>
            )}
          </>
        )}
      </div>

      {/* Actions */}
      <div className="flex items-center gap-2 shrink-0">
        {isInstalledPlugin ? (
          <>
            {hasUpdate && (
              <Button
                variant="primary"
                size="sm"
                onClick={onUpdate}
                disabled={isInstalling || isUninstalling}
                title="Update plugin"
              >
                <ArrowUpCircle className="w-4 h-4 mr-1" />
                Update
              </Button>
            )}
            <Button
              variant="ghost"
              size="sm"
              onClick={onUninstall}
              disabled={isInstalling || isUninstalling}
              isLoading={isUninstalling}
              title="Uninstall plugin"
              className="text-red-600 hover:text-red-700 hover:bg-red-50 dark:text-red-400 dark:hover:bg-red-900/20"
            >
              <Trash2 className="w-4 h-4" />
            </Button>
          </>
        ) : (
          <Button
            variant="primary"
            size="sm"
            onClick={onInstall}
            disabled={isInstalling}
            isLoading={isThisPluginInstalling}
          >
            <Download className="w-4 h-4 mr-1" />
            Install
          </Button>
        )}
      </div>
    </div>
  )
}

const capitalizeStatus = (status: string): string => {
  return status.charAt(0).toUpperCase() + status.slice(1).replace('_', ' ')
}
