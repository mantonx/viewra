export interface VideoControlsProps {
  // Playback state
  isPlaying: boolean;
  currentTime: number;
  duration: number;
  volume: number;
  isMuted: boolean;
  isFullscreen: boolean;
  
  // Buffering state
  bufferedRanges: Array<{ start: number; end: number }>;
  isSeekingAhead: boolean;
  
  // Control actions
  onPlayPause: () => void;
  onStop?: () => void;
  onRestart?: () => void;
  onSeek: (progress: number) => void;
  onSeekIntent?: (time: number) => void;
  onSkipBackward?: () => void;
  onSkipForward?: () => void;
  onVolumeChange: (volume: number) => void;
  onToggleMute: () => void;
  onToggleFullscreen: () => void;
  
  // UI options
  className?: string;
  showStopButton?: boolean;
  showSkipButtons?: boolean;
  showVolumeControl?: boolean;
  showFullscreenButton?: boolean;
  showTimeDisplay?: boolean;
  skipSeconds?: number;
}