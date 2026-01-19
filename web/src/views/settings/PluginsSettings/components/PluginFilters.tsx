import type { Tab } from '@/components/ui/Tabs'
import { Input, Tabs } from '@/components/ui'
import type { FilterTab } from '../PluginsSettings.types'

interface PluginFiltersProps {
  activeTabId: string
  onTabChange: (tabId: string) => void
  tabs: FilterTab[]
  searchQuery: string
  onSearchChange: (query: string) => void
}

export const PluginFilters = ({
  activeTabId,
  onTabChange,
  tabs,
  searchQuery,
  onSearchChange,
}: PluginFiltersProps) => {
  const uiTabs: Tab[] = tabs.map((tab) => ({
    id: tab.id ?? '',
    label: tab.label ?? '',
    badge: tab.count,
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
        tabs={uiTabs}
        activeTab={activeTabId}
        onTabChange={onTabChange}
        variant="underline"
      />
    </div>
  )
}
