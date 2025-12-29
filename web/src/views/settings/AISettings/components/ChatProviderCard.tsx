import type { ReactNode } from 'react'
import { MessageSquare } from 'lucide-react'
import { ProviderTypeCard } from './ProviderTypeCard'
import type { GithubComMantonxViewraInternalDomainAiProviderInfo as ProviderInfo } from '@/lib/api/generated/models'
import type { PluginMeta, ProviderOption } from '../AISettings.types'

type ChatProviderCardProps = {
  /** Pre-rendered select field from form.Field */
  children: ReactNode
  options: ProviderOption[]
  selectedProvider?: ProviderInfo
  pluginId?: string
  meta?: PluginMeta
}

/**
 * Card for selecting and configuring the chat provider.
 */
export const ChatProviderCard = ({
  children,
  options,
  selectedProvider,
  pluginId,
  meta,
}: ChatProviderCardProps) => {
  return (
    <ProviderTypeCard
      icon={MessageSquare}
      title="Chat Provider"
      description="Provider for AI chat completions (mood tags, analysis)"
      capability="chat"
      noProvidersMessage="No chat providers available. Make sure provider plugins are installed and running."
      options={options}
      selectedProvider={selectedProvider}
      pluginId={pluginId}
      meta={meta}
    >
      {children}
    </ProviderTypeCard>
  )
}
