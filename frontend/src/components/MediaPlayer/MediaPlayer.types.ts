/**
 * Type definitions for MediaPlayer component
 */

// MediaIdentifier type (keeping consistent structure)
export type MediaIdentifier = 
  | { type: 'movie'; movieId: number }
  | { type: 'episode'; tvShowId: number; seasonNumber: number; episodeNumber: number };

export type MediaPlayerProps = MediaIdentifier & {
  className?: string;
  autoplay?: boolean;
  onBack?: () => void;
};