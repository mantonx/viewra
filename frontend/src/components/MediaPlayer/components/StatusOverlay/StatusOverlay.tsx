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
          'absolute inset-0 flex items-center justify-center bg-black/50 z-40',
          className
        )} data-testid="loading-indicator">
          <div className="bg-slate-800/80 rounded-lg p-4 flex items-center space-x-3 backdrop-blur-sm">
            <div className="animate-spin rounded-full h-6 w-6 border-b-2 border-white"></div>
            <span className="text-white" data-testid="loading-message">
              {isSeekingAhead ? 'Seeking...' : isBuffering ? 'Buffering...' : 'Starting playback...'}
            </span>
          </div>
        </div>
      )}

      {/* Error overlay */}
      {error && (
        <div className={cn(
          'absolute inset-0 flex items-center justify-center bg-black/80 z-50',
          className
        )} data-testid="error-message">
          <div className="bg-red-600/90 rounded-lg p-6 max-w-md text-center backdrop-blur-sm">
            <h3 className="text-xl font-bold text-white mb-2">Failed to start playback</h3>
            <p className="text-white/80">{error}</p>
          </div>
        </div>
      )}

      {/* Playback info */}
      {showPlaybackInfo && playbackInfo && !error && (
        <div className="absolute top-4 right-4 z-30 bg-slate-800/70 text-white p-3 rounded-lg text-sm backdrop-blur-sm">
          <div className="font-semibold mb-1">
            {playbackInfo.isTranscoding ? '🎬 Transcoding' : '📺 Direct Play'}
          </div>
          <div className="text-xs text-gray-300">
            {playbackInfo.reason}
          </div>
          {isSeekingAhead && (
            <div className="text-xs text-blue-400 mt-1 animate-pulse">
              ⚡ Transcoding ahead...
            </div>
          )}
          {playbackInfo.sessionCount > 1 && (
            <div className="text-xs text-yellow-400 mt-1">
              📊 {playbackInfo.sessionCount} active sessions
            </div>
          )}
        </div>
      )}
    </>
  );
};