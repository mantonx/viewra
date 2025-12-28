import type { ReactNode } from 'react'
import { Card, CardHeader, CardContent } from '@/components/ui'
import { text } from '@/styles/semantic'
import { cn } from '@/lib/utils'
import { Sparkles } from 'lucide-react'

type AIFeaturesCardProps = {
  /** Pre-rendered toggle field from form.Field */
  children: ReactNode
}

/**
 * Card for enabling/disabling AI features globally.
 */
export const AIFeaturesCard = ({ children }: AIFeaturesCardProps) => {
  return (
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
      <CardContent>{children}</CardContent>
    </Card>
  )
}
