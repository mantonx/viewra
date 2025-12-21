import { createFileRoute } from '@tanstack/react-router'
import { useState, useEffect } from 'react'
import { useAuth } from '@/contexts'
import {
  Card,
  CardHeader,
  CardContent,
  Button,
  Input,
  Select,
  Alert,
  Loading,
  SettingToggle,
  Modal,
  ModalContent,
  ModalFooter,
} from '@/components/ui'
import { PageHeader } from '@/components/common'
import { useToast } from '@/lib/hooks/useToast'
import { text } from '@/styles/semantic'
import { cn } from '@/lib/utils'
import {
  Server,
  Key,
  RefreshCw,
  Check,
  X,
  Download,
  Trash2,
  AlertCircle,
  CheckCircle,
  Loader2,
  Sparkles,
  Settings2,
  MessageSquare,
  Cpu,
  Eye,
  EyeOff,
  Info,
  Zap,
  Globe,
  HardDrive,
} from 'lucide-react'
import {
  useGetApiSettingsAi,
  usePutApiSettingsAi,
  useGetApiSettingsAiProviders,
  useGetApiSettingsAiProvidersProviderModels,
  usePostApiSettingsAiProvidersProviderTest,
  useDeleteApiSettingsAiModels,
  useGetApiSettingsAiModelsRecommended,
  useGetApiSettingsAiModelsRecommendedChat,
} from '@/lib/api/generated/settings/settings'
import type {
  InternalApiHandlersAISettingsRequest,
  GithubComMantonxViewraInternalApplicationSettingsRecommendedModel as RecommendedModel,
} from '@/lib/api/generated/models'

// Provider metadata with descriptions and icons
const providerMeta: Record<
  string,
  {
    name: string
    description: string
    icon: typeof Server
    isLocal?: boolean
    tip?: string
  }
> = {
  ollama: {
    name: 'Ollama',
    description: 'Local inference, no external API calls',
    icon: HardDrive,
    isLocal: true,
    tip: 'Runs on your hardware. Needs a GPU with enough VRAM for the model you choose.',
  },
  openai: {
    name: 'OpenAI',
    description: 'GPT-4o, GPT-4o-mini, text-embedding-3',
    icon: Zap,
    tip: 'Embeddings: ~$0.02/1M tokens. Chat: varies by model.',
  },
  anthropic: {
    name: 'Anthropic',
    description: 'Claude 3.5 Sonnet, Claude 3 Haiku',
    icon: MessageSquare,
    tip: 'Haiku is fast and cheap. Sonnet for more complex tasks.',
  },
  voyage: {
    name: 'Voyage AI',
    description: 'Embedding-focused, good retrieval quality',
    icon: Globe,
    tip: 'Optimized for search and retrieval use cases.',
  },
  openrouter: {
    name: 'OpenRouter',
    description: 'Proxy to many providers with unified billing',
    icon: Globe,
    tip: 'Access models from OpenAI, Anthropic, Google, and others through one API key.',
  },
}

// Local badge component
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

// Provider options for embedding (subset that supports embeddings)
const embeddingProviderOptions = [
  { value: 'ollama', label: 'Ollama (Local)' },
  { value: 'openai', label: 'OpenAI' },
  { value: 'voyage', label: 'Voyage AI' },
]

// Provider options for chat
const chatProviderOptions = [
  { value: 'ollama', label: 'Ollama (Local)' },
  { value: 'openai', label: 'OpenAI' },
  { value: 'anthropic', label: 'Anthropic' },
  { value: 'openrouter', label: 'OpenRouter' },
]



// Connection status badge
const ConnectionStatus = ({
  available,
  checking,
}: {
  available: boolean | undefined
  checking: boolean
}) => {
  if (checking) {
    return (
      <span
        className={cn(
          'inline-flex items-center gap-1.5 px-2 py-1 rounded-full text-xs font-medium',
          'bg-neutral-100 dark:bg-neutral-800 text-neutral-600 dark:text-neutral-400'
        )}
      >
        <Loader2 className="w-3 h-3 animate-spin" />
        Checking...
      </span>
    )
  }

  if (available === undefined) {
    return null
  }

  return available ? (
    <span
      className={cn(
        'inline-flex items-center gap-1.5 px-2 py-1 rounded-full text-xs font-medium',
        'bg-emerald-100 dark:bg-emerald-900/50 text-emerald-700 dark:text-emerald-400'
      )}
    >
      <CheckCircle className="w-3 h-3" />
      Connected
    </span>
  ) : (
    <span
      className={cn(
        'inline-flex items-center gap-1.5 px-2 py-1 rounded-full text-xs font-medium',
        'bg-red-100 dark:bg-red-900/50 text-red-700 dark:text-red-400'
      )}
    >
      <AlertCircle className="w-3 h-3" />
      Unavailable
    </span>
  )
}

// API Key input with visibility toggle
const ApiKeyInput = ({
  value,
  onChange,
  placeholder,
}: {
  value: string
  onChange: (value: string) => void
  placeholder: string
}) => {
  const [visible, setVisible] = useState(false)

  return (
    <div className="relative">
      <Input
        type={visible ? 'text' : 'password'}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder={placeholder}
        className="font-mono pr-10"
      />
      <button
        type="button"
        onClick={() => setVisible(!visible)}
        className={cn(
          'absolute right-2 top-1/2 -translate-y-1/2 p-1.5 rounded-md',
          'hover:bg-neutral-100 dark:hover:bg-neutral-800',
          'text-neutral-500 hover:text-neutral-700 dark:hover:text-neutral-300',
          'transition-colors'
        )}
        aria-label={visible ? 'Hide API key' : 'Show API key'}
      >
        {visible ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
      </button>
    </div>
  )
}

// Model list item for Ollama model management
const ModelItem = ({
  model,
  isInstalled,
  onPull,
  onDelete,
  pullProgress,
}: {
  model: RecommendedModel
  isInstalled: boolean
  onPull: () => void
  onDelete: () => void
  pullProgress?: { status: string; percent?: number } | null
}) => (
  <div
    className={cn(
      'py-3 px-3 rounded-lg',
      'bg-neutral-50 dark:bg-neutral-900/50',
      model.recommended && !isInstalled && 'ring-1 ring-primary-200 dark:ring-primary-800'
    )}
  >
    <div className="flex items-start justify-between gap-3">
      <div className="flex-1 min-w-0">
        <div className="flex items-center gap-2 flex-wrap">
          <span className={cn('text-sm font-medium', text.primary)}>{model.name}</span>
          <span className={cn('text-xs', text.tertiary)}>{model.size}</span>
          {isInstalled && (
            <span
              className={cn(
                'px-1.5 py-0.5 rounded text-[10px] font-medium',
                'bg-emerald-100 dark:bg-emerald-900/50 text-emerald-700 dark:text-emerald-400'
              )}
            >
              Installed
            </span>
          )}
          {model.recommended && !isInstalled && (
            <span
              className={cn(
                'px-1.5 py-0.5 rounded text-[10px] font-medium',
                'bg-primary-100 dark:bg-primary-900/50 text-primary-700 dark:text-primary-400'
              )}
            >
              Recommended
            </span>
          )}
        </div>
        <p className={cn('text-xs mt-1', text.secondary)}>{model.description}</p>
      </div>
      <div className="flex items-center gap-2 flex-shrink-0">
        {pullProgress ? (
          <div className="flex items-center gap-2">
            <div className="w-20 h-1.5 bg-neutral-200 dark:bg-neutral-700 rounded-full overflow-hidden">
              <div
                className="h-full bg-primary-500 transition-all duration-300"
                style={{ width: `${pullProgress.percent ?? 0}%` }}
              />
            </div>
            <span className={cn('text-xs tabular-nums w-10', text.secondary)}>
              {pullProgress.percent !== undefined ? `${Math.round(pullProgress.percent)}%` : '...'}
            </span>
          </div>
        ) : isInstalled ? (
          <Button variant="ghost" size="sm" onClick={onDelete} className="text-red-500">
            <Trash2 className="w-3.5 h-3.5" />
          </Button>
        ) : (
          <Button variant="ghost" size="sm" onClick={onPull}>
            <Download className="w-3.5 h-3.5 mr-1" />
            Pull
          </Button>
        )}
      </div>
    </div>
  </div>
)

// Info tip component
const ProviderTip = ({ provider }: { provider: string }) => {
  const meta = providerMeta[provider]
  if (!meta?.tip) {
    return null
  }

  return (
    <div
      className={cn(
        'flex items-start gap-2 p-3 rounded-lg mt-3',
        'bg-blue-50 dark:bg-blue-950/30',
        'border border-blue-100 dark:border-blue-900/50'
      )}
    >
      <Info className="w-4 h-4 text-blue-500 mt-0.5 flex-shrink-0" />
      <p className={cn('text-xs', 'text-blue-700 dark:text-blue-300')}>{meta.tip}</p>
    </div>
  )
}

// Empty models state
const EmptyModelsState = ({ provider, onTest }: { provider: string; onTest: () => void }) => (
  <div
    className={cn(
      'flex flex-col items-center justify-center py-4 px-3 rounded-lg',
      'bg-neutral-50 dark:bg-neutral-900/50',
      'border border-dashed border-neutral-200 dark:border-neutral-700'
    )}
  >
    <AlertCircle className={cn('w-5 h-5 mb-2', text.tertiary)} />
    <p className={cn('text-sm text-center', text.secondary)}>No models available</p>
    <p className={cn('text-xs text-center mt-1', text.tertiary)}>
      {provider === 'ollama'
        ? 'Check that Ollama is running and has models installed'
        : 'Test the connection to load available models'}
    </p>
    <Button variant="secondary" size="sm" className="mt-3" onClick={onTest}>
      <RefreshCw className="w-3.5 h-3.5 mr-1" />
      Test Connection
    </Button>
  </div>
)

// Provider configuration card
const ProviderCard = ({
  title,
  icon: Icon,
  connectionStatus,
  isTesting,
  onTest,
  children,
}: {
  title: string
  icon: typeof Server
  connectionStatus?: boolean
  isTesting: boolean
  onTest: () => void
  children: React.ReactNode
}) => (
  <div className="space-y-4 pt-4 border-t border-neutral-100 dark:border-neutral-800">
    <div className="flex items-center justify-between">
      <div className="flex items-center gap-2">
        <Icon className={cn('w-4 h-4', text.secondary)} />
        <span className={cn('text-sm font-medium', text.primary)}>{title}</span>
      </div>
      <div className="flex items-center gap-2">
        <ConnectionStatus available={connectionStatus} checking={isTesting} />
        <Button variant="secondary" size="sm" onClick={onTest} disabled={isTesting}>
          {isTesting ? (
            <Loader2 className="w-4 h-4 animate-spin" />
          ) : (
            <RefreshCw className="w-4 h-4" />
          )}
        </Button>
      </div>
    </div>
    {children}
  </div>
)

// Provider selector with description
const ProviderSelect = ({
  value,
  onChange,
  options,
  label,
}: {
  value: string
  onChange: (value: string) => void
  options: Array<{ value: string; label: string }>
  label: string
}) => {
  const selectedMeta = providerMeta[value]

  return (
    <div className="space-y-3">
      <div>
        <label className={cn('block text-sm font-medium mb-2', text.primary)}>{label}</label>
        <Select
          value={value}
          onChange={(e) => onChange(e.target.value)}
          options={options}
        />
      </div>
      {selectedMeta && (
        <div className="flex items-center gap-2 pl-1">
          <span className={cn('text-xs', text.secondary)}>{selectedMeta.description}</span>
          {selectedMeta.isLocal && <LocalBadge />}
        </div>
      )}
    </div>
  )
}

const AISettings = () => {
  const { user } = useAuth()
  const toast = useToast()

  // Fetch current AI settings
  const { data: settingsData, isLoading, error, refetch: refetchSettings } = useGetApiSettingsAi()

  // Fetch available providers (hook result used to trigger data fetch)
  useGetApiSettingsAiProviders()

  // Mutations
  const updateSettings = usePutApiSettingsAi()
  const testProvider = usePostApiSettingsAiProvidersProviderTest()
  const deleteModel = useDeleteApiSettingsAiModels()

  // Local state for form
  const [formValues, setFormValues] = useState<InternalApiHandlersAISettingsRequest>({})
  const [hasChanges, setHasChanges] = useState(false)
  const [testingProvider, setTestingProvider] = useState<string | null>(null)
  const [providerStatus, setProviderStatus] = useState<Record<string, boolean>>({})
  const [pullProgress, setPullProgress] = useState<Record<string, { status: string; percent?: number }>>({})
  const [deleteConfirm, setDeleteConfirm] = useState<string | null>(null)

  // Fetch recommended models for Ollama (includes system info)
  // Check both formValues (user changes) and settingsData (initial load) for ollama provider
  const initialEmbeddingProvider = settingsData?.status === 200 ? settingsData.data.embeddingProvider : undefined
  const initialChatProvider = settingsData?.status === 200 ? settingsData.data.chatProvider : undefined
  const isOllamaEmbedding =
    formValues.embeddingProvider === 'ollama' ||
    (formValues.embeddingProvider === undefined && initialEmbeddingProvider === 'ollama')
  const isOllamaChat =
    formValues.chatProvider === 'ollama' ||
    (formValues.chatProvider === undefined && initialChatProvider === 'ollama')
  const { data: recommendedModelsData } = useGetApiSettingsAiModelsRecommended({
    query: { enabled: isOllamaEmbedding },
  })
  const { data: recommendedChatModelsData } = useGetApiSettingsAiModelsRecommendedChat({
    query: { enabled: isOllamaChat },
  })

  // Dynamic model fetching for each provider
  const { data: ollamaModelsData, refetch: refetchOllamaModels } =
    useGetApiSettingsAiProvidersProviderModels('ollama', {
      query: { enabled: formValues.embeddingProvider === 'ollama' || formValues.chatProvider === 'ollama' },
    })
  const { data: openaiModelsData } = useGetApiSettingsAiProvidersProviderModels('openai', {
    query: { enabled: formValues.embeddingProvider === 'openai' || formValues.chatProvider === 'openai' },
  })
  const { data: anthropicModelsData } = useGetApiSettingsAiProvidersProviderModels('anthropic', {
    query: { enabled: formValues.chatProvider === 'anthropic' },
  })
  const { data: voyageModelsData } = useGetApiSettingsAiProvidersProviderModels('voyage', {
    query: { enabled: formValues.embeddingProvider === 'voyage' },
  })
  const { data: openrouterModelsData } = useGetApiSettingsAiProvidersProviderModels('openrouter', {
    query: { enabled: formValues.chatProvider === 'openrouter' },
  })

  // Initialize form with fetched data
  useEffect(() => {
    if (settingsData?.status === 200) {
      const s = settingsData.data
      setFormValues({
        enabled: s.enabled,
        embeddingProvider: s.embeddingProvider,
        chatProvider: s.chatProvider,
        ollamaUrl: s.ollamaUrl,
        ollamaEmbeddingModel: s.ollamaEmbeddingModel,
        ollamaChatModel: s.ollamaChatModel,
        openaiEmbeddingModel: s.openaiEmbeddingModel,
        openaiChatModel: s.openaiChatModel,
        anthropicChatModel: s.anthropicChatModel,
        voyageEmbeddingModel: s.voyageEmbeddingModel,
        openrouterChatModel: s.openrouterChatModel,
        maxResults: s.maxResults,
        similarityThreshold: s.similarityThreshold,
      })
      setProviderStatus({ ollama: s.ollamaAvailable ?? false })
    }
  }, [settingsData])

  // Check if user is admin
  if (!user?.is_admin) {
    return (
      <div className="h-full overflow-auto">
        <div className="p-8 page-enter">
          <Alert variant="error">You must be an administrator to access AI settings.</Alert>
        </div>
      </div>
    )
  }

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

  // Helper to get models for a provider
  const getModelsForProvider = (provider: string, forEmbedding: boolean) => {
    const filterByType = (models: Array<{ id?: string; name?: string; isEmbedding?: boolean; isChat?: boolean }>) =>
      models.filter((m) => (forEmbedding ? m.isEmbedding : m.isChat))

    switch (provider) {
      case 'ollama':
        return ollamaModelsData?.status === 200
          ? filterByType(ollamaModelsData.data.models || [])
          : []
      case 'openai':
        return openaiModelsData?.status === 200
          ? filterByType(openaiModelsData.data.models || [])
          : []
      case 'anthropic':
        return anthropicModelsData?.status === 200
          ? filterByType(anthropicModelsData.data.models || [])
          : []
      case 'voyage':
        return voyageModelsData?.status === 200
          ? filterByType(voyageModelsData.data.models || [])
          : []
      case 'openrouter':
        return openrouterModelsData?.status === 200
          ? filterByType(openrouterModelsData.data.models || [])
          : []
      default:
        return []
    }
  }

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
        ollamaUrl: s.ollamaUrl,
        ollamaEmbeddingModel: s.ollamaEmbeddingModel,
        ollamaChatModel: s.ollamaChatModel,
        openaiEmbeddingModel: s.openaiEmbeddingModel,
        openaiChatModel: s.openaiChatModel,
        anthropicChatModel: s.anthropicChatModel,
        voyageEmbeddingModel: s.voyageEmbeddingModel,
        openrouterChatModel: s.openrouterChatModel,
        maxResults: s.maxResults,
        similarityThreshold: s.similarityThreshold,
      })
      setHasChanges(false)
      toast.info('Changes discarded')
    }
  }

  const handleTestProvider = async (provider: string) => {
    if (hasChanges) {
      await handleSave()
    }
    setTestingProvider(provider)
    try {
      const response = await testProvider.mutateAsync({ provider })
      const success = response.status === 200 && response.data.success
      setProviderStatus((prev) => ({ ...prev, [provider]: success }))
      if (success) {
        toast.success(`${providerMeta[provider]?.name || provider} connection successful`)
        if (provider === 'ollama') {
          refetchOllamaModels()
        }
      } else {
        toast.error(`Could not connect to ${providerMeta[provider]?.name || provider}`)
      }
    } catch {
      setProviderStatus((prev) => ({ ...prev, [provider]: false }))
      toast.error(`Failed to connect to ${providerMeta[provider]?.name || provider}`)
    } finally {
      setTestingProvider(null)
    }
  }

  const handlePullModel = async (modelName: string) => {
    setPullProgress((prev) => ({ ...prev, [modelName]: { status: 'Starting...' } }))

    try {
      const response = await fetch('/api/settings/ai/models/pull', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ model: modelName }),
        credentials: 'include',
      })

      if (!response.ok) {
        throw new Error(`HTTP ${response.status}`)
      }

      const reader = response.body?.getReader()
      if (!reader) {
        throw new Error('No response body')
      }

      const decoder = new TextDecoder()
      let buffer = ''

      while (true) {
        const { done, value } = await reader.read()
        if (done) {
          break
        }

        buffer += decoder.decode(value, { stream: true })
        const lines = buffer.split('\n')
        buffer = lines.pop() || ''

        for (const line of lines) {
          if (line.startsWith('data: ')) {
            try {
              const data = JSON.parse(line.slice(6))
              if (data.error) {
                throw new Error(data.error)
              }
              if (data.total && data.completed) {
                const percent = (data.completed / data.total) * 100
                setPullProgress((prev) => ({
                  ...prev,
                  [modelName]: { status: data.status || 'Downloading...', percent },
                }))
              } else {
                setPullProgress((prev) => ({
                  ...prev,
                  [modelName]: { status: data.status || 'Processing...' },
                }))
              }
              if (data.done) {
                toast.success(`Model ${modelName} pulled successfully`)
                refetchOllamaModels()
              }
            } catch (e) {
              if (e instanceof SyntaxError) {
                continue // Skip malformed JSON
              }
              throw e
            }
          }
        }
      }
    } catch (err) {
      toast.error(`Failed to pull model ${modelName}: ${err instanceof Error ? err.message : 'Unknown error'}`)
    } finally {
      setPullProgress((prev) => {
        const next = { ...prev }
        delete next[modelName]
        return next
      })
    }
  }

  const handleDeleteModel = async (modelName: string) => {
    try {
      await deleteModel.mutateAsync({ data: { model: modelName } })
      toast.success(`Model ${modelName} deleted`)
      refetchOllamaModels()
    } catch {
      toast.error(`Failed to delete model ${modelName}`)
    } finally {
      setDeleteConfirm(null)
    }
  }

  const ollamaModels = ollamaModelsData?.status === 200 ? ollamaModelsData.data.models || [] : []

  // Model select with empty state
  const ModelSelect = ({
    provider,
    value,
    onChange,
    forEmbedding,
    label,
  }: {
    provider: string
    value: string
    onChange: (value: string) => void
    forEmbedding: boolean
    label: string
  }) => {
    const models = getModelsForProvider(provider, forEmbedding)
    const options = models.map((m) => ({
      value: m.id || '',
      label: m.name || m.id || '',
    }))

    if (options.length === 0) {
      return (
        <div>
          <label className={cn('block text-sm font-medium mb-2', text.primary)}>{label}</label>
          <EmptyModelsState provider={provider} onTest={() => handleTestProvider(provider)} />
        </div>
      )
    }

    return (
      <div>
        <label className={cn('block text-sm font-medium mb-2', text.primary)}>{label}</label>
        <Select value={value} onChange={(e) => onChange(e.target.value)} options={options} />
      </div>
    )
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
                  <ProviderSelect
                    value={formValues.embeddingProvider ?? 'ollama'}
                    onChange={(value) => handleChange('embeddingProvider', value)}
                    options={embeddingProviderOptions}
                    label="Provider"
                  />

                  {/* Ollama Embedding Config */}
                  {formValues.embeddingProvider === 'ollama' && (
                    <ProviderCard
                      title="Ollama Server"
                      icon={Server}
                      connectionStatus={providerStatus.ollama}
                      isTesting={testingProvider === 'ollama'}
                      onTest={() => handleTestProvider('ollama')}
                    >
                      <div>
                        <label className={cn('block text-sm font-medium mb-2', text.primary)}>
                          Server URL
                        </label>
                        <Input
                          value={formValues.ollamaUrl ?? ''}
                          onChange={(e) => handleChange('ollamaUrl', e.target.value)}
                          placeholder="http://localhost:11434"
                        />
                      </div>
                      <ModelSelect
                        provider="ollama"
                        value={formValues.ollamaEmbeddingModel ?? ''}
                        onChange={(value) => handleChange('ollamaEmbeddingModel', value)}
                        forEmbedding={true}
                        label="Embedding Model"
                      />
                      {/* Ollama Model Management */}
                      <div className="pt-4 border-t border-neutral-100 dark:border-neutral-800">
                        <div className="flex items-center justify-between mb-3">
                          <span className={cn('text-sm font-medium', text.primary)}>
                            Recommended Embedding Models
                          </span>
                          <Button
                            variant="ghost"
                            size="sm"
                            onClick={() => refetchOllamaModels()}
                          >
                            <RefreshCw className="w-3.5 h-3.5" />
                          </Button>
                        </div>
                        {recommendedModelsData?.status === 200 && recommendedModelsData.data.systemInfo && (
                          <div className={cn('text-xs mb-3 flex items-center gap-4', text.tertiary)}>
                            <span>RAM: {recommendedModelsData.data.systemInfo.ramFormatted}</span>
                            {recommendedModelsData.data.systemInfo.hasGpu && (
                              <span>VRAM: {recommendedModelsData.data.systemInfo.vramFormatted}</span>
                            )}
                          </div>
                        )}
                        <div className="space-y-2">
                          {(recommendedModelsData?.status === 200 ? recommendedModelsData.data.models || [] : []).map((model) => {
                            const installedModel = ollamaModels.find(
                              (m: { id?: string }) => m.id === model.id || m.id?.startsWith(`${model.id}:`)
                            )
                            return (
                              <ModelItem
                                key={model.id}
                                model={model}
                                isInstalled={!!installedModel}
                                onPull={() => handlePullModel(model.id ?? '')}
                                onDelete={() => setDeleteConfirm(installedModel?.id ?? model.id ?? '')}
                                pullProgress={pullProgress[model.id ?? '']}
                              />
                            )
                          })}
                        </div>
                      </div>
                      <ProviderTip provider="ollama" />
                    </ProviderCard>
                  )}

                  {/* OpenAI Embedding Config */}
                  {formValues.embeddingProvider === 'openai' && (
                    <ProviderCard
                      title="OpenAI API"
                      icon={Key}
                      connectionStatus={providerStatus.openai}
                      isTesting={testingProvider === 'openai'}
                      onTest={() => handleTestProvider('openai')}
                    >
                      <div>
                        <label className={cn('block text-sm font-medium mb-2', text.primary)}>
                          API Key
                        </label>
                        <ApiKeyInput
                          value={formValues.openaiApiKey ?? ''}
                          onChange={(value) => handleChange('openaiApiKey', value)}
                          placeholder="sk-..."
                        />
                        <p className={cn('text-xs mt-1.5', text.tertiary)}>
                          Your API key is encrypted and stored securely.
                        </p>
                      </div>
                      <ModelSelect
                        provider="openai"
                        value={formValues.openaiEmbeddingModel ?? 'text-embedding-3-small'}
                        onChange={(value) => handleChange('openaiEmbeddingModel', value)}
                        forEmbedding={true}
                        label="Embedding Model"
                      />
                      <ProviderTip provider="openai" />
                    </ProviderCard>
                  )}

                  {/* Voyage Embedding Config */}
                  {formValues.embeddingProvider === 'voyage' && (
                    <ProviderCard
                      title="Voyage AI API"
                      icon={Key}
                      connectionStatus={providerStatus.voyage}
                      isTesting={testingProvider === 'voyage'}
                      onTest={() => handleTestProvider('voyage')}
                    >
                      <div>
                        <label className={cn('block text-sm font-medium mb-2', text.primary)}>
                          API Key
                        </label>
                        <ApiKeyInput
                          value={formValues.voyageApiKey ?? ''}
                          onChange={(value) => handleChange('voyageApiKey', value)}
                          placeholder="pa-..."
                        />
                      </div>
                      <ModelSelect
                        provider="voyage"
                        value={formValues.voyageEmbeddingModel ?? 'voyage-3'}
                        onChange={(value) => handleChange('voyageEmbeddingModel', value)}
                        forEmbedding={true}
                        label="Embedding Model"
                      />
                      <ProviderTip provider="voyage" />
                    </ProviderCard>
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
                  <ProviderSelect
                    value={formValues.chatProvider ?? 'ollama'}
                    onChange={(value) => handleChange('chatProvider', value)}
                    options={chatProviderOptions}
                    label="Provider"
                  />

                  {/* Ollama Chat Config */}
                  {formValues.chatProvider === 'ollama' && (
                    <ProviderCard
                      title="Ollama Server"
                      icon={Server}
                      connectionStatus={providerStatus.ollama}
                      isTesting={testingProvider === 'ollama'}
                      onTest={() => handleTestProvider('ollama')}
                    >
                      {formValues.embeddingProvider !== 'ollama' && (
                        <div>
                          <label className={cn('block text-sm font-medium mb-2', text.primary)}>
                            Server URL
                          </label>
                          <Input
                            value={formValues.ollamaUrl ?? ''}
                            onChange={(e) => handleChange('ollamaUrl', e.target.value)}
                            placeholder="http://localhost:11434"
                          />
                        </div>
                      )}
                      <ModelSelect
                        provider="ollama"
                        value={formValues.ollamaChatModel ?? ''}
                        onChange={(value) => handleChange('ollamaChatModel', value)}
                        forEmbedding={false}
                        label="Chat Model"
                      />
                      {/* Ollama Chat Model Management */}
                      <div className="pt-4 border-t border-neutral-100 dark:border-neutral-800">
                        <div className="flex items-center justify-between mb-3">
                          <span className={cn('text-sm font-medium', text.primary)}>
                            Recommended Chat Models
                          </span>
                          <Button
                            variant="ghost"
                            size="sm"
                            onClick={() => refetchOllamaModels()}
                          >
                            <RefreshCw className="w-3.5 h-3.5" />
                          </Button>
                        </div>
                        {recommendedChatModelsData?.status === 200 && recommendedChatModelsData.data.systemInfo && (
                          <div className={cn('text-xs mb-3 flex items-center gap-4', text.tertiary)}>
                            <span>RAM: {recommendedChatModelsData.data.systemInfo.ramFormatted}</span>
                            {recommendedChatModelsData.data.systemInfo.hasGpu && (
                              <span>VRAM: {recommendedChatModelsData.data.systemInfo.vramFormatted}</span>
                            )}
                          </div>
                        )}
                        <div className="space-y-2">
                          {(recommendedChatModelsData?.status === 200 ? recommendedChatModelsData.data.models || [] : []).map((model) => {
                            const installedModel = ollamaModels.find(
                              (m: { id?: string }) => m.id === model.id || m.id?.startsWith(`${model.id}:`)
                            )
                            return (
                              <ModelItem
                                key={model.id}
                                model={model}
                                isInstalled={!!installedModel}
                                onPull={() => handlePullModel(model.id ?? '')}
                                onDelete={() => setDeleteConfirm(installedModel?.id ?? model.id ?? '')}
                                pullProgress={pullProgress[model.id ?? '']}
                              />
                            )
                          })}
                        </div>
                      </div>
                      {formValues.embeddingProvider !== 'ollama' && <ProviderTip provider="ollama" />}
                    </ProviderCard>
                  )}

                  {/* OpenAI Chat Config */}
                  {formValues.chatProvider === 'openai' && (
                    <ProviderCard
                      title="OpenAI API"
                      icon={Key}
                      connectionStatus={providerStatus.openai}
                      isTesting={testingProvider === 'openai'}
                      onTest={() => handleTestProvider('openai')}
                    >
                      {formValues.embeddingProvider !== 'openai' && (
                        <div>
                          <label className={cn('block text-sm font-medium mb-2', text.primary)}>
                            API Key
                          </label>
                          <ApiKeyInput
                            value={formValues.openaiApiKey ?? ''}
                            onChange={(value) => handleChange('openaiApiKey', value)}
                            placeholder="sk-..."
                          />
                        </div>
                      )}
                      <ModelSelect
                        provider="openai"
                        value={formValues.openaiChatModel ?? 'gpt-4o-mini'}
                        onChange={(value) => handleChange('openaiChatModel', value)}
                        forEmbedding={false}
                        label="Chat Model"
                      />
                      {formValues.embeddingProvider !== 'openai' && <ProviderTip provider="openai" />}
                    </ProviderCard>
                  )}

                  {/* Anthropic Chat Config */}
                  {formValues.chatProvider === 'anthropic' && (
                    <ProviderCard
                      title="Anthropic API"
                      icon={Key}
                      connectionStatus={providerStatus.anthropic}
                      isTesting={testingProvider === 'anthropic'}
                      onTest={() => handleTestProvider('anthropic')}
                    >
                      <div>
                        <label className={cn('block text-sm font-medium mb-2', text.primary)}>
                          API Key
                        </label>
                        <ApiKeyInput
                          value={formValues.anthropicApiKey ?? ''}
                          onChange={(value) => handleChange('anthropicApiKey', value)}
                          placeholder="sk-ant-..."
                        />
                      </div>
                      <ModelSelect
                        provider="anthropic"
                        value={formValues.anthropicChatModel ?? 'claude-3-haiku-20240307'}
                        onChange={(value) => handleChange('anthropicChatModel', value)}
                        forEmbedding={false}
                        label="Chat Model"
                      />
                      <ProviderTip provider="anthropic" />
                    </ProviderCard>
                  )}

                  {/* OpenRouter Chat Config */}
                  {formValues.chatProvider === 'openrouter' && (
                    <ProviderCard
                      title="OpenRouter API"
                      icon={Key}
                      connectionStatus={providerStatus.openrouter}
                      isTesting={testingProvider === 'openrouter'}
                      onTest={() => handleTestProvider('openrouter')}
                    >
                      <div>
                        <label className={cn('block text-sm font-medium mb-2', text.primary)}>
                          API Key
                        </label>
                        <ApiKeyInput
                          value={formValues.openrouterApiKey ?? ''}
                          onChange={(value) => handleChange('openrouterApiKey', value)}
                          placeholder="sk-or-..."
                        />
                      </div>
                      <ModelSelect
                        provider="openrouter"
                        value={formValues.openrouterChatModel ?? ''}
                        onChange={(value) => handleChange('openrouterChatModel', value)}
                        forEmbedding={false}
                        label="Chat Model"
                      />
                      <ProviderTip provider="openrouter" />
                    </ProviderCard>
                  )}
                </CardContent>
              </Card>

              {/* Search Settings */}
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
                        Configure semantic search behavior
                      </p>
                    </div>
                  </div>
                </CardHeader>
                <CardContent className="space-y-4">
                  <div>
                    <label className={cn('block text-sm font-medium mb-2', text.primary)}>
                      Maximum Results
                    </label>
                    <Input
                      type="number"
                      value={formValues.maxResults ?? 20}
                      onChange={(e) => handleChange('maxResults', parseInt(e.target.value, 10))}
                      min={5}
                      max={100}
                      className="max-w-xs"
                    />
                    <p className={cn('text-xs mt-1.5', text.tertiary)}>
                      Maximum number of results to return from semantic search (5-100)
                    </p>
                  </div>

                  <div>
                    <label className={cn('block text-sm font-medium mb-2', text.primary)}>
                      Similarity Threshold
                    </label>
                    <Input
                      type="number"
                      value={formValues.similarityThreshold ?? '0.5'}
                      onChange={(e) => handleChange('similarityThreshold', e.target.value)}
                      min={0}
                      max={1}
                      step={0.05}
                      className="max-w-xs"
                    />
                    <p className={cn('text-xs mt-1.5', text.tertiary)}>
                      Minimum similarity score (0-1). Higher values return more relevant but fewer
                      results.
                    </p>
                  </div>
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

      {/* Delete Confirmation Modal */}
      <Modal isOpen={deleteConfirm !== null} onClose={() => setDeleteConfirm(null)} title="Delete Model">
        <ModalContent>
          <div className="flex items-start gap-4">
            <div
              className={cn(
                'p-2 rounded-full',
                'bg-red-100 dark:bg-red-900/50',
                'text-red-600 dark:text-red-400'
              )}
            >
              <Trash2 className="w-5 h-5" />
            </div>
            <div>
              <p className={cn('text-sm', text.secondary)}>
                Are you sure you want to delete <span className="font-mono font-medium">{deleteConfirm}</span>?
                This will remove the model from Ollama.
              </p>
            </div>
          </div>
        </ModalContent>
        <ModalFooter>
          <Button variant="ghost" onClick={() => setDeleteConfirm(null)}>
            Cancel
          </Button>
          <Button
            variant="danger"
            onClick={() => deleteConfirm && handleDeleteModel(deleteConfirm)}
          >
            <Trash2 className="w-4 h-4 mr-1" />
            Delete
          </Button>
        </ModalFooter>
      </Modal>
    </div>
  )
}

export const Route = createFileRoute('/_layout/settings/ai')({
  component: AISettings,
})
