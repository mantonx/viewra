import { Component } from 'react'
import { Alert } from '@/components/ui'
import type { ErrorBoundaryProps, ErrorBoundaryState } from './ErrorBoundary.types'

/**
 * Error boundary component that catches JavaScript errors in child components.
 * Displays a fallback UI instead of crashing the entire app.
 */
export class ErrorBoundary extends Component<ErrorBoundaryProps, ErrorBoundaryState> {
  constructor(props: ErrorBoundaryProps) {
    super(props)
    this.state = { hasError: false, error: null }
  }

  static getDerivedStateFromError(error: Error): ErrorBoundaryState {
    return { hasError: true, error }
  }

  componentDidCatch(error: Error, errorInfo: React.ErrorInfo): void {
    // Log error to console in development
    console.error('ErrorBoundary caught an error:', error, errorInfo)

    // Call optional error handler
    this.props.onError?.(error, errorInfo)
  }

  render() {
    if (this.state.hasError) {
      // Use custom fallback if provided
      if (this.props.fallback) {
        return this.props.fallback
      }

      // Default fallback UI
      return (
        <div className="p-8">
          <Alert variant="error">
            <div className="space-y-2">
              <p className="font-semibold">Something went wrong</p>
              <p className="text-sm opacity-80">
                {this.state.error?.message || 'An unexpected error occurred'}
              </p>
              <button
                onClick={() => this.setState({ hasError: false, error: null })}
                className="mt-4 px-4 py-2 text-sm bg-white/10 hover:bg-white/20 rounded transition-colors"
              >
                Try again
              </button>
            </div>
          </Alert>
        </div>
      )
    }

    return this.props.children
  }
}
