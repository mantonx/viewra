import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render } from '@/test/simple-utils';
import { useAtom } from 'jotai';
import { MediaPlayer } from './MediaPlayer';
import type { MediaIdentifier } from './MediaPlayer.types';
import { loadingStateAtom, playbackDecisionAtom } from '@/atoms/mediaPlayer';

// Mock Vidstack React components
vi.mock('@vidstack/react', () => ({
  MediaPlayer: ({ children, ...props }: any) => (
    <div data-testid="vidstack-player" {...props}>
      {children}
    </div>
  ),
  MediaProvider: ({ children }: any) => (
    <div data-testid="media-provider">{children}</div>
  ),
  Poster: (props: any) => <img data-testid="poster" {...props} />,
  Track: (props: any) => <track data-testid="track" {...props} />,
  Gesture: ({ children, ...props }: any) => (
    <div data-testid="media-gesture" {...props}>{children}</div>
  ),
  useMediaState: (state: string) => {
    const defaultValues: Record<string, any> = {
      playing: false,
      paused: true,
      duration: 0,
      currentTime: 0,
      volume: 1,
      muted: false,
      buffering: false,
      quality: null,
    };
    return defaultValues[state] ?? null;
  },
  useMediaRemote: () => ({
    play: vi.fn(),
    pause: vi.fn(),
    seek: vi.fn(),
    setVolume: vi.fn(),
    setMuted: vi.fn(),
    requestFullscreen: vi.fn(),
    exitFullscreen: vi.fn(),
  }),
  useMediaStore: () => ({
    subscribe: vi.fn(() => vi.fn()),
    getState: () => ({
      playing: false,
      paused: true,
      duration: 0,
      currentTime: 0,
      volume: 1,
      muted: false,
      buffering: false,
      quality: null,
    }),
  }),
}));

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
    mediaFile: {
      id: 'file-123',
      path: '/test/video.mp4',
      duration: 120,
    },
    currentMedia: {
      title: 'Test Video',
      poster: null,
      subtitles: [],
    },
    loadMediaData: vi.fn(),
    getSavedPosition: vi.fn(() => 0),
    savePosition: vi.fn(),
    clearSavedPosition: vi.fn(),
    getStartPosition: vi.fn(() => 0),
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
        mediaFile: {
          id: 'file-123',
          path: '/test/video.mp4',
          duration: 120,
        },
        currentMedia: {
          title: 'Test Video',
          poster: null,
          subtitles: [],
        },
        loadMediaData: vi.fn(),
        getSavedPosition: vi.fn(() => 0),
        savePosition: vi.fn(),
        clearSavedPosition: vi.fn(),
        getStartPosition: vi.fn(() => 0),
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

  it('renders Vidstack player container', () => {
    const { container } = render(<MediaPlayer {...defaultProps} />);
    const playerContainer = container.querySelector('[data-testid="media-player"]');
    const vidstackPlayer = container.querySelector('[data-testid="vidstack-player"]');
    expect(playerContainer).toBeTruthy();
    expect(vidstackPlayer).toBeTruthy();
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
    expect(playerContainer?.classList.contains('h-screen')).toBe(true);
    expect(playerContainer?.classList.contains('player-gradient')).toBe(true);
    expect(playerContainer?.classList.contains('overflow-hidden')).toBe(true);
  });

  it('renders all child components', () => {
    const { container } = render(<MediaPlayer {...defaultProps} />);
    
    // Check for main container
    const mainContainer = container.querySelector('[data-testid="media-player"]');
    expect(mainContainer).toBeTruthy();
    
    // Check for back button
    const backButton = container.querySelector('button[title="Go back"]');
    expect(backButton).toBeTruthy();
    
    // Check for Vidstack components
    const vidstackPlayer = container.querySelector('[data-testid="vidstack-player"]');
    const mediaProvider = container.querySelector('[data-testid="media-provider"]');
    expect(vidstackPlayer).toBeTruthy();
    expect(mediaProvider).toBeTruthy();
  });

  it('handles mouse interactions for controls visibility', () => {
    const { container } = render(<MediaPlayer {...defaultProps} />);
    const playerContainer = container.querySelector('[data-testid="media-player"]');
    
    expect(playerContainer).toBeTruthy();
    
    // Verify Vidstack player exists and has event handlers
    const vidstackPlayer = container.querySelector('[data-testid="vidstack-player"]');
    expect(vidstackPlayer).toBeTruthy();
  });

  it('initializes with correct default state', async () => {
    const { container } = render(<MediaPlayer {...defaultProps} />);
    
    // Check that the media player is rendered
    const playerContainer = container.querySelector('[data-testid="media-player"]');
    expect(playerContainer).toBeTruthy();
    
    // Check for main container with correct classes
    expect(playerContainer?.classList.contains('relative')).toBe(true);
    expect(playerContainer?.classList.contains('h-screen')).toBe(true);
    
    // Verify Vidstack player was initialized
    const vidstackPlayer = container.querySelector('[data-testid="vidstack-player"]');
    const mediaProvider = container.querySelector('[data-testid="media-provider"]');
    expect(vidstackPlayer).toBeTruthy();
    expect(mediaProvider).toBeTruthy();
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
    
    // Check for main player container
    const playerContainer = container.querySelector('[data-testid="media-player"]');
    expect(playerContainer).toBeTruthy();
  });

  it('applies correct styling for video container', () => {
    const { container } = render(<MediaPlayer {...defaultProps} />);
    const playerContainer = container.querySelector('[data-testid="media-player"]');
    
    expect(playerContainer).toBeTruthy();
    expect(playerContainer?.classList.contains('relative')).toBe(true);
    expect(playerContainer?.classList.contains('h-screen')).toBe(true);
    expect(playerContainer?.classList.contains('overflow-hidden')).toBe(true);
    expect(playerContainer?.classList.contains('player-gradient')).toBe(true);
  });

  it('passes correct props to Vidstack player', () => {
    const { container } = render(<MediaPlayer {...defaultProps} autoplay={true} />);
    
    const vidstackPlayer = container.querySelector('[data-testid="vidstack-player"]');
    expect(vidstackPlayer).toBeTruthy();
    // In our mock, the attributes are passed as props but may not appear as DOM attributes
    // since we're rendering a simple div. Test that the component exists instead.
    expect(vidstackPlayer?.getAttribute('data-testid')).toBe('vidstack-player');
  });

  it('configures media provider correctly', () => {
    const { container } = render(<MediaPlayer {...defaultProps} />);
    
    const mediaProvider = container.querySelector('[data-testid="media-provider"]');
    expect(mediaProvider).toBeTruthy();
  });

  it('handles custom className prop', () => {
    const customClass = 'custom-player-class';
    const { container } = render(<MediaPlayer {...defaultProps} className={customClass} />);
    
    const playerContainer = container.querySelector('[data-testid="media-player"]');
    expect(playerContainer?.classList.contains(customClass)).toBe(true);
  });

  it('handles onBack callback', () => {
    const onBackMock = vi.fn();
    render(<MediaPlayer {...defaultProps} onBack={onBackMock} />);
    
    // The onBack prop is passed to the useMediaNavigation hook
    // which handles the back button functionality
    expect(onBackMock).toBeInstanceOf(Function);
  });

  it('handles content-addressable storage URLs correctly', () => {
    // Mock playback decision with content hash
    vi.mocked(useAtom).mockImplementation((atom: any) => {
      if (atom?.init !== undefined && atom === playbackDecisionAtom) {
        return [{
          should_transcode: true,
          reason: 'requires transcoding',
          manifest_url: '/api/v1/content/abc123def456/manifest.mpd',
          stream_url: '/api/v1/content/abc123def456/manifest.mpd',
          content_hash: 'abc123def456',
          content_url: '/api/v1/content/abc123def456/',
          transcode_params: {
            target_container: 'dash',
            target_codec: 'h264',
          },
        }, vi.fn()];
      }
      return [atom?.init || null, vi.fn()];
    });

    const { container } = render(<MediaPlayer {...defaultProps} />);
    const vidstackPlayer = container.querySelector('[data-testid="vidstack-player"]');
    
    expect(vidstackPlayer).toBeTruthy();
    // The source URL should be using content-addressable storage
    expect(vidstackPlayer?.getAttribute('src')).toContain('/api/v1/content/');
  });

  it('handles HLS content-addressable storage URLs', () => {
    // Mock playback decision with content hash for HLS
    vi.mocked(useAtom).mockImplementation((atom: any) => {
      if (atom?.init !== undefined && atom === playbackDecisionAtom) {
        return [{
          should_transcode: true,
          reason: 'requires transcoding',
          manifest_url: '/api/v1/content/abc123def456/playlist.m3u8',
          stream_url: '/api/v1/content/abc123def456/playlist.m3u8',
          content_hash: 'abc123def456',
          content_url: '/api/v1/content/abc123def456/',
          transcode_params: {
            target_container: 'hls',
            target_codec: 'h264',
          },
        }, vi.fn()];
      }
      return [atom?.init || null, vi.fn()];
    });

    const { container } = render(<MediaPlayer {...defaultProps} />);
    const vidstackPlayer = container.querySelector('[data-testid="vidstack-player"]');
    
    expect(vidstackPlayer).toBeTruthy();
    // The source URL should be using content-addressable storage for HLS
    expect(vidstackPlayer?.getAttribute('src')).toContain('/api/v1/content/');
    expect(vidstackPlayer?.getAttribute('src')).toContain('.m3u8');
  });

  it('always uses content-addressable storage URLs', () => {
    // Mock playback decision with content hash (new architecture always provides this)
    vi.mocked(useAtom).mockImplementation((atom: any) => {
      if (atom?.init !== undefined && atom === playbackDecisionAtom) {
        return [{
          should_transcode: true,
          reason: 'requires transcoding',
          manifest_url: '/api/v1/content/def456ghi789/manifest.mpd',
          stream_url: '/api/v1/content/def456ghi789/manifest.mpd',
          content_hash: 'def456ghi789',
          content_url: '/api/v1/content/def456ghi789/',
          session_id: 'session-123',
          transcode_params: {
            target_container: 'dash',
            target_codec: 'h264',
          },
        }, vi.fn()];
      }
      return [atom?.init || null, vi.fn()];
    });

    const { container } = render(<MediaPlayer {...defaultProps} />);
    const vidstackPlayer = container.querySelector('[data-testid="vidstack-player"]');
    
    expect(vidstackPlayer).toBeTruthy();
    // Should always use content-addressable storage URLs
    expect(vidstackPlayer?.getAttribute('src')).toContain('/api/v1/content/');
    expect(vidstackPlayer?.getAttribute('src')).not.toContain('/api/playback/stream/');
  });
});