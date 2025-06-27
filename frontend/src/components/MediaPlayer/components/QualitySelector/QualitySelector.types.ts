export interface QualitySelectorProps {
  className?: string;
  onQualityChange?: (quality: VideoQuality | null) => void;
}

export interface VideoQuality {
  height: number;
  width: number;
  bitrate?: number;
  bandwidth?: number;
  label?: string;
  codec?: string;
  fps?: number;
}

export interface QualityOption {
  quality: VideoQuality | null;
  label: string;
  isAuto: boolean;
  isSelected: boolean;
  bitrate?: number;
}