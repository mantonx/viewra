import { useState, useCallback } from 'react'
import { Button, Input } from '@/components/ui'
import { useToast } from '@/lib/hooks/useToast'
import { text } from '@/styles/semantic'
import { cn } from '@/lib/utils'
import { Download, Loader2, CheckCircle, AlertCircle } from 'lucide-react'
import type { CreateAction } from '@/lib/types/schema-actions'

type StreamProgress = {
  status: string
  digest?: string
  total?: number
  completed?: number
  done?: boolean
  error?: string
}

type ActionCreateProps = {
  action: CreateAction
  pluginId: string
  onSuccess?: () => void
  className?: string
}

export const ActionCreate = ({ action, pluginId, onSuccess, className }: ActionCreateProps) => {
  const toast = useToast()
  const [formData, setFormData] = useState<Record<string, string>>({})
  const [progress, setProgress] = useState<StreamProgress | null>(null)
  const [isSubmitting, setIsSubmitting] = useState(false)

  const getProgressPercent = (p: StreamProgress) => {
    if (p.total && p.completed) {
      return Math.round((p.completed / p.total) * 100)
    }
    return undefined
  }

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

    setIsSubmitting(true)
    setProgress({ status: 'Starting...' })

    try {
      const response = await fetch(`/api/plugin/${pluginId}${action.endpoint}`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(formData),
        credentials: 'include',
      })

      if (!response.ok) {
        throw new Error(`HTTP ${response.status}`)
      }

      if (action.streaming) {
        // Handle SSE streaming response
        const reader = response.body?.getReader()
        if (!reader) {
          throw new Error('No response body')
        }

        const decoder = new TextDecoder()
        let buffer = ''

        while (true) {
          const { done, value } = await reader.read()
          if (done) {
            break
          }

          buffer += decoder.decode(value, { stream: true })
          const lines = buffer.split('\n')
          buffer = lines.pop() || ''

          for (const line of lines) {
            if (line.startsWith('data: ')) {
              try {
                const data = JSON.parse(line.slice(6)) as StreamProgress
                if (data.error) {
                  throw new Error(data.error)
                }
                setProgress(data)
                if (data.done) {
                  toast.success('Operation completed successfully')
                  setFormData({})
                  onSuccess?.()
                }
              } catch (e) {
                if (e instanceof SyntaxError) {
                  continue
                }
                throw e
              }
            }
          }
        }
      } else {
        // Non-streaming response
        const data = await response.json()
        if (data.error) {
          throw new Error(data.error)
        }
        toast.success('Operation completed successfully')
        setFormData({})
        onSuccess?.()
      }
    } catch (err) {
      toast.error(`Operation failed: ${err instanceof Error ? err.message : 'Unknown error'}`)
      setProgress({ status: 'Failed', error: String(err) })
    } finally {
      setIsSubmitting(false)
      // Clear progress after a delay
      setTimeout(() => setProgress(null), 3000)
    }
  }, [action, pluginId, formData, toast, onSuccess])

  const renderFormFields = () => {
    if (!action.schema?.properties) {
      return null
    }

    return Object.entries(action.schema.properties).map(([key, prop]) => (
      <div key={key} className="flex-1">
        <Input
          value={formData[key] || ''}
          onChange={(e) => setFormData((prev) => ({ ...prev, [key]: e.target.value }))}
          placeholder={prop.description || prop.title || key}
          disabled={isSubmitting}
          onKeyDown={(e) => {
            if (e.key === 'Enter') {
              handleSubmit()
            }
          }}
        />
      </div>
    ))
  }

  return (
    <div className={cn('space-y-4', className)}>
      {/* Form */}
      <div className="pt-4 border-t border-neutral-100 dark:border-neutral-800">
        <span className={cn('text-sm font-medium block mb-3', text.primary)}>{action.title}</span>
        <div className="flex gap-2">
          {renderFormFields()}
          <Button onClick={handleSubmit} disabled={isSubmitting}>
            {isSubmitting ? (
              <Loader2 className="w-4 h-4 animate-spin" />
            ) : (
              <Download className="w-4 h-4" />
            )}
          </Button>
        </div>
      </div>

      {/* Progress display */}
      {progress && (
        <div
          className={cn(
            'py-3 px-3 rounded-lg',
            'bg-neutral-50 dark:bg-neutral-900/50',
            progress.error && 'border border-red-200 dark:border-red-900'
          )}
        >
          <div className="flex items-center justify-between gap-3">
            <div className="flex-1 min-w-0">
              <span className={cn('text-sm font-medium', text.primary)}>
                {Object.values(formData).join(' ')}
              </span>
              <p className={cn('text-xs mt-0.5', progress.error ? 'text-red-500' : text.secondary)}>
                {progress.error || progress.status}
              </p>
            </div>
            <div className="flex items-center gap-2">
              {progress.done ? (
                <CheckCircle className="w-4 h-4 text-emerald-500" />
              ) : progress.error ? (
                <AlertCircle className="w-4 h-4 text-red-500" />
              ) : (
                <>
                  <div className="w-20 h-1.5 bg-neutral-200 dark:bg-neutral-700 rounded-full overflow-hidden">
                    <div
                      className="h-full bg-primary-500 transition-all duration-300"
                      style={{ width: `${getProgressPercent(progress) ?? 0}%` }}
                    />
                  </div>
                  <span className={cn('text-xs tabular-nums w-10', text.secondary)}>
                    {getProgressPercent(progress) !== undefined
                      ? `${getProgressPercent(progress)}%`
                      : '...'}
                  </span>
                </>
              )}
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
