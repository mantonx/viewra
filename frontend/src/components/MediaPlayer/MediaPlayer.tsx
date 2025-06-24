import React, { useRef, useCallback, useEffect, useState } from 'react';
import { ArrowLeft } from 'lucide-react';
import { useAtom } from 'jotai';
import { MediaPlayer as VidstackPlayer, MediaProvider, Poster, Track, Gesture, useMediaStore, useMediaRemote } from '@vidstack/react';
import '@vidstack/react/player/styles/default/theme.css';
import '@vidstack/react/player/styles/default/layouts/video.css';
import { cn } from '@/utils/cn';
import '@/styles/player-theme.css';
import { PlaybackSessionTracker } from '@/utils/analytics';
import { getDeviceProfile } from '@/utils/deviceProfile';
import { initializeDashWithFixes } from '@/utils/player/dashCompatibility';
import { getOptimalSource } from '@/utils/videoPlayerConfig';

import {
  playerStateAtom,
  loadingStateAtom,
  currentMediaAtom,
  playbackDecisionAtom,
  activeSessionsAtom,
  configAtom,
} from '@/atoms/mediaPlayer';
import { useMediaNavigation } from '@/hooks/media/useMediaNavigation';
import { useSessionManager } from '@/hooks/session/useSessionManager';
import { useSeekAhead } from '@/hooks/session/useSeekAhead';

import { useControlsVisibility } from '@/hooks/ui/useControlsVisibility';
import { usePositionSaving } from '@/hooks/ui/usePositionSaving';
import { useFullscreenManager } from '@/hooks/ui/useFullscreenManager';
import { VideoControls } from './components/VideoControls';
import { StatusOverlay } from './components/StatusOverlay';
import { MediaInfoOverlay } from './components/MediaInfoOverlay';
import { QualityIndicator } from './components/QualityIndicator';
import { getBufferedRanges } from '@/utils/video';
import type { MediaPlayerProps } from './MediaPlayer.types';

export const MediaPlayer: React.FC<MediaPlayerProps> = (props) => {
  const { className, autoplay = true, onBack, ...mediaType } = props;
  const containerRef = useRef<HTMLDivElement>(null);
  const playerRef = useRef<HTMLMediaElement>(null);

  // State atoms
  const [playerState, setPlayerState] = useAtom(playerStateAtom);
  const [loadingState] = useAtom(loadingStateAtom);
  const [currentMedia] = useAtom(currentMediaAtom);
  const [playbackDecision] = useAtom(playbackDecisionAtom);
  const [activeSessions] = useAtom(activeSessionsAtom);
  const [config] = useAtom(configAtom);

  // Hooks
  const { mediaId, handleBack, loadingState: navLoadingState } = useMediaNavigation(mediaType);
  const { stopTranscodingSession, stopAllSessions } = useSessionManager();
  const { requestSeekAhead, isSeekAheadNeeded, seekAheadState } = useSeekAhead();
  const { isFullscreen, toggleFullscreen } = useFullscreenManager();
  const { showControls, handleMouseMove, handleMouseLeave } = useControlsVisibility({
    containerRef,
    enabled: !loadingState.isLoading,
  });
  const { savePosition, clearSavedPosition } = usePositionSaving({
    mediaId: mediaId || '',
    enabled: !!mediaId,
  });

  // Use Vidstack's built-in store and remote
  const store = useMediaStore();
  const remote = useMediaRemote();
  
  // Extract state from store
  const playing = store?.playing ?? false;
  const paused = store?.paused ?? true;
  const duration = store?.duration ?? 0;
  const currentTime = store?.currentTime ?? 0;
  const volume = store?.volume ?? 1;
  const muted = store?.muted ?? false;
  const buffering = store?.buffering ?? false;
  const quality = store?.quality ?? null;
  
  // Session tracking
  const [sessionTracker, setSessionTracker] = useState<PlaybackSessionTracker | null>(null);
  
  // Initialize session tracking when playback decision is available
  useEffect(() => {
    if (playbackDecision && currentMedia && !sessionTracker) {
      const initSessionTracking = async () => {
        try {
          const deviceProfile = await getDeviceProfile();
          const tracker = new PlaybackSessionTracker(
            playbackDecision.session_id || 'unknown',
            mediaId || 'unknown',
            'type' in mediaType ? mediaType.type : 'movie',
            deviceProfile
          );
          setSessionTracker(tracker);
          console.log('📊 Session tracking initialized for Vidstack player');
        } catch (error) {
          console.warn('Failed to initialize session tracking:', error);
        }
      };
      initSessionTracking();
    }
  }, [playbackDecision, currentMedia, sessionTracker, mediaId, mediaType]);

  // Track state changes for analytics and position saving
  useEffect(() => {
    if (sessionTracker) {
      sessionTracker.updatePlaybackState(currentTime + seekAheadState.seekOffset, duration);
    }
  }, [sessionTracker, currentTime, duration, seekAheadState.seekOffset]);

  // Track play/pause events
  useEffect(() => {
    if (!sessionTracker) return;
    
    if (playing && !playerState.isPlaying) {
      sessionTracker.trackEvent('play');
    } else if (!playing && playerState.isPlaying) {
      sessionTracker.trackEvent('pause');
    }
  }, [playing, playerState.isPlaying, sessionTracker]);

  // Track buffering events
  useEffect(() => {
    if (!sessionTracker) return;
    
    if (buffering && !playerState.isBuffering) {
      sessionTracker.trackEvent('buffer_start');
    } else if (!buffering && playerState.isBuffering) {
      sessionTracker.trackEvent('buffer_end');
    }
  }, [buffering, playerState.isBuffering, sessionTracker]);

  // Track quality changes
  useEffect(() => {
    if (sessionTracker && quality && quality !== playerState.currentQuality) {
      sessionTracker.trackEvent('quality_change', { quality, bitrate: quality.bitrate });
    }
  }, [quality, playerState.currentQuality, sessionTracker]);

  // Update local player state from Vidstack state
  useEffect(() => {
    setPlayerState(prev => ({
      ...prev,
      isPlaying: playing,
      duration: duration,
      currentTime: currentTime + seekAheadState.seekOffset,
      volume: volume,
      isMuted: muted,
      isBuffering: buffering,
      currentQuality: quality,
    }));
  }, [playing, duration, currentTime, volume, muted, buffering, quality, seekAheadState.seekOffset, setPlayerState]);

  // Save position periodically
  useEffect(() => {
    if (Math.floor(currentTime) % 5 === 0) {
      savePosition(currentTime + seekAheadState.seekOffset);
    }
  }, [currentTime, seekAheadState.seekOffset, savePosition]);

  // Handle seek with seek-ahead support
  const handleSeek = useCallback(async (progress: number) => {
    if (!remote || !duration || duration <= 0) return;
    
    const seekTime = progress * duration;
    const startTime = Date.now();
    
    try {
      // Check if seek-ahead is needed
      if (isSeekAheadNeeded(seekTime)) {
        await requestSeekAhead(seekTime);
        if (sessionTracker) {
          sessionTracker.trackEvent('seek', { 
            seekTime, 
            seekAhead: true,
            seekDuration: Date.now() - startTime
          });
        }
      } else {
        // Normal seek using Vidstack remote
        remote.seek(seekTime);
        if (sessionTracker) {
          sessionTracker.trackEvent('seek', { 
            seekTime, 
            seekAhead: false,
            seekDuration: Date.now() - startTime
          });
        }
      }
    } catch (error) {
      if (sessionTracker) {
        sessionTracker.trackEvent('error', {
          type: 'seek_error',
          message: error instanceof Error ? error.message : 'Unknown seek error'
        });
      }
    }
  }, [remote, duration, isSeekAheadNeeded, requestSeekAhead, sessionTracker]);

  // Handle seek intent (hover)
  const handleSeekIntent = useCallback((time: number) => {
    // Could pre-load segments here if needed
    console.log('Seek intent:', time);
  }, []);

  // Video control handlers using Vidstack remote
  const videoControls = {
    togglePlayPause: useCallback(() => {
      if (!remote) return;
      if (paused) {
        remote.play();
      } else {
        remote.pause();
      }
    }, [remote, paused]),
    
    stop: useCallback(() => {
      if (!remote) return;
      remote.pause();
      remote.seek(0);
      clearSavedPosition();
    }, [remote, clearSavedPosition]),
    
    restartFromBeginning: useCallback(() => {
      if (!remote) return;
      remote.seek(0);
      remote.play();
      clearSavedPosition();
    }, [remote, clearSavedPosition]),
    
    skipForward: useCallback((seconds: number) => {
      if (!remote) return;
      remote.seek(Math.min(currentTime + seconds, duration));
    }, [remote, currentTime, duration]),
    
    skipBackward: useCallback((seconds: number) => {
      if (!remote) return;
      remote.seek(Math.max(currentTime - seconds, 0));
    }, [remote, currentTime]),
    
    setVolume: useCallback((newVolume: number) => {
      if (!remote) return;
      remote.setVolume(newVolume);
    }, [remote]),
    
    toggleMute: useCallback(() => {
      if (!remote) return;
      remote.setMuted(!muted);
    }, [remote, muted]),
  };

  const handlePause = useCallback(() => {
    // Stop transcoding when paused to save resources
    if (playbackDecision?.session_id) {
      stopTranscodingSession(playbackDecision.session_id);
    }
  }, [playbackDecision?.session_id, stopTranscodingSession]);

  // Vidstack has built-in keyboard shortcuts:
  // Space: Play/Pause
  // Arrow Left/Right: Seek backward/forward (10s)
  // Arrow Up/Down: Volume control
  // M: Mute toggle
  // F: Fullscreen toggle
  // Home/End: Seek to beginning/end
  // 0-9: Seek to percentage (0=0%, 5=50%, 9=90%)

  // Get buffered ranges for display  
  const bufferedRanges = playerRef.current && 'buffered' in playerRef.current ? getBufferedRanges(playerRef.current as HTMLVideoElement) : [];

  // Custom back handler
  const handleBackClick = useCallback(async () => {
    console.log('🔙 Back button clicked, stopping sessions...');
    await stopAllSessions();
    if (onBack) {
      onBack();
    } else {
      handleBack();
    }
  }, [onBack, handleBack, stopAllSessions]);

  // Update fullscreen state
  useEffect(() => {
    setPlayerState(prev => ({ ...prev, isFullscreen }));
  }, [isFullscreen, setPlayerState]);

  // Cleanup on unmount
  useEffect(() => {
    return () => {
      if (loadingState.error) {
        console.log('🧹 VidstackPlayer unmounting due to error, cleaning up sessions...');
        stopAllSessions();
      }
      
      // Stop session tracking
      if (sessionTracker) {
        console.log('📊 Stopping session tracking for Vidstack player');
        sessionTracker.stopTracking();
      }
    };
  }, [loadingState.error, stopAllSessions, sessionTracker]);
  
  // Get the stream URL with format preference - must be before conditional returns
  const streamUrl = playbackDecision?.manifest_url || playbackDecision?.stream_url || '';
  
  // Get optimal source configuration with device-specific optimizations
  const mediaSource = getOptimalSource(streamUrl);
  
  // Apply DASH.js compatibility fixes for DASH content - MUST be before any conditional returns
  useEffect(() => {
    if (streamUrl && streamUrl.includes('.mpd')) {
      console.log('🔧 Applying DASH.js compatibility enhancements');
      initializeDashWithFixes(() => {
        console.log('✅ DASH.js compatibility enhancements applied');
      });
    }
  }, [streamUrl]);

  if ((loadingState.isLoading || navLoadingState.isLoading) && !playbackDecision) {
    return (
      <div className="flex items-center justify-center h-screen player-gradient text-white">
        <div className="text-center">
          <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-white mx-auto mb-4"></div>
          <p>Loading video...</p>
        </div>
      </div>
    );
  }

  if (loadingState.error || navLoadingState.error) {
    return (
      <div className="flex items-center justify-center h-screen player-gradient text-white">
        <div className="text-center max-w-md">
          <h2 className="text-xl font-bold mb-4">Playback Error</h2>
          <p className="text-red-400 mb-4">{loadingState.error || navLoadingState.error}</p>
          <button
            onClick={() => window.location.reload()}
            className="px-4 py-2 rounded transition-all duration-200 player-accent-gradient text-white hover:brightness-110 hover:scale-105 active:scale-95"
          >
            Reload Player
          </button>
        </div>
      </div>
    );
  }

  // Device detection is now handled by getOptimalSource utility

  return (
    <div 
      ref={containerRef} 
      data-testid="media-player"
      className={cn('relative h-screen player-gradient overflow-hidden', className)}
      onMouseMove={handleMouseMove}
      onMouseLeave={handleMouseLeave}
    >
      {/* Back button */}
      <button
        onClick={handleBackClick}
        className="absolute top-4 left-4 z-50 p-2 rounded-full text-white transition-all player-control-button"
        style={{
          backgroundColor: `rgb(var(--player-surface-overlay))`,
          boxShadow: 'var(--player-shadow-md)',
          backdropFilter: 'blur(12px)',
          transitionDuration: 'var(--player-transition-fast)'
        }}
        title="Go back"
      >
        <ArrowLeft className="w-6 h-6" />
      </button>

      {/* Vidstack Media Player */}
      <VidstackPlayer
        className="w-full h-full"
        style={{
          '--media-focus-ring': 'rgb(var(--player-accent-500))',
          '--media-accent-color': 'rgb(var(--player-accent-500))',
          '--media-accent-color-hover': 'rgb(var(--player-accent-400))',
        }}
        title={currentMedia?.title || 'Video Player'}
        src={{
          src: mediaSource.src,
          type: mediaSource.type,
        }}
        autoPlay={autoplay && config.autoplay}
        crossOrigin="anonymous"
        playsInline
        onPause={handlePause}
        ref={playerRef}
        // Ensure DASH provider is loaded
        load="eager"
      >
        <MediaProvider />
        
        {/* Add touch gesture support */}
        <Gesture 
          action="toggle:paused"
          className="absolute inset-0 z-0"
        />
        <Gesture 
          action="seek:-10"
          className="absolute left-0 top-0 bottom-0 w-1/3 z-10"
        />
        <Gesture 
          action="seek:10"
          className="absolute right-0 top-0 bottom-0 w-1/3 z-10"
        />
        <Gesture 
          action="toggle:fullscreen"
          className="absolute inset-0 z-0"
          event="dblclick"
        />
        
        {/* Add poster if available */}
        {currentMedia?.poster && (
          <Poster 
            className="absolute inset-0 w-full h-full object-cover"
            src={currentMedia.poster}
            alt={currentMedia.title}
          />
        )}

        {/* Add subtitles if available */}
        {currentMedia?.subtitles?.map((track, index) => (
          <Track
            key={index}
            src={track.src}
            kind={track.kind || 'subtitles'}
            label={track.label}
            srclang={track.srclang}
            default={track.default}
          />
        ))}
      </VidstackPlayer>

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
          'absolute bottom-0 left-0 right-0 p-6 transition-opacity',
          showControls ? 'opacity-100' : 'opacity-0'
        )}
        style={{
          background: `linear-gradient(to top, rgb(var(--player-surface-backdrop)), transparent)`,
          transitionDuration: 'var(--player-transition-slow)'
        }}
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
          onStop={videoControls.stop}
          onRestart={videoControls.restartFromBeginning}
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
  );
};