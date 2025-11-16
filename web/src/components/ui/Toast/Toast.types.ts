export type ToastVariant = 'success' | 'error' | 'warning' | 'info'

export interface Toast {
  id: string
  variant: ToastVariant
  message: string
  duration?: number
}

export interface ToastProps {
  toast: Toast
  onDismiss: (id: string) => void
}

export interface ToastContainerProps {
  toasts: Toast[]
  onDismiss: (id: string) => void
}
