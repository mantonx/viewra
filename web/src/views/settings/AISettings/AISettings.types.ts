import type { GithubComMantonxViewraInternalDomainAiProviderInfo as ProviderInfo } from '@/lib/api/generated/models'

export type PluginMeta = {
  displayName?: string
  description?: string
  tip?: string
  isLocal?: boolean
  icon?: string
}

export type ProviderCardProps = {
  provider: ProviderInfo
  pluginId?: string
  meta?: PluginMeta
  /** Show only sections for this capability */
  capability?: 'embedding' | 'chat'
}

export type ProviderOption = {
  value: string
  label: string
}
