import type { GithubComMantonxViewraInternalApplicationPluginsPluginSummary as PluginSummary } from '@/lib/api/generated/models'
import type { GithubComMantonxViewraInternalApplicationPluginsFilterTab as FilterTab } from '@/lib/api/generated/models'

export type { FilterTab }

export interface PluginGroupInfo {
  id: string
  name: string
  plugins: PluginSummary[]
}
