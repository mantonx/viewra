import type { ReactNode } from 'react'
import type { DeepKeys } from '@tanstack/react-form'
import { cn } from '@/lib/utils'
import { Toggle } from '@/components/ui/Toggle'
import type { AnyFieldApi } from './Form.types'

type FormSettingToggleProps<TFormData, TName extends DeepKeys<TFormData>> = {
  field: AnyFieldApi<TFormData, TName>
  /** Label for the setting */
  label: string
  /** Description text explaining what the setting does */
  description: string
  /** Optional content to show on the right side when enabled */
  previewContent?: ReactNode
  /** Whether the setting is disabled */
  disabled?: boolean
}

/**
 * Form-connected SettingToggle component.
 * Renders a toggle with label and description in a styled card container.
 * Automatically binds to TanStack Form field state.
 *
 * @example
 * <form.Field name="monitoring_enabled">
 *   {(field) => (
 *     <FormSettingToggle
 *       field={field}
 *       label="Filesystem Monitoring"
 *       description="Automatically detect new and changed files"
 *     />
 *   )}
 * </form.Field>
 */
export const FormSettingToggle = <TFormData, TName extends DeepKeys<TFormData>>({
  field,
  label,
  description,
  previewContent,
  disabled = false,
}: FormSettingToggleProps<TFormData, TName>) => {
  const enabled = field.state.value as boolean

  return (
    <div
      className={cn(
        'flex items-center justify-between p-4 rounded-xl transition-all duration-150',
        'bg-white/80 dark:bg-white/5',
        'border border-neutral-200/50 dark:border-white/10',
        'backdrop-blur-sm',
        'hover:bg-white dark:hover:bg-white/[0.07]'
      )}
    >
      <div className="flex-1">
        <div className="flex items-center gap-3">
          <Toggle
            enabled={enabled}
            // eslint-disable-next-line @typescript-eslint/no-explicit-any
            onChange={(val) => field.handleChange(val as any)}
            label={label}
            disabled={disabled}
          />
          <div>
            <span className="text-sm font-medium text-neutral-900 dark:text-neutral-50">
              {label}
            </span>
            <p className="text-xs mt-0.5 text-neutral-500 dark:text-neutral-500">{description}</p>
          </div>
        </div>
      </div>
      {previewContent && enabled && <div className="ml-4">{previewContent}</div>}
    </div>
  )
}
