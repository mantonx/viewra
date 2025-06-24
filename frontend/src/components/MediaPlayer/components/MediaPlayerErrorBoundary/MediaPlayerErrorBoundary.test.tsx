import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, fireEvent } from '@testing-library/react';
import { MediaPlayerErrorBoundary } from './MediaPlayerErrorBoundary';

// Component that throws an error
const ThrowError = ({ shouldThrow }: { shouldThrow: boolean }) => {
  if (shouldThrow) {
    throw new Error('Test error');
  }
  return <div>Child component</div>;
};

// Mock console.error to avoid noise in tests
const originalConsoleError = console.error;

describe('MediaPlayerErrorBoundary', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    // Suppress console.error for these tests
    console.error = vi.fn();
  });

  afterEach(() => {
    // Restore console.error
    console.error = originalConsoleError;
  });

  it('renders children when there is no error', () => {
    const { getByText } = render(
      <MediaPlayerErrorBoundary>
        <div>Test content</div>
      </MediaPlayerErrorBoundary>
    );

    expect(getByText('Test content')).toBeInTheDocument();
  });

  it('catches errors and displays error UI', () => {
    const { getByText } = render(
      <MediaPlayerErrorBoundary>
        <ThrowError shouldThrow={true} />
      </MediaPlayerErrorBoundary>
    );

    expect(getByText('Something went wrong')).toBeInTheDocument();
    expect(getByText('An unexpected error occurred while loading the media player.')).toBeInTheDocument();
  });

  it('displays video-specific error message for video errors', () => {
    const VideoError = () => {
      throw new Error('Video codec not supported');
    };

    const { getByText } = render(
      <MediaPlayerErrorBoundary>
        <VideoError />
      </MediaPlayerErrorBoundary>
    );

    expect(getByText('Video Playback Error')).toBeInTheDocument();
    expect(getByText(/There was a problem loading or playing this video/)).toBeInTheDocument();
  });

  it('displays player-specific error message for player errors', () => {
    const PlayerError = () => {
      throw new Error('Player initialization failed');
    };

    const { getByText } = render(
      <MediaPlayerErrorBoundary>
        <PlayerError />
      </MediaPlayerErrorBoundary>
    );

    expect(getByText('Video Playback Error')).toBeInTheDocument();
  });

  it('shows technical details in collapsible section', () => {
    const TestError = () => {
      throw new Error('Detailed error message');
    };

    const { getByText } = render(
      <MediaPlayerErrorBoundary>
        <TestError />
      </MediaPlayerErrorBoundary>
    );

    const detailsElement = getByText('Technical Details');
    expect(detailsElement).toBeInTheDocument();
    
    // Click to expand details
    fireEvent.click(detailsElement);
    
    expect(getByText(/Detailed error message/)).toBeInTheDocument();
  });

  it('calls onRetry when retry button is clicked', () => {
    const onRetry = vi.fn();
    const { getByText, rerender } = render(
      <MediaPlayerErrorBoundary onRetry={onRetry}>
        <ThrowError shouldThrow={true} />
      </MediaPlayerErrorBoundary>
    );

    const retryButton = getByText('Try Again');
    fireEvent.click(retryButton);

    expect(onRetry).toHaveBeenCalledTimes(1);

    // Should reset error state after retry
    rerender(
      <MediaPlayerErrorBoundary onRetry={onRetry}>
        <ThrowError shouldThrow={false} />
      </MediaPlayerErrorBoundary>
    );

    expect(getByText('Child component')).toBeInTheDocument();
  });

  it('reloads page when reload button is clicked', () => {
    const mockReload = vi.fn();
    Object.defineProperty(window.location, 'reload', {
      value: mockReload,
      writable: true,
    });

    const { getByText } = render(
      <MediaPlayerErrorBoundary>
        <ThrowError shouldThrow={true} />
      </MediaPlayerErrorBoundary>
    );

    const reloadButton = getByText('Reload Page');
    fireEvent.click(reloadButton);

    expect(mockReload).toHaveBeenCalledTimes(1);
  });

  it('navigates back when go back button is clicked', () => {
    const mockBack = vi.fn();
    Object.defineProperty(window.history, 'back', {
      value: mockBack,
      writable: true,
    });

    const { getByText } = render(
      <MediaPlayerErrorBoundary>
        <ThrowError shouldThrow={true} />
      </MediaPlayerErrorBoundary>
    );

    const backButton = getByText('Go Back');
    fireEvent.click(backButton);

    expect(mockBack).toHaveBeenCalledTimes(1);
  });

  it('recovers from error state when retry is successful', () => {
    let shouldThrow = true;
    
    const TestComponent = () => {
      if (shouldThrow) {
        throw new Error('Test error');
      }
      return <div>Recovered successfully</div>;
    };

    const { getByText, rerender } = render(
      <MediaPlayerErrorBoundary>
        <TestComponent />
      </MediaPlayerErrorBoundary>
    );

    expect(getByText('Something went wrong')).toBeInTheDocument();

    // Fix the error condition
    shouldThrow = false;

    // Click retry
    const retryButton = getByText('Try Again');
    fireEvent.click(retryButton);

    // Force re-render
    rerender(
      <MediaPlayerErrorBoundary>
        <TestComponent />
      </MediaPlayerErrorBoundary>
    );

    expect(getByText('Recovered successfully')).toBeInTheDocument();
  });

  it('logs errors to console in development', () => {
    const TestError = () => {
      throw new Error('Console error test');
    };

    render(
      <MediaPlayerErrorBoundary>
        <TestError />
      </MediaPlayerErrorBoundary>
    );

    expect(console.error).toHaveBeenCalledWith(
      'MediaPlayer Error Boundary:',
      expect.any(Error),
      expect.any(Object)
    );
  });

  it('handles errors without error message gracefully', () => {
    const ErrorWithoutMessage = () => {
      throw {};
    };

    const { container } = render(
      <MediaPlayerErrorBoundary>
        <ErrorWithoutMessage />
      </MediaPlayerErrorBoundary>
    );

    // Should still render error UI
    expect(container.querySelector('h2')).toHaveTextContent('Something went wrong');
  });
});