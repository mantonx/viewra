import React, { useState, useRef, useEffect } from 'react';
import { Provider } from 'jotai';
import { useHydrateAtoms } from 'jotai/utils';
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
import type { Episode, Movie } from '@/components/MediaPlayer/types';

interface MediaPlayerMockProps {
  children: React.ReactNode;
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

function MediaPlayerAtomsProvider({ 
  children, 
  mediaType,
  autoplay = false,
  initialTime = 0,
  isPlaying = false,
  showBuffering = false,
  showError = false,
  errorMessage = 'Failed to load video',
}: MediaPlayerMockProps) {
  const media = mediaType === 'episode' ? mockEpisode : mockMovie;
  const duration = media.duration || 3600;
  const [currentTime, setCurrentTime] = useState(initialTime);

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

  const initialValues = [
    [loadingStateAtom, {
      isLoading: false,
      error: showError ? errorMessage : null,
      isVideoLoading: false,
    }],
    [videoElementAtom, mockVideo.current || null],
    [playerStateAtom, {
      isPlaying,
      currentTime,
      duration,
      volume: 0.7,
      isMuted: false,
      isBuffering: showBuffering,
      isSeekingAhead: false,
      isFullscreen: false,
      showControls: true,
    }],
    [currentMediaAtom, media],
    [mediaFileAtom, {
      id: 'mock-file-1',
      path: '/mock/path/to/video.mp4',
      container: 'mp4',
      video_codec: 'h264',
      audio_codec: 'aac',
      resolution: '1920x1080',
      duration,
      size_bytes: 1000000,
    }],
    [playbackDecisionAtom, {
      should_transcode: false,
      reason: 'Direct play supported',
      stream_url: 'mock-stream-url',
      media_info: {
        id: 'mock-file-1',
        container: 'mp4',
        video_codec: 'h264',
        audio_codec: 'aac',
        resolution: '1920x1080',
        duration,
        size_bytes: 1000000,
      },
    }],
    [configAtom, {
      debug: false,
      autoplay,
      startTime: 0,
    }],
    [progressStateAtom, {
      seekableDuration: duration,
      originalDuration: duration,
      hoverTime: null,
    }],
    [sessionStateAtom, {
      activeSessions: new Set(),
      isStoppingSession: false,
    }],
    [seekAheadStateAtom, {
      isSeekingAhead: false,
      seekOffset: 0,
    }],
    [activeSessionsAtom, new Set()],
    [shakaPlayerAtom, null],
    [playerInitializedAtom, true],
  ] as const;

  useHydrateAtoms(initialValues);

  // Update atoms when props change
  useEffect(() => {
    // Update currentTime in playerState atom
    const interval = setInterval(() => {
      if (mockVideo.current) {
        Object.defineProperty(mockVideo.current, 'currentTime', { 
          value: currentTime,
          configurable: true 
        });
      }
    }, 100);
    return () => clearInterval(interval);
  }, [currentTime]);

  return <>{children}</>;
}

export function MediaPlayerMock(props: MediaPlayerMockProps) {
  return (
    <Provider>
      <MediaPlayerAtomsProvider {...props}>
        {props.children}
      </MediaPlayerAtomsProvider>
    </Provider>
  );
}