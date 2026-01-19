import type { InternalApiHandlersAdminStatusEvent } from '@/lib/api/generated/models'
import { useSSE, type SSEConnectionState } from '@/lib/hooks/useSSE'
import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useRef,
  useState,
  type ReactNode,
} from 'react'
import { flushSync } from 'react-dom'
import { useAuth } from './AuthContext'

type ServerStatus = 'online' | 'offline' | 'restarting'

interface ServerStatusContextValue {
  status: ServerStatus
  /** Mark server as restarting (called before triggering restart) */
  setRestarting: () => void
  /** Time when server went offline/restarting */
  offlineSince: Date | null
  /** Whether a restart is pending (from SSE) */
  restartPending: boolean
  /** Settings that require restart (from SSE) */
  pendingSettings: string[]
}

const ServerStatusContext = createContext<ServerStatusContextValue | null>(null)

interface ServerStatusProviderProps {
  children: ReactNode
}

const ServerStatusProvider = ({ children }: ServerStatusProviderProps) => {
  const { isAuthenticated } = useAuth()
  const [_internalStatus, setInternalStatus] = useState<'online' | 'restarting'>('online')
  const [offlineSince, setOfflineSince] = useState<Date | null>(null)
  const isRestartingRef = useRef(false)
  const hasDisconnectedRef = useRef(false) // Track if server actually went down

  const handleStateChange = useCallback(
    (state: SSEConnectionState) => {
      if (state === 'disconnected' || state === 'error' || state === 'connecting') {
        // SSE disconnected - mark that server went down
        hasDisconnectedRef.current = true
        // If we didn't trigger a restart, this is an unexpected disconnect
        if (!isRestartingRef.current && !offlineSince) {
          setOfflineSince(new Date())
        }
      }
    },
    [offlineSince]
  )

  const { connectionState, lastEvent } = useSSE<InternalApiHandlersAdminStatusEvent>(
    '/api/admin/status/stream',
    {
      enabled: isAuthenticated, // Only connect when authenticated
      maxReconnectAttempts: 0, // Unlimited retries
      reconnectDelay: 1000, // 1 second between attempts
      eventTypes: ['status', 'shutdown'],
      onStateChange: handleStateChange,
    }
  )

  const setRestarting = useCallback(() => {
    // Use flushSync to ensure the overlay renders immediately before
    // the restart API call potentially blocks or the server goes down
    flushSync(() => {
      isRestartingRef.current = true
      setInternalStatus('restarting')
      setOfflineSince(new Date())
    })
  }, [])

  // Derive actual status from SSE state and ready field
  const isConnected = connectionState === 'connected'
  const isReady = lastEvent?.ready === true
  const isMaintenanceMode = lastEvent?.maintenance === true

  // Determine the final status
  // Only show overlay if:
  // 1. We explicitly triggered a restart (isRestartingRef.current)
  // 2. SSE is disconnected (server actually down)
  // Do NOT show overlay during maintenance mode - that's intentional
  // Do NOT show overlay when not authenticated (SSE won't connect)
  let status: ServerStatus
  if (!isAuthenticated) {
    // Not authenticated - SSE won't connect, don't show offline overlay
    status = 'online'
  } else if (isRestartingRef.current) {
    // User triggered restart - show overlay immediately and keep showing
    // until server disconnects AND reconnects with ready: true
    const serverCameBack = hasDisconnectedRef.current && isConnected && isReady
    status = serverCameBack ? 'online' : 'restarting'
  } else if (!isConnected) {
    // SSE disconnected unexpectedly - server is offline
    status = 'offline'
  } else if (isMaintenanceMode) {
    // Maintenance mode - server is up, don't show overlay
    status = 'online'
  } else {
    // Connected and not in maintenance - check ready status
    status = 'online'
  }

  // When we transition back to online, reset internal state
  useEffect(() => {
    if (isConnected && isReady && hasDisconnectedRef.current) {
      isRestartingRef.current = false
      hasDisconnectedRef.current = false
      setInternalStatus('online')
      setOfflineSince(null)
    }
  }, [isConnected, isReady])

  const value: ServerStatusContextValue = {
    status,
    setRestarting,
    offlineSince,
    restartPending: lastEvent?.restart_pending ?? false,
    pendingSettings: lastEvent?.pending_settings ?? [],
  }

  return <ServerStatusContext.Provider value={value}>{children}</ServerStatusContext.Provider>
}

const useServerStatus = (): ServerStatusContextValue => {
  const context = useContext(ServerStatusContext)
  if (!context) {
    throw new Error('useServerStatus must be used within a ServerStatusProvider')
  }
  return context
}

export { ServerStatusProvider, useServerStatus }
export type { ServerStatus, ServerStatusContextValue }
