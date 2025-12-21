import { useState, useEffect, useCallback } from 'react'
import { authFetch } from '@/lib/utils/authFetch'
import { useToast } from './useToast'
import type { GithubComMantonxViewraInternalApplicationAuthSessionResponse as Session } from '@/lib/api/generated/models'

export const useActiveSessions = () => {
  const toast = useToast()
  const [sessions, setSessions] = useState<Session[]>([])
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    const fetchSessions = async () => {
      try {
        const response = await authFetch('/api/auth/sessions')
        if (!response.ok) {
          throw new Error('Failed to fetch sessions')
        }
        const data = await response.json()
        setSessions(data.sessions || [])
      } catch (err) {
        setError(err instanceof Error ? err.message : 'Failed to load sessions')
      } finally {
        setIsLoading(false)
      }
    }

    fetchSessions()
  }, [])

  const revokeSession = useCallback(async (sessionId: string) => {
    try {
      const response = await authFetch(`/api/auth/sessions/${sessionId}`, {
        method: 'DELETE',
      })

      if (!response.ok) {
        throw new Error('Failed to revoke session')
      }

      setSessions(prev => prev.filter(s => s.id !== sessionId))
      toast.success('Session revoked')
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to revoke session')
    }
  }, [toast])

  return {
    sessions,
    isLoading,
    error,
    revokeSession,
  }
}
