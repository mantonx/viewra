import { useEffect, useState } from 'react'
import { cn } from '@/lib/utils'
import { useServerStatus } from '@/contexts/ServerStatusContext'
import { Spinner } from '@/components/ui/Spinner'

/**
 * Full-screen overlay displayed when the server is offline or restarting.
 * Takes over the entire app to prevent interaction with stale data.
 */
const ServerStatusOverlay = () => {
  const { status, offlineSince } = useServerStatus()
  const [elapsedSeconds, setElapsedSeconds] = useState(0)

  // Update elapsed time counter
  useEffect(() => {
    if (!offlineSince) {
      setElapsedSeconds(0)
      return
    }

    const updateElapsed = () => {
      const elapsed = Math.floor((Date.now() - offlineSince.getTime()) / 1000)
      setElapsedSeconds(elapsed)
    }

    updateElapsed()
    const interval = setInterval(updateElapsed, 1000)
    return () => clearInterval(interval)
  }, [offlineSince])

  // Don't render anything if server is online
  if (status === 'online') {
    return null
  }

  const isRestarting = status === 'restarting'

  const getMessage = () => {
    if (isRestarting) {
      if (elapsedSeconds > 30) {
        return 'Running database migrations...'
      }
      if (elapsedSeconds > 10) {
        return 'Applying changes...'
      }
      return 'Restarting server...'
    }
    // Server went offline unexpectedly
    return 'Server is offline'
  }

  const getHelpText = () => {
    if (isRestarting && elapsedSeconds > 45) {
      return 'This is taking longer than expected. Check the server logs.'
    }
    if (!isRestarting) {
      return 'Waiting for server to come back online...'
    }
    return null
  }

  const helpText = getHelpText()

  return (
    <div
      className={cn(
        'fixed inset-0 z-[9999]',
        'flex flex-col items-center justify-center gap-5',
        'bg-black/30 dark:bg-black/50 backdrop-blur-sm',
        'animate-in fade-in duration-200'
      )}
    >
      <Spinner size="lg" className="text-primary-500" />

      <div className="flex flex-col items-center gap-1">
        <p className="text-sm font-medium text-neutral-700 dark:text-neutral-200">
          {getMessage()}
        </p>
        {elapsedSeconds > 5 && (
          <p className="text-xs text-neutral-500 dark:text-neutral-400">
            {elapsedSeconds}s elapsed
          </p>
        )}
      </div>

      {helpText && (
        <p className="text-xs text-neutral-500 dark:text-neutral-400 max-w-xs text-center">
          {helpText}
        </p>
      )}
    </div>
  )
}

export { ServerStatusOverlay }
