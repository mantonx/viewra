import type { ReactNode } from 'react'

export type ErrorBoundaryProps = {
  children: ReactNode
  fallback?: ReactNode
  onError?: (error: Error, errorInfo: React.ErrorInfo) => void
}

export type ErrorBoundaryState = {
  hasError: boolean
  error: Error | null
}
