/**
 * SchedulerSettings View
 * Main view for managing scheduled tasks
 */

import { useState } from 'react'
import { SettingsPage } from '@/components/common'
import { useSchedulerData, useSchedulerFilters } from './hooks'
import { TaskFilters, TaskList, TaskHistoryModal, ScheduleEditorModal } from './components'
import type { TaskStatus } from './SchedulerSettings.types'

export const SchedulerSettings = () => {
  // Data hook
  const {
    tasks,
    isLoading,
    error,
    selectedHistoryTaskId,
    setSelectedHistoryTaskId,
    historyData,
    isLoadingHistory,
    triggerTask,
    enableTask,
    disableTask,
    updateSchedule,
    isTriggeringTask,
    isEnablingTask,
    isDisablingTask,
    isUpdatingSchedule,
  } = useSchedulerData()

  // Filter hook
  const {
    activeTab,
    setActiveTab,
    tabCounts,
    searchQuery,
    setSearchQuery,
    groupedTasks,
    expandedTasks,
    toggleTask,
  } = useSchedulerFilters(tasks)

  // Modal state
  const [editingTask, setEditingTask] = useState<TaskStatus | null>(null)

  // Find task name for history modal
  const historyTaskName = selectedHistoryTaskId
    ? tasks.find((t) => t.id === selectedHistoryTaskId)?.name || 'Task'
    : 'Task'

  return (
    <SettingsPage>
      <SettingsPage.Header
        title="Scheduler"
        description="Manage scheduled tasks and their execution frequency"
      />

      <SettingsPage.Card>
        <TaskFilters
          activeTab={activeTab}
          onTabChange={setActiveTab}
          tabCounts={tabCounts}
          searchQuery={searchQuery}
          onSearchChange={setSearchQuery}
        />

        <div className="mt-4 -mx-5 -mb-5">
          <TaskList
            groups={groupedTasks}
            isLoading={isLoading}
            error={error}
            expandedTasks={expandedTasks}
            onToggleTask={toggleTask}
            onTriggerTask={triggerTask}
            onEnableTask={enableTask}
            onDisableTask={disableTask}
            onEditSchedule={setEditingTask}
            onViewHistory={setSelectedHistoryTaskId}
            isTriggeringTask={isTriggeringTask}
            isEnablingTask={isEnablingTask}
            isDisablingTask={isDisablingTask}
          />
        </div>
      </SettingsPage.Card>

      {/* History Modal */}
      <TaskHistoryModal
        isOpen={!!selectedHistoryTaskId}
        onClose={() => setSelectedHistoryTaskId(null)}
        taskName={historyTaskName}
        history={historyData}
        isLoading={isLoadingHistory}
      />

      {/* Schedule Editor Modal */}
      <ScheduleEditorModal
        isOpen={!!editingTask}
        onClose={() => setEditingTask(null)}
        task={editingTask}
        onSave={(taskId, schedule) => {
          updateSchedule(taskId, schedule)
          setEditingTask(null)
        }}
        isSaving={isUpdatingSchedule}
      />
    </SettingsPage>
  )
}
