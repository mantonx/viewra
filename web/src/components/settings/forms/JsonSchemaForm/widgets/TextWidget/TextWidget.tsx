import { Input } from '@/components/ui'
import type { TextWidgetProps } from './TextWidget.types'

export const TextWidget = ({
  id,
  value,
  onChange,
  placeholder,
  disabled,
  readonly,
  autofocus,
  options,
  schema,
}: TextWidgetProps) => {
  const inputType = options?.inputType || (schema?.format === 'password' ? 'password' : 'text')

  return (
    <Input
      id={id}
      type={inputType}
      value={value ?? ''}
      onChange={(e) => onChange(e.target.value)}
      placeholder={placeholder}
      disabled={disabled}
      readOnly={readonly}
      autoFocus={autofocus}
      className="font-mono"
    />
  )
}
