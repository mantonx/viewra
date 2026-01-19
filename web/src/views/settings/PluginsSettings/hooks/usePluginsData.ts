import { useMemo, useCallback } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import {
  useGetApiPlugins,
  usePostApiPluginsIdEnable,
  usePostApiPluginsIdDisable,
  getGetApiPluginsQueryKey,
} from '@/lib/api/generated/plugins/plugins'
import { useToast } from '@/lib/hooks/useToast'

/**
 * Hook for managing plugins data and actions.
 * Provides list of plugins with enable/disable functionality.
 */
export const usePluginsData = () => {
  const toast = useToast()
  const queryClient = useQueryClient()

  const { data, isLoading, error, refetch } = useGetApiPlugins()

  const enableMutation = usePostApiPluginsIdEnable()
  const disableMutation = usePostApiPluginsIdDisable()

  const plugins = useMemo(() => {
    if (data?.status !== 200) {
      return []
    }
    return data.data.plugins || []
  }, [data])

  const tabs = useMemo(() => {
    if (data?.status !== 200) {
      return []
    }
    return data.data.tabs || []
  }, [data])

  const enablePlugin = useCallback(
    async (id: string) => {
      try {
        const result = await enableMutation.mutateAsync({ id })
        if (result.status === 200) {
          toast.success('Plugin enabled')
          await queryClient.invalidateQueries({ queryKey: getGetApiPluginsQueryKey() })
        } else {
          toast.error('Failed to enable plugin')
        }
      } catch {
        toast.error('Failed to enable plugin')
      }
    },
    [enableMutation, toast, queryClient]
  )

  const disablePlugin = useCallback(
    async (id: string) => {
      try {
        const result = await disableMutation.mutateAsync({ id })
        if (result.status === 200) {
          toast.success('Plugin disabled')
          await queryClient.invalidateQueries({ queryKey: getGetApiPluginsQueryKey() })
        } else {
          toast.error('Failed to disable plugin')
        }
      } catch {
        toast.error('Failed to disable plugin')
      }
    },
    [disableMutation, toast, queryClient]
  )

  return {
    plugins,
    tabs,
    isLoading,
    error,
    refetch,
    enablePlugin,
    disablePlugin,
    isEnabling: enableMutation.isPending,
    isDisabling: disableMutation.isPending,
  }
}
