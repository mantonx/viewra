import { createFileRoute } from '@tanstack/react-router'
import { useState, useEffect, useMemo } from 'react'
import { useAuth } from '@/contexts'
import {
  Card,
  CardHeader,
  CardContent,
  Button,
  Select,
  Alert,
  Loading,
  SettingToggle,
  Tooltip,
} from '@/components/ui'
import { PageHeader } from '@/components/common'
import { useToast } from '@/lib/hooks/useToast'
import { text } from '@/styles/semantic'
import { cn, getIcon } from '@/lib/utils'
import { Check, X, Sparkles, Settings2, MessageSquare, Cpu, HardDrive } from 'lucide-react'
import {
  useGetApiSettingsAi,
  usePutApiSettingsAi,
  useGetApiSettingsAiProviders,
} from '@/lib/api/generated/settings/settings'
import { useGetApiPlugins } from '@/lib/api/generated/plugins/plugins'
import type {
  InternalApiHandlersAISettingsRequest,
  GithubComMantonxViewraInternalDomainAiProviderInfo as ProviderInfo,
} from '@/lib/api/generated/models'
import { PluginSettingsForm } from '@/components/settings/PluginSettingsForm'

type PluginMeta = {
  displayName?: string
  description?: string
  tip?: string
  isLocal?: boolean
  icon?: string
}

const LocalBadge = () => (
  <span
    className={cn(
      'inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-[10px] font-medium',
      'bg-emerald-100 dark:bg-emerald-900/50 text-emerald-700 dark:text-emerald-400'
    )}
  >
    <HardDrive className="w-2.5 h-2.5" />
    Local
  </span>
)

type ProviderCardProps = {
  provider: ProviderInfo
  pluginId?: string
  meta?: PluginMeta
  /** Show only sections for this capability */
  capability?: 'embedding' | 'chat'
}

const ProviderCard = ({ provider, pluginId, meta, capability }: ProviderCardProps) => {
  const displayName = meta?.displayName || provider.name
  const description = meta?.description || provider.description
  const isLocal = meta?.isLocal ?? false
  const tip = meta?.tip
  const Icon = getIcon(meta?.icon)

  return (
    <div className="space-y-4 pt-4 border-t border-neutral-100 dark:border-neutral-800">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <Icon className={cn('w-4 h-4', text.secondary)} />
          <span className={cn('text-sm font-medium', text.primary)}>{displayName}</span>
          {isLocal && <LocalBadge />}
          {tip && <Tooltip content={tip} />}
        </div>
      </div>
      {description && <p className={cn('text-xs', text.secondary)}>{description}</p>}
      {pluginId && <PluginSettingsForm pluginId={pluginId} capability={capability} className="mt-4" />}
    </div>
  )
}

const AISettings = () => {
  const { user } = useAuth()
  const toast = useToast()

  const { data: settingsData, isLoading, error, refetch: refetchSettings } = useGetApiSettingsAi()
  const { data: providersData, isLoading: providersLoading } = useGetApiSettingsAiProviders()
  const { data: pluginsData, isLoading: pluginsLoading } = useGetApiPlugins()

  const updateSettings = usePutApiSettingsAi()

  const [formValues, setFormValues] = useState<InternalApiHandlersAISettingsRequest>({})
  const [hasChanges, setHasChanges] = useState(false)

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

  const embeddingProviders = providers.filter((p) => p.supportsEmbedding)
  const chatProviders = providers.filter((p) => p.supportsChat)

  const embeddingProviderOptions = embeddingProviders.map((p) => {
    const meta = providerMetaMap[String(p.type)]
    return {
      value: String(p.type),
      label: meta?.displayName || p.name || String(p.type),
    }
  })

  const chatProviderOptions = chatProviders.map((p) => {
    const meta = providerMetaMap[String(p.type)]
    return {
      value: String(p.type),
      label: meta?.displayName || p.name || String(p.type),
    }
  })

  useEffect(() => {
    if (settingsData?.status === 200) {
      const s = settingsData.data
      setFormValues({
        enabled: s.enabled,
        embeddingProvider: s.embeddingProvider,
        chatProvider: s.chatProvider,
      })
    }
  }, [settingsData])

  if (!user?.is_admin) {
    return (
      <div className="h-full overflow-auto">
        <div className="p-8 page-enter">
          <Alert variant="error">You must be an administrator to access AI settings.</Alert>
        </div>
      </div>
    )
  }

  if (isLoading || providersLoading || pluginsLoading) {
    return (
      <div className="h-full overflow-auto">
        <div className="p-8 page-enter">
          <Loading text="Loading AI settings..." />
        </div>
      </div>
    )
  }

  if (error) {
    return (
      <div className="h-full overflow-auto">
        <div className="p-8 page-enter">
          <Alert variant="error">Failed to load AI settings. Please try again later.</Alert>
        </div>
      </div>
    )
  }

  const selectedEmbeddingProvider = embeddingProviders.find(
    (p) => String(p.type) === formValues.embeddingProvider
  )
  const selectedChatProvider = chatProviders.find(
    (p) => String(p.type) === formValues.chatProvider
  )

  const handleChange = <K extends keyof InternalApiHandlersAISettingsRequest>(
    key: K,
    value: InternalApiHandlersAISettingsRequest[K]
  ) => {
    setFormValues((prev) => ({ ...prev, [key]: value }))
    setHasChanges(true)
  }

  const handleSave = async () => {
    try {
      const response = await updateSettings.mutateAsync({ data: formValues })
      if (response.status === 200) {
        toast.success('AI settings saved successfully')
        setHasChanges(false)
        refetchSettings()
      }
    } catch {
      toast.error('Failed to save AI settings')
    }
  }

  const handleDiscard = () => {
    if (settingsData?.status === 200) {
      const s = settingsData.data
      setFormValues({
        enabled: s.enabled,
        embeddingProvider: s.embeddingProvider,
        chatProvider: s.chatProvider,
      })
      setHasChanges(false)
      toast.info('Changes discarded')
    }
  }

  return (
    <div className="h-full overflow-auto">
      <div className="p-8 page-enter">
        <PageHeader
          title="AI Settings"
          description="Configure AI-powered features like semantic search and content analysis"
        />

        <div className="mt-6 space-y-6 max-w-3xl">
          {/* Enable AI Card */}
          <Card>
            <CardHeader className="border-b border-neutral-100 dark:border-neutral-800">
              <div className="flex items-center gap-3">
                <div
                  className={cn(
                    'p-2 rounded-lg',
                    'bg-gradient-to-br from-violet-500 to-purple-600',
                    'text-white shadow-lg shadow-violet-500/25'
                  )}
                >
                  <Sparkles className="w-5 h-5" />
                </div>
                <div>
                  <h2 className={cn('text-lg font-semibold', text.primary)}>AI Features</h2>
                  <p className={cn('text-sm mt-0.5', text.secondary)}>
                    Enable AI-powered semantic search and recommendations
                  </p>
                </div>
              </div>
            </CardHeader>
            <CardContent>
              <SettingToggle
                enabled={formValues.enabled ?? false}
                onChange={(value) => handleChange('enabled', value)}
                label="Enable AI Features"
                description="When enabled, media content will be analyzed and indexed for semantic search capabilities."
                ariaLabel="Enable AI features"
              />
            </CardContent>
          </Card>

          {formValues.enabled && (
            <>
              {/* Embedding Provider */}
              <Card>
                <CardHeader className="border-b border-neutral-100 dark:border-neutral-800">
                  <div className="flex items-center gap-3">
                    <div
                      className={cn(
                        'p-2 rounded-lg',
                        'bg-primary-50 dark:bg-primary-950/50',
                        'text-primary-600 dark:text-primary-400'
                      )}
                    >
                      <Cpu className="w-5 h-5" />
                    </div>
                    <div>
                      <h2 className={cn('text-lg font-semibold', text.primary)}>
                        Embedding Provider
                      </h2>
                      <p className={cn('text-sm mt-0.5', text.secondary)}>
                        Provider for generating text embeddings (semantic search)
                      </p>
                    </div>
                  </div>
                </CardHeader>
                <CardContent className="space-y-4">
                  {embeddingProviderOptions.length > 0 ? (
                    <>
                      <div>
                        <label className={cn('block text-sm font-medium mb-2', text.primary)}>
                          Provider
                        </label>
                        <Select
                          value={formValues.embeddingProvider ?? ''}
                          onChange={(e) => handleChange('embeddingProvider', e.target.value)}
                          options={embeddingProviderOptions}
                        />
                      </div>
                      {selectedEmbeddingProvider && (
                        <ProviderCard
                          provider={selectedEmbeddingProvider}
                          pluginId={providerPluginMap[String(selectedEmbeddingProvider.type)]}
                          meta={providerMetaMap[String(selectedEmbeddingProvider.type)]}
                          capability="embedding"
                        />
                      )}
                    </>
                  ) : (
                    <Alert variant="warning">
                      No embedding providers available. Make sure provider plugins are installed and
                      running.
                    </Alert>
                  )}
                </CardContent>
              </Card>

              {/* Chat Provider */}
              <Card>
                <CardHeader className="border-b border-neutral-100 dark:border-neutral-800">
                  <div className="flex items-center gap-3">
                    <div
                      className={cn(
                        'p-2 rounded-lg',
                        'bg-primary-50 dark:bg-primary-950/50',
                        'text-primary-600 dark:text-primary-400'
                      )}
                    >
                      <MessageSquare className="w-5 h-5" />
                    </div>
                    <div>
                      <h2 className={cn('text-lg font-semibold', text.primary)}>Chat Provider</h2>
                      <p className={cn('text-sm mt-0.5', text.secondary)}>
                        Provider for AI chat completions (mood tags, analysis)
                      </p>
                    </div>
                  </div>
                </CardHeader>
                <CardContent className="space-y-4">
                  {chatProviderOptions.length > 0 ? (
                    <>
                      <div>
                        <label className={cn('block text-sm font-medium mb-2', text.primary)}>
                          Provider
                        </label>
                        <Select
                          value={formValues.chatProvider ?? ''}
                          onChange={(e) => handleChange('chatProvider', e.target.value)}
                          options={chatProviderOptions}
                        />
                      </div>
                      {/* Show ProviderCard with chat models */}
                      {selectedChatProvider && (
                        <ProviderCard
                          provider={selectedChatProvider}
                          pluginId={providerPluginMap[String(selectedChatProvider.type)]}
                          meta={providerMetaMap[String(selectedChatProvider.type)]}
                          capability="chat"
                        />
                      )}
                    </>
                  ) : (
                    <Alert variant="warning">
                      No chat providers available. Make sure provider plugins are installed and
                      running.
                    </Alert>
                  )}
                </CardContent>
              </Card>

              {/* Search Settings - from ai-search plugin */}
              <Card>
                <CardHeader className="border-b border-neutral-100 dark:border-neutral-800">
                  <div className="flex items-center gap-3">
                    <div
                      className={cn(
                        'p-2 rounded-lg',
                        'bg-primary-50 dark:bg-primary-950/50',
                        'text-primary-600 dark:text-primary-400'
                      )}
                    >
                      <Settings2 className="w-5 h-5" />
                    </div>
                    <div>
                      <h2 className={cn('text-lg font-semibold', text.primary)}>Search Settings</h2>
                      <p className={cn('text-sm mt-0.5', text.secondary)}>
                        Configure indexing, search, and mood tag behavior
                      </p>
                    </div>
                  </div>
                </CardHeader>
                <CardContent>
                  <PluginSettingsForm pluginId="ai-search" />
                </CardContent>
              </Card>
            </>
          )}

          {/* Save/Discard Footer */}
          {hasChanges && (
            <div
              className={cn(
                'sticky bottom-4 flex items-center justify-end gap-3 p-4 rounded-xl',
                'bg-white dark:bg-neutral-900',
                'border border-neutral-200 dark:border-neutral-700',
                'shadow-lg'
              )}
            >
              <span className={cn('text-sm mr-auto', text.secondary)}>You have unsaved changes</span>
              <Button variant="ghost" onClick={handleDiscard}>
                <X className="w-4 h-4 mr-1" />
                Discard
              </Button>
              <Button onClick={handleSave} isLoading={updateSettings.isPending}>
                <Check className="w-4 h-4 mr-1" />
                Save Changes
              </Button>
            </div>
          )}
        </div>
      </div>
    </div>
  )
}

export const Route = createFileRoute('/_layout/settings/ai')({
  component: AISettings,
})
