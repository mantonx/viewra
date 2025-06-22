export interface BaseMedia {
  id: string;
  title: string;
  description?: string;
  duration?: number;
  poster?: string;
  backdrop?: string;
}

export interface Episode extends BaseMedia {
  type: 'episode';
  episode_number: number;
  season_number: number;
  air_date?: string;
  still_image?: string;
  series: {
    id: string;
    title: string;
    description?: string;
    poster?: string;
    backdrop?: string;
    tmdb_id?: string;
  };
}

export interface Movie extends BaseMedia {
  type: 'movie';
  release_date?: string;
  runtime?: number;
}

export type MediaItem = Episode | Movie;
export type MediaType = 'episode' | 'movie';

export interface MediaFile {
  id: string;
  path: string;
  container?: string;
  video_codec?: string;
  audio_codec?: string;
  resolution?: string;
  duration?: number;
  size_bytes: number;
}

export interface PlaybackDecision {
  should_transcode: boolean;
  reason: string;
  direct_play_url?: string;
  stream_url: string;
  manifest_url?: string;
  media_info: {
    id: string;
    container: string;
    video_codec: string;
    audio_codec: string;
    resolution: string;
    duration: number;
    size_bytes: number;
  };
  transcode_params?: {
    target_codec: string;
    target_container: string;
    resolution: string;
    bitrate: number;
  };
  session_id?: string;
}

export interface DeviceProfile {
  user_agent: string;
  supported_codecs: string[];
  max_resolution: string;
  max_bitrate: number;
  supports_hevc: boolean;
  target_container: string;
}

export interface PlayerState {
  isPlaying: boolean;
  duration: number;
  currentTime: number;
  volume: number;
  isMuted: boolean;
  isFullscreen: boolean;
  isBuffering: boolean;
  isSeekingAhead: boolean;
  showControls: boolean;
}

export interface SessionState {
  activeSessions: Set<string>;
  isStoppingSession: boolean;
}

export interface SeekAheadState {
  isSeekingAhead: boolean;
  seekOffset: number;
}

export interface ProgressState {
  seekableDuration: number;
  originalDuration: number;
  hoverTime: number | null;
}

export interface MediaPlayerConfig {
  debug?: boolean;
  autoplay?: boolean;
  startTime?: number;
  onBack?: () => void;
}

export interface VideoEventHandlers {
  onLoadedMetadata: () => void;
  onLoadedData: () => void;
  onTimeUpdate: () => void;
  onPlay: () => void;
  onPause: () => void;
  onVolumeChange: () => void;
  onDurationChange: () => void;
  onCanPlay: () => void;
  onWaiting: () => void;
  onPlaying: () => void;
  onStalled: () => void;
}

export interface ShakaPlayerConfig {
  manifest: {
    defaultPresentationDelay: number;
    availabilityWindowOverride: number;
    dash: {
      ignoreSuggestedPresentationDelay: boolean;
      autoCorrectDrift: boolean;
    };
  };
  streaming: {
    bufferingGoal: number;
    rebufferingGoal: number;
    bufferBehind: number;
    retryParameters: {
      maxAttempts: number;
      baseDelay: number;
      backoffFactor: number;
      fuzzFactor: number;
      timeout: number;
      stallTimeout: number;
      connectionTimeout: number;
    };
    jumpLargeGaps: boolean;
    forceTransmuxTS: boolean;
    forceHTTPS: boolean;
    segmentPrefetchLimit: number;
    stallEnabled: boolean;
    stallThreshold: number;
    stallSkip: number;
    maxDisabledTime: number;
    inaccurateManifestTolerance: number;
  };
  abr: {
    enabled: boolean;
    defaultBandwidthEstimate: number;
    switchInterval: number;
    bandwidthUpgradeTarget: number;
    bandwidthDowngradeTarget: number;
    restrictToElementSize: boolean;
    restrictToScreenSize: boolean;
    ignoreDevicePixelRatio: boolean;
    clearBufferSwitch: boolean;
    useNetworkInformation: boolean;
  };
  drm: {
    retryParameters: {
      maxAttempts: number;
      baseDelay: number;
      backoffFactor: number;
    };
  };
  preferredAudioLanguage: string;
  preferredTextLanguage: string;
  preferredVariantRole: string;
}

export interface TranscodingSession {
  id: string;
  input_path: string;
  output_path: string;
  container: string;
  video_codec: string;
  audio_codec: string;
  quality: number;
  speed_priority: string;
}

export interface SeekAheadRequest {
  session_id: string;
  seek_position: number;
}

export interface SeekAheadResponse {
  session_id: string;
  manifest_url: string;
  seek_position: number;
}