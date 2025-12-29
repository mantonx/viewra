/**
 * Scheduler Filters Hook
 * Manages filtering, search, and grouping state for the scheduler settings view
 */

import { useState, useMemo, useCallback } from 'react'
import type { TaskStatus, FilterTab, TabCounts, TaskGroupInfo } from '../SchedulerSettings.types'
import { getGroupDisplayName } from '../SchedulerSettings.types'

export interface UseSchedulerFiltersReturn {
  // Tab state
  activeTab: FilterTab
  setActiveTab: (tab: FilterTab) => void
  tabCounts: TabCounts

  // Search
  searchQuery: string
  setSearchQuery: (query: string) => void

  // Filtered data
  filteredTasks: TaskStatus[]
  groupedTasks: TaskGroupInfo[]

  // Expansion state
  expandedGroups: Set<string>
  toggleGroup: (groupId: string) => void
  isGroupExpanded: (groupId: string) => boolean
  expandAllGroups: () => void
  collapseAllGroups: () => void

  expandedTasks: Set<string>
  toggleTask: (taskId: string) => void
  isTaskExpanded: (taskId: string) => boolean
}

export const useSchedulerFilters = (tasks: TaskStatus[]): UseSchedulerFiltersReturn => {
  const [activeTab, setActiveTab] = useState<FilterTab>('all')
  const [searchQuery, setSearchQuery] = useState('')
  const [expandedGroups, setExpandedGroups] = useState<Set<string>>(() => new Set())
  const [expandedTasks, setExpandedTasks] = useState<Set<string>>(() => new Set())

  // Calculate tab counts
  const tabCounts = useMemo<TabCounts>(() => {
    const counts = { all: 0, system: 0, plugins: 0, disabled: 0 }

    for (const task of tasks) {
      counts.all++
      if (!task.enabled) {
        counts.disabled++
      }
      if (task.source === 'internal') {
        counts.system++
      } else if (task.source === 'plugin') {
        counts.plugins++
      }
    }

    return counts
  }, [tasks])

  // Filter tasks based on active tab and search query
  const filteredTasks = useMemo(() => {
    let result = tasks

    // Filter by tab
    switch (activeTab) {
      case 'system':
        result = result.filter((t) => t.source === 'internal')
        break
      case 'plugins':
        result = result.filter((t) => t.source === 'plugin')
        break
      case 'disabled':
        result = result.filter((t) => !t.enabled)
        break
      // 'all' shows everything
    }

    // Filter by search query
    if (searchQuery.trim()) {
      const query = searchQuery.toLowerCase()
      result = result.filter(
        (t) =>
          t.name.toLowerCase().includes(query) ||
          t.description.toLowerCase().includes(query) ||
          t.id.toLowerCase().includes(query)
      )
    }

    return result
  }, [tasks, activeTab, searchQuery])

  // Group filtered tasks by source_id
  const groupedTasks = useMemo<TaskGroupInfo[]>(() => {
    const groups = new Map<string, TaskGroupInfo>()

    for (const task of filteredTasks) {
      const groupId = task.source_id || task.source
      
      if (!groups.has(groupId)) {
        groups.set(groupId, {
          id: groupId,
          name: getGroupDisplayName(groupId, task.source),
          source: task.source as 'internal' | 'plugin',
          tasks: [],
        })
      }

      const group = groups.get(groupId)
      if (group) {
        group.tasks.push(task)
      }
    }

    // Sort groups: internal first, then plugins alphabetically
    return Array.from(groups.values()).sort((a, b) => {
      if (a.source !== b.source) {
        return a.source === 'internal' ? -1 : 1
      }
      return a.name.localeCompare(b.name)
    })
  }, [filteredTasks])

  // Group expansion handlers
  const toggleGroup = useCallback((groupId: string) => {
    setExpandedGroups((prev) => {
      const next = new Set(prev)
      if (next.has(groupId)) {
        next.delete(groupId)
      } else {
        next.add(groupId)
      }
      return next
    })
  }, [])

  const isGroupExpanded = useCallback(
    (groupId: string) => expandedGroups.has(groupId),
    [expandedGroups]
  )

  const expandAllGroups = useCallback(() => {
    setExpandedGroups(new Set(groupedTasks.map((g) => g.id)))
  }, [groupedTasks])

  const collapseAllGroups = useCallback(() => {
    setExpandedGroups(new Set())
  }, [])

  // Task expansion handlers
  const toggleTask = useCallback((taskId: string) => {
    setExpandedTasks((prev) => {
      const next = new Set(prev)
      if (next.has(taskId)) {
        next.delete(taskId)
      } else {
        next.add(taskId)
      }
      return next
    })
  }, [])

  const isTaskExpanded = useCallback(
    (taskId: string) => expandedTasks.has(taskId),
    [expandedTasks]
  )

  // Initialize groups as expanded by default
  useMemo(() => {
    if (expandedGroups.size === 0 && groupedTasks.length > 0) {
      setExpandedGroups(new Set(groupedTasks.map((g) => g.id)))
    }
  }, [groupedTasks, expandedGroups.size])

  return {
    // Tab state
    activeTab,
    setActiveTab,
    tabCounts,

    // Search
    searchQuery,
    setSearchQuery,

    // Filtered data
    filteredTasks,
    groupedTasks,

    // Expansion state
    expandedGroups,
    toggleGroup,
    isGroupExpanded,
    expandAllGroups,
    collapseAllGroups,

    expandedTasks,
    toggleTask,
    isTaskExpanded,
  }
}
