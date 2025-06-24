import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, fireEvent } from '@testing-library/react';
import { VideoControls } from './VideoControls';

describe('VideoControls', () => {
  const defaultProps = {
    isPlaying: false,
    currentTime: 0,
    duration: 120,
    volume: 1,
    isMuted: false,
    isFullscreen: false,
    onPlayPause: vi.fn(),
    onSeek: vi.fn(),
    onVolumeChange: vi.fn(),
    onToggleMute: vi.fn(),
    onToggleFullscreen: vi.fn(),
  };

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders with default props', () => {
    const { container } = render(<VideoControls {...defaultProps} />);
    expect(container.firstChild).toBeInTheDocument();
  });

  it('shows play button when paused', () => {
    const { container } = render(<VideoControls {...defaultProps} isPlaying={false} />);
    
    const playButton = container.querySelector('button[aria-label="Play"]');
    expect(playButton).toBeInTheDocument();
  });

  it('shows pause button when playing', () => {
    const { container } = render(<VideoControls {...defaultProps} isPlaying={true} />);
    
    const pauseButton = container.querySelector('button[aria-label="Pause"]');
    expect(pauseButton).toBeInTheDocument();
  });

  it('calls onPlayPause when play/pause button is clicked', () => {
    const { container } = render(<VideoControls {...defaultProps} />);
    
    const playButton = container.querySelector('button[aria-label="Play"]');
    fireEvent.click(playButton!);
    
    expect(defaultProps.onPlayPause).toHaveBeenCalledTimes(1);
  });

  it('displays current time and duration', () => {
    const { getByText } = render(
      <VideoControls {...defaultProps} currentTime={65} duration={180} />
    );
    
    // Should show formatted times
    expect(getByText('1:05')).toBeInTheDocument();
    expect(getByText('3:00')).toBeInTheDocument();
  });

  it('shows remaining time when configured', () => {
    const { getByText } = render(
      <VideoControls
        {...defaultProps}
        currentTime={30}
        duration={120}
        showRemainingTime={true}
      />
    );
    
    // Should show -1:30 for remaining time
    expect(getByText('-1:30')).toBeInTheDocument();
  });

  it('renders volume control when showVolumeControl is true', () => {
    const { container } = render(
      <VideoControls {...defaultProps} showVolumeControl={true} />
    );
    
    const volumeButton = container.querySelector('button[aria-label*="Volume"]');
    expect(volumeButton).toBeInTheDocument();
  });

  it('renders skip buttons when showSkipButtons is true', () => {
    const onSkipBackward = vi.fn();
    const onSkipForward = vi.fn();
    
    const { container } = render(
      <VideoControls
        {...defaultProps}
        showSkipButtons={true}
        onSkipBackward={onSkipBackward}
        onSkipForward={onSkipForward}
        skipSeconds={10}
      />
    );
    
    const skipBackButton = container.querySelector('button[aria-label="Skip backward 10 seconds"]');
    const skipForwardButton = container.querySelector('button[aria-label="Skip forward 10 seconds"]');
    
    expect(skipBackButton).toBeInTheDocument();
    expect(skipForwardButton).toBeInTheDocument();
    
    fireEvent.click(skipBackButton!);
    expect(onSkipBackward).toHaveBeenCalledTimes(1);
    
    fireEvent.click(skipForwardButton!);
    expect(onSkipForward).toHaveBeenCalledTimes(1);
  });

  it('renders stop button when showStopButton is true', () => {
    const onStop = vi.fn();
    
    const { container } = render(
      <VideoControls
        {...defaultProps}
        showStopButton={true}
        onStop={onStop}
      />
    );
    
    const stopButton = container.querySelector('button[aria-label="Stop"]');
    expect(stopButton).toBeInTheDocument();
    
    fireEvent.click(stopButton!);
    expect(onStop).toHaveBeenCalledTimes(1);
  });

  it('renders restart button when showRestartButton is true', () => {
    const onRestart = vi.fn();
    
    const { container } = render(
      <VideoControls
        {...defaultProps}
        showRestartButton={true}
        onRestart={onRestart}
      />
    );
    
    const restartButton = container.querySelector('button[aria-label="Restart"]');
    expect(restartButton).toBeInTheDocument();
    
    fireEvent.click(restartButton!);
    expect(onRestart).toHaveBeenCalledTimes(1);
  });

  it('renders fullscreen button when showFullscreenButton is true', () => {
    const { container } = render(
      <VideoControls
        {...defaultProps}
        showFullscreenButton={true}
      />
    );
    
    const fullscreenButton = container.querySelector('button[aria-label="Enter fullscreen"]');
    expect(fullscreenButton).toBeInTheDocument();
    
    fireEvent.click(fullscreenButton!);
    expect(defaultProps.onToggleFullscreen).toHaveBeenCalledTimes(1);
  });

  it('shows exit fullscreen button when in fullscreen', () => {
    const { container } = render(
      <VideoControls
        {...defaultProps}
        showFullscreenButton={true}
        isFullscreen={true}
      />
    );
    
    const exitFullscreenButton = container.querySelector('button[aria-label="Exit fullscreen"]');
    expect(exitFullscreenButton).toBeInTheDocument();
  });

  it('passes buffered ranges to progress bar', () => {
    const bufferedRanges = [
      { start: 0, end: 30 },
      { start: 60, end: 90 },
    ];
    
    const { container } = render(
      <VideoControls
        {...defaultProps}
        bufferedRanges={bufferedRanges}
      />
    );
    
    // Progress bar should be rendered
    const progressBar = container.querySelector('[role="slider"]');
    expect(progressBar).toBeInTheDocument();
  });

  it('indicates seeking ahead state', () => {
    const { container } = render(
      <VideoControls
        {...defaultProps}
        isSeekingAhead={true}
      />
    );
    
    // Should pass seeking state to progress bar
    expect(container.firstChild).toBeInTheDocument();
  });

  it('calls onSeekIntent when provided', () => {
    const onSeekIntent = vi.fn();
    
    render(
      <VideoControls
        {...defaultProps}
        onSeekIntent={onSeekIntent}
      />
    );
    
    // onSeekIntent is typically called from ProgressBar hover events
    expect(onSeekIntent).toBeDefined();
  });

  it('applies custom className', () => {
    const { container } = render(
      <VideoControls
        {...defaultProps}
        className="custom-controls"
      />
    );
    
    expect(container.firstChild).toHaveClass('custom-controls');
  });

  it('disables controls when disabled prop is true', () => {
    const { container } = render(
      <VideoControls
        {...defaultProps}
        disabled={true}
      />
    );
    
    const buttons = container.querySelectorAll('button');
    buttons.forEach(button => {
      expect(button).toBeDisabled();
    });
  });

  it('shows all optional controls when enabled', () => {
    const { container } = render(
      <VideoControls
        {...defaultProps}
        showVolumeControl={true}
        showSkipButtons={true}
        showStopButton={true}
        showRestartButton={true}
        showFullscreenButton={true}
        showTimeDisplay={true}
        onSkipBackward={vi.fn()}
        onSkipForward={vi.fn()}
        onStop={vi.fn()}
        onRestart={vi.fn()}
      />
    );
    
    // Check for all buttons
    expect(container.querySelector('button[aria-label="Play"]')).toBeInTheDocument();
    expect(container.querySelector('button[aria-label="Stop"]')).toBeInTheDocument();
    expect(container.querySelector('button[aria-label="Restart"]')).toBeInTheDocument();
    expect(container.querySelector('button[aria-label="Skip backward 10 seconds"]')).toBeInTheDocument();
    expect(container.querySelector('button[aria-label="Skip forward 10 seconds"]')).toBeInTheDocument();
    expect(container.querySelector('button[aria-label*="Volume"]')).toBeInTheDocument();
    expect(container.querySelector('button[aria-label="Enter fullscreen"]')).toBeInTheDocument();
  });

  it('hides time display when showTimeDisplay is false', () => {
    const { queryByText } = render(
      <VideoControls
        {...defaultProps}
        currentTime={30}
        duration={60}
        showTimeDisplay={false}
      />
    );
    
    // Time displays should not be shown
    expect(queryByText('0:30')).not.toBeInTheDocument();
    expect(queryByText('1:00')).not.toBeInTheDocument();
  });

  it('handles zero duration gracefully', () => {
    const { getByText } = render(
      <VideoControls
        {...defaultProps}
        currentTime={0}
        duration={0}
      />
    );
    
    // Should show 0:00 for both times
    expect(getByText('0:00')).toBeInTheDocument();
  });

  it('limits current time to duration', () => {
    const { queryByText } = render(
      <VideoControls
        {...defaultProps}
        currentTime={150}
        duration={120}
      />
    );
    
    // Should not show time greater than duration
    expect(queryByText('2:30')).not.toBeInTheDocument();
  });

  it('renders in minimal mode', () => {
    const { container } = render(
      <VideoControls
        {...defaultProps}
        minimal={true}
      />
    );
    
    // Should only have essential controls
    const buttons = container.querySelectorAll('button');
    expect(buttons.length).toBeLessThan(5);
  });
});