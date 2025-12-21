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
} from '@/components/ui'
import { PageHeader } from '@/components/common'
import { useToast } from '@/lib/hooks/useToast'
import { text } from '@/styles/semantic'
import { cn } from '@/lib/utils'
import {
  Bot,
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
} from 'lucide-react'
import {
  useGetApiSettingsAi,
  usePutApiSettingsAi,
  useGetApiSettingsAiModels,
  usePostApiSettingsAiModelsPull,
  useDeleteApiSettingsAiModels,
  usePostApiSettingsAiTestOllama,
  usePostApiSettingsAiTestOpenai,
} from '@/lib/api/generated/settings/settings'
import type { InternalApiHandlersAISettingsRequest } from '@/lib/api/generated/models'

// Provider options
const providerOptions = [
  { value: 'ollama', label: 'Ollama (Local)' },
  { value: 'openai', label: 'OpenAI' },
]

// Common embedding models for Ollama
const defaultOllamaModels = [
  'nomic-embed-text',
  'mxbai-embed-large',
  'all-minilm',
  'bge-base-en-v1.5',
  'bge-large-en-v1.5',
]

// OpenAI embedding models
const openaiModels = [
  { value: 'text-embedding-3-small', label: 'text-embedding-3-small (Recommended)' },
  { value: 'text-embedding-3-large', label: 'text-embedding-3-large (Higher quality)' },
  { value: 'text-embedding-ada-002', label: 'text-embedding-ada-002 (Legacy)' },
]

// Connection status badge
const ConnectionStatus = ({ 
  available, 
  checking 
}: { 
  available: boolean | undefined
  checking: boolean 
}) => {
  if (checking) {
    return (
      <span className={cn(
        'inline-flex items-center gap-1.5 px-2 py-1 rounded-full text-xs font-medium',
        'bg-neutral-100 dark:bg-neutral-800 text-neutral-600 dark:text-neutral-400'
      )}>
        <Loader2 className="w-3 h-3 animate-spin" />
        Checking...
      </span>
    )
  }
  
  if (available === undefined) {
    return null
  }
  
  return available ? (
    <span className={cn(
      'inline-flex items-center gap-1.5 px-2 py-1 rounded-full text-xs font-medium',
      'bg-emerald-100 dark:bg-emerald-900/50 text-emerald-700 dark:text-emerald-400'
    )}>
      <CheckCircle className="w-3 h-3" />
      Connected
    </span>
  ) : (
    <span className={cn(
      'inline-flex items-center gap-1.5 px-2 py-1 rounded-full text-xs font-medium',
      'bg-red-100 dark:bg-red-900/50 text-red-700 dark:text-red-400'
    )}>
      <AlertCircle className="w-3 h-3" />
      Unavailable
    </span>
  )
}

// Model list item
const ModelItem = ({ 
  name, 
  isInstalled, 
  onPull, 
  onDelete,
  isPulling,
  pullProgress,
}: { 
  name: string
  isInstalled: boolean
  onPull: () => void
  onDelete: () => void
  isPulling: boolean
  pullProgress?: string
}) => (
  <div className={cn(
    'flex items-center justify-between py-2 px-3 rounded-lg',
    'bg-neutral-50 dark:bg-neutral-900/50'
  )}>
    <div className="flex items-center gap-2">
      <span className={cn('text-sm font-mono', text.primary)}>{name}</span>
      {isInstalled && (
        <span className={cn(
          'px-1.5 py-0.5 rounded text-[10px] font-medium',
          'bg-emerald-100 dark:bg-emerald-900/50 text-emerald-700 dark:text-emerald-400'
        )}>
          Installed
        </span>
      )}
    </div>
    <div className="flex items-center gap-2">
      {isPulling ? (
        <span className={cn('text-xs', text.secondary)}>
          {pullProgress || 'Downloading...'}
        </span>
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
)

const AISettings = () => {
  const { user } = useAuth()
  const toast = useToast()

  // Fetch current AI settings
  const { 
    data: settingsData, 
    isLoading, 
    error,
    refetch: refetchSettings 
  } = useGetApiSettingsAi()

  // Fetch Ollama models
  const { 
    data: modelsData, 
    refetch: refetchModels,
    isLoading: modelsLoading,
  } = useGetApiSettingsAiModels({ query: { enabled: false } })

  // Mutations
  const updateSettings = usePutApiSettingsAi()
  const pullModel = usePostApiSettingsAiModelsPull()
  const deleteModel = useDeleteApiSettingsAiModels()
  const testOllama = usePostApiSettingsAiTestOllama()
  const testOpenai = usePostApiSettingsAiTestOpenai()

  // Local state for form
  const [formValues, setFormValues] = useState<InternalApiHandlersAISettingsRequest>({})
  const [hasChanges, setHasChanges] = useState(false)
  const [testingOllama, setTestingOllama] = useState(false)
  const [testingOpenai, setTestingOpenai] = useState(false)
  const [ollamaAvailable, setOllamaAvailable] = useState<boolean | undefined>()
  const [openaiValid, setOpenaiValid] = useState<boolean | undefined>()
  const [pullingModel, setPullingModel] = useState<string | null>(null)

  // Initialize form with fetched data
  useEffect(() => {
    if (settingsData?.status === 200) {
      const settings = settingsData.data
      setFormValues({
        enabled: settings.enabled,
        provider: settings.provider,
        ollamaUrl: settings.ollamaUrl,
        ollamaModel: settings.ollamaModel,
        openaiModel: settings.openaiModel,
        maxResults: settings.maxResults,
        similarityThreshold: settings.similarityThreshold,
      })
      setOllamaAvailable(settings.ollamaAvailable)
    }
  }, [settingsData])

  // Load Ollama models when provider is Ollama and URL is set
  useEffect(() => {
    if (formValues.provider === 'ollama' && formValues.ollamaUrl) {
      refetchModels()
    }
  }, [formValues.provider, formValues.ollamaUrl, refetchModels])

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

  const settings = settingsData?.status === 200 ? settingsData.data : null
  const installedModels = modelsData?.status === 200 ? (modelsData.data.models || []) : []

  const handleChange = <K extends keyof InternalApiHandlersAISettingsRequest>(
    key: K,
    value: InternalApiHandlersAISettingsRequest[K]
  ) => {
    setFormValues(prev => ({ ...prev, [key]: value }))
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
    } catch (err) {
      toast.error('Failed to save AI settings')
    }
  }

  const handleDiscard = () => {
    if (settingsData?.status === 200) {
      const s = settingsData.data
      setFormValues({
        enabled: s.enabled,
        provider: s.provider,
        ollamaUrl: s.ollamaUrl,
        ollamaModel: s.ollamaModel,
        openaiModel: s.openaiModel,
        maxResults: s.maxResults,
        similarityThreshold: s.similarityThreshold,
      })
      setHasChanges(false)
      toast.info('Changes discarded')
    }
  }

  const handleTestOllama = async () => {
    // Save settings first so test uses latest URL
    if (hasChanges) {
      await handleSave()
    }
    setTestingOllama(true)
    try {
      const response = await testOllama.mutateAsync()
      const available = response.status === 200 && response.data.success
      setOllamaAvailable(available)
      if (available) {
        toast.success('Ollama connection successful')
        refetchModels()
      } else {
        toast.error('Could not connect to Ollama')
      }
    } catch {
      setOllamaAvailable(false)
      toast.error('Failed to connect to Ollama')
    } finally {
      setTestingOllama(false)
    }
  }

  const handleTestOpenai = async () => {
    if (!formValues.openaiApiKey || formValues.openaiApiKey.includes('••')) {
      toast.error('Please enter and save an API key first')
      return
    }
    // Save settings first so test uses latest key
    if (hasChanges) {
      await handleSave()
    }
    setTestingOpenai(true)
    try {
      const response = await testOpenai.mutateAsync()
      const valid = response.status === 200 && response.data.success
      setOpenaiValid(valid)
      if (valid) {
        toast.success('OpenAI API key is valid')
      } else {
        toast.error('Invalid OpenAI API key')
      }
    } catch {
      setOpenaiValid(false)
      toast.error('Failed to validate OpenAI API key')
    } finally {
      setTestingOpenai(false)
    }
  }

  const handlePullModel = async (modelName: string) => {
    setPullingModel(modelName)
    try {
      // Note: This should ideally use SSE for progress, but for now we just wait
      await pullModel.mutateAsync({ data: { model: modelName } })
      toast.success(`Model ${modelName} pulled successfully`)
      refetchModels()
    } catch {
      toast.error(`Failed to pull model ${modelName}`)
    } finally {
      setPullingModel(null)
    }
  }

  const handleDeleteModel = async (modelName: string) => {
    try {
      await deleteModel.mutateAsync({ data: { model: modelName } })
      toast.success(`Model ${modelName} deleted`)
      refetchModels()
    } catch {
      toast.error(`Failed to delete model ${modelName}`)
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
                <div className={cn(
                  'p-2 rounded-lg',
                  'bg-gradient-to-br from-violet-500 to-purple-600',
                  'text-white shadow-lg shadow-violet-500/25'
                )}>
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

          {/* Provider Selection */}
          {formValues.enabled && (
            <Card>
              <CardHeader className="border-b border-neutral-100 dark:border-neutral-800">
                <div className="flex items-center gap-3">
                  <div className={cn(
                    'p-2 rounded-lg',
                    'bg-primary-50 dark:bg-primary-950/50',
                    'text-primary-600 dark:text-primary-400'
                  )}>
                    <Bot className="w-5 h-5" />
                  </div>
                  <div>
                    <h2 className={cn('text-lg font-semibold', text.primary)}>AI Provider</h2>
                    <p className={cn('text-sm mt-0.5', text.secondary)}>
                      Choose between local (Ollama) or cloud (OpenAI) AI processing
                    </p>
                  </div>
                </div>
              </CardHeader>
              <CardContent className="space-y-4">
                <div>
                  <label className={cn('block text-sm font-medium mb-2', text.primary)}>
                    Provider
                  </label>
                  <Select
                    value={formValues.provider ?? 'ollama'}
                    onChange={(e) => handleChange('provider', e.target.value)}
                    options={providerOptions}
                  />
                </div>

                {/* Ollama Configuration */}
                {formValues.provider === 'ollama' && (
                  <div className="space-y-4 pt-4 border-t border-neutral-100 dark:border-neutral-800">
                    <div className="flex items-center justify-between">
                      <div className="flex items-center gap-2">
                        <Server className={cn('w-4 h-4', text.secondary)} />
                        <span className={cn('text-sm font-medium', text.primary)}>
                          Ollama Server
                        </span>
                      </div>
                      <ConnectionStatus available={ollamaAvailable} checking={testingOllama} />
                    </div>

                    <div className="flex gap-2">
                      <Input
                        value={formValues.ollamaUrl ?? ''}
                        onChange={(e) => handleChange('ollamaUrl', e.target.value)}
                        placeholder="http://localhost:11434"
                        className="flex-1"
                      />
                      <Button variant="secondary" onClick={handleTestOllama} disabled={testingOllama}>
                        {testingOllama ? (
                          <Loader2 className="w-4 h-4 animate-spin" />
                        ) : (
                          <RefreshCw className="w-4 h-4" />
                        )}
                      </Button>
                    </div>

                    <div>
                      <label className={cn('block text-sm font-medium mb-2', text.primary)}>
                        Embedding Model
                      </label>
                      <Select
                        value={formValues.ollamaModel ?? ''}
                        onChange={(e) => handleChange('ollamaModel', e.target.value)}
                        options={
                          installedModels
                            .filter((m: { isEmbedding?: boolean }) => m.isEmbedding)
                            .map((m: { id?: string; name?: string }) => ({ 
                              value: m.id || '', 
                              label: m.name || m.id || '' 
                            }))
                        }
                      />
                      <p className={cn('text-xs mt-1.5', text.tertiary)}>
                        Select an embedding model for semantic search. Recommended: nomic-embed-text
                      </p>
                    </div>

                    {/* Model Management */}
                    <div className="pt-4 border-t border-neutral-100 dark:border-neutral-800">
                      <div className="flex items-center justify-between mb-3">
                        <span className={cn('text-sm font-medium', text.primary)}>
                          Available Models
                        </span>
                        <Button 
                          variant="ghost" 
                          size="sm" 
                          onClick={() => refetchModels()}
                          disabled={modelsLoading}
                        >
                          <RefreshCw className={cn('w-3.5 h-3.5', modelsLoading && 'animate-spin')} />
                        </Button>
                      </div>
                      <div className="space-y-2">
                        {defaultOllamaModels.map(name => (
                          <ModelItem
                            key={name}
                            name={name}
                            isInstalled={installedModels.some((m: { id?: string }) => m.id === name)}
                            onPull={() => handlePullModel(name)}
                            onDelete={() => handleDeleteModel(name)}
                            isPulling={pullingModel === name}
                          />
                        ))}
                      </div>
                    </div>
                  </div>
                )}

                {/* OpenAI Configuration */}
                {formValues.provider === 'openai' && (
                  <div className="space-y-4 pt-4 border-t border-neutral-100 dark:border-neutral-800">
                    <div className="flex items-center justify-between">
                      <div className="flex items-center gap-2">
                        <Key className={cn('w-4 h-4', text.secondary)} />
                        <span className={cn('text-sm font-medium', text.primary)}>
                          OpenAI API
                        </span>
                      </div>
                      <ConnectionStatus 
                        available={openaiValid} 
                        checking={testingOpenai} 
                      />
                    </div>

                    <div className="flex gap-2">
                      <Input
                        type="password"
                        value={formValues.openaiApiKey ?? ''}
                        onChange={(e) => handleChange('openaiApiKey', e.target.value)}
                        placeholder="sk-..."
                        className="flex-1 font-mono"
                      />
                      <Button variant="secondary" onClick={handleTestOpenai} disabled={testingOpenai}>
                        {testingOpenai ? (
                          <Loader2 className="w-4 h-4 animate-spin" />
                        ) : (
                          <Check className="w-4 h-4" />
                        )}
                      </Button>
                    </div>
                    <p className={cn('text-xs', text.tertiary)}>
                      Your API key is encrypted and stored securely.
                    </p>

                    <div>
                      <label className={cn('block text-sm font-medium mb-2', text.primary)}>
                        Embedding Model
                      </label>
                      <Select
                        value={formValues.openaiModel ?? 'text-embedding-3-small'}
                        onChange={(e) => handleChange('openaiModel', e.target.value)}
                        options={openaiModels}
                      />
                    </div>
                  </div>
                )}
              </CardContent>
            </Card>
          )}

          {/* Search Settings */}
          {formValues.enabled && (
            <Card>
              <CardHeader className="border-b border-neutral-100 dark:border-neutral-800">
                <div className="flex items-center gap-3">
                  <div className={cn(
                    'p-2 rounded-lg',
                    'bg-primary-50 dark:bg-primary-950/50',
                    'text-primary-600 dark:text-primary-400'
                  )}>
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
                    Minimum similarity score (0-1). Higher values return more relevant but fewer results.
                  </p>
                </div>
              </CardContent>
            </Card>
          )}

          {/* Save/Discard Footer */}
          {hasChanges && (
            <div className={cn(
              'sticky bottom-4 flex items-center justify-end gap-3 p-4 rounded-xl',
              'bg-white dark:bg-neutral-900',
              'border border-neutral-200 dark:border-neutral-700',
              'shadow-lg'
            )}>
              <span className={cn('text-sm mr-auto', text.secondary)}>
                You have unsaved changes
              </span>
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
