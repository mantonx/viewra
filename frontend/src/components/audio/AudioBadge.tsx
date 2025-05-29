import React from 'react';
import { cn } from '@/lib/utils';
import { getAudioBadge } from './audioBadgeUtils';

interface AudioBadgeProps {
  format: string;
  bitrate: number;
  isMinimized?: boolean;
  className?: string;
}

const AudioBadge: React.FC<AudioBadgeProps> = ({
  format,
  bitrate,
  isMinimized = false,
  className,
}) => {
  const badgeInfo = getAudioBadge(format, bitrate);

  return (
    <div
      className={cn(
        'inline-flex items-center justify-center px-3 py-1.5 rounded-full font-semibold border transition-all duration-300 shadow-md text-sm',
        'backdrop-blur-md bg-black/10 dark:bg-white/5',
        'hover:scale-105 hover:shadow-lg hover:backdrop-blur-lg hover:bg-black/20 dark:hover:bg-white/10',
        'ring-1 ring-white/20 dark:ring-white/10 cursor-help',
        badgeInfo.className,
        isMinimized ? 'px-2 py-1 text-xs' : 'px-3 py-1.5 text-sm',
        className
      )}
      data-tooltip-id="audio-quality-tooltip"
      data-tooltip-content={badgeInfo.tooltip}
    >
      <span className="font-semibold tracking-wide uppercase drop-shadow-sm">
        {badgeInfo.label}
      </span>
    </div>
  );
};

export default AudioBadge;
