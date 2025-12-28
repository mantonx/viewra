import { Card, CardHeader, CardContent } from '@/components/ui'
import { PluginSettingsForm } from '@/components/settings'
import { text } from '@/styles/semantic'
import { cn } from '@/lib/utils'
import { Settings2 } from 'lucide-react'

/**
 * Card for configuring AI search settings via the ai-search plugin.
 */
export const SearchSettingsCard = () => {
  return (
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
  )
}
