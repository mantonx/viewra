/**
 * This file re-exports all icons from lucide-react to ensure they're
 * properly bundled and available in all environments including Docker
 */
import * as LucideIcons from 'lucide-react';

export const {
  Play,
  Pause,
  Volume2,
  VolumeX,
  SkipForward,
  SkipBack,
  Repeat,
  Shuffle,
  Info,
  ChevronUp,
  ChevronDown,
  Crown,
  Music,
  BadgeCheck,
  Sparkles,
  MinusCircle,
  MaximizeIcon,
  MinimizeIcon,
  X,
  Clock,
  FileAudio,
  HardDrive,
  Calendar,
  Hash,
  Disc,
  Activity,
} = LucideIcons;

// Re-export all icons if needed
export * from 'lucide-react';
