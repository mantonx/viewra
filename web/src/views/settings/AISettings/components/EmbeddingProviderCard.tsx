import type { ReactNode } from 'react'
import { Cpu } from 'lucide-react'
import { ProviderTypeCard } from './ProviderTypeCard'
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
    <ProviderTypeCard
      icon={Cpu}
      title="Embedding Provider"
      description="Provider for generating text embeddings (semantic search)"
      capability="embedding"
      noProvidersMessage="No embedding providers available. Make sure provider plugins are installed and running."
      options={options}
      selectedProvider={selectedProvider}
      pluginId={pluginId}
      meta={meta}
    >
      {children}
    </ProviderTypeCard>
  )
}
