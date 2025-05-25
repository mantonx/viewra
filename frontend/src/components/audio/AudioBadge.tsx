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

interface AudioBadgeProps {
  format: string;
  bitrate: number;
  isMinimized?: boolean;
  className?: string;
}

export interface AudioBadgeInfo {
  label: string;
  className: string;
  icon?: React.ReactNode;
  tooltip: string;
}

export const getAudioBadge = (format: string, bitrate: number): AudioBadgeInfo => {
  const upperFormat = format.toUpperCase();

  // FLAC - Lossless format
  if (upperFormat === 'FLAC') {
    return {
      label: 'FLAC',
      className: 'bg-green-500 text-white',
      icon: <Crown className="drop-shadow-lg" />,
      tooltip: 'Lossless FLAC format - highest quality',
    };
  }

  // WAV - Raw PCM
  if (upperFormat === 'WAV') {
    return {
      label: 'WAV',
      className: 'bg-gray-500 text-white',
      icon: <AudioWaveform className="drop-shadow-lg" />,
      tooltip: 'Uncompressed WAV format - lossless',
    };
  }

  // MP3 High Quality (320kbps)
  if (upperFormat === 'MP3' && bitrate >= 320000) {
    return {
      label: 'MP3 320',
      className: 'bg-blue-500 text-white',
      icon: <Volume2 className="drop-shadow-lg" />,
      tooltip: 'High quality MP3 at 320kbps',
    };
  }

  // MP3 Variable Bitrate or standard quality
  if (upperFormat === 'MP3') {
    return {
      label: 'MP3 VBR',
      className: 'bg-blue-400 text-white',
      icon: <Signal className="drop-shadow-lg" />,
      tooltip: `MP3 variable bitrate (~${Math.round(bitrate / 1000)}kbps)`,
    };
  }

  // AAC High Quality
  if (upperFormat === 'AAC' || upperFormat === 'M4A') {
    return {
      label: 'AAC 256',
      className: 'bg-purple-500 text-white',
      icon: <Radio className="drop-shadow-lg" />,
      tooltip: 'Advanced Audio Codec at 256kbps',
    };
  }

  // OGG Vorbis
  if (upperFormat === 'OGG') {
    return {
      label: 'OGG',
      className: 'bg-yellow-500 text-black',
      icon: <Zap className="drop-shadow-lg" />,
      tooltip: 'OGG Vorbis format (~192-256kbps)',
    };
  }

  // Low bitrate warning
  if (bitrate <= 128000) {
    return {
      label: 'Low',
      className: 'bg-red-500 text-white',
      icon: <AlertTriangle className="drop-shadow-lg" />,
      tooltip: `Low quality audio (${Math.round(bitrate / 1000)}kbps or lower)`,
    };
  }

  // Default fallback
  return {
    label: upperFormat,
    className: 'bg-gray-500 text-white',
    icon: <AudioWaveform className="drop-shadow-lg" />,
    tooltip: `${upperFormat} format at ${Math.round(bitrate / 1000)}kbps`,
  };
};

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
        'inline-flex items-center gap-2 px-4 py-2 rounded-full font-bold border-2 transition-all duration-200 shadow-lg',
        badgeInfo.className,
        'hover:scale-105 hover:shadow-xl hover:brightness-110',
        isMinimized ? 'px-3 py-1.5 text-sm gap-1.5' : 'text-base gap-2',
        className
      )}
      title={badgeInfo.tooltip}
    >
      <div className={cn(isMinimized ? 'w-3 h-3' : 'w-4 h-4')}>
        {React.cloneElement(badgeInfo.icon as React.ReactElement, {
          size: isMinimized ? 12 : 16,
          className: 'drop-shadow-lg',
        })}
      </div>
      <span className="font-bold tracking-wide uppercase">{badgeInfo.label}</span>
    </div>
  );
};

export default AudioBadge;
