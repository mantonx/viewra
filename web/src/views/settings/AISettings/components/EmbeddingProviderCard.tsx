import type { ReactNode } from 'react'
import { Card, CardHeader, CardContent, Alert } from '@/components/ui'
import { text } from '@/styles/semantic'
import { cn } from '@/lib/utils'
import { Cpu } from 'lucide-react'
import { ProviderCard } from './ProviderCard'
import type { GithubComMantonxViewraInternalDomainAiProviderInfo as ProviderInfo } from '@/lib/api/generated/models'
import type { PluginMeta, ProviderOption } from '../AISettings.types'

type EmbeddingProviderCardProps = {
  /** Pre-rendered select field from form.Field */
  children: ReactNode
  options: ProviderOption[]
  selectedProvider?: ProviderInfo
  pluginId?: string
  meta?: PluginMeta
}

/**
 * Card for selecting and configuring the embedding provider.
 */
export const EmbeddingProviderCard = ({
  children,
  options,
  selectedProvider,
  pluginId,
  meta,
}: EmbeddingProviderCardProps) => {
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
            <Cpu className="w-5 h-5" />
          </div>
          <div>
            <h2 className={cn('text-lg font-semibold', text.primary)}>Embedding Provider</h2>
            <p className={cn('text-sm mt-0.5', text.secondary)}>
              Provider for generating text embeddings (semantic search)
            </p>
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
                capability="embedding"
              />
            )}
          </>
        ) : (
          <Alert variant="warning">
            No embedding providers available. Make sure provider plugins are installed and running.
          </Alert>
        )}
      </CardContent>
    </Card>
  )
}
