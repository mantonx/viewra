import { useEffect, useState } from 'react'
import { cn } from '@/lib/utils'
import type { ToastProps, ToastContainerProps, ToastVariant } from './Toast.types'

const ToastItem = ({ toast, onDismiss }: ToastProps) => {
  const [isExiting, setIsExiting] = useState(false)

  useEffect(() => {
    const duration = toast.duration ?? 5000
    if (duration > 0) {
      const timer = setTimeout(() => {
        setIsExiting(true)
        setTimeout(() => onDismiss(toast.id), 300) // Wait for exit animation
      }, duration)
      return () => clearTimeout(timer)
    }
  }, [toast.id, toast.duration, onDismiss])

  const variantStyles: Record<ToastVariant, string> = {
    success: 'bg-green-50 dark:bg-green-950 border-green-500 dark:border-green-600 text-green-900 dark:text-green-100',
    error: 'bg-red-50 dark:bg-red-950 border-red-500 dark:border-red-600 text-red-900 dark:text-red-100',
    warning: 'bg-yellow-50 dark:bg-yellow-950 border-yellow-500 dark:border-yellow-600 text-yellow-900 dark:text-yellow-100',
    info: 'bg-blue-50 dark:bg-blue-950 border-blue-500 dark:border-blue-600 text-blue-900 dark:text-blue-100',
  }

  const iconMap: Record<ToastVariant, string> = {
    success: '✓',
    error: '✕',
    warning: '⚠',
    info: 'ℹ',
  }

  return (
    <div
      role="alert"
      aria-live="polite"
      className={cn(
        'flex items-start gap-3 p-4 rounded-lg border-l-4 shadow-lg dark:shadow-neutral-950/50 max-w-md w-full',
        'transition-all duration-300 ease-in-out',
        isExiting ? 'opacity-0 translate-x-full' : 'opacity-100 translate-x-0',
        variantStyles[toast.variant]
      )}
    >
      <span className="text-xl font-bold shrink-0" aria-hidden="true">
        {iconMap[toast.variant]}
      </span>
      <p className="flex-1 text-sm font-medium break-words">{toast.message}</p>
      <button
        onClick={() => {
          setIsExiting(true)
          setTimeout(() => onDismiss(toast.id), 300)
        }}
        className="shrink-0 text-current opacity-50 hover:opacity-100 transition-opacity"
        aria-label="Dismiss notification"
      >
        ×
      </button>
    </div>
  )
}

export const ToastContainer = ({ toasts, onDismiss }: ToastContainerProps) => {
  if (toasts.length === 0) {
    return null
  }

  return (
    <div
      className="fixed top-4 right-4 z-50 flex flex-col gap-2 pointer-events-none"
      aria-label="Notifications"
    >
      {toasts.map((toast) => (
        <div key={toast.id} className="pointer-events-auto">
          <ToastItem toast={toast} onDismiss={onDismiss} />
        </div>
      ))}
    </div>
  )
}
