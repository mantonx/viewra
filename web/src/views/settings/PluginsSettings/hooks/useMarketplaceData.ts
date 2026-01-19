import { useState, useCallback, useRef, useEffect } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import {
  useGetApiPluginsMarketplace,
  useDeleteApiPluginsId,
  usePostApiPluginsMarketplaceRefresh,
  getGetApiPluginsMarketplaceQueryKey,
} from '@/lib/api/generated/marketplace/marketplace'
import { getGetApiPluginsQueryKey } from '@/lib/api/generated/plugins/plugins'
import { useToast } from '@/lib/hooks/useToast'
import { getAuthHeaders } from '@/lib/utils/authFetch'
import type { GithubComMantonxViewraInternalApplicationPluginsMarketplacePlugin as MarketplacePlugin } from '@/lib/api/generated/models'

export interface InstallProgress {
  plugin_id: string
  status: 'pending' | 'downloading' | 'installing' | 'complete' | 'failed'
  progress: number
  message?: string
  error?: string
  version?: string
}

interface UseMarketplaceDataReturn {
  plugins: MarketplacePlugin[]
  isLoading: boolean
  error: Error | null
  cacheAge?: string
  refreshCatalog: () => Promise<void>
  isRefreshing: boolean
  installPlugin: (pluginId: string) => Promise<void>
  uninstallPlugin: (pluginId: string) => Promise<void>
  isUninstalling: boolean
  installProgress: InstallProgress | null
  isInstalling: boolean
  cancelInstall: () => void
}

/**
 * Parse SSE events from a ReadableStream
 */
const parseSSEStream = async function* <T>(
  reader: ReadableStreamDefaultReader<Uint8Array>
): AsyncGenerator<T> {
  const decoder = new TextDecoder()
  let buffer = ''

  while (true) {
    const { done, value } = await reader.read()
    if (done) {
      break
    }

    buffer += decoder.decode(value, { stream: true })
    const lines = buffer.split('\n')
    buffer = lines.pop() ?? ''

    for (const line of lines) {
      if (line.startsWith('data: ')) {
        try {
          const data = JSON.parse(line.slice(6)) as T
          yield data
        } catch {
          // Ignore parse errors
        }
      }
    }
  }
}

/**
 * Hook for managing marketplace data and install/uninstall actions.
 */
export const useMarketplaceData = (): UseMarketplaceDataReturn => {
  const toast = useToast()
  const queryClient = useQueryClient()

  const { data, isLoading, error } = useGetApiPluginsMarketplace()
  const refreshMutation = usePostApiPluginsMarketplaceRefresh()
  const uninstallMutation = useDeleteApiPluginsId()

  const [installProgress, setInstallProgress] = useState<InstallProgress | null>(null)
  const [isInstalling, setIsInstalling] = useState(false)
  const abortRef = useRef<AbortController | null>(null)

  const plugins = data?.status === 200 ? (data.data.plugins ?? []) : []
  const cacheAge = data?.status === 200 ? data.data.cache_age : undefined

  const refreshCatalog = useCallback(async () => {
    try {
      const result = await refreshMutation.mutateAsync()
      if (result.status === 200) {
        toast.success('Catalog refreshed')
        await queryClient.invalidateQueries({ queryKey: getGetApiPluginsMarketplaceQueryKey() })
      }
    } catch {
      toast.error('Failed to refresh catalog')
    }
  }, [refreshMutation, toast, queryClient])

  const cancelInstall = useCallback(() => {
    abortRef.current?.abort()
    setIsInstalling(false)
    setInstallProgress(null)
  }, [])

  const installPlugin = useCallback(
    async (pluginId: string) => {
      cancelInstall()

      setIsInstalling(true)
      setInstallProgress({ plugin_id: pluginId, status: 'pending', progress: 0 })

      const controller = new AbortController()
      abortRef.current = controller

      try {
        const response = await fetch('/api/plugins/marketplace/install', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json', ...getAuthHeaders() },
          body: JSON.stringify({ plugin_id: pluginId }),
          signal: controller.signal,
        })

        if (!response.ok) {
          const err = await response.json()
          throw new Error(err.error || 'Install failed')
        }

        if (!response.body) {
          throw new Error('No response body')
        }

        const reader = response.body.getReader()

        for await (const event of parseSSEStream<InstallProgress>(reader)) {
          setInstallProgress(event)

          if (event.status === 'failed') {
            toast.error(event.error || 'Installation failed')
            break
          }

          if (event.status === 'complete') {
            toast.success(`${pluginId} installed successfully`)
            await queryClient.invalidateQueries({ queryKey: getGetApiPluginsMarketplaceQueryKey() })
            await queryClient.invalidateQueries({ queryKey: getGetApiPluginsQueryKey() })
          }
        }
      } catch (err) {
        if ((err as Error).name !== 'AbortError') {
          const message = err instanceof Error ? err.message : 'Installation failed'
          toast.error(message)
          setInstallProgress((prev) =>
            prev ? { ...prev, status: 'failed', error: message } : null
          )
        }
      } finally {
        abortRef.current = null
        setIsInstalling(false)
      }
    },
    [cancelInstall, toast, queryClient]
  )

  const uninstallPlugin = useCallback(
    async (pluginId: string) => {
      try {
        const result = await uninstallMutation.mutateAsync({ id: pluginId })
        if (result.status === 200) {
          toast.success(`${pluginId} uninstalled`)
          await queryClient.invalidateQueries({ queryKey: getGetApiPluginsMarketplaceQueryKey() })
          await queryClient.invalidateQueries({ queryKey: getGetApiPluginsQueryKey() })
        } else {
          toast.error('Failed to uninstall plugin')
        }
      } catch {
        toast.error('Failed to uninstall plugin')
      }
    },
    [uninstallMutation, toast, queryClient]
  )

  // Cleanup on unmount
  useEffect(() => {
    return () => {
      abortRef.current?.abort()
    }
  }, [])

  return {
    plugins,
    isLoading,
    error: error ? new Error(error.message ?? 'Unknown error') : null,
    cacheAge,
    refreshCatalog,
    isRefreshing: refreshMutation.isPending,
    installPlugin,
    uninstallPlugin,
    isUninstalling: uninstallMutation.isPending,
    installProgress,
    isInstalling,
    cancelInstall,
  }
}
