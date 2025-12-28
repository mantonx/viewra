import { Card, CardHeader, CardContent, CardFooter, Button } from '@/components/ui'
import { text } from '@/styles/semantic'
import { cn } from '@/lib/utils'
import { RotateCcw } from 'lucide-react'
import { PreferenceSettingRow } from './PreferenceSettingRow'
import type { PreferencesCategoryCardProps } from '../PreferencesSettings.types'

/**
 * Card displaying all settings for a preferences category.
 */
export const PreferencesCategoryCard = ({
  config,
  definitions,
  hasChanges,
  changeCount,
  hasFieldChanged,
  isFieldDefault,
  onSave,
  onDiscard,
  isSaving,
  renderField,
}: PreferencesCategoryCardProps) => {
  return (
    <Card>
      <CardHeader>
        <div className="flex items-center gap-3">
          <div
            className={cn(
              'p-2 rounded-lg bg-blue-100 text-blue-600 dark:bg-blue-900/30 dark:text-blue-400'
            )}
          >
            {config.icon}
          </div>
          <div>
            <h2 className={cn('text-lg font-semibold', text.primary)}>{config.label}</h2>
            <p className={cn('text-sm', text.secondary)}>{config.description}</p>
          </div>
        </div>
      </CardHeader>

      <CardContent className="space-y-1">
        {definitions.map((def) => {
          const key = def.key ?? ''
          if (!key) {
            return null
          }

          return (
            <PreferenceSettingRow
              key={key}
              definition={def}
              isChanged={hasFieldChanged(key)}
              isDefault={isFieldDefault(key)}
            >
              {renderField(def)}
            </PreferenceSettingRow>
          )
        })}
      </CardContent>

      {hasChanges && (
        <CardFooter className="flex justify-end gap-2 border-t border-neutral-200 dark:border-neutral-700 pt-4">
          <Button variant="ghost" onClick={onDiscard} disabled={isSaving}>
            <RotateCcw className="w-4 h-4 mr-1.5" />
            Discard
          </Button>
          <Button onClick={onSave} isLoading={isSaving}>
            Save {changeCount > 1 ? `(${changeCount})` : ''}
          </Button>
        </CardFooter>
      )}
    </Card>
  )
}
