import type { ReactNode } from 'react'
import { Card, CardHeader, CardContent, Alert } from '@/components/ui'
import { text } from '@/styles/semantic'
import { cn } from '@/lib/utils'
import { MessageSquare } from 'lucide-react'
import { ProviderCard } from './ProviderCard'
import type { GithubComMantonxViewraInternalDomainAiProviderInfo as ProviderInfo } from '@/lib/api/generated/models'
import type { PluginMeta, ProviderOption } from '../AISettings.types'

type ChatProviderCardProps = {
  /** Pre-rendered select field from form.Field */
  children: ReactNode
  options: ProviderOption[]
  selectedProvider?: ProviderInfo
  pluginId?: string
  meta?: PluginMeta
}

/**
 * Card for selecting and configuring the chat provider.
 */
export const ChatProviderCard = ({
  children,
  options,
  selectedProvider,
  pluginId,
  meta,
}: ChatProviderCardProps) => {
  return (
    <Card variant="glass">
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
        {options.length > 0 ? (
          <>
            {children}
            {selectedProvider && (
              <ProviderCard
                provider={selectedProvider}
                pluginId={pluginId}
                meta={meta}
                capability="chat"
              />
            )}
          </>
        ) : (
          <Alert variant="warning">
            No chat providers available. Make sure provider plugins are installed and running.
          </Alert>
        )}
      </CardContent>
    </Card>
  )
}
