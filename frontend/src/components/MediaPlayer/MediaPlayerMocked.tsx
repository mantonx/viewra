import React, { useState, useRef, useEffect } from 'react';
import { Provider } from 'jotai';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { 
  playerStateAtom, 
  videoElementAtom, 
  shakaPlayerAtom,
  loadingStateAtom,
  currentMediaAtom,
  playbackDecisionAtom,
  configAtom,
  progressStateAtom,
  activeSessionsAtom,
  seekAheadStateAtom,
  sessionStateAtom,
  mediaFileAtom,
  playerInitializedAtom
} from '@/atoms/mediaPlayer';
import type { Episode, Movie } from './types';

// Mock the hooks at module level
const mockMediaNavigation = {
  mediaId: 'mock-id',
  currentMedia: null,
  mediaFile: null,
  handleBack: () => console.log('Back clicked'),
  loadMediaData: () => {},
  getSavedPosition: () => 0,
  savePosition: () => {},
  clearSavedPosition: () => {},
  getStartPosition: () => 0,
  config: { debug: false, autoplay: false, startTime: 0 },
  loadingState: { isLoading: false, error: null, isVideoLoading: false },
};

// Override the hook module
jest.mock('@/hooks/media/useMediaNavigation', () => ({
  useMediaNavigation: () => mockMediaNavigation
}));

jest.mock('@/hooks/player/useMediaPlayer', () => ({
  useMediaPlayer: () => ({})
}));

jest.mock('@/hooks/player/useVideoControls', () => ({
  useVideoControls: () => ({
    togglePlayPause: () => {},
    stop: () => {},
    restartFromBeginning: () => {},
    seek: () => {},
    skipForward: () => {},
    skipBackward: () => {},
    setVolume: () => {},
    toggleMute: () => {},
  })
}));

jest.mock('@/hooks/session/useSessionManager', () => ({
  useSessionManager: () => ({
    stopTranscodingSession: () => {},
  })
}));

jest.mock('@/hooks/session/useSeekAhead', () => ({
  useSeekAhead: () => ({
    requestSeekAhead: () => Promise.resolve(),
    isSeekAheadNeeded: () => false,
    seekAheadState: { isSeekingAhead: false, seekOffset: 0 },
  })
}));

jest.mock('@/hooks/ui/useFullscreenManager', () => ({
  useFullscreenManager: () => ({
    isFullscreen: false,
    toggleFullscreen: () => {},
  })
}));

jest.mock('@/hooks/ui/useControlsVisibility', () => ({
  useControlsVisibility: () => ({
    showControls: true,
  })
}));

jest.mock('@/hooks/ui/usePositionSaving', () => ({
  usePositionSaving: () => ({
    savePosition: () => {},
    clearSavedPosition: () => {},
  })
}));

jest.mock('@/hooks/ui/useKeyboardShortcuts', () => ({
  useKeyboardShortcuts: () => {}
}));

// Import MediaPlayer after mocking
import { MediaPlayer } from './MediaPlayer';

interface MediaPlayerMockedProps {
  mediaType: 'episode' | 'movie';
  autoplay?: boolean;
  initialTime?: number;
  isPlaying?: boolean;
  showBuffering?: boolean;
  showError?: boolean;
  errorMessage?: string;
}

// Mock data
const mockEpisode: Episode = {
  id: 'ep-123',
  type: 'episode',
  title: 'The Beginning',
  episode_number: 1,
  season_number: 1,
  description: 'In the series premiere, our heroes embark on an epic journey that will change their lives forever.',
  duration: 2700,
  air_date: '2024-01-15',
  series: {
    id: 'series-456',
    title: 'Epic Adventure Series',
    description: 'An amazing series about adventure and discovery',
  },
};

const mockMovie: Movie = {
  id: 'movie-789',
  type: 'movie',
  title: 'The Great Adventure',
  description: 'An epic tale of courage, friendship, and discovery in a world of wonder.',
  duration: 7200,
  release_date: '2023-12-25',
  runtime: 120,
};

export const MediaPlayerMocked: React.FC<MediaPlayerMockedProps> = ({
  mediaType,
  autoplay = false,
  initialTime = 0,
  isPlaying = false,
  showBuffering = false,
  showError = false,
  errorMessage = 'Failed to load video',
}) => {
  const media = mediaType === 'episode' ? mockEpisode : mockMovie;
  const duration = media.duration || 3600;
  const [currentTime, setCurrentTime] = useState(initialTime);

  // Create mock store
  const mockStore = new Map();

  // Create mock video element
  const mockVideo = useRef<HTMLVideoElement>();
  if (typeof document !== 'undefined' && !mockVideo.current) {
    mockVideo.current = document.createElement('video');
    Object.defineProperty(mockVideo.current, 'duration', { value: duration, configurable: true });
    Object.defineProperty(mockVideo.current, 'currentTime', { 
      get: () => currentTime,
      set: (value) => setCurrentTime(value),
      configurable: true 
    });
    Object.defineProperty(mockVideo.current, 'volume', { value: 0.7, configurable: true });
    Object.defineProperty(mockVideo.current, 'muted', { value: false, configurable: true });
  }

  // Update time if playing
  useEffect(() => {
    if (isPlaying && !showError && !showBuffering) {
      const interval = setInterval(() => {
        setCurrentTime(prev => Math.min(prev + 1, duration));
      }, 1000);
      return () => clearInterval(interval);
    }
  }, [isPlaying, showError, showBuffering, duration]);

  // Set up all atoms
  mockStore.set(loadingStateAtom, {
    isLoading: false,
    error: showError ? errorMessage : null,
    isVideoLoading: false,
  });

  mockStore.set(videoElementAtom, mockVideo.current || null);
  
  mockStore.set(playerStateAtom, {
    isPlaying,
    currentTime,
    duration,
    volume: 0.7,
    isMuted: false,
    isBuffering: showBuffering,
    isSeekingAhead: false,
    isFullscreen: false,
    showControls: true,
  });

  mockStore.set(currentMediaAtom, media);

  mockStore.set(mediaFileAtom, {
    id: 'mock-file-1',
    path: '/mock/path/to/video.mp4',
    container: 'mp4',
    video_codec: 'h264',
    audio_codec: 'aac',
    resolution: '1920x1080',
    duration,
    size_bytes: 1000000,
  });

  mockStore.set(playbackDecisionAtom, {
    should_transcode: false,
    reason: 'Direct play supported',
    stream_url: 'https://test-videos.co.uk/vids/bigbuckbunny/mp4/h264/1080/Big_Buck_Bunny_1080_10s_1MB.mp4',
    media_info: {
      id: 'mock-file-1',
      container: 'mp4',
      video_codec: 'h264',
      audio_codec: 'aac',
      resolution: '1920x1080',
      duration,
      size_bytes: 1000000,
    },
  });

  mockStore.set(configAtom, {
    debug: false,
    autoplay,
    startTime: 0,
  });

  mockStore.set(progressStateAtom, {
    seekableDuration: duration,
    originalDuration: duration,
    hoverTime: null,
  });

  mockStore.set(sessionStateAtom, {
    activeSessions: new Set<string>(),
    isStoppingSession: false,
  });

  mockStore.set(seekAheadStateAtom, {
    isSeekingAhead: false,
    seekOffset: 0,
  });

  mockStore.set(activeSessionsAtom, new Set<string>());
  mockStore.set(shakaPlayerAtom, null);
  mockStore.set(playerInitializedAtom, true);

  return (
    <Provider store={mockStore as any}>
      <MemoryRouter initialEntries={[`/player/${mediaType}/${media.id}`]}>
        <Routes>
          <Route 
            path="/player/:mediaType/:id" 
            element={
              mediaType === 'episode' ? (
                <MediaPlayer 
                  type="episode" 
                  tvShowId={1} 
                  seasonNumber={1} 
                  episodeNumber={1} 
                  autoplay={autoplay} 
                />
              ) : (
                <MediaPlayer 
                  type="movie" 
                  movieId={1} 
                  autoplay={autoplay} 
                />
              )
            } 
          />
        </Routes>
      </MemoryRouter>
    </Provider>
  );
};