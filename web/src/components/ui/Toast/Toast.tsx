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
    success: cn(
      'bg-white/95 dark:bg-neutral-900/95 backdrop-blur-xl',
      'border-l-green-500 text-green-800 dark:text-green-300',
      'shadow-lg shadow-green-500/10 dark:shadow-green-500/5'
    ),
    error: cn(
      'bg-white/95 dark:bg-neutral-900/95 backdrop-blur-xl',
      'border-l-red-500 text-red-800 dark:text-red-300',
      'shadow-lg shadow-red-500/10 dark:shadow-red-500/5'
    ),
    warning: cn(
      'bg-white/95 dark:bg-neutral-900/95 backdrop-blur-xl',
      'border-l-amber-500 text-amber-800 dark:text-amber-300',
      'shadow-lg shadow-amber-500/10 dark:shadow-amber-500/5'
    ),
    info: cn(
      'bg-white/95 dark:bg-neutral-900/95 backdrop-blur-xl',
      'border-l-blue-500 text-blue-800 dark:text-blue-300',
      'shadow-lg shadow-blue-500/10 dark:shadow-blue-500/5'
    ),
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
        'flex items-start gap-3 p-4 rounded-xl border border-neutral-200/50 dark:border-white/10 border-l-4 max-w-md w-full',
        'transition-all duration-300 ease-out',
        isExiting ? 'opacity-0 translate-x-full scale-95' : 'opacity-100 translate-x-0 scale-100',
        variantStyles[toast.variant]
      )}
    >
      <span className="text-lg font-bold shrink-0 opacity-80" aria-hidden="true">
        {iconMap[toast.variant]}
      </span>
      <p className="flex-1 text-sm font-medium break-words text-neutral-900 dark:text-neutral-100">{toast.message}</p>
      <button
        onClick={() => {
          setIsExiting(true)
          setTimeout(() => onDismiss(toast.id), 300)
        }}
        className="shrink-0 text-neutral-400 hover:text-neutral-600 dark:text-neutral-500 dark:hover:text-neutral-300 transition-colors"
        aria-label="Dismiss notification"
      >
        <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
          <path strokeLinecap="round" strokeLinejoin="round" d="M6 18L18 6M6 6l12 12" />
        </svg>
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
