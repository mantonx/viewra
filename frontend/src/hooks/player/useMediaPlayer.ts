import { useCallback, useEffect, useRef } from 'react';
import { useAtom } from 'jotai';
import { useMediaRemote, useMediaStore } from '@vidstack/react';
import {
  playbackDecisionAtom,
  configAtom,
  playerInitializedAtom,
  loadingStateAtom,
} from '@/atoms/mediaPlayer';
import { MediaService } from '@/services/MediaService';

/**
 * Hook for managing Vidstack media player operations
 * Simplified from the original Shaka Player implementation
 */
export const useMediaPlayer = () => {
  const [playbackDecision] = useAtom(playbackDecisionAtom);
  const [config] = useAtom(configAtom);
  const [playerInitialized, setPlayerInitialized] = useAtom(playerInitializedAtom);
  const [, setLoadingState] = useAtom(loadingStateAtom);
  
  // Vidstack APIs
  const remote = useMediaRemote();
  const store = useMediaStore();
  
  const initializationRef = useRef(false);

  /**
   * Initialize the player with a manifest URL
   * Vidstack handles most of the initialization internally
   */
  const initializePlayer = useCallback(async (retryCount: number = 0) => {
    if (!playbackDecision || initializationRef.current || !store) {
      return;
    }

    initializationRef.current = true;
    const maxRetries = 2;

    try {
      if (config.debug) console.log('🎬 Initializing Vidstack Player...');

      const manifestUrl = playbackDecision.manifest_url || playbackDecision.stream_url;
      
      if (!manifestUrl) {
        throw new Error('No manifest URL or stream URL provided in playback decision');
      }
      
      if (manifestUrl.includes('undefined') || manifestUrl.includes('null')) {
        throw new Error(`Invalid manifest URL contains undefined/null: ${manifestUrl}`);
      }

      // For transcoded content, wait for manifest availability
      if (playbackDecision.manifest_url && playbackDecision.should_transcode) {
        console.log('⚡ Fast initialization mode - checking manifest availability...', {
          manifestUrl,
          sessionId: playbackDecision.session_id,
        });
        
        try {
          const startTime = Date.now();
          
          // Run checks in parallel
          await Promise.race([
            Promise.all([
              // Wait for transcoding to start
              playbackDecision.session_id ? 
                MediaService.waitForTranscodingProgress(playbackDecision.session_id, { 
                  maxAttempts: 2,
                  checkInterval: 250
                }) : Promise.resolve(),
              
              // Check manifest availability
              MediaService.waitForManifest(manifestUrl, {
                maxAttempts: 10,
                checkInterval: 200,
                requireSegments: false
              }),
            ]),
            new Promise((_, reject) => 
              setTimeout(() => reject(new Error('Initialization timeout')), 3000)
            )
          ]);
          
          const elapsed = Date.now() - startTime;
          console.log(`✅ Fast initialization complete in ${elapsed}ms`);
          
        } catch (waitError) {
          console.warn('⚡ Fast init incomplete, proceeding optimistically:', waitError);
          // Don't throw - let Vidstack handle retries
        }
      }

      // Vidstack will handle loading the manifest through the src prop
      // We just need to mark that initialization is complete
      setPlayerInitialized(true);
      setLoadingState(prev => ({ 
        ...prev, 
        isLoading: false,
        isVideoLoading: false,
        error: null 
      }));
      
      if (config.debug) console.log('✅ Player initialized successfully');
      
    } catch (err) {
      console.error(`❌ Player initialization failed (attempt ${retryCount + 1}/${maxRetries + 1}):`, err);
      
      // Reset initialization flag for potential retry
      initializationRef.current = false;
      
      // Retry logic for certain types of errors
      if (retryCount < maxRetries && err instanceof Error) {
        const isRetryableError = 
          err.message.includes('manifest') ||
          err.message.includes('not accessible') ||
          err.message.includes('timeout') ||
          err.message.includes('network');
          
        if (isRetryableError) {
          const retryDelay = Math.min(1000 * Math.pow(2, retryCount), 5000);
          console.log(`🔄 Retrying player initialization in ${retryDelay}ms...`);
          
          setTimeout(() => {
            initializePlayer(retryCount + 1);
          }, retryDelay);
          return;
        }
      }
      
      // Provide specific error messages
      let errorMessage = 'Player initialization failed';
      if (err instanceof Error) {
        if (err.message.includes('manifest')) {
          errorMessage = 'Failed to load video manifest. The video may not be ready for playback yet.';
        } else if (err.message.includes('format') || err.message.includes('codec')) {
          errorMessage = 'Video format not supported by your browser.';
        } else if (err.message.includes('not accessible')) {
          errorMessage = 'Video stream is not accessible. Please try again later.';
        } else {
          errorMessage = `Player error: ${err.message}`;
        }
      }
      
      setLoadingState(prev => ({
        ...prev,
        error: errorMessage,
        isLoading: false,
      }));
    }
  }, [playbackDecision, config.debug, store, setPlayerInitialized, setLoadingState]);

  /**
   * Clean up player resources
   * Vidstack handles most cleanup internally
   */
  const cleanupPlayer = useCallback(() => {
    if (initializationRef.current) {
      return;
    }

    try {
      setPlayerInitialized(false);
      initializationRef.current = false;
      
      if (config.debug) console.log('🧹 Player cleanup completed');
    } catch (error) {
      console.warn('Error during player cleanup:', error);
    }
  }, [setPlayerInitialized, config.debug]);

  /**
   * Load a new manifest URL
   * With Vidstack, this is handled by changing the src prop
   */
  const loadNewManifest = useCallback(async (manifestUrl: string) => {
    if (!remote) {
      console.warn('⚠️ No Vidstack remote available for manifest loading');
      return;
    }

    try {
      if (config.debug) console.log('🔄 Loading new manifest:', manifestUrl);
      
      await MediaService.waitForManifest(manifestUrl);
      
      // Vidstack will handle loading through src prop change
      // The MediaPlayer component should handle updating the src
      
      if (config.debug) console.log('✅ New manifest ready for loading');
    } catch (error) {
      console.error('❌ Failed to validate new manifest:', error);
      throw error;
    }
  }, [remote, config.debug]);

  // Initialize player when playback decision is available
  useEffect(() => {
    if (playbackDecision && !playerInitialized && !initializationRef.current) {
      if (config.debug) console.log('✅ Triggering player initialization');
      initializePlayer();
    }
  }, [playbackDecision, playerInitialized, initializePlayer, config.debug]);

  // Clean up when navigating away
  useEffect(() => {
    if (!playbackDecision && playerInitialized) {
      if (config.debug) console.log('🧹 Playback decision cleared, cleaning up player');
      cleanupPlayer();
    }
  }, [playbackDecision, playerInitialized, cleanupPlayer, config.debug]);

  // Cleanup on unmount
  useEffect(() => {
    return () => {
      cleanupPlayer();
    };
  }, [cleanupPlayer]);

  // Media control methods using Vidstack remote
  const play = useCallback(() => remote?.play(), [remote]);
  const pause = useCallback(() => remote?.pause(), [remote]);
  const seek = useCallback((time: number) => remote?.seek(time), [remote]);
  const setVolume = useCallback((volume: number) => remote?.setVolume(volume), [remote]);
  const toggleMute = useCallback(() => {
    if (store) {
      const { muted } = store.getState();
      remote?.setMuted(!muted);
    }
  }, [remote, store]);
  const toggleFullscreen = useCallback(() => {
    if (store) {
      const { fullscreen } = store.getState();
      if (fullscreen) {
        remote?.exitFullscreen();
      } else {
        remote?.requestFullscreen();
      }
    }
  }, [remote, store]);

  return {
    initializePlayer,
    cleanupPlayer,
    loadNewManifest,
    playerInitialized,
    // Media control methods
    play,
    pause,
    seek,
    setVolume,
    toggleMute,
    toggleFullscreen,
    // Expose Vidstack APIs for advanced usage
    remote,
    store,
  };
};