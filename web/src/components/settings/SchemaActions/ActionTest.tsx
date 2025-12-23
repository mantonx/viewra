import { useState, useCallback } from 'react'
import { Button } from '@/components/ui'
import { cn } from '@/lib/utils'
import { CheckCircle, XCircle, Loader2, Wifi } from 'lucide-react'
import type { TestAction } from '@/lib/types/schema-actions'

type TestResult = {
  success: boolean
  message?: string
  error?: string
}

type ActionTestProps = {
  action: TestAction
  pluginId: string
  className?: string
}

export const ActionTest = ({ action, pluginId, className }: ActionTestProps) => {
  const [result, setResult] = useState<TestResult | null>(null)
  const [isTesting, setIsTesting] = useState(false)

  const handleTest = useCallback(async () => {
    setIsTesting(true)
    setResult(null)

    try {
      const response = await fetch(`/api/plugin/${pluginId}${action.endpoint}`, {
        credentials: 'include',
      })

      const data = await response.json()
      setResult({
        success: data.success === true,
        message: data.message,
        error: data.error,
      })
    } catch (err) {
      setResult({
        success: false,
        error: err instanceof Error ? err.message : 'Connection failed',
      })
    } finally {
      setIsTesting(false)
    }
  }, [pluginId, action.endpoint])

  return (
    <div className={cn('space-y-3', className)}>
      <div className="flex items-center gap-3">
        <Button onClick={handleTest} disabled={isTesting} variant="secondary" size="sm">
          {isTesting ? (
            <Loader2 className="w-4 h-4 mr-2 animate-spin" />
          ) : (
            <Wifi className="w-4 h-4 mr-2" />
          )}
          {action.title}
        </Button>

        {result && (
          <div className="flex items-center gap-2">
            {result.success ? (
              <>
                <CheckCircle className="w-4 h-4 text-emerald-500" />
                <span className={cn('text-sm', 'text-emerald-600 dark:text-emerald-400')}>
                  {result.message || 'Connected'}
                </span>
              </>
            ) : (
              <>
                <XCircle className="w-4 h-4 text-red-500" />
                <span className={cn('text-sm', 'text-red-600 dark:text-red-400')}>
                  {result.error || 'Connection failed'}
                </span>
              </>
            )}
          </div>
        )}
      </div>
    </div>
  )
}
