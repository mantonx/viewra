import React from 'react';
import {
  AudioWaveform,
  Radio,
  Signal,
  Crown,
  Volume2,
  AlertTriangle,
  Zap,
} from '@/components/ui/icons';
import { cn } from '@/lib/utils';
import { getAudioBadge } from './audioBadgeUtils';

interface AudioBadgeProps {
  format: string;
  bitrate: number;
  isMinimized?: boolean;
  className?: string;
}

// Icon mapping object for better type safety
const iconMap = {
  Crown,
  AudioWaveform,
  Volume2,
  Signal,
  Radio,
  Zap,
  AlertTriangle,
} as const;

const AudioBadge: React.FC<AudioBadgeProps> = ({
  format,
  bitrate,
  isMinimized = false,
  className,
}) => {
  const badgeInfo = getAudioBadge(format, bitrate);
  const IconComponent = iconMap[badgeInfo.iconName as keyof typeof iconMap];

  return (
    <div
      className={cn(
        'inline-flex items-center gap-1.5 px-3 py-1.5 rounded-full font-semibold border transition-all duration-300 shadow-md text-sm',
        'backdrop-blur-md bg-black/10 dark:bg-white/5',
        'hover:scale-105 hover:shadow-lg hover:backdrop-blur-lg hover:bg-black/20 dark:hover:bg-white/10',
        'ring-1 ring-white/20 dark:ring-white/10',
        badgeInfo.className,
        isMinimized ? 'px-2 py-1 text-xs gap-1' : 'px-3 py-1.5 text-sm gap-1.5',
        className
      )}
      title={badgeInfo.tooltip}
    >
      <div className="flex items-center justify-center">
        <IconComponent className={cn('drop-shadow-lg', isMinimized ? 'w-3 h-3' : 'w-3.5 h-3.5')} />
      </div>
      <span className="font-semibold tracking-wide uppercase drop-shadow-sm">
        {badgeInfo.label}
      </span>
    </div>
  );
};

export default AudioBadge;
