import React from 'react';
import { Provider } from 'jotai';
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

// Create a mock store with all required atoms
export const createMockStore = (overrides?: any) => {
  const store = new Map();
  
  // Mock loading state - NOT loading
  store.set(loadingStateAtom, {
    isLoading: false,
    error: null,
    isVideoLoading: false,
    ...overrides?.loadingState,
  });

  // Mock video element
  if (typeof document !== 'undefined') {
    const mockVideo = document.createElement('video');
    store.set(videoElementAtom, mockVideo);
  } else {
    store.set(videoElementAtom, null);
  }
  
  // Mock player state
  store.set(playerStateAtom, {
    isPlaying: false,
    currentTime: 0,
    duration: 3600, // 1 hour
    volume: 1,
    isMuted: false,
    isBuffering: false,
    isSeekingAhead: false,
    isFullscreen: false,
    showControls: true,
    ...overrides?.playerState,
  });

  // Mock media
  store.set(currentMediaAtom, overrides?.currentMedia || {
    id: 'mock-episode-1',
    type: 'episode',
    title: 'The Beginning',
    episode_number: 1,
    season_number: 1,
    description: 'The first episode of an amazing series',
    duration: 3600,
    air_date: '2024-01-01',
    series: {
      id: 'mock-series-1',
      title: 'Amazing Series',
      description: 'An amazing series for testing',
    },
  });

  // Mock media file
  store.set(mediaFileAtom, {
    id: 'mock-file-1',
    path: '/mock/path/to/video.mp4',
    container: 'mp4',
    video_codec: 'h264',
    audio_codec: 'aac',
    resolution: '1920x1080',
    duration: 3600,
    size_bytes: 1000000,
    ...overrides?.mediaFile,
  });

  // Mock playback decision
  store.set(playbackDecisionAtom, {
    should_transcode: false,
    reason: 'Direct play supported',
    stream_url: 'https://test-videos.co.uk/vids/bigbuckbunny/mp4/h264/1080/Big_Buck_Bunny_1080_10s_1MB.mp4',
    media_info: {
      id: 'mock-file-1',
      container: 'mp4',
      video_codec: 'h264',
      audio_codec: 'aac',
      resolution: '1920x1080',
      duration: 3600,
      size_bytes: 1000000,
    },
    ...overrides?.playbackDecision,
  });

  // Mock config
  store.set(configAtom, {
    debug: false,
    autoplay: false,
    startTime: 0,
    ...overrides?.config,
  });

  // Mock progress state
  store.set(progressStateAtom, {
    seekableDuration: 3600,
    originalDuration: 3600,
    hoverTime: null,
    ...overrides?.progressState,
  });

  // Mock session state
  store.set(sessionStateAtom, {
    activeSessions: new Set(),
    isStoppingSession: false,
    ...overrides?.sessionState,
  });

  // Mock seek ahead state
  store.set(seekAheadStateAtom, {
    isSeekingAhead: false,
    seekOffset: 0,
    ...overrides?.seekAheadState,
  });

  // Mock active sessions
  store.set(activeSessionsAtom, new Set());

  // Mock shaka player
  store.set(shakaPlayerAtom, null);

  // Mock player initialized
  store.set(playerInitializedAtom, true);

  return store;
};

export const MediaPlayerDecorator = (overrides?: any) => (Story: any) => {
  const mockStore = createMockStore(overrides);
  
  return (
    <Provider store={mockStore as any}>
      <Story />
    </Provider>
  );
};