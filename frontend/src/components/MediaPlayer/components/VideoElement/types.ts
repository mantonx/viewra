import type shaka from 'shaka-player';

export interface VideoElementProps {
  className?: string;
  onLoadedMetadata?: () => void;
  onLoadedData?: () => void;
  onTimeUpdate?: () => void;
  onPlay?: () => void;
  onPause?: () => void;
  onVolumeChange?: () => void;
  onDurationChange?: () => void;
  onCanPlay?: () => void;
  onWaiting?: () => void;
  onPlaying?: () => void;
  onStalled?: () => void;
  onDoubleClick?: () => void;
  autoPlay?: boolean;
  muted?: boolean;
  preload?: 'none' | 'metadata' | 'auto';
}

export interface VideoElementRef {
  videoElement: HTMLVideoElement | null;
  shakaPlayer: shaka.Player | null;
  loadManifest: (url: string) => Promise<void>;
  destroy: () => void;
}