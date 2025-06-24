import type { MediaItem } from '../../types';

export interface MediaInfoOverlayProps {
  media: MediaItem | null;
  className?: string;
  position?: 'top-left' | 'top-right' | 'bottom-left' | 'bottom-right';
  showOnHover?: boolean;
  autoHide?: boolean;
  autoHideDelay?: number;
}