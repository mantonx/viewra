import { useState, useMemo } from 'react'
import { Button, Input, Spinner } from '@/components/ui'
import { RefreshCw, Search, Store } from 'lucide-react'
import { useMarketplaceData } from '../hooks/useMarketplaceData'
import { MarketplacePluginRow } from './MarketplacePluginRow'

/**
 * Marketplace tab content showing available plugins from GitHub releases.
 * Supports install, update, and uninstall operations with streaming progress.
 */
export const MarketplaceTab = () => {
  const {
    plugins,
    isLoading,
    error,
    cacheAge,
    refreshCatalog,
    isRefreshing,
    installPlugin,
    uninstallPlugin,
    isUninstalling,
    installProgress,
    isInstalling,
  } = useMarketplaceData()

  const [searchQuery, setSearchQuery] = useState('')

  // Filter plugins by search
  const filteredPlugins = useMemo(() => {
    if (!searchQuery.trim()) {
      return plugins
    }

    const query = searchQuery.toLowerCase()
    return plugins.filter(
      (p) =>
        p.name?.toLowerCase().includes(query) ||
        p.id?.toLowerCase().includes(query) ||
        p.description?.toLowerCase().includes(query) ||
        p.capabilities?.some((c) => c.toLowerCase().includes(query))
    )
  }, [plugins, searchQuery])

  // Split into installed and available
  const { installed, available } = useMemo(() => {
    const inst = filteredPlugins.filter((p) => p.is_installed)
    const avail = filteredPlugins.filter((p) => !p.is_installed)
    return { installed: inst, available: avail }
  }, [filteredPlugins])

  if (isLoading) {
    return (
      <div className="flex items-center justify-center py-12">
        <Spinner size="lg" />
      </div>
    )
  }

  if (error) {
    return (
      <div className="text-center py-12 text-neutral-500 dark:text-neutral-400">
        <p>Failed to load marketplace</p>
        <p className="text-sm mt-1">{error.message}</p>
      </div>
    )
  }

  return (
    <div className="space-y-4">
      {/* Search and refresh */}
      <div className="flex items-center gap-3 px-4 pt-4">
        <div className="relative flex-1">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-neutral-400" />
          <Input
            placeholder="Search plugins..."
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            className="pl-9"
          />
        </div>
        <Button
          variant="secondary"
          size="sm"
          onClick={refreshCatalog}
          disabled={isRefreshing}
          title="Refresh marketplace catalog"
        >
          <RefreshCw className={`w-4 h-4 ${isRefreshing ? 'animate-spin' : ''}`} />
        </Button>
      </div>

      {/* Cache age indicator */}
      {cacheAge && (
        <p className="px-4 text-xs text-neutral-400 dark:text-neutral-500">
          Catalog cached {cacheAge}
        </p>
      )}

      {/* Empty state */}
      {filteredPlugins.length === 0 && (
        <div className="text-center py-12 text-neutral-500 dark:text-neutral-400">
          <Store className="w-12 h-12 mx-auto mb-3 opacity-30" />
          <p>{searchQuery ? 'No plugins match your search' : 'No plugins available'}</p>
        </div>
      )}

      {/* Installed section */}
      {installed.length > 0 && (
        <div>
          <div className="px-4 py-2 text-xs font-medium text-neutral-500 dark:text-neutral-400 uppercase tracking-wide bg-neutral-50 dark:bg-neutral-800/50">
            Installed ({installed.length})
          </div>
          <div>
            {installed.map((plugin) => (
              <MarketplacePluginRow
                key={plugin.id}
                plugin={plugin}
                onInstall={() => installPlugin(plugin.id ?? '')}
                onUninstall={() => uninstallPlugin(plugin.id ?? '')}
                onUpdate={() => installPlugin(plugin.id ?? '')}
                installProgress={installProgress}
                isInstalling={isInstalling}
                isUninstalling={isUninstalling}
              />
            ))}
          </div>
        </div>
      )}

      {/* Available section */}
      {available.length > 0 && (
        <div>
          <div className="px-4 py-2 text-xs font-medium text-neutral-500 dark:text-neutral-400 uppercase tracking-wide bg-neutral-50 dark:bg-neutral-800/50">
            Available ({available.length})
          </div>
          <div>
            {available.map((plugin) => (
              <MarketplacePluginRow
                key={plugin.id}
                plugin={plugin}
                onInstall={() => installPlugin(plugin.id ?? '')}
                onUninstall={() => uninstallPlugin(plugin.id ?? '')}
                onUpdate={() => installPlugin(plugin.id ?? '')}
                installProgress={installProgress}
                isInstalling={isInstalling}
                isUninstalling={isUninstalling}
              />
            ))}
          </div>
        </div>
      )}
    </div>
  )
}
