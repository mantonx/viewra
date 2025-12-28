import { Tooltip } from '@/components/ui'
import { PluginSettingsForm, LocalBadge } from '@/components/settings'
import { text } from '@/styles/semantic'
import { cn, getIcon } from '@/lib/utils'
import type { ProviderCardProps } from '../AISettings.types'

/**
 * Card section showing provider info and its plugin settings.
 */
export const ProviderCard = ({ provider, pluginId, meta, capability }: ProviderCardProps) => {
  const displayName = meta?.displayName || provider.name
  const description = meta?.description || provider.description
  const isLocal = meta?.isLocal ?? false
  const tip = meta?.tip
  const Icon = getIcon(meta?.icon)

  return (
    <div className="space-y-4 pt-4 border-t border-neutral-100 dark:border-neutral-800">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <Icon className={cn('w-4 h-4', text.secondary)} />
          <span className={cn('text-sm font-medium', text.primary)}>{displayName}</span>
          {isLocal && <LocalBadge />}
          {tip && <Tooltip content={tip} />}
        </div>
      </div>
      {description && <p className={cn('text-xs', text.secondary)}>{description}</p>}
      {pluginId && <PluginSettingsForm pluginId={pluginId} capability={capability} className="mt-4" />}
    </div>
  )
}
