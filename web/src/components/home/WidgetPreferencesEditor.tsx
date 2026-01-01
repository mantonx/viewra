import { useState, useEffect, useCallback } from 'react'
import { GripVertical, Eye, EyeOff, RotateCcw } from 'lucide-react'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/Button'
import {
  useHomeSections,
  useWidgetPreferences,
  useUpdateWidgetPreferences,
  useResetWidgetPreferences,
  type WidgetPreference,
} from '@/lib/hooks/useWidgets'
import type { HomeSection } from '@/components/home/widgets/widget.types'

interface WidgetItem {
  id: string
  title: string
  type: string
  pluginId: string
  position: number
  hidden: boolean
}

/**
 * WidgetPreferencesEditor - Manage widget visibility and order
 *
 * Allows users to:
 * - Show/hide widgets
 * - Reorder widgets via drag and drop
 * - Reset to default order
 */
export const WidgetPreferencesEditor = () => {
  const { data: homeSections } = useHomeSections()
  const { data: preferences } = useWidgetPreferences()
  const updatePreferences = useUpdateWidgetPreferences()
  const resetPreferences = useResetWidgetPreferences()

  const [widgets, setWidgets] = useState<WidgetItem[]>([])
  const [draggedIndex, setDraggedIndex] = useState<number | null>(null)
  const [hasChanges, setHasChanges] = useState(false)

  // Get display title for a widget
  const getWidgetTitle = useCallback((section: HomeSection): string => {
    const data = section.data as unknown as Record<string, unknown> | undefined
    if (data && typeof data.title === 'string') {
      return data.title
    }
    if (section.type === 'search-hero') {
      return 'Search'
    }
    if (section.type === 'continue-row') {
      return 'Continue Watching'
    }
    return section.id
  }, [])

  // Build widget list from sections and preferences
  useEffect(() => {
    if (!homeSections?.sections) {
      return
    }

    const prefMap = new Map(preferences?.map((p) => [p.id, p]) ?? [])

    const items: WidgetItem[] = homeSections.sections
      .map((section: HomeSection) => {
        const pref = prefMap.get(section.id)
        return {
          id: section.id,
          title: getWidgetTitle(section),
          type: section.type,
          pluginId: section.plugin_id,
          position: pref?.position ?? section.priority,
          hidden: pref?.hidden ?? false,
        }
      })
      .sort((a, b) => a.position - b.position)

    setWidgets(items)
    setHasChanges(false)
  }, [homeSections, preferences, getWidgetTitle])

  // Toggle widget visibility
  const toggleVisibility = useCallback((index: number) => {
    setWidgets((prev) => {
      const updated = [...prev]
      updated[index] = { ...updated[index], hidden: !updated[index].hidden }
      return updated
    })
    setHasChanges(true)
  }, [])

  // Handle drag start
  const handleDragStart = useCallback((index: number) => {
    setDraggedIndex(index)
  }, [])

  // Handle drag over
  const handleDragOver = useCallback(
    (e: React.DragEvent, index: number) => {
      e.preventDefault()
      if (draggedIndex === null || draggedIndex === index) {
        return
      }

      setWidgets((prev) => {
        const updated = [...prev]
        const [dragged] = updated.splice(draggedIndex, 1)
        updated.splice(index, 0, dragged)
        return updated.map((w, i) => ({ ...w, position: i }))
      })
      setDraggedIndex(index)
      setHasChanges(true)
    },
    [draggedIndex]
  )

  // Handle drag end
  const handleDragEnd = useCallback(() => {
    setDraggedIndex(null)
  }, [])

  // Save preferences
  const handleSave = useCallback(() => {
    const sections: WidgetPreference[] = widgets.map((w) => ({
      id: w.id,
      position: w.position,
      hidden: w.hidden,
    }))
    updatePreferences.mutate({ sections })
    setHasChanges(false)
  }, [widgets, updatePreferences])

  // Reset to defaults
  const handleReset = useCallback(() => {
    resetPreferences.mutate()
  }, [resetPreferences])

  if (!widgets.length) {
    return (
      <div className="text-sm text-neutral-500 dark:text-neutral-400 py-4">
        No widgets available. Make sure plugins are enabled.
      </div>
    )
  }

  return (
    <div className="space-y-4">
      {/* Widget list */}
      <div className="space-y-2">
        {widgets.map((widget, index) => (
          <div
            key={widget.id}
            draggable
            onDragStart={() => handleDragStart(index)}
            onDragOver={(e) => handleDragOver(e, index)}
            onDragEnd={handleDragEnd}
            className={cn(
              'flex items-center gap-3 p-3 rounded-lg',
              'bg-white/80 dark:bg-white/5',
              'border border-neutral-200/50 dark:border-white/10',
              'transition-all duration-150',
              draggedIndex === index && 'opacity-50 scale-98',
              widget.hidden && 'opacity-60'
            )}
          >
            {/* Drag handle */}
            <button
              type="button"
              className="cursor-grab active:cursor-grabbing text-neutral-400 hover:text-neutral-600 dark:hover:text-neutral-300"
              aria-label="Drag to reorder"
            >
              <GripVertical className="w-4 h-4" />
            </button>

            {/* Widget info */}
            <div className="flex-1 min-w-0">
              <div className="flex items-center gap-2">
                <span
                  className={cn(
                    'text-sm font-medium',
                    widget.hidden
                      ? 'text-neutral-500 dark:text-neutral-400'
                      : 'text-neutral-900 dark:text-neutral-50'
                  )}
                >
                  {widget.title}
                </span>
                <span className="text-xs text-neutral-400 dark:text-neutral-500">
                  {widget.type}
                </span>
              </div>
              <span className="text-xs text-neutral-400 dark:text-neutral-500">
                {widget.pluginId}
              </span>
            </div>

            {/* Visibility toggle */}
            <button
              type="button"
              onClick={() => toggleVisibility(index)}
              className={cn(
                'p-2 rounded-lg transition-colors',
                widget.hidden
                  ? 'text-neutral-400 hover:text-neutral-600 dark:hover:text-neutral-300 hover:bg-neutral-100 dark:hover:bg-white/10'
                  : 'text-primary-600 dark:text-primary-400 hover:bg-primary-50 dark:hover:bg-primary-500/10'
              )}
              aria-label={widget.hidden ? 'Show widget' : 'Hide widget'}
            >
              {widget.hidden ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
            </button>
          </div>
        ))}
      </div>

      {/* Actions */}
      <div className="flex items-center justify-between pt-2">
        <Button
          variant="ghost"
          size="sm"
          onClick={handleReset}
          disabled={resetPreferences.isPending}
        >
          <RotateCcw className="w-4 h-4 mr-2" />
          Reset to Defaults
        </Button>

        <Button
          variant="primary"
          size="sm"
          onClick={handleSave}
          disabled={!hasChanges || updatePreferences.isPending}
        >
          {updatePreferences.isPending ? 'Saving...' : 'Save Changes'}
        </Button>
      </div>
    </div>
  )
}
