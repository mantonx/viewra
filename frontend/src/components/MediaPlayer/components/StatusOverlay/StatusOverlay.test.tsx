import { describe, it, expect } from 'vitest';
import { render } from '@testing-library/react';
import { StatusOverlay } from './StatusOverlay';

describe('StatusOverlay', () => {
  it('renders nothing when no status is active', () => {
    const { container } = render(
      <StatusOverlay
        isBuffering={false}
        isSeekingAhead={false}
        isLoading={false}
      />
    );

    expect(container.firstChild).toBeNull();
  });

  it('shows buffering indicator when isBuffering is true', () => {
    const { getByText, container } = render(
      <StatusOverlay
        isBuffering={true}
        isSeekingAhead={false}
        isLoading={false}
      />
    );

    expect(getByText('Buffering...')).toBeInTheDocument();
    
    // Check for spinner animation
    const spinner = container.querySelector('.animate-spin');
    expect(spinner).toBeInTheDocument();
  });

  it('shows seeking ahead indicator when isSeekingAhead is true', () => {
    const { getByText, container } = render(
      <StatusOverlay
        isBuffering={false}
        isSeekingAhead={true}
        isLoading={false}
      />
    );

    expect(getByText('Preparing playback...')).toBeInTheDocument();
    
    // Check for pulse animation
    const pulsingElement = container.querySelector('.animate-pulse');
    expect(pulsingElement).toBeInTheDocument();
  });

  it('shows loading indicator when isLoading is true', () => {
    const { getByText, container } = render(
      <StatusOverlay
        isBuffering={false}
        isSeekingAhead={false}
        isLoading={true}
      />
    );

    expect(getByText('Loading video...')).toBeInTheDocument();
    
    // Check for spinner
    const spinner = container.querySelector('.animate-spin');
    expect(spinner).toBeInTheDocument();
  });

  it('shows error message when error is provided', () => {
    const errorMessage = 'Failed to load video';
    
    const { getByText } = render(
      <StatusOverlay
        isBuffering={false}
        isSeekingAhead={false}
        isLoading={false}
        error={errorMessage}
      />
    );

    expect(getByText(errorMessage)).toBeInTheDocument();
  });

  it('shows playback info when showPlaybackInfo is true', () => {
    const playbackInfo = {
      isTranscoding: true,
      reason: 'Codec not supported',
      sessionCount: 3,
    };

    const { getByText } = render(
      <StatusOverlay
        isBuffering={false}
        isSeekingAhead={false}
        isLoading={false}
        showPlaybackInfo={true}
        playbackInfo={playbackInfo}
      />
    );

    expect(getByText('Transcoding')).toBeInTheDocument();
    expect(getByText('Codec not supported')).toBeInTheDocument();
    expect(getByText('3 active sessions')).toBeInTheDocument();
  });

  it('shows direct play in playback info when not transcoding', () => {
    const playbackInfo = {
      isTranscoding: false,
      reason: 'Direct play supported',
      sessionCount: 1,
    };

    const { getByText } = render(
      <StatusOverlay
        isBuffering={false}
        isSeekingAhead={false}
        isLoading={false}
        showPlaybackInfo={true}
        playbackInfo={playbackInfo}
      />
    );

    expect(getByText('Direct Play')).toBeInTheDocument();
    expect(getByText('Direct play supported')).toBeInTheDocument();
    expect(getByText('1 active sessions')).toBeInTheDocument();
  });

  it('prioritizes error display over other states', () => {
    const { getByText, queryByText } = render(
      <StatusOverlay
        isBuffering={true}
        isSeekingAhead={true}
        isLoading={true}
        error="Critical error"
      />
    );

    // Should only show error
    expect(getByText('Critical error')).toBeInTheDocument();
    expect(queryByText('Buffering...')).not.toBeInTheDocument();
    expect(queryByText('Preparing playback...')).not.toBeInTheDocument();
    expect(queryByText('Loading video...')).not.toBeInTheDocument();
  });

  it('shows buffering over seeking and loading', () => {
    const { getByText, queryByText } = render(
      <StatusOverlay
        isBuffering={true}
        isSeekingAhead={true}
        isLoading={true}
        error={null}
      />
    );

    expect(getByText('Buffering...')).toBeInTheDocument();
    expect(queryByText('Preparing playback...')).not.toBeInTheDocument();
    expect(queryByText('Loading video...')).not.toBeInTheDocument();
  });

  it('shows seeking over loading when not buffering', () => {
    const { getByText, queryByText } = render(
      <StatusOverlay
        isBuffering={false}
        isSeekingAhead={true}
        isLoading={true}
        error={null}
      />
    );

    expect(getByText('Preparing playback...')).toBeInTheDocument();
    expect(queryByText('Loading video...')).not.toBeInTheDocument();
  });

  it('applies correct styling classes', () => {
    const { container } = render(
      <StatusOverlay
        isBuffering={true}
        isSeekingAhead={false}
        isLoading={false}
      />
    );

    const overlay = container.firstChild;
    expect(overlay).toHaveClass('absolute');
    expect(overlay).toHaveClass('inset-0');
    expect(overlay).toHaveClass('flex');
    expect(overlay).toHaveClass('items-center');
    expect(overlay).toHaveClass('justify-center');
    expect(overlay).toHaveClass('z-30');
  });

  it('does not show playback info when showPlaybackInfo is false', () => {
    const playbackInfo = {
      isTranscoding: true,
      reason: 'Codec not supported',
      sessionCount: 3,
    };

    const { queryByText } = render(
      <StatusOverlay
        isBuffering={false}
        isSeekingAhead={false}
        isLoading={false}
        showPlaybackInfo={false}
        playbackInfo={playbackInfo}
      />
    );

    expect(queryByText('Transcoding')).not.toBeInTheDocument();
  });

  it('does not show playback info when playbackInfo is not provided', () => {
    const { queryByText } = render(
      <StatusOverlay
        isBuffering={false}
        isSeekingAhead={false}
        isLoading={false}
        showPlaybackInfo={true}
      />
    );

    expect(queryByText('Transcoding')).not.toBeInTheDocument();
    expect(queryByText('Direct Play')).not.toBeInTheDocument();
  });

  it('renders multiple overlays when multiple states are active with playback info', () => {
    const playbackInfo = {
      isTranscoding: true,
      reason: 'Test reason',
      sessionCount: 1,
    };

    const { getByText } = render(
      <StatusOverlay
        isBuffering={true}
        isSeekingAhead={false}
        isLoading={false}
        showPlaybackInfo={true}
        playbackInfo={playbackInfo}
      />
    );

    // Should show both buffering and playback info
    expect(getByText('Buffering...')).toBeInTheDocument();
    expect(getByText('Transcoding')).toBeInTheDocument();
  });

  it('handles long error messages gracefully', () => {
    const longError = 'This is a very long error message that contains detailed information about what went wrong';
    
    const { container } = render(
      <StatusOverlay
        isBuffering={false}
        isSeekingAhead={false}
        isLoading={false}
        error={longError}
      />
    );

    const errorElement = container.querySelector('.text-red-400');
    expect(errorElement).toHaveTextContent(longError);
    expect(errorElement).toHaveClass('max-w-md');
  });
});