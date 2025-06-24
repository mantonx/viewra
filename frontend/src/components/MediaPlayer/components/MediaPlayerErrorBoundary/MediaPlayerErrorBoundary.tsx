import React, { Component, ErrorInfo } from 'react';
import { RefreshCw, AlertTriangle } from 'lucide-react';
import type { MediaPlayerErrorBoundaryProps, MediaPlayerErrorBoundaryState } from './MediaPlayerErrorBoundary.types';

export class MediaPlayerErrorBoundary extends Component<MediaPlayerErrorBoundaryProps, MediaPlayerErrorBoundaryState> {
  constructor(props: MediaPlayerErrorBoundaryProps) {
    super(props);
    this.state = { hasError: false, error: null, errorInfo: null };
  }

  static getDerivedStateFromError(error: Error): MediaPlayerErrorBoundaryState {
    return {
      hasError: true,
      error,
      errorInfo: null,
    };
  }

  componentDidCatch(error: Error, errorInfo: ErrorInfo) {
    console.error('MediaPlayer Error Boundary:', error, errorInfo);
    this.setState({
      error,
      errorInfo,
    });
  }

  handleRetry = () => {
    this.setState({ hasError: false, error: null, errorInfo: null });
    if (this.props.onRetry) {
      this.props.onRetry();
    }
  };

  render() {
    if (this.state.hasError) {
      const isVideoError = this.state.error?.message?.toLowerCase().includes('video') || 
                          this.state.error?.message?.toLowerCase().includes('player') ||
                          this.state.error?.message?.toLowerCase().includes('shaka');

      return (
        <div className="flex items-center justify-center h-screen bg-gradient-to-br from-slate-900 to-slate-800 text-white">
          <div className="text-center max-w-md mx-4">
            <div className="mb-6">
              <AlertTriangle className="w-16 h-16 text-red-400 mx-auto mb-4" />
              <h2 className="text-2xl font-bold mb-2">
                {isVideoError ? 'Video Playback Error' : 'Something went wrong'}
              </h2>
            </div>
            
            <div className="mb-6">
              <p className="text-slate-300 mb-4">
                {isVideoError 
                  ? 'There was a problem loading or playing this video. This could be due to an unsupported format or network issues.'
                  : 'An unexpected error occurred while loading the media player.'
                }
              </p>
              
              {this.state.error && (
                <details className="text-left bg-slate-800 p-3 rounded text-sm text-slate-400 mb-4">
                  <summary className="cursor-pointer hover:text-white">
                    Technical Details
                  </summary>
                  <div className="mt-2 space-y-2">
                    <div>
                      <strong>Error:</strong> {this.state.error.message}
                    </div>
                    {this.state.errorInfo?.componentStack && (
                      <div>
                        <strong>Component Stack:</strong>
                        <pre className="text-xs mt-1 whitespace-pre-wrap">
                          {this.state.errorInfo.componentStack}
                        </pre>
                      </div>
                    )}
                  </div>
                </details>
              )}
            </div>

            <div className="space-y-3">
              <button
                onClick={this.handleRetry}
                className="flex items-center justify-center gap-2 w-full bg-blue-600 hover:bg-blue-700 px-6 py-3 rounded-lg transition-colors duration-200 font-medium"
              >
                <RefreshCw className="w-4 h-4" />
                Try Again
              </button>
              
              <button
                onClick={() => window.location.reload()}
                className="w-full bg-slate-700 hover:bg-slate-600 px-6 py-3 rounded-lg transition-colors duration-200"
              >
                Reload Page
              </button>
              
              <button
                onClick={() => window.history.back()}
                className="w-full text-slate-400 hover:text-white px-6 py-2 transition-colors duration-200"
              >
                Go Back
              </button>
            </div>
          </div>
        </div>
      );
    }

    return this.props.children;
  }
}

export default MediaPlayerErrorBoundary;