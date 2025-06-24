export interface ProgressBarProps {
  currentTime: number;
  duration: number;
  bufferedRanges: Array<{ start: number; end: number }>;
  isSeekable: boolean;
  isSeekingAhead: boolean;
  onSeek: (progress: number) => void;
  onSeekIntent?: (time: number) => void;
  className?: string;
  showTooltip?: boolean;
  showBuffered?: boolean;
  showSeekAheadIndicator?: boolean;
}

export interface ProgressBarState {
  hoverTime: number | null;
  isHovering: boolean;
  isDragging: boolean;
}