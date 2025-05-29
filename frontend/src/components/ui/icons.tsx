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
  AudioWaveform,
  Radio,
  Signal,
  Zap,
  AlertTriangle,
  Grid,
  List,
  TrendingUp,
} = LucideIcons;

// Note: We don't re-export all icons to avoid conflicts with other types
// If you need additional icons, add them to the destructured export above
