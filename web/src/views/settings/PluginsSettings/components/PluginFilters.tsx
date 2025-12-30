import type { Tab } from '@/components/ui/Tabs'
import { Input, Tabs } from '@/components/ui'
import type { FilterTab, TabCounts } from '../PluginsSettings.types'

interface PluginFiltersProps {
  activeTab: FilterTab
  onTabChange: (tab: FilterTab) => void
  tabCounts: TabCounts
  searchQuery: string
  onSearchChange: (query: string) => void
}

const TAB_DEFINITIONS: { id: FilterTab; label: string }[] = [
  { id: 'all', label: 'All Plugins' },
  { id: 'enrichers', label: 'Enrichers' },
  { id: 'providers', label: 'Providers' },
  { id: 'disabled', label: 'Disabled' },
]

export const PluginFilters = ({
  activeTab,
  onTabChange,
  tabCounts,
  searchQuery,
  onSearchChange,
}: PluginFiltersProps) => {
  const tabs: Tab[] = TAB_DEFINITIONS.map(({ id, label }) => ({
    id,
    label,
    badge: tabCounts[id],
  }))

  return (
    <div className="space-y-4">
      <Input
        placeholder="Search plugins..."
        value={searchQuery}
        onChange={(e) => onSearchChange(e.target.value)}
        className="max-w-sm"
      />
      <Tabs
        tabs={tabs}
        activeTab={activeTab}
        onTabChange={(id) => onTabChange(id as FilterTab)}
        variant="underline"
      />
    </div>
  )
}
