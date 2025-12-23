/**
 * ActionTest - Button to test connectivity/health of a plugin endpoint.
 */

import { useState, useCallback } from 'react'
import { cn } from '@/lib/utils'
import { CheckCircle, XCircle, Loader2, Wifi } from 'lucide-react'
import { Button } from '@/components/ui'
import { pluginApi } from '@/lib/api/pluginApi'
import type { ActionTestProps, TestResult } from './ActionTest.types'

export const ActionTest = ({ action, pluginId, className }: ActionTestProps) => {
  const [result, setResult] = useState<TestResult | null>(null)
  const [isTesting, setIsTesting] = useState(false)

  const handleTest = useCallback(async () => {
    setIsTesting(true)
    setResult(null)

    try {
      const data = await pluginApi.get<TestResult>(pluginId, action.endpoint)
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
                <span className="text-sm text-emerald-600 dark:text-emerald-400">
                  {result.message || 'Connected'}
                </span>
              </>
            ) : (
              <>
                <XCircle className="w-4 h-4 text-red-500" />
                <span className="text-sm text-red-600 dark:text-red-400">
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

ActionTest.displayName = 'ActionTest'
