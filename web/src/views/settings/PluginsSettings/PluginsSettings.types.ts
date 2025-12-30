import type { GithubComMantonxViewraInternalApplicationPluginsPluginSummary as PluginSummary } from '@/lib/api/generated/models'

export type FilterTab = 'all' | 'enrichers' | 'providers' | 'disabled'

export interface TabCounts {
  all: number
  enrichers: number
  providers: number
  disabled: number
}

export interface PluginGroupInfo {
  id: string
  name: string
  plugins: PluginSummary[]
}
