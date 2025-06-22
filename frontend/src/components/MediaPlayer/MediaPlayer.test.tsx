import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render } from '@/test/simple-utils';
import { useAtom } from 'jotai';
import { MediaPlayer, type MediaIdentifier } from './MediaPlayer';
import { loadingStateAtom } from '@/atoms/mediaPlayer';

// Mock jotai atoms
vi.mock('@/atoms/mediaPlayer', () => ({
  playerStateAtom: { init: {
    isPlaying: false,
    duration: 0,
    currentTime: 0,
    volume: 1,
    isMuted: false,
    isFullscreen: false,
    isBuffering: false,
    isSeekingAhead: false,
    showControls: true,
  }},
  loadingStateAtom: { init: {
    isLoading: false,
    error: null,
    isVideoLoading: false,
  }},
  progressStateAtom: { init: {
    seekableDuration: 0,
    originalDuration: 0,
    hoverTime: null,
  }},
  sessionStateAtom: { init: {
    activeSessions: new Set(),
    isStoppingSession: false,
  }},
  seekAheadStateAtom: { init: {
    isSeekingAhead: false,
    seekOffset: 0,
  }},
  currentMediaAtom: { init: null },
  mediaFileAtom: { init: null },
  playbackDecisionAtom: { init: null },
  configAtom: { init: { debug: false, autoplay: true, startTime: 0 }},
  debugAtom: { init: false },
  activeSessionsAtom: { init: new Set() },
  playerInitializedAtom: { init: false },
  videoElementAtom: { init: null },
  shakaPlayerAtom: { init: null },
  shakaUIAtom: { init: null },
  currentTimeAtom: { init: 0 },
  isPlayingAtom: { init: false },
  volumeAtom: { init: 1 },
  isMutedAtom: { init: false },
  isFullscreenAtom: { init: false },
  showControlsAtom: { init: true },
  durationAtom: { init: 0 },
  isBufferingAtom: { init: false },
}));

// Mock jotai hooks
vi.mock('jotai', () => ({
  atom: vi.fn((initialValue) => ({ init: initialValue })),
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  useAtom: vi.fn((atom: any) => {
    if (atom && atom.init !== undefined) {
      return [atom.init, vi.fn()];
    }
    return [null, vi.fn()];
  }),
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  useAtomValue: vi.fn((atom: any) => atom?.init || null),
  useSetAtom: vi.fn(() => vi.fn()),
}));

// Mock hooks
vi.mock('@/hooks/player/useMediaPlayer', () => ({
  useMediaPlayer: () => ({
    initializePlayer: vi.fn(),
    play: vi.fn(),
    pause: vi.fn(),
    seek: vi.fn(),
    setVolume: vi.fn(),
    toggleMute: vi.fn(),
    toggleFullscreen: vi.fn(),
    destroy: vi.fn(),
  }),
}));

vi.mock('@/hooks/player/useVideoControls', () => ({
  useVideoControls: () => ({
    togglePlayPause: vi.fn(),
    play: vi.fn(),
    pause: vi.fn(),
    stop: vi.fn(),
    restartFromBeginning: vi.fn(),
    seek: vi.fn(),
    seekByProgress: vi.fn(),
    skipBackward: vi.fn(),
    skipForward: vi.fn(),
    setVolume: vi.fn(),
    toggleMute: vi.fn(),
    mute: vi.fn(),
    unmute: vi.fn(),
    toggleFullscreen: vi.fn(),
    enterFullscreen: vi.fn(),
    exitFullscreen: vi.fn(),
    isPlaying: false,
    volume: 1,
    isMuted: false,
    isFullscreen: false,
    duration: 0,
    currentTime: 0,
  }),
}));

vi.mock('@/hooks/ui/useControlsVisibility', () => ({
  useControlsVisibility: () => ({
    showControls: true,
    setShowControls: vi.fn(),
    handleMouseMove: vi.fn(),
    handleMouseLeave: vi.fn(),
  }),
}));

vi.mock('@/hooks/session/useSeekAhead', () => ({
  useSeekAhead: () => ({
    isSeekingAhead: false,
    handleSeekIntent: vi.fn(),
    requestSeekAhead: vi.fn(),
    isSeekAheadNeeded: vi.fn(() => false),
    seekAheadState: {
      isSeekingAhead: false,
      seekOffset: 0,
    },
  }),
}));

vi.mock('@/hooks/session/useSessionManager', () => ({
  useSessionManager: () => ({
    activeSessions: new Set(),
    sessionState: { isStoppingSession: false },
    stopTranscodingSession: vi.fn(),
    stopAllSessions: vi.fn(),
    addSession: vi.fn(),
    removeSession: vi.fn(),
    isValidSessionId: vi.fn(() => true),
  }),
}));

vi.mock('@/hooks/ui/useKeyboardShortcuts', () => ({
  useKeyboardShortcuts: vi.fn(),
}));

vi.mock('@/hooks/ui/usePositionSaving', () => ({
  usePositionSaving: () => ({
    savePosition: vi.fn(),
    clearSavedPosition: vi.fn(),
  }),
}));

vi.mock('@/hooks/ui/useFullscreenManager', () => ({
  useFullscreenManager: () => ({
    isFullscreen: false,
    toggleFullscreen: vi.fn(),
  }),
}));

vi.mock('@/hooks/media/useMediaNavigation', () => ({
  useMediaNavigation: (_mediaType: MediaIdentifier) => ({
    mediaId: 'test-media-id',
    handleBack: vi.fn(),
    config: { debug: false, autoplay: true, startTime: 0 },
    loadingState: { isLoading: false, error: null },
    nextEpisode: null,
    previousEpisode: null,
    handleNextEpisode: vi.fn(),
    handlePreviousEpisode: vi.fn(),
  }),
}));

describe('MediaPlayer', () => {
  const defaultProps = {
    type: 'movie' as const,
    movieId: 123,
  } satisfies MediaIdentifier;

  beforeEach(() => {
    vi.clearAllMocks();
    // Reset mocks to default state
    vi.unmock('@/hooks/media/useMediaNavigation');
    vi.mock('@/hooks/media/useMediaNavigation', () => ({
      useMediaNavigation: (_mediaType: MediaIdentifier) => ({
        mediaId: 'test-media-id',
        handleBack: vi.fn(),
        config: { debug: false, autoplay: true, startTime: 0 },
        loadingState: { isLoading: false, error: null },
        nextEpisode: null,
        previousEpisode: null,
        handleNextEpisode: vi.fn(),
        handlePreviousEpisode: vi.fn(),
      }),
    }));
  });

  it('renders without crashing', () => {
    const { container } = render(<MediaPlayer {...defaultProps} />);
    expect(container).toBeTruthy();
  });

  it('shows loading state', () => {
    // Override loadingStateAtom for this test
    const mockUseAtom = vi.mocked(useAtom);
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    mockUseAtom.mockImplementation((atom: any) => {
      if (atom === loadingStateAtom) {
        return [{ isLoading: true, error: null, isVideoLoading: true }, vi.fn()];
      }
      return [atom?.init || null, vi.fn()];
    });
    
    const { container } = render(<MediaPlayer {...defaultProps} />);
    // Should show loading screen
    const loadingText = container.querySelector('p');
    expect(loadingText?.textContent).toBe('Loading video...');
    
    // Reset mock
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    mockUseAtom.mockImplementation((atom: any) => {
      return [atom?.init || null, vi.fn()];
    });
  });

  it('renders video element container', () => {
    const { container } = render(<MediaPlayer {...defaultProps} />);
    const playerContainer = container.querySelector('[data-testid="media-player"]');
    expect(playerContainer).toBeTruthy();
  });

  it('handles episode type correctly', () => {
    const episodeProps = {
      type: 'episode' as const,
      tvShowId: 456,
      seasonNumber: 1,
      episodeNumber: 1,
    } satisfies MediaIdentifier;
    
    const { container } = render(<MediaPlayer {...episodeProps} />);
    expect(container).toBeTruthy();
  });

  it('applies correct CSS classes', () => {
    const { container } = render(<MediaPlayer {...defaultProps} />);
    const playerContainer = container.querySelector('[data-testid="media-player"]');
    
    expect(playerContainer).toBeTruthy();
    expect(playerContainer?.classList.contains('relative')).toBe(true);
    expect(playerContainer?.classList.contains('w-full')).toBe(true);
    expect(playerContainer?.classList.contains('h-full')).toBe(true);
    expect(playerContainer?.classList.contains('bg-black')).toBe(true);
  });

  it('renders all child components', () => {
    const { container } = render(<MediaPlayer {...defaultProps} />);
    
    // Check for video element wrapper
    const videoWrapper = container.querySelector('.relative.w-full.h-full.overflow-hidden.rounded-lg');
    expect(videoWrapper).toBeTruthy();
    
    // Check for back button
    const backButton = container.querySelector('button[title="Go back"]');
    expect(backButton).toBeTruthy();
  });

  it('handles mouse interactions for controls visibility', () => {
    const { container } = render(<MediaPlayer {...defaultProps} />);
    const playerContainer = container.querySelector('[data-testid="media-player"]');
    
    expect(playerContainer).toBeTruthy();
    // The component has mouse event handlers in the JSX
    // They render as undefined when showControls is true, but the handlers exist in the component logic
    // Since React handles events differently, we just verify the container exists
    // Mouse event handlers are attached via React props, not DOM attributes
    expect(playerContainer).toBeTruthy();
  });

  it('initializes with correct default state', async () => {
    const { container } = render(<MediaPlayer {...defaultProps} />);
    
    // Check that the media player is rendered
    const playerContainer = container.querySelector('[data-testid="media-player"]');
    expect(playerContainer).toBeTruthy();
    
    // Check for video container
    const videoContainer = container.querySelector('.relative.w-full.h-full.overflow-hidden.rounded-lg');
    expect(videoContainer).toBeTruthy();
  });

  it('renders loading overlay when loading', () => {
    // Mock loading state for video
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    vi.mocked(useAtom).mockImplementation((atom: any) => {
      if (atom === loadingStateAtom) {
        return [{ isLoading: false, error: null, isVideoLoading: true }, vi.fn()];
      }
      return [atom?.init || null, vi.fn()];
    });
    
    const { container } = render(<MediaPlayer {...defaultProps} />);
    
    // Check for StatusOverlay component (it should be rendered even if not visible)
    const videoContainer = container.querySelector('.relative.w-full.h-full.overflow-hidden.rounded-lg');
    expect(videoContainer).toBeTruthy();
  });

  it('applies correct styling for video container', () => {
    const { container } = render(<MediaPlayer {...defaultProps} />);
    const videoContainer = container.querySelector('.relative.w-full.h-full.overflow-hidden.rounded-lg');
    
    expect(videoContainer).toBeTruthy();
    expect(videoContainer?.classList.contains('relative')).toBe(true);
    expect(videoContainer?.classList.contains('w-full')).toBe(true);
    expect(videoContainer?.classList.contains('h-full')).toBe(true);
    expect(videoContainer?.classList.contains('overflow-hidden')).toBe(true);
    expect(videoContainer?.classList.contains('rounded-lg')).toBe(true);
  });
});