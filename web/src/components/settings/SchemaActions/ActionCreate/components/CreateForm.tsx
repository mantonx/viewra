import { Download, Loader2 } from 'lucide-react'
import { Input, Button } from '@/components/ui'
import type { CreateFormProps } from './CreateForm.types'

export const CreateForm = ({
  schema,
  formData,
  isSubmitting,
  onChange,
  onSubmit,
}: CreateFormProps) => {
  if (!schema?.properties) {
    return null
  }

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter') {
      onSubmit()
    }
  }

  return (
    <div className="flex gap-2">
      {Object.entries(schema.properties).map(([key, prop]) => (
        <div key={key} className="flex-1">
          <Input
            value={formData[key] || ''}
            onChange={(e) => onChange(key, e.target.value)}
            placeholder={prop.description || prop.title || key}
            disabled={isSubmitting}
            onKeyDown={handleKeyDown}
          />
        </div>
      ))}
      <Button onClick={onSubmit} disabled={isSubmitting}>
        {isSubmitting ? (
          <Loader2 className="w-4 h-4 animate-spin" />
        ) : (
          <Download className="w-4 h-4" />
        )}
      </Button>
    </div>
  )
}

CreateForm.displayName = 'CreateForm'
