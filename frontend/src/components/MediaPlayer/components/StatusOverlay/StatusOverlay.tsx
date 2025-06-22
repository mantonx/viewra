import React from 'react';
import { cn } from '@/utils/cn';
import type { StatusOverlayProps } from './types';

export const StatusOverlay: React.FC<StatusOverlayProps> = ({
  isBuffering,
  isSeekingAhead,
  isLoading,
  error,
  playbackInfo,
  className,
  showPlaybackInfo = true,
}) => {
  if (!isBuffering && !isSeekingAhead && !isLoading && !error && !showPlaybackInfo) {
    return null;
  }

  return (
    <>
      {/* Loading/Buffering overlay */}
      {(isBuffering || isLoading) && !error && (
        <div className={cn(
          'absolute inset-0 flex items-center justify-center bg-background-overlay/50 z-40',
          className
        )}>
          <div className="bg-player-controls-bg/80 rounded-lg p-4 flex items-center space-x-3 backdrop-blur-sm">
            <div className="animate-spin rounded-full h-6 w-6 border-b-2 border-player-text"></div>
            <span className="text-player-text">
              {isSeekingAhead ? 'Seeking...' : isBuffering ? 'Buffering...' : 'Loading...'}
            </span>
          </div>
        </div>
      )}

      {/* Error overlay */}
      {error && (
        <div className={cn(
          'absolute inset-0 flex items-center justify-center bg-background-overlay/80 z-50',
          className
        )}>
          <div className="bg-error/90 rounded-lg p-6 max-w-md text-center backdrop-blur-sm">
            <h3 className="text-xl font-bold text-player-text mb-2">Playback Error</h3>
            <p className="text-player-text/80">{error}</p>
          </div>
        </div>
      )}

      {/* Playback info */}
      {showPlaybackInfo && playbackInfo && !error && (
        <div className="absolute top-4 right-4 z-30 bg-player-controls-bg/70 text-player-text p-3 rounded-lg text-sm backdrop-blur-sm">
          <div className="font-semibold mb-1">
            {playbackInfo.isTranscoding ? '🎬 Transcoding' : '📺 Direct Play'}
          </div>
          <div className="text-xs text-player-text-secondary">
            {playbackInfo.reason}
          </div>
          {isSeekingAhead && (
            <div className="text-xs text-info mt-1 animate-pulse">
              ⚡ Transcoding ahead...
            </div>
          )}
          {playbackInfo.sessionCount > 1 && (
            <div className="text-xs text-warning mt-1">
              📊 {playbackInfo.sessionCount} active sessions
            </div>
          )}
        </div>
      )}
    </>
  );
};