/**
 * TaskFilters Component
 * Search input and filter tabs for scheduler tasks
 */

import type { Tab } from '@/components/ui/Tabs'
import { Input, Tabs } from '@/components/ui'
import type { FilterTab, TabCounts } from '../SchedulerSettings.types'

interface TaskFiltersProps {
  activeTab: FilterTab
  onTabChange: (tab: FilterTab) => void
  tabCounts: TabCounts
  searchQuery: string
  onSearchChange: (query: string) => void
}

const TAB_DEFINITIONS: { id: FilterTab; label: string }[] = [
  { id: 'all', label: 'All Tasks' },
  { id: 'system', label: 'System' },
  { id: 'plugins', label: 'Plugins' },
  { id: 'disabled', label: 'Disabled' },
]

export const TaskFilters = ({
  activeTab,
  onTabChange,
  tabCounts,
  searchQuery,
  onSearchChange,
}: TaskFiltersProps) => {
  const tabs: Tab[] = TAB_DEFINITIONS.map(({ id, label }) => ({
    id,
    label,
    badge: tabCounts[id],
  }))

  return (
    <div className="space-y-4">
      <Input
        placeholder="Search tasks..."
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
