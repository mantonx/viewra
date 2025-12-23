/**
 * Hook for fetching list data from a plugin endpoint.
 *
 * @example
 * ```tsx
 * const { items, isLoading, refresh } = useListData('ollama', '/models/recommended')
 * // With query params
 * const { items } = useListData('ollama', '/models/recommended', { params: { type: 'embedding' } })
 * ```
 */

import { useState, useCallback, useEffect, useMemo } from 'react'
import { pluginApi } from '@/lib/api/pluginApi'
import type { ListItem, UseListDataOptions, UseListDataReturn } from './useListData.types'

type ListResponse = {
  items?: ListItem[]
}

export const useListData = (
  pluginId: string,
  endpoint: string,
  options: UseListDataOptions = {}
): UseListDataReturn => {
  const { enabled = true, params } = options

  const [items, setItems] = useState<ListItem[]>([])
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState<Error | null>(null)

  // Memoize params to avoid unnecessary re-fetches
  const stableParams = useMemo(
    () => (params ? JSON.stringify(params) : ''),
    [params]
  )

  const refresh = useCallback(async () => {
    setIsLoading(true)
    setError(null)
    try {
      const parsedParams = stableParams ? JSON.parse(stableParams) : undefined
      const data = await pluginApi.get<ListResponse>(pluginId, endpoint, parsedParams)
      setItems(data.items || [])
    } catch (err) {
      const error = err instanceof Error ? err : new Error(String(err))
      setError(error)
      setItems([])
    } finally {
      setIsLoading(false)
    }
  }, [pluginId, endpoint, stableParams])

  useEffect(() => {
    if (enabled) {
      refresh()
    }
  }, [enabled, refresh])

  return {
    items,
    isLoading,
    error,
    refresh,
  }
}

export type { ListItem, UseListDataOptions, UseListDataReturn }
