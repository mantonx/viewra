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
import { VidstackControls } from './VidstackControls';
import { API_ENDPOINTS, buildApiUrl } from '@/constants/api';
import type { PlaybackDecision } from './types';
import { TestDashPlayer } from './TestDashPlayer/TestDashPlayer';

export const MediaPlayer: React.FC<MediaPlayerProps> = (props) => {
  const { className, autoplay = true, onBack, ...mediaType } = props;
  const containerRef = useRef<HTMLDivElement>(null);
  const playerRef = useRef<MediaPlayerInstance>(null);

  // State atoms
  const [playerState, setPlayerState] = useAtom(playerStateAtom);
  const [loadingState] = useAtom(loadingStateAtom);
  const [currentMedia] = useAtom(currentMediaAtom);
  const [playbackDecision] = useAtom(playbackDecisionAtom);
  const [activeSessions] = useAtom(activeSessionsAtom);
  const [config] = useAtom(configAtom);

  // Hooks
  const { mediaId, handleBack, loadingState: navLoadingState, mediaFile } = useMediaNavigation(mediaType);
  const { stopTranscodingSession, stopAllSessions } = useSessionManager();
  const { requestSeekAhead, isSeekAheadNeeded, seekAheadState } = useSeekAhead();
  const { toggleFullscreen } = useFullscreenManager();
  
  // Remove the vidstackRemoteRef as we'll use playerRef instead
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
  const quality = vidstackStore?.quality ?? null;
  const fullscreen = vidstackStore?.fullscreen ?? false;
  
  // Debug duration issues
  useEffect(() => {
    if (duration < 1 && vidstackStore && mediaFile?.duration && mediaFile.duration > 1) {
      console.log('🔴 Duration issue detected:', {
        duration,
        currentTime,
        vidstackStore,
        mediaFile: mediaFile?.duration,
        playbackDecision
      });
      
      // Try to manually set duration if we have it from mediaFile
      if (vidstackRemote && mediaFile.duration > 1) {
        console.log('🔧 Attempting to fix duration using mediaFile duration:', mediaFile.duration);
        // Force a re-render by seeking to current position
        vidstackRemote.seek(currentTime);
      }
    }
  }, [duration, currentTime, vidstackStore, mediaFile, playbackDecision, vidstackRemote]);
  
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

  // Track quality changes
  useEffect(() => {
    if (sessionTracker && quality && quality !== playerState.currentQuality) {
      sessionTracker.trackEvent('quality_change', { quality, bitrate: quality.bitrate });
    }
  }, [quality, playerState.currentQuality, sessionTracker]);

  // Update local player state from Vidstack state
  useEffect(() => {
    setPlayerState(prev => {
      // When paused, only update currentTime if the change is significant (> 0.1s)
      // This prevents small fluctuations from moving the scrubber
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
        currentQuality: quality,
      };
    });
  }, [playing, duration, currentTime, volume, muted, buffering, quality, setPlayerState, paused, mediaFile]);
  


  // Save position periodically
  useEffect(() => {
    if (Math.floor(currentTime) % 5 === 0) {
      savePosition(currentTime);
    }
  }, [currentTime, savePosition]);

  // Handle seek with seek-ahead support
  const handleSeek = useCallback(async (progress: number) => {
    if (!vidstackRemote || !duration || duration <= 0) return;
    
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
        vidstackRemote.seek(seekTime);
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
  }, [vidstackRemote, duration, isSeekAheadNeeded, requestSeekAhead, sessionTracker]);

  // Handle seek intent (hover)
  const handleSeekIntent = useCallback((time: number) => {
    // Could pre-load segments here if needed
    console.log('Seek intent:', time);
  }, []);

  // Video control handlers using Vidstack remote
  const videoControls = {
    togglePlayPause: useCallback(() => {
      if (!vidstackRemote) {
        console.warn('⚠️ Vidstack remote not available');
        return;
      }
      // Use playerState.isPlaying to determine current state
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

  const handlePause = useCallback(() => {
    // Don't stop transcoding session on pause
    // Stopping the session causes the player to reset
  }, []);

  // Vidstack has built-in keyboard shortcuts:
  // Space: Play/Pause
  // Arrow Left/Right: Seek backward/forward (10s)
  // Arrow Up/Down: Volume control
  // M: Mute toggle
  // F: Fullscreen toggle
  // Home/End: Seek to beginning/end
  // 0-9: Seek to percentage (0=0%, 5=50%, 9=90%)

  // Get buffered ranges for display  
  const bufferedRanges: Array<{ start: number; end: number }> = [];

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
  
  // Device detection utility
  const detectDevice = () => {
    const userAgent = navigator.userAgent;
    const isIOS = /iPad|iPhone|iPod/.test(userAgent);
    const isSafari = /^((?!chrome|android).)*safari/i.test(userAgent);
    const isMobile = /Mobi|Android/i.test(userAgent) || isIOS;
    
    return {
      type: isMobile ? 'mobile' : 'desktop',
      isIOS,
      isSafari,
    };
  };
  
  // Get the stream URL with content-addressable storage support
  const getStreamUrl = (decision: PlaybackDecision | null): string => {
    if (!decision) return '';
    
    // If we have a content hash, use the content-addressable storage URL
    if (decision.content_hash) {
      // Determine the format based on transcode params
      const isHLS = decision.transcode_params?.target_container === 'hls';
      if (isHLS) {
        // Add timestamp to force cache bust
        const url = buildApiUrl(API_ENDPOINTS.CONTENT.HLS_MANIFEST.path(decision.content_hash));
        return `${url}?t=${Date.now()}`;
      } else {
        // Default to DASH for content-addressable storage
        const url = buildApiUrl(API_ENDPOINTS.CONTENT.MANIFEST.path(decision.content_hash));
        return `${url}?t=${Date.now()}`;
      }
    }
    
    // Fall back to legacy URLs
    return decision.manifest_url || decision.stream_url || '';
  };
  
  const streamUrl = getStreamUrl(playbackDecision);
  
  // Get optimal source configuration with device-specific optimizations
  const getOptimalSource = (url: string) => {
    const device = detectDevice();
    
    // Use HLS for iOS devices and Safari
    const preferHLS = device.isIOS || device.isSafari;
    const isHLS = url.includes('.m3u8') || preferHLS;
    
    // Ensure absolute URL for better compatibility with DASH.js
    const absoluteUrl = url.startsWith('http') ? url : 
      (url.startsWith('/') ? window.location.origin + url : url);
    
    console.log('📺 Media source:', {
      original: url,
      absolute: absoluteUrl,
      format: isHLS ? 'hls' : 'dash',
      device: device.type,
      windowOrigin: window.location.origin
    });
    
    return {
      src: absoluteUrl,
      type: isHLS ? 'application/vnd.apple.mpegurl' : 'application/dash+xml',
    };
  };
  
  const mediaSource = getOptimalSource(streamUrl);
  const [delayedSource, setDelayedSource] = useState<{ src: string; type: string } | null>(null);
  const [useTestPlayer, setUseTestPlayer] = useState(false);
  
  // Delay setting source to ensure manifest is ready
  useEffect(() => {
    if (mediaSource && mediaSource.src) {
      // Small delay to ensure backend is ready
      const timer = setTimeout(() => {
        setDelayedSource(mediaSource);
      }, 500);
      return () => clearTimeout(timer);
    }
  }, [mediaSource.src, mediaSource.type]);
  
  // Debug: Log media source
  useEffect(() => {
    if (mediaSource && streamUrl) {
      console.log('🎬 Media source:', {
        mediaSource,
        streamUrl,
        playbackDecision,
        mediaFile,
        contentHash: playbackDecision?.content_hash,
        isUsingContentStore: !!playbackDecision?.content_hash
      });
    }
  }, [mediaSource, streamUrl, playbackDecision, mediaFile]);
  
  // Log when DASH source is ready
  useEffect(() => {
    if (delayedSource && delayedSource.type === 'application/dash+xml' && delayedSource.src) {
      console.log('🔍 DASH source ready:', {
        url: delayedSource.src,
        type: delayedSource.type
      });
    }
  }, [delayedSource]);
  
  // Vidstack handles DASH.js internally, no compatibility patches needed

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
      
      {/* Test player toggle - TEMPORARY */}
      {mediaSource.type === 'application/dash+xml' && (
        <button
          onClick={() => setUseTestPlayer(!useTestPlayer)}
          className="absolute top-4 right-4 z-50 p-2 rounded text-white bg-red-600 hover:bg-red-700"
        >
          {useTestPlayer ? 'Use Vidstack' : 'Test DASH.js'}
        </button>
      )}

      {/* Conditionally render test player or Vidstack */}
      {useTestPlayer && mediaSource.type === 'application/dash+xml' ? (
        <TestDashPlayer src={mediaSource.src} />
      ) : (
      <VidstackPlayer
        className="w-full h-full"
        style={{
          '--media-focus-ring': 'rgb(var(--player-accent-500))',
          '--media-accent-color': 'rgb(var(--player-accent-500))',
          '--media-accent-color-hover': 'rgb(var(--player-accent-400))',
        }}
        title={currentMedia?.title || 'Video Player'}
        src={delayedSource?.src || ''}
        type={delayedSource?.type}
        autoPlay={autoplay && config.autoplay}
        crossOrigin="anonymous"
        playsInline
        onPause={handlePause}
        onLoadedMetadata={(event: any) => {
          console.log('📹 Video metadata loaded');
          console.log('📹 Metadata details:', {
            duration: playerRef.current?.duration,
            width: playerRef.current?.videoWidth,
            height: playerRef.current?.videoHeight,
            readyState: playerRef.current?.readyState,
            src: playerRef.current?.src
          });
        }}
        onLoadStart={(event: any) => {
          console.log('🎬 Load start:', {
            src: mediaSource.src,
            type: mediaSource.type,
            event
          });
        }}
        onCanPlay={() => {
          console.log('▶️ Can play event fired');
        }}
        onDurationChange={(event: any) => {
          console.log('⏱️ Duration changed:', event?.detail);
        }}
        onError={(event: any) => {
          console.error('❌ Video error:', event);
          // Log detailed error information
          if (event?.detail) {
            console.error('❌ Video error details:', {
              error: event.detail,
              src: mediaSource.src,
              type: mediaSource.type,
              streamUrl,
              contentHash: playbackDecision?.content_hash
            });
          }
        }}
        ref={playerRef}
        // Ensure DASH provider is loaded
        load="eager"
        onProviderChange={(event: any) => {
          console.log('🎬 Provider changed:', {
            provider: event?.detail,
            loader: event?.detail?.loader,
            type: event?.detail?.type
          });
        }}
        onProviderSetup={(event: any) => {
          // Provider is ready
          console.log('🎞️ Provider setup:', event);
          console.log('🎞️ Provider details:', {
            provider: event?.detail?.provider,
            type: event?.detail?.type,
            src: mediaSource.src
          });
          
          // Check if DASH provider is loaded
          if (event?.detail?.provider?.name === 'dash' || event?.detail?.type === 'dash') {
            console.log('✅ DASH provider loaded successfully');
          }
        }}
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
      )}

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
          currentTime={Math.min(playerState.currentTime, playerState.duration || 0)}
          duration={Math.max(playerState.duration, mediaFile?.duration || 0)}
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
          onToggleFullscreen={async () => {
            // Use Vidstack's native fullscreen through player ref
            if (playerRef.current) {
              try {
                if (fullscreen) {
                  await playerRef.current.exitFullscreen();
                } else {
                  await playerRef.current.enterFullscreen();
                }
              } catch (error) {
                console.error('Fullscreen error:', error);
                // Fallback to custom fullscreen
                toggleFullscreen();
              }
            } else {
              // Fallback to custom fullscreen
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