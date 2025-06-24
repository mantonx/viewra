export interface StatusOverlayProps {
  isBuffering: boolean;
  isSeekingAhead: boolean;
  isLoading: boolean;
  error?: string | null;
  playbackInfo?: {
    isTranscoding: boolean;
    reason: string;
    sessionCount: number;
  };
  className?: string;
  showPlaybackInfo?: boolean;
}