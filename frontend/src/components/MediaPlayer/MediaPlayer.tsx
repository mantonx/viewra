import React, { useRef, useCallback, useEffect } from 'react';
import { ArrowLeft } from 'lucide-react';
import { useAtom } from 'jotai';
import { cn } from '@/utils/cn';
import {
  playerStateAtom,
  loadingStateAtom,
  progressStateAtom,
  currentMediaAtom,
  playbackDecisionAtom,
  activeSessionsAtom,
} from '@/atoms/mediaPlayer';
import { useMediaPlayer } from '@/hooks/player/useMediaPlayer';
import { useVideoControls } from '@/hooks/player/useVideoControls';
import { useMediaNavigation } from '@/hooks/media/useMediaNavigation';
import { useSessionManager } from '@/hooks/session/useSessionManager';
import { useSeekAhead } from '@/hooks/session/useSeekAhead';
import { useKeyboardShortcuts } from '@/hooks/ui/useKeyboardShortcuts';
import { useControlsVisibility } from '@/hooks/ui/useControlsVisibility';
import { usePositionSaving } from '@/hooks/ui/usePositionSaving';
import { useFullscreenManager } from '@/hooks/ui/useFullscreenManager';
import { VideoElement } from './components/VideoElement';
import type { VideoElementRef } from './components/VideoElement/types';
import { VideoControls } from './components/VideoControls';
import { StatusOverlay } from './components/StatusOverlay';
import { MediaInfoOverlay } from './components/MediaInfoOverlay';
import { getBufferedRanges } from '@/utils/video';
// import type { MediaType as MediaTypeString } from './types';

// Define the actual prop structure expected
export type MediaIdentifier = 
  | { type: 'movie'; movieId: number }
  | { type: 'episode'; tvShowId: number; seasonNumber: number; episodeNumber: number };

export type MediaPlayerProps = MediaIdentifier & {
  className?: string;
  autoplay?: boolean;
  onBack?: () => void;
};

export const MediaPlayer: React.FC<MediaPlayerProps> = (props) => {
  const { className, autoplay = true, onBack, ...mediaType } = props;
  const containerRef = useRef<HTMLDivElement>(null);
  const videoElementRef = useRef<VideoElementRef>(null);

  // State atoms
  const [playerState, setPlayerState] = useAtom(playerStateAtom);
  const [loadingState] = useAtom(loadingStateAtom);
  const [progressState] = useAtom(progressStateAtom);
  const [currentMedia] = useAtom(currentMediaAtom);
  const [playbackDecision] = useAtom(playbackDecisionAtom);
  const [activeSessions] = useAtom(activeSessionsAtom);

  // Hooks
  const { mediaId, handleBack, config, loadingState: navLoadingState } = useMediaNavigation(mediaType);
  useMediaPlayer();
  const videoControls = useVideoControls();
  const { stopTranscodingSession } = useSessionManager();
  const { requestSeekAhead, isSeekAheadNeeded, seekAheadState } = useSeekAhead();
  const { isFullscreen, toggleFullscreen } = useFullscreenManager();
  const { showControls } = useControlsVisibility({
    containerRef,
    enabled: !loadingState.isLoading,
  });
  const { savePosition, clearSavedPosition } = usePositionSaving({
    mediaId: mediaId || '',
    enabled: !!mediaId,
  });

  // Handle seek with seek-ahead support
  const handleSeek = useCallback(async (progress: number) => {
    if (!videoElementRef.current?.videoElement) return;
    
    const video = videoElementRef.current.videoElement;
    const duration = progressState.originalDuration || video.duration;
    
    if (!duration || duration <= 0) return;
    
    const seekTime = progress * duration;
    
    // Check if seek-ahead is needed
    if (isSeekAheadNeeded(seekTime)) {
      await requestSeekAhead(seekTime);
    } else {
      // Normal seek
      videoControls.seek(seekTime);
    }
  }, [progressState.originalDuration, isSeekAheadNeeded, requestSeekAhead, videoControls]);

  // Handle seek intent (hover)
  const handleSeekIntent = useCallback((time: number) => {
    // Could pre-load segments here if needed
    console.log('Seek intent:', time);
  }, []);

  // Video event handlers
  const handleLoadedMetadata = useCallback(() => {
    const video = videoElementRef.current?.videoElement;
    if (!video) return;
    
    // Set duration and initial state
    setPlayerState(prev => ({
      ...prev,
      duration: video.duration,
    }));
  }, [setPlayerState]);

  const handleTimeUpdate = useCallback(() => {
    const video = videoElementRef.current?.videoElement;
    if (!video) return;
    
    const actualTime = video.currentTime + seekAheadState.seekOffset;
    setPlayerState(prev => ({
      ...prev,
      currentTime: actualTime,
    }));
    
    // Save position periodically
    if (Math.floor(actualTime) % 5 === 0) {
      savePosition(actualTime);
    }
  }, [seekAheadState.seekOffset, setPlayerState, savePosition]);

  const handlePlay = useCallback(() => {
    setPlayerState(prev => ({ ...prev, isPlaying: true }));
  }, [setPlayerState]);

  const handlePause = useCallback(() => {
    setPlayerState(prev => ({ ...prev, isPlaying: false }));
    
    // Stop transcoding when paused to save resources
    if (playbackDecision?.session_id) {
      stopTranscodingSession(playbackDecision.session_id);
    }
  }, [setPlayerState, playbackDecision?.session_id, stopTranscodingSession]);

  const handleWaiting = useCallback(() => {
    setPlayerState(prev => ({ ...prev, isBuffering: true }));
  }, [setPlayerState]);

  const handlePlaying = useCallback(() => {
    setPlayerState(prev => ({ ...prev, isBuffering: false }));
  }, [setPlayerState]);

  const handleVolumeChange = useCallback(() => {
    const video = videoElementRef.current?.videoElement;
    if (!video) return;
    
    setPlayerState(prev => ({
      ...prev,
      volume: video.volume,
      isMuted: video.muted,
    }));
  }, [setPlayerState]);

  const handleStop = useCallback(() => {
    videoControls.stop();
    clearSavedPosition();
  }, [videoControls.stop, clearSavedPosition]);

  const handleRestart = useCallback(() => {
    videoControls.restartFromBeginning();
    clearSavedPosition();
  }, [videoControls.restartFromBeginning, clearSavedPosition]);

  // Keyboard shortcuts
  useKeyboardShortcuts({
    onSeek: handleSeek,
    onTogglePlayPause: videoControls.togglePlayPause,
    onToggleMute: videoControls.toggleMute,
    onToggleFullscreen: toggleFullscreen,
    skipSeconds: 10,
    enabled: !loadingState.isLoading,
  });

  // Get buffered ranges for display
  const bufferedRanges = videoElementRef.current?.videoElement 
    ? getBufferedRanges(videoElementRef.current.videoElement)
    : [];

  // Custom back handler
  const handleBackClick = useCallback(() => {
    if (onBack) {
      onBack();
    } else {
      handleBack();
    }
  }, [onBack, handleBack]);

  // Update fullscreen state
  useEffect(() => {
    setPlayerState(prev => ({ ...prev, isFullscreen }));
  }, [isFullscreen, setPlayerState]);

  if (loadingState.isLoading || navLoadingState.isLoading) {
    return (
      <div className="flex items-center justify-center h-screen bg-player-bg text-player-text">
        <div className="text-center">
          <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-player-text mx-auto mb-4"></div>
          <p>Loading video...</p>
        </div>
      </div>
    );
  }

  if (loadingState.error || navLoadingState.error) {
    return (
      <div className="flex items-center justify-center h-screen bg-player-bg text-player-text">
        <div className="text-center max-w-md">
          <h2 className="text-xl font-bold mb-4">Playback Error</h2>
          <p className="text-error mb-4">{loadingState.error || navLoadingState.error}</p>
          <button
            onClick={() => window.location.reload()}
            className="bg-primary hover:bg-primary/80 px-4 py-2 rounded transition-colors duration-normal"
          >
            Reload Player
          </button>
        </div>
      </div>
    );
  }

  return (
    <div 
      ref={containerRef} 
      data-testid="media-player"
      className={cn('relative w-full h-full bg-black overflow-hidden', className)}
      onMouseMove={showControls ? undefined : () => {}}
      onMouseLeave={showControls ? undefined : () => {}}
    >
      {/* Back button */}
      <button
        onClick={handleBackClick}
        className="absolute top-4 left-4 z-50 bg-player-controls-bg/50 hover:bg-player-controls-bg/80 hover:scale-110 text-player-text p-2 rounded-full transition-all duration-normal shadow-lg backdrop-blur-sm"
        title="Go back"
      >
        <ArrowLeft className="w-6 h-6" />
      </button>

      {/* Video container */}
      <div className="relative w-full h-full overflow-hidden rounded-lg">
        <VideoElement
          ref={videoElementRef}
          className="w-full h-full"
          onLoadedMetadata={handleLoadedMetadata}
          onTimeUpdate={handleTimeUpdate}
          onPlay={handlePlay}
          onPause={handlePause}
          onWaiting={handleWaiting}
          onPlaying={handlePlaying}
          onVolumeChange={handleVolumeChange}
          onDoubleClick={handleRestart}
          autoPlay={autoplay && config.autoplay}
          preload="auto"
        />

        {/* Status overlays */}
        <StatusOverlay
          isBuffering={playerState.isBuffering}
          isSeekingAhead={seekAheadState.isSeekingAhead}
          isLoading={loadingState.isVideoLoading}
          error={loadingState.error}
          playbackInfo={playbackDecision ? {
            isTranscoding: playbackDecision.should_transcode,
            reason: playbackDecision.reason,
            sessionCount: activeSessions.size,
          } : undefined}
          showPlaybackInfo={config.debug}
        />

        {/* Media info overlay */}
        <MediaInfoOverlay
          media={currentMedia}
          position="top-left"
          autoHide
          autoHideDelay={5000}
        />

        {/* Video controls */}
        <div
          className={cn(
            'absolute bottom-0 left-0 right-0 bg-gradient-to-t from-player-controls-bg/80 to-transparent p-6 transition-opacity duration-slow',
            showControls ? 'opacity-100' : 'opacity-0'
          )}
        >
          <VideoControls
            isPlaying={playerState.isPlaying}
            currentTime={playerState.currentTime}
            duration={playerState.duration}
            volume={playerState.volume}
            isMuted={playerState.isMuted}
            isFullscreen={playerState.isFullscreen}
            bufferedRanges={bufferedRanges}
            isSeekingAhead={seekAheadState.isSeekingAhead}
            onPlayPause={videoControls.togglePlayPause}
            onStop={handleStop}
            onRestart={handleRestart}
            onSeek={handleSeek}
            onSeekIntent={handleSeekIntent}
            onSkipBackward={() => videoControls.skipBackward(10)}
            onSkipForward={() => videoControls.skipForward(10)}
            onVolumeChange={videoControls.setVolume}
            onToggleMute={videoControls.toggleMute}
            onToggleFullscreen={toggleFullscreen}
            showStopButton
            showSkipButtons
            showVolumeControl
            showFullscreenButton
            showTimeDisplay
            skipSeconds={10}
          />
        </div>
      </div>
    </div>
  );
};