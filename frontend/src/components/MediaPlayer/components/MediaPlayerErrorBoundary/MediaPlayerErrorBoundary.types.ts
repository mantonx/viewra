import { ReactNode } from 'react';

export interface MediaPlayerErrorBoundaryProps {
  children: ReactNode;
  onRetry?: () => void;
}

export interface MediaPlayerErrorBoundaryState {
  hasError: boolean;
  error: Error | null;
  errorInfo: React.ErrorInfo | null;
}