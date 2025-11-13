import { cn } from '@/lib/utils'

export interface ProgressProps {
  value: number // 0-100
  className?: string
  variant?: 'default' | 'success' | 'warning'
  size?: 'sm' | 'md' | 'lg'
  showLabel?: boolean
}

export const Progress = ({
  value,
  className,
  variant = 'default',
  size = 'md',
  showLabel = false
}: ProgressProps) => {
  const clampedValue = Math.min(Math.max(value, 0), 100)

  const containerSizes = {
    sm: 'h-1',
    md: 'h-2',
    lg: 'h-3',
  }

  const variants = {
    default: 'bg-blue-500',
    success: 'bg-green-500',
    warning: 'bg-yellow-500',
  }

  return (
    <div className="w-full">
      <div className={cn('w-full bg-gray-200 rounded-full overflow-hidden', containerSizes[size], className)}>
        <div
          className={cn('h-full transition-all duration-300 ease-in-out', variants[variant])}
          style={{ width: `${clampedValue}%` }}
        />
      </div>
      {showLabel && (
        <div className="text-xs text-gray-600 mt-1 text-right">
          {Math.floor(clampedValue)}%
        </div>
      )}
    </div>
  )
}
