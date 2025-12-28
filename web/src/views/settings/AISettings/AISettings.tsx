import { Alert, Loading } from '@/components/ui'
import { SettingsPage } from '@/components/common'
import { SettingRow } from '@/components/settings/ui'
import { useToast } from '@/lib/hooks/useToast'
import {
  EmbeddingProviderCard,
  ChatProviderCard,
  SearchSettingsCard,
} from './components'
import { useAISettingsData } from './hooks'
import { Sparkles } from 'lucide-react'
import { cn } from '@/lib/utils'

/**
 * AI Settings page component.
 * Manages AI features, embedding/chat providers, and search settings.
 * Auto-saves on field change.
 */
export const AISettings = () => {
  const toast = useToast()

  const {
    isLoading,
    error,
    currentSettings,
    embeddingProviderOptions,
    chatProviderOptions,
    providerPluginMap,
    providerMetaMap,
    getEmbeddingProvider,
    getChatProvider,
    updateSettings,
    refetchSettings,
  } = useAISettingsData()

  // Auto-save a single setting
  const saveSetting = async (key: string, value: unknown) => {
    try {
      const response = await updateSettings.mutateAsync({
        data: { ...currentSettings, [key]: value },
      })
      if (response.status === 200) {
        toast.success('Setting saved')
        refetchSettings()
      }
    } catch {
      toast.error('Failed to save setting')
    }
  }

  if (isLoading) {
    return (
      <SettingsPage>
        <SettingsPage.Header
          title="AI Settings"
          description="Configure AI-powered features like semantic search and content analysis"
        />
        <SettingsPage.Card>
          <Loading text="Loading AI settings..." />
        </SettingsPage.Card>
      </SettingsPage>
    )
  }

  if (error) {
    return (
      <SettingsPage>
        <SettingsPage.Header
          title="AI Settings"
          description="Configure AI-powered features like semantic search and content analysis"
        />
        <SettingsPage.Card>
          <Alert variant="error">Failed to load AI settings. Please try again later.</Alert>
        </SettingsPage.Card>
      </SettingsPage>
    )
  }

  const enabled = currentSettings?.enabled === true
  const embeddingProvider = String(currentSettings?.embeddingProvider ?? '')
  const chatProvider = String(currentSettings?.chatProvider ?? '')

  const selectedEmbeddingProvider = getEmbeddingProvider(embeddingProvider)
  const selectedChatProvider = getChatProvider(chatProvider)

  return (
    <SettingsPage>
      <SettingsPage.Header
        title="AI Settings"
        description="Configure AI-powered features like semantic search and content analysis"
      />

      {/* AI Features Info Banner */}
      <div
        className={cn(
          'flex items-start gap-3 p-4 rounded-xl mb-6',
          'bg-primary-50/50 dark:bg-primary-500/10',
          'border border-primary-200/50 dark:border-primary-500/20'
        )}
      >
        <Sparkles className="w-5 h-5 text-primary-500 shrink-0 mt-0.5" />
        <div>
          <p className="text-sm font-medium text-primary-700 dark:text-primary-300">
            About AI Features
          </p>
          <p className="text-xs mt-1 text-primary-600/80 dark:text-primary-400/80">
            ViewRA uses AI to provide intelligent recommendations, semantic search, and
            personalized collections. All processing happens securely and your data remains
            private.
          </p>
        </div>
      </div>

      {/* AI Enable Toggle */}
      <SettingsPage.Card className="mb-6">
        <SettingRow
          type="toggle"
          label="Enable AI Features"
          description="When enabled, media content will be analyzed and indexed for semantic search capabilities"
          value={enabled}
          onChange={(newValue) => saveSetting('enabled', newValue)}
        />
      </SettingsPage.Card>

      {/* Provider cards - conditionally rendered based on enabled state */}
      {enabled && (
        <div className="space-y-6">
          {/* Embedding Provider */}
          <EmbeddingProviderCard
            options={embeddingProviderOptions}
            selectedProvider={selectedEmbeddingProvider}
            pluginId={
              selectedEmbeddingProvider
                ? providerPluginMap[String(selectedEmbeddingProvider.type)]
                : undefined
            }
            meta={
              selectedEmbeddingProvider
                ? providerMetaMap[String(selectedEmbeddingProvider.type)]
                : undefined
            }
          >
            <SettingRow
              type="select"
              label="Provider"
              description="Select the provider for generating text embeddings"
              value={embeddingProvider}
              onChange={(newValue) => saveSetting('embeddingProvider', newValue)}
              options={embeddingProviderOptions}
            />
          </EmbeddingProviderCard>

          {/* Chat Provider */}
          <ChatProviderCard
            options={chatProviderOptions}
            selectedProvider={selectedChatProvider}
            pluginId={
              selectedChatProvider
                ? providerPluginMap[String(selectedChatProvider.type)]
                : undefined
            }
            meta={
              selectedChatProvider
                ? providerMetaMap[String(selectedChatProvider.type)]
                : undefined
            }
          >
            <SettingRow
              type="select"
              label="Provider"
              description="Select the provider for AI chat and recommendations"
              value={chatProvider}
              onChange={(newValue) => saveSetting('chatProvider', newValue)}
              options={chatProviderOptions}
            />
          </ChatProviderCard>

          {/* Search Settings - from ai-search plugin */}
          <SearchSettingsCard />
        </div>
      )}
    </SettingsPage>
  )
}
