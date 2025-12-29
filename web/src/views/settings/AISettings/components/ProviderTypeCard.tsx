import type { ReactNode } from 'react'
import type { LucideIcon } from 'lucide-react'
import { Card, CardHeader, CardContent, Alert } from '@/components/ui'
import { text } from '@/styles/semantic'
import { cn } from '@/lib/utils'
import { ProviderCard } from './ProviderCard'
import type { GithubComMantonxViewraInternalDomainAiProviderInfo as ProviderInfo } from '@/lib/api/generated/models'
import type { PluginMeta, ProviderOption } from '../AISettings.types'

type ProviderTypeCardProps = {
  /** Pre-rendered select field from form.Field */
  children: ReactNode
  /** Lucide icon component */
  icon: LucideIcon
  /** Card title */
  title: string
  /** Card description */
  description: string
  /** Provider capability type */
  capability: 'chat' | 'embedding'
  /** Alert message when no providers available */
  noProvidersMessage: string
  options: ProviderOption[]
  selectedProvider?: ProviderInfo
  pluginId?: string
  meta?: PluginMeta
}

/**
 * Generic card for selecting and configuring AI providers.
 * Used as the base for ChatProviderCard and EmbeddingProviderCard.
 */
export const ProviderTypeCard = ({
  children,
  icon: Icon,
  title,
  description,
  capability,
  noProvidersMessage,
  options,
  selectedProvider,
  pluginId,
  meta,
}: ProviderTypeCardProps) => {
  return (
    <Card variant="glass">
      <CardHeader className="border-b border-neutral-100 dark:border-neutral-800">
        <div className="flex items-center gap-3">
          <div
            className={cn(
              'p-2 rounded-lg',
              'bg-primary-50 dark:bg-primary-950/50',
              'text-primary-600 dark:text-primary-400'
            )}
          >
            <Icon className="w-5 h-5" />
          </div>
          <div>
            <h2 className={cn('text-lg font-semibold', text.primary)}>{title}</h2>
            <p className={cn('text-sm mt-0.5', text.secondary)}>{description}</p>
          </div>
        </div>
      </CardHeader>
      <CardContent className="space-y-4">
        {options.length > 0 ? (
          <>
            {children}
            {selectedProvider && (
              <ProviderCard
                provider={selectedProvider}
                pluginId={pluginId}
                meta={meta}
                capability={capability}
              />
            )}
          </>
        ) : (
          <Alert variant="warning">{noProvidersMessage}</Alert>
        )}
      </CardContent>
    </Card>
  )
}
