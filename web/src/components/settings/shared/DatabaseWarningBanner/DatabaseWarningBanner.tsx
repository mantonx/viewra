import { useGetApiSystemInfo } from '@/lib/api/generated/system/system'
import { cn } from '@/lib/utils'
import { AlertTriangle, Info, X } from 'lucide-react'
import { useState } from 'react'

import type { InternalApiHandlersWarningResponse } from '@/lib/api/generated/models'

/**
 * Banner displaying database-related warnings (e.g., SQLite in Kubernetes).
 * Shown at the top of the settings page when warnings are present.
 */
const DatabaseWarningBanner = () => {
  const { data: systemInfo } = useGetApiSystemInfo()
  const [dismissedCodes, setDismissedCodes] = useState<Set<string>>(new Set())

  const warnings: InternalApiHandlersWarningResponse[] =
    systemInfo?.status === 200 ? systemInfo.data.warnings ?? [] : []

  // Filter out dismissed warnings
  const activeWarnings = warnings.filter((w) => w.code && !dismissedCodes.has(w.code))

  if (activeWarnings.length === 0) {
    return null
  }

  const handleDismiss = (code: string) => {
    setDismissedCodes((prev) => new Set([...prev, code]))
  }

  const getIcon = (severity?: string) => {
    switch (severity) {
      case 'warning':
      case 'error':
        return <AlertTriangle className="w-5 h-5 shrink-0" />
      default:
        return <Info className="w-5 h-5 shrink-0" />
    }
  }

  const getStyles = (severity?: string) => {
    switch (severity) {
      case 'error':
        return {
          container: 'bg-red-50 dark:bg-red-950/30 border-red-200 dark:border-red-900',
          icon: 'text-red-600 dark:text-red-400',
          title: 'text-red-800 dark:text-red-300',
          message: 'text-red-700 dark:text-red-400',
          dismiss: 'text-red-500 hover:text-red-700 dark:text-red-400 dark:hover:text-red-200',
        }
      case 'warning':
        return {
          container: 'bg-amber-50 dark:bg-amber-950/30 border-amber-200 dark:border-amber-900',
          icon: 'text-amber-600 dark:text-amber-400',
          title: 'text-amber-800 dark:text-amber-300',
          message: 'text-amber-700 dark:text-amber-400',
          dismiss: 'text-amber-500 hover:text-amber-700 dark:text-amber-400 dark:hover:text-amber-200',
        }
      default:
        return {
          container: 'bg-blue-50 dark:bg-blue-950/30 border-blue-200 dark:border-blue-900',
          icon: 'text-blue-600 dark:text-blue-400',
          title: 'text-blue-800 dark:text-blue-300',
          message: 'text-blue-700 dark:text-blue-400',
          dismiss: 'text-blue-500 hover:text-blue-700 dark:text-blue-400 dark:hover:text-blue-200',
        }
    }
  }

  const getTitle = (code?: string) => {
    switch (code) {
      case 'SQLITE_IN_K8S':
        return 'SQLite in Kubernetes'
      case 'SQLITE_IN_CONTAINER':
        return 'SQLite in Container'
      default:
        return 'System Notice'
    }
  }

  return (
    <div className="space-y-3 mb-6">
      {activeWarnings.map((warning) => {
        const styles = getStyles(warning.severity)
        return (
          <div
            key={warning.code}
            className={cn('flex items-start gap-3 p-4 rounded-xl border', styles.container)}
          >
            <div className={styles.icon}>{getIcon(warning.severity)}</div>
            <div className="flex-1 min-w-0">
              <p className={cn('text-sm font-medium', styles.title)}>{getTitle(warning.code)}</p>
              <p className={cn('text-sm mt-1', styles.message)}>{warning.message}</p>
            </div>
            <button
              onClick={() => warning.code && handleDismiss(warning.code)}
              className={cn('p-1 rounded-lg hover:bg-black/5 dark:hover:bg-white/5', styles.dismiss)}
              aria-label="Dismiss warning"
            >
              <X className="w-4 h-4" />
            </button>
          </div>
        )
      })}
    </div>
  )
}

export { DatabaseWarningBanner }
