import { useState, useCallback, createContext, useContext } from 'react'
import type { Toast, ToastVariant } from '@/components/ui/Toast'

interface ToastContextValue {
  toasts: Toast[]
  addToast: (message: string, variant: ToastVariant, duration?: number) => void
  removeToast: (id: string) => void
  toast: {
    success: (message: string, duration?: number) => void
    error: (message: string, duration?: number) => void
    warning: (message: string, duration?: number) => void
    info: (message: string, duration?: number) => void
  }
}

const ToastContext = createContext<ToastContextValue | undefined>(undefined)

export const useToastState = () => {
  const [toasts, setToasts] = useState<Toast[]>([])

  const addToast = useCallback((message: string, variant: ToastVariant, duration = 5000) => {
    const id = `toast-${Date.now()}-${Math.random().toString(36).substring(2, 9)}`
    const newToast: Toast = { id, message, variant, duration }
    setToasts((prev) => [...prev, newToast])
  }, [])

  const removeToast = useCallback((id: string) => {
    setToasts((prev) => prev.filter((toast) => toast.id !== id))
  }, [])

  const toast = {
    success: (message: string, duration?: number) => addToast(message, 'success', duration),
    error: (message: string, duration?: number) => addToast(message, 'error', duration),
    warning: (message: string, duration?: number) => addToast(message, 'warning', duration),
    info: (message: string, duration?: number) => addToast(message, 'info', duration),
  }

  return { toasts, addToast, removeToast, toast }
}

export const ToastProvider = ToastContext.Provider

export const useToast = (): ToastContextValue['toast'] => {
  const context = useContext(ToastContext)
  if (!context) {
    throw new Error('useToast must be used within a ToastProvider')
  }
  return context.toast
}

export const useToastContainer = () => {
  const context = useContext(ToastContext)
  if (!context) {
    throw new Error('useToastContainer must be used within a ToastProvider')
  }
  return { toasts: context.toasts, removeToast: context.removeToast }
}
