import { useEffect } from 'react'
import { Alert, Loading } from '@/components/ui'
import { FormSettingsFooter } from '@/components/ui/Form'
import { FormToggle, FormSelect } from '@/components/ui/Form'
import { PageHeader } from '@/components/common'
import { useToast } from '@/lib/hooks/useToast'
import { useSettingsForm } from '@/lib/hooks'
import {
  AIFeaturesCard,
  EmbeddingProviderCard,
  ChatProviderCard,
  SearchSettingsCard,
} from './components'
import { useAISettingsData } from './hooks'
import { AI_SETTINGS_DEFAULT_VALUES } from './AISettings.schema'

/**
 * AI Settings page component.
 * Manages AI features, embedding/chat providers, and search settings.
 * Note: Admin access is enforced by AdminRoute wrapper in the route file.
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

  const { form, hasChanges, saveCategory, discardCategory, isSaving, resetOriginalValues } =
    useSettingsForm({
      defaultValues: AI_SETTINGS_DEFAULT_VALUES as Record<string, unknown>,
      categories: [
        {
          id: 'ai',
          fields: ['enabled', 'embeddingProvider', 'chatProvider'],
          onSave: async (values) => {
            const response = await updateSettings.mutateAsync({ data: values })
            if (response.status === 200) {
              toast.success('AI settings saved successfully')
              refetchSettings()
            }
          },
        },
      ],
    })

  // Initialize form with API data
  useEffect(() => {
    if (currentSettings) {
      resetOriginalValues(currentSettings)
    }
  }, [currentSettings, resetOriginalValues])

  if (isLoading) {
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

  const handleSave = async () => {
    try {
      await saveCategory('ai')
    } catch {
      toast.error('Failed to save AI settings')
    }
  }

  const handleDiscard = () => {
    discardCategory('ai')
    toast.info('Changes discarded')
  }

  const aiHasChanges = hasChanges('ai')
  const aiIsSaving = isSaving('ai')

  return (
    <div className="h-full overflow-auto">
      <div className="p-8 page-enter">
        <PageHeader
          title="AI Settings"
          description="Configure AI-powered features like semantic search and content analysis"
        />

        <div className="mt-6 space-y-6 max-w-3xl">
          {/* Enable AI Card */}
          <AIFeaturesCard>
            <form.Field name="enabled">
              {(field) => (
                <FormToggle
                  field={field}
                  label="Enable AI Features"
                  description="When enabled, media content will be analyzed and indexed for semantic search capabilities."
                />
              )}
            </form.Field>
          </AIFeaturesCard>

          {/* Provider cards - conditionally rendered based on enabled state */}
          <form.Subscribe
            selector={
              (state) =>
                [state.values.enabled, state.values.embeddingProvider, state.values.chatProvider] as const
            }
          >
            {([enabled, embeddingProvider, chatProvider]) => {
              if (!enabled) {
                return null
              }

              const selectedEmbeddingProvider = getEmbeddingProvider(embeddingProvider as string)
              const selectedChatProvider = getChatProvider(chatProvider as string)

              return (
                <>
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
                    <form.Field name="embeddingProvider">
                      {(field) => (
                        <FormSelect field={field} label="Provider" options={embeddingProviderOptions} />
                      )}
                    </form.Field>
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
                    <form.Field name="chatProvider">
                      {(field) => (
                        <FormSelect field={field} label="Provider" options={chatProviderOptions} />
                      )}
                    </form.Field>
                  </ChatProviderCard>

                  {/* Search Settings - from ai-search plugin */}
                  <SearchSettingsCard />
                </>
              )
            }}
          </form.Subscribe>

          {/* Save/Discard Footer */}
          <FormSettingsFooter
            hasChanges={aiHasChanges}
            isSaving={aiIsSaving}
            onSave={handleSave}
            onDiscard={handleDiscard}
          />
        </div>
      </div>
    </div>
  )
}
