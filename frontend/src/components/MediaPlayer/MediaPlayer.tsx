import React, { useRef, useCallback, useEffect, useState } from 'react';
import { ArrowLeft } from 'lucide-react';
import { useAtom } from 'jotai';
import { MediaPlayer as VidstackPlayer, MediaProvider, Poster, Track, Gesture, type MediaPlayerInstance } from '@vidstack/react';
import '@vidstack/react/player/styles/default/theme.css';
import '@vidstack/react/player/styles/default/layouts/video.css';
import { cn } from '@/utils/cn';
import '@/styles/player-theme.css';
import { PlaybackSessionTracker } from '@/utils/analytics';
import { getDeviceProfile } from '@/utils/deviceProfile';

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

import { useControlsVisibility } from '@/hooks/ui/useControlsVisibility';
import { usePositionSaving } from '@/hooks/ui/usePositionSaving';
import { useFullscreenManager } from '@/hooks/ui/useFullscreenManager';
import { VideoControls } from './components/VideoControls';
import { StatusOverlay } from './components/StatusOverlay';
import { MediaInfoOverlay } from './components/MediaInfoOverlay';
import type { MediaPlayerProps } from './MediaPlayer.types';
import { VidstackControls } from './VidstackControls';
import { API_ENDPOINTS, buildApiUrl } from '@/constants/api';

/**
 * MediaPlayer - Comprehensive media playback component using Vidstack
 * 
 * Features:
 * - Supports all playback methods: direct play, remux, and transcode
 * - Uses direct MP4 URLs (no DASH/HLS complexity)
 * - Supports both video and audio playback
 * - Progressive download with HTTP range support
 * - Analytics tracking for all playback events
 * - Session management for transcoding
 * - Custom controls with Vidstack integration
 */

// Helper function to get MIME type from container
const getMimeType = (container?: string): string => {
  if (!container) return 'video/mp4';
  
  switch (container.toLowerCase()) {
    case 'mp4':
    case 'm4v':
      return 'video/mp4';
    case 'mkv':
    case 'matroska':
      return 'video/x-matroska';
    case 'webm':
      return 'video/webm';
    case 'mov':
      return 'video/quicktime';
    case 'avi':
      return 'video/x-msvideo';
    case 'flv':
      return 'video/x-flv';
    case 'wmv':
      return 'video/x-ms-wmv';
    case 'mp3':
      return 'audio/mpeg';
    case 'm4a':
      return 'audio/mp4';
    case 'flac':
      return 'audio/flac';
    case 'ogg':
      return 'audio/ogg';
    default:
      return 'video/mp4'; // Default fallback
  }
};

export const MediaPlayer: React.FC<MediaPlayerProps> = (props) => {
  const { className, autoplay = true, onBack, ...mediaType } = props;
  const containerRef = useRef<HTMLDivElement>(null);
  const playerRef = useRef<MediaPlayerInstance>(null);

  // State atoms
  const [playerState, setPlayerState] = useAtom(playerStateAtom);
  const [loadingState, setLoadingState] = useAtom(loadingStateAtom);
  const [currentMedia] = useAtom(currentMediaAtom);
  const [playbackDecision] = useAtom(playbackDecisionAtom);
  const [activeSessions] = useAtom(activeSessionsAtom);
  const [config] = useAtom(configAtom);

  // Hooks
  const { mediaId, handleBack, loadingState: navLoadingState, mediaFile } = useMediaNavigation(mediaType);
  const { stopAllSessions } = useSessionManager();
  const { toggleFullscreen } = useFullscreenManager();
  
  const { showControls, handleMouseMove, handleMouseLeave } = useControlsVisibility({
    containerRef,
    enabled: !loadingState.isLoading,
  });
  const { savePosition, clearSavedPosition } = usePositionSaving({
    mediaId: mediaId || '',
    enabled: !!mediaId,
  });

  // Vidstack state and remote control
  const [vidstackRemote, setVidstackRemote] = useState<any>(null);
  const [vidstackStore, setVidstackStore] = useState<any>(null);
  
  // Playback state
  const [playbackUrl, setPlaybackUrl] = useState<string>('');
  const [sessionId, setSessionId] = useState<string | null>(null);
  
  // Memoized callbacks for VidstackControls
  const handleRemoteReady = useCallback((remote: any) => {
    setVidstackRemote(remote);
  }, []);
  
  const handleStoreUpdate = useCallback((store: any) => {
    setVidstackStore(store);
  }, []);
  
  // Extract state from store
  const playing = vidstackStore?.playing ?? false;
  const paused = vidstackStore?.paused ?? true;
  const duration = vidstackStore?.duration ?? 0;
  const currentTime = vidstackStore?.currentTime ?? 0;
  const volume = vidstackStore?.volume ?? 1;
  const muted = vidstackStore?.muted ?? false;
  const buffering = vidstackStore?.buffering ?? false;
  const fullscreen = vidstackStore?.fullscreen ?? false;
  
  // Session tracking
  const [sessionTracker, setSessionTracker] = useState<PlaybackSessionTracker | null>(null);
  
  // Track initialization attempts to prevent infinite retries
  const initAttemptedRef = useRef<string | null>(null);

  // Initialize playback URL from atoms (useMediaNavigation handles the decision making)
  useEffect(() => {
    if (!playbackDecision || !mediaId) return;
    
    // Don't retry if we've already set up playback for this decision
    if (initAttemptedRef.current === playbackDecision.stream_url) return;
    
    initAttemptedRef.current = playbackDecision.stream_url;

    console.log('🎬 MediaPlayer using playback decision:', {
      should_transcode: playbackDecision.should_transcode,
      stream_url: playbackDecision.stream_url,
      reason: playbackDecision.reason
    });

    // Use the stream_url from the playback decision (set by useMediaNavigation)
    if (playbackDecision.stream_url) {
      setPlaybackUrl(playbackDecision.stream_url);
      
      // Initialize analytics tracking
      const initAnalytics = async () => {
        const deviceProfile = await getDeviceProfile();
        const tracker = new PlaybackSessionTracker(
          playbackDecision.session_id || 'direct-play',
          mediaId,
          'type' in mediaType ? mediaType.type : 'movie',
          deviceProfile
        );
        setSessionTracker(tracker);
      };
      
      initAnalytics();
    }

    return () => {
      // Stop analytics on cleanup
      if (sessionTracker) {
        sessionTracker.stopTracking();
      }
    };
  }, [playbackDecision, mediaId, mediaType]);
  
  // Update local player state from Vidstack state
  useEffect(() => {
    setPlayerState(prev => {
      // When paused, only update currentTime if the change is significant (> 0.1s)
      const shouldUpdateTime = !paused || Math.abs(currentTime - prev.currentTime) > 0.1;
      
      // Use mediaFile duration as fallback if Vidstack duration is invalid
      const validDuration = duration > 1 ? duration : (mediaFile?.duration || duration);
      
      return {
        ...prev,
        isPlaying: playing,
        duration: validDuration,
        currentTime: shouldUpdateTime ? currentTime : prev.currentTime,
        volume: volume,
        isMuted: muted,
        isBuffering: buffering,
        currentQuality: null,
      };
    });
  }, [playing, duration, currentTime, volume, muted, buffering, setPlayerState, paused, mediaFile]);

  // Track state changes for analytics
  useEffect(() => {
    if (sessionTracker) {
      sessionTracker.updatePlaybackState(currentTime, duration);
    }
  }, [sessionTracker, currentTime, duration]);

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

  // Save position periodically
  useEffect(() => {
    if (Math.floor(currentTime) % 5 === 0) {
      savePosition(currentTime);
    }
  }, [currentTime, savePosition]);

  // Handle seek
  const handleSeek = useCallback(async (progress: number) => {
    if (!vidstackRemote || !duration || duration <= 0) return;
    
    const seekTime = progress * duration;
    vidstackRemote.seek(seekTime);
    
    if (sessionTracker) {
      sessionTracker.trackEvent('seek', { seekTime });
    }
  }, [vidstackRemote, duration, sessionTracker]);

  // Video control handlers using Vidstack remote
  const videoControls = {
    togglePlayPause: useCallback(() => {
      if (!vidstackRemote) return;
      if (!playerState.isPlaying) {
        vidstackRemote.play();
      } else {
        vidstackRemote.pause();
      }
    }, [vidstackRemote, playerState.isPlaying]),
    
    stop: useCallback(() => {
      if (!vidstackRemote) return;
      vidstackRemote.pause();
      vidstackRemote.seek(0);
      clearSavedPosition();
    }, [vidstackRemote, clearSavedPosition]),
    
    restartFromBeginning: useCallback(() => {
      if (!vidstackRemote) return;
      vidstackRemote.seek(0);
      vidstackRemote.play();
      clearSavedPosition();
    }, [vidstackRemote, clearSavedPosition]),
    
    skipForward: useCallback((seconds: number) => {
      if (!vidstackRemote) return;
      vidstackRemote.seek(Math.min(currentTime + seconds, duration));
    }, [vidstackRemote, currentTime, duration]),
    
    skipBackward: useCallback((seconds: number) => {
      if (!vidstackRemote) return;
      vidstackRemote.seek(Math.max(currentTime - seconds, 0));
    }, [vidstackRemote, currentTime]),
    
    setVolume: useCallback((newVolume: number) => {
      if (!vidstackRemote) return;
      vidstackRemote.setVolume(newVolume);
    }, [vidstackRemote]),
    
    toggleMute: useCallback(() => {
      if (!vidstackRemote) return;
      vidstackRemote.setMuted(!muted);
    }, [vidstackRemote, muted]),
  };

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

  // Update fullscreen state from Vidstack
  useEffect(() => {
    setPlayerState(prev => ({ ...prev, isFullscreen: fullscreen }));
  }, [fullscreen, setPlayerState]);

  // Cleanup on unmount
  useEffect(() => {
    return () => {
      if (sessionTracker) {
        sessionTracker.stopTracking();
      }
    };
  }, [sessionTracker]);

  // Get buffered ranges from video element
  const bufferedRanges: Array<{ start: number; end: number }> = [];
  if (playerRef.current && playerRef.current.buffered) {
    const buffered = playerRef.current.buffered;
    for (let i = 0; i < buffered.length; i++) {
      bufferedRanges.push({
        start: buffered.start(i),
        end: buffered.end(i)
      });
    }
  }

  if ((loadingState.isLoading || navLoadingState.isLoading) && !playbackUrl) {
    return (
      <div className="flex items-center justify-center h-screen player-gradient text-white">
        <div className="text-center">
          <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-white mx-auto mb-4"></div>
          <p>Loading media...</p>
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

      <VidstackPlayer
        className="w-full h-full"
        style={{
          '--media-focus-ring': 'rgb(var(--player-accent-500))',
          '--media-accent-color': 'rgb(var(--player-accent-500))',
          '--media-accent-color-hover': 'rgb(var(--player-accent-400))',
        }}
        title={currentMedia?.title || 'Media Player'}
        src={playbackUrl ? {
          src: playbackUrl,
          type: getMimeType(mediaFile?.container)
        } : ''}
        autoPlay={autoplay && config.autoplay}
        crossOrigin="anonymous"
        playsInline
        onLoadedMetadata={(event: any) => {
          console.log('📹 Media metadata loaded:', {
            duration: playerRef.current?.duration,
            src: playbackUrl,
            decision: playbackDecision?.decision
          });
        }}
        onError={(event: any) => {
          console.error('❌ Media error:', event);
          sessionTracker?.trackEvent('error', { 
            error: event?.detail || 'Unknown error',
            decision: playbackDecision?.decision
          });
        }}
        ref={playerRef}
        load="eager"
      >
        <MediaProvider />
        
        {/* VidstackControls to properly handle hooks */}
        <VidstackControls 
          onRemoteReady={handleRemoteReady}
          onStoreUpdate={handleStoreUpdate}
        />
        
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
        isSeekingAhead={false}
        isLoading={loadingState.isVideoLoading}
        error={loadingState.error}
        playbackInfo={playbackDecision ? {
          isTranscoding: playbackDecision.decision === 'transcode',
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
          currentTime={Math.min(playerState.currentTime, playerState.duration || 0)}
          duration={Math.max(playerState.duration, mediaFile?.duration || 0)}
          volume={playerState.volume}
          isMuted={playerState.isMuted}
          isFullscreen={playerState.isFullscreen}
          bufferedRanges={bufferedRanges}
          isSeekingAhead={false}
          onPlayPause={videoControls.togglePlayPause}
          onStop={videoControls.stop}
          onRestart={videoControls.restartFromBeginning}
          onSeek={handleSeek}
          onSeekIntent={(time) => console.log('Seek intent:', time)}
          onSkipBackward={() => videoControls.skipBackward(10)}
          onSkipForward={() => videoControls.skipForward(10)}
          onVolumeChange={videoControls.setVolume}
          onToggleMute={videoControls.toggleMute}
          onToggleFullscreen={async () => {
            if (playerRef.current) {
              try {
                if (fullscreen) {
                  await playerRef.current.exitFullscreen();
                } else {
                  await playerRef.current.enterFullscreen();
                }
              } catch (error) {
                console.error('Fullscreen error:', error);
                toggleFullscreen();
              }
            } else {
              toggleFullscreen();
            }
          }}
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