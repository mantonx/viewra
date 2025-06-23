import React from 'react';
import { Play, Pause, Square, SkipBack, SkipForward, Maximize, Minimize2, RotateCcw } from 'lucide-react';
import { cn } from '@/utils/cn';
import { formatTime, formatRemainingTime } from '@/utils/time';
import { ProgressBar } from '../ProgressBar';
import { VolumeControl } from '../VolumeControl';
import type { VideoControlsProps } from './types';

export const VideoControls: React.FC<VideoControlsProps> = ({
  // State
  isPlaying,
  currentTime,
  duration,
  volume,
  isMuted,
  isFullscreen,
  bufferedRanges,
  isSeekingAhead,
  
  // Actions
  onPlayPause,
  onStop,
  onRestart,
  onSeek,
  onSeekIntent,
  onSkipBackward,
  onSkipForward,
  onVolumeChange,
  onToggleMute,
  onToggleFullscreen,
  
  // Options
  className,
  showStopButton = false,
  showSkipButtons = true,
  showVolumeControl = true,
  showFullscreenButton = true,
  showTimeDisplay = true,
  skipSeconds = 10,
}) => {
  const hasValidDuration = duration > 0 && isFinite(duration);

  return (
    <div className={cn('space-y-4', className)}>
      {/* Progress bar */}
      <ProgressBar
        currentTime={currentTime}
        duration={duration}
        bufferedRanges={bufferedRanges}
        isSeekable={hasValidDuration}
        isSeekingAhead={isSeekingAhead}
        onSeek={onSeek}
        onSeekIntent={onSeekIntent}
        className="mb-4"
      />
      
      {/* Time display */}
      {showTimeDisplay && (
        <div className="flex justify-between text-xs text-gray-300 mt-2" data-testid="time-display">
          <span>{formatTime(currentTime)}</span>
          <span>{hasValidDuration ? formatRemainingTime(currentTime, duration) : '--:--'}</span>
        </div>
      )}

      {/* Control buttons */}
      <div className="flex items-center justify-between">
        <div className="flex items-center space-x-4">
          {/* Skip backward */}
          {showSkipButtons && onSkipBackward && (
            <button
              onClick={onSkipBackward}
              className="text-white hover:text-blue-400 hover:scale-110 transition-all duration-200 p-2 rounded-full hover:bg-white/10 disabled:opacity-50 disabled:cursor-not-allowed disabled:hover:scale-100 disabled:hover:bg-transparent"
              disabled={!hasValidDuration}
              title={`Skip backward ${skipSeconds} seconds`}
            >
              <SkipBack className="w-6 h-6" />
            </button>
          )}

          {/* Play/Pause */}
          <button
            onClick={onPlayPause}
            data-testid="play-button"
            className="text-white hover:text-blue-400 hover:scale-110 transition-all duration-200 p-3 rounded-full hover:bg-white/10"
            title={isPlaying ? 'Pause' : 'Play'}
          >
            {isPlaying ? (
              <Pause className="w-8 h-8" />
            ) : (
              <Play className="w-8 h-8" />
            )}
          </button>

          {/* Skip forward */}
          {showSkipButtons && onSkipForward && (
            <button
              onClick={onSkipForward}
              className="text-white hover:text-blue-400 hover:scale-110 transition-all duration-200 p-2 rounded-full hover:bg-white/10 disabled:opacity-50 disabled:cursor-not-allowed disabled:hover:scale-100 disabled:hover:bg-transparent"
              disabled={!hasValidDuration}
              title={`Skip forward ${skipSeconds} seconds`}
            >
              <SkipForward className="w-6 h-6" />
            </button>
          )}

          {/* Stop */}
          {showStopButton && onStop && (
            <button
              onClick={onStop}
              className="text-white hover:text-blue-400 hover:scale-110 transition-all duration-200 p-2 rounded-full hover:bg-white/10"
              title="Stop"
            >
              <Square className="w-6 h-6" />
            </button>
          )}

          {/* Restart */}
          {onRestart && (
            <button
              onClick={onRestart}
              className="text-white hover:text-blue-400 hover:scale-110 transition-all duration-200 p-2 rounded-full hover:bg-white/10"
              title="Restart from beginning"
            >
              <RotateCcw className="w-6 h-6" />
            </button>
          )}

          {/* Volume control */}
          {showVolumeControl && (
            <div data-testid="volume-control">
              <VolumeControl
                volume={volume}
                isMuted={isMuted}
                onVolumeChange={onVolumeChange}
                onToggleMute={onToggleMute}
              />
            </div>
          )}
        </div>

        <div className="flex items-center space-x-4">
          {/* Fullscreen */}
          {showFullscreenButton && (
            <button
              onClick={onToggleFullscreen}
              className="text-white hover:text-blue-400 hover:scale-110 transition-all duration-200 p-2 rounded-full hover:bg-white/10"
              title={isFullscreen ? 'Exit fullscreen' : 'Enter fullscreen'}
            >
              {isFullscreen ? (
                <Minimize2 className="w-6 h-6" />
              ) : (
                <Maximize className="w-6 h-6" />
              )}
            </button>
          )}
        </div>
      </div>
    </div>
  );
};