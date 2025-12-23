/**
 * ActionCreate - Form for creating items via a plugin endpoint.
 *
 * Supports streaming responses for long-running operations.
 */

import { useState, useCallback } from 'react'
import { cn } from '@/lib/utils'
import { text } from '@/styles/semantic'
import { useStreamingAction } from '@/lib/hooks'
import { useToast } from '@/lib/hooks/useToast'
import { pluginApi } from '@/lib/api/pluginApi'
import { CreateForm, ProgressDisplay } from './components'
import type { ActionCreateProps } from './ActionCreate.types'

export const ActionCreate = ({
  action,
  pluginId,
  onSuccess,
  className,
}: ActionCreateProps) => {
  const toast = useToast()
  const [formData, setFormData] = useState<Record<string, string>>({})
  const [showProgress, setShowProgress] = useState(false)

  const {
    execute: executeStreaming,
    progress,
    isStreaming,
  } = useStreamingAction(pluginId, {
    onComplete: () => {
      toast.success('Operation completed successfully')
      setFormData({})
      setShowProgress(false)
      onSuccess?.()
    },
    onError: () => {
      // Error is shown via progress display
      setTimeout(() => setShowProgress(false), 3000)
    },
  })

  const handleChange = useCallback((field: string, value: string) => {
    setFormData((prev) => ({ ...prev, [field]: value }))
  }, [])

  const handleSubmit = useCallback(async () => {
    // Validate required fields
    const schema = action.schema
    if (schema?.required) {
      for (const field of schema.required) {
        if (!formData[field]?.trim()) {
          toast.error(`${schema.properties?.[field]?.title || field} is required`)
          return
        }
      }
    }

    setShowProgress(true)

    if (action.streaming) {
      await executeStreaming(action.endpoint, formData)
    } else {
      try {
        await pluginApi.post(pluginId, action.endpoint, formData)
        toast.success('Operation completed successfully')
        setFormData({})
        setShowProgress(false)
        onSuccess?.()
      } catch (err) {
        toast.error(`Operation failed: ${err instanceof Error ? err.message : 'Unknown error'}`)
        setShowProgress(false)
      }
    }
  }, [action, pluginId, formData, executeStreaming, onSuccess, toast])

  return (
    <div className={cn('space-y-4', className)}>
      {/* Form section */}
      <div className="pt-4 border-t border-neutral-100 dark:border-neutral-800">
        <span className={cn('text-sm font-medium block mb-3', text.primary)}>
          {action.title}
        </span>
        <CreateForm
          schema={action.schema}
          formData={formData}
          isSubmitting={isStreaming}
          onChange={handleChange}
          onSubmit={handleSubmit}
        />
      </div>

      {/* Progress display */}
      {showProgress && progress && (
        <ProgressDisplay
          progress={progress}
          label={Object.values(formData).filter(Boolean).join(' ')}
        />
      )}
    </div>
  )
}

ActionCreate.displayName = 'ActionCreate'
