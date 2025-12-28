import { useState } from 'react'
import { Input } from '@/components/ui'
import { cn } from '@/lib/utils'
import { Eye, EyeOff } from 'lucide-react'
import type { PasswordWidgetProps } from './PasswordWidget.types'

export const PasswordWidget = ({
  id,
  value,
  onChange,
  placeholder,
  disabled,
  readonly,
}: PasswordWidgetProps) => {
  const [visible, setVisible] = useState(false)

  return (
    <div className="relative">
      <Input
        id={id}
        type={visible ? 'text' : 'password'}
        value={value ?? ''}
        onChange={(e) => onChange(e.target.value)}
        placeholder={placeholder}
        disabled={disabled}
        readOnly={readonly}
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
        aria-label={visible ? 'Hide value' : 'Show value'}
      >
        {visible ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
      </button>
    </div>
  )
}
