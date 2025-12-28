import { useMemo } from 'react'
import {
  useGetApiSettingsAi,
  usePutApiSettingsAi,
  useGetApiSettingsAiProviders,
} from '@/lib/api/generated/settings/settings'
import { useGetApiPlugins } from '@/lib/api/generated/plugins/plugins'
import type { PluginMeta, ProviderOption } from '../AISettings.types'

/**
 * Hook for fetching and managing AI settings data.
 * Handles settings, providers, and plugin metadata.
 */
export const useAISettingsData = () => {
  const {
    data: settingsData,
    isLoading: settingsLoading,
    error: settingsError,
    refetch: refetchSettings,
  } = useGetApiSettingsAi()

  const { data: providersData, isLoading: providersLoading } = useGetApiSettingsAiProviders()
  const { data: pluginsData, isLoading: pluginsLoading } = useGetApiPlugins()

  const updateSettings = usePutApiSettingsAi()

  // Extract providers from response
  const providers = providersData?.status === 200 ? providersData.data.providers || [] : []

  // Map provider_id -> plugin id
  const providerPluginMap = useMemo(() => {
    const plugins = pluginsData?.status === 200 ? pluginsData.data.plugins || [] : []
    const map: Record<string, string> = {}
    for (const plugin of plugins) {
      if (plugin.provider_id && plugin.id) {
        map[plugin.provider_id] = plugin.id
      }
    }
    return map
  }, [pluginsData])

  // Map provider_id -> plugin meta
  const providerMetaMap = useMemo(() => {
    const plugins = pluginsData?.status === 200 ? pluginsData.data.plugins || [] : []
    const map: Record<string, PluginMeta> = {}
    for (const plugin of plugins) {
      if (plugin.provider_id && plugin.meta) {
        map[plugin.provider_id] = plugin.meta as PluginMeta
      }
    }
    return map
  }, [pluginsData])

  // Filter providers by capability
  const embeddingProviders = providers.filter((p) => p.supportsEmbedding)
  const chatProviders = providers.filter((p) => p.supportsChat)

  // Build select options
  const embeddingProviderOptions: ProviderOption[] = embeddingProviders.map((p) => {
    const meta = providerMetaMap[String(p.type)]
    return {
      value: String(p.type),
      label: meta?.displayName || p.name || String(p.type),
    }
  })

  const chatProviderOptions: ProviderOption[] = chatProviders.map((p) => {
    const meta = providerMetaMap[String(p.type)]
    return {
      value: String(p.type),
      label: meta?.displayName || p.name || String(p.type),
    }
  })

  // Get current settings values
  const currentSettings =
    settingsData?.status === 200
      ? {
          enabled: settingsData.data.enabled ?? false,
          embeddingProvider: settingsData.data.embeddingProvider ?? '',
          chatProvider: settingsData.data.chatProvider ?? '',
        }
      : null

  // Helper to find provider by type
  const getEmbeddingProvider = (providerValue: string) =>
    embeddingProviders.find((p) => String(p.type) === providerValue)

  const getChatProvider = (providerValue: string) =>
    chatProviders.find((p) => String(p.type) === providerValue)

  return {
    // Loading states
    isLoading: settingsLoading || providersLoading || pluginsLoading,
    error: settingsError,

    // Current settings
    currentSettings,

    // Providers
    embeddingProviders,
    chatProviders,
    embeddingProviderOptions,
    chatProviderOptions,

    // Maps
    providerPluginMap,
    providerMetaMap,

    // Helpers
    getEmbeddingProvider,
    getChatProvider,

    // Mutations
    updateSettings,
    refetchSettings,
  }
}
