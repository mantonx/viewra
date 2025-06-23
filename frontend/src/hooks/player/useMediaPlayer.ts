import { useCallback, useEffect, useRef } from 'react';
import { useAtom } from 'jotai';
import shaka from 'shaka-player/dist/shaka-player.ui.js';
import {
  playbackDecisionAtom,
  configAtom,
  shakaPlayerAtom,
  shakaUIAtom,
  videoElementAtom,
  playerInitializedAtom,
  loadingStateAtom,
} from '@/atoms/mediaPlayer';
import { MediaService } from '@/services/MediaService';
import type { ShakaPlayerConfig } from '@/components/MediaPlayer/types';

export const useMediaPlayer = () => {
  const [playbackDecision] = useAtom(playbackDecisionAtom);
  const [config] = useAtom(configAtom);
  const [shakaPlayer, setShakaPlayer] = useAtom(shakaPlayerAtom);
  const [shakaUI, setShakaUI] = useAtom(shakaUIAtom);
  const [videoElement] = useAtom(videoElementAtom);
  const [playerInitialized, setPlayerInitialized] = useAtom(playerInitializedAtom);
  const [, setLoadingState] = useAtom(loadingStateAtom);

  const initializationRef = useRef(false);

  const getShakaPlayerConfig = useCallback((): ShakaPlayerConfig => ({
    manifest: {
      defaultPresentationDelay: 10,
      availabilityWindowOverride: 300,
      dash: {
        ignoreSuggestedPresentationDelay: false,
        autoCorrectDrift: true,
      },
    },
    streaming: {
      bufferingGoal: 15,
      rebufferingGoal: 5,
      bufferBehind: 15,
      retryParameters: {
        maxAttempts: 2,
        baseDelay: 500,
        backoffFactor: 2,
        fuzzFactor: 0.3,
        timeout: 20000,
        stallTimeout: 3000,
        connectionTimeout: 8000,
      },
      gapDetectionThreshold: 0.5, // replaces jumpLargeGaps
      alwaysStreamText: false,
      startAtSegmentBoundary: false,
      segmentPrefetchLimit: 3,
      stallEnabled: true,
      stallThreshold: 0.5,
      stallSkip: 0.05,
      maxDisabledTime: 30,
      inaccurateManifestTolerance: 0.2,
    },
    abr: {
      enabled: true,
      defaultBandwidthEstimate: 500000,
      switchInterval: 8,
      bandwidthUpgradeTarget: 0.85,
      bandwidthDowngradeTarget: 0.95,
      restrictToElementSize: true,
      restrictToScreenSize: true,
      ignoreDevicePixelRatio: false,
      clearBufferSwitch: true,
      useNetworkInformation: true,
    },
    drm: {
      retryParameters: {
        maxAttempts: 2,
        baseDelay: 1000,
        backoffFactor: 2,
      },
    },
    preferredAudioLanguage: 'en',
    preferredTextLanguage: 'en',
    preferredVariantRole: 'main',
  }), []);

  const cleanupPlayer = useCallback(() => {
    // Prevent cleanup during initialization
    if (initializationRef.current) {
      return;
    }

    try {
      if (shakaUI && typeof shakaUI.destroy === 'function') {
        try {
          shakaUI.destroy();
        } catch (e) {
          console.warn('Error destroying UI:', e);
        }
        setShakaUI(null);
      }

      if (shakaPlayer && typeof shakaPlayer.destroy === 'function') {
        try {
          // Ensure player is detached before destroying
          if (shakaPlayer.getMediaElement()) {
            shakaPlayer.detach();
          }
          shakaPlayer.destroy();
        } catch (e) {
          console.warn('Error destroying player:', e);
        }
        setShakaPlayer(null);
      }

      setPlayerInitialized(false);
    } catch (error) {
      console.warn('Error during player cleanup:', error);
    }
  }, [shakaUI, shakaPlayer, setShakaUI, setShakaPlayer, setPlayerInitialized]);

  const initializePlayer = useCallback(async (retryCount: number = 0) => {
    if (!videoElement || !playbackDecision || initializationRef.current) {
      return;
    }

    initializationRef.current = true;
    const maxRetries = 2;

    try {
      if (config.debug) console.log('🎬 Initializing Shaka Player...');

      // Clean up any existing player first
      if (shakaPlayer) {
        cleanupPlayer();
      }

      if (shaka.polyfill) {
        shaka.polyfill.installAll();
      }

      if (!shaka.Player.isBrowserSupported()) {
        throw new Error('Browser not supported');
      }

      const player = new shaka.Player();
      
      // Set up error handling before anything else
      player.addEventListener('error', (event: any) => {
        const error = event.detail;
        const errorInfo = {
          severity: error.severity,
          category: error.category,
          code: error.code,
          data: error.data,
          message: error.message || 'Unknown Shaka error',
          manifestUrl: manifestUrl,
          sessionId: playbackDecision.session_id
        };
        
        console.error('🚨 Shaka Player Error:', errorInfo);
        
        // Provide detailed error messages based on error codes
        let userMessage = 'Player initialization failed';
        
        if (error.code === 4000) {
          userMessage = 'Manifest loading failed. The video stream may not be ready yet.';
        } else if (error.code === 4001) {
          userMessage = 'Invalid manifest format. The video stream is corrupted.';
        } else if (error.code === 4002) {
          userMessage = 'Manifest parsing failed. The video format is not supported.';
        } else if (error.code === 1001) {
          userMessage = 'Network error. Please check your connection and try again.';
        } else if (error.category === 4) { // Manifest category
          userMessage = `Manifest error (${error.code}): The video stream cannot be loaded.`;
        } else if (error.severity === 2) { // Critical errors
          userMessage = `Critical playback error (${error.code}): ${error.message || 'Unknown error'}`;
        }
        
        setLoadingState(prev => ({
          ...prev,
          error: userMessage,
          isLoading: false,
        }));
      });

      setShakaPlayer(player);
      
      // Note: We don't need to wait for video element metadata here.
      // The video element is empty at this point - it will only have metadata
      // AFTER Shaka Player loads the manifest and starts streaming content.
      // Removing the readyState check that was causing false timeouts.
      
      await player.attach(videoElement);

      const shakaConfig = getShakaPlayerConfig();
      player.configure(shakaConfig as any);

      const manifestUrl = playbackDecision.manifest_url || playbackDecision.stream_url;
      if (config.debug) {
        console.log('🎬 Loading manifest/stream:', manifestUrl);
        console.log('🎬 Playback decision details:', {
          sessionId: playbackDecision.session_id,
          shouldTranscode: playbackDecision.should_transcode,
          manifestUrl: playbackDecision.manifest_url,
          streamUrl: playbackDecision.stream_url,
          reason: playbackDecision.reason
        });
      }
      
      // Validate manifest URL format
      if (!manifestUrl) {
        throw new Error('No manifest URL or stream URL provided in playback decision');
      }
      
      if (manifestUrl.includes('undefined') || manifestUrl.includes('null')) {
        throw new Error(`Invalid manifest URL contains undefined/null: ${manifestUrl}`);
      }

      if (playbackDecision.manifest_url && playbackDecision.should_transcode) {
        console.log('⏳ Starting transcoding wait process...', {
          manifestUrl,
          sessionId: playbackDecision.session_id,
          shouldTranscode: playbackDecision.should_transcode
        });
        
        try {
          // First wait for transcoding to make some progress
          if (playbackDecision.session_id) {
            console.log('🔄 Step 1: Waiting for transcoding progress...');
            await MediaService.waitForTranscodingProgress(playbackDecision.session_id);
            console.log('✅ Step 1 complete: Transcoding has made progress');
          }
          
          // Then wait for manifest and video segments to be ready
          console.log('🔄 Step 2: Waiting for manifest and segments...');
          await MediaService.waitForManifest(manifestUrl);
          console.log('✅ Step 2 complete: Manifest and segments ready');
          
          // Finally validate manifest content
          console.log('🔄 Step 3: Validating manifest content...');
          const isValid = await MediaService.validateManifest(manifestUrl);
          if (!isValid) {
            throw new Error('Generated manifest is invalid or corrupted');
          }
          console.log('✅ Step 3 complete: Manifest validated');
          
        } catch (waitError) {
          console.error('❌ Transcoding wait process failed:', waitError);
          throw new Error(`Transcoding preparation failed: ${waitError.message}`);
        }
        
        console.log('✅ All transcoding steps complete, proceeding with Shaka Player load');
      }
      
      // Final pre-flight check before Shaka Player load
      console.log('🔍 Final pre-flight check of manifest URL:', manifestUrl);
      try {
        const preflightResponse = await fetch(manifestUrl, { method: 'HEAD' });
        if (!preflightResponse.ok) {
          throw new Error(`Manifest not accessible: ${preflightResponse.status} ${preflightResponse.statusText}`);
        }
        if (config.debug) console.log('✅ Pre-flight check passed, manifest is accessible');
      } catch (preflightError) {
        console.error('❌ Pre-flight check failed:', preflightError);
        throw new Error(`Manifest URL is not accessible: ${preflightError.message}`);
      }

      await player.load(manifestUrl);
      if (config.debug) console.log('✅ Player loaded successfully');

      setPlayerInitialized(true);
      setLoadingState(prev => ({ 
        ...prev, 
        isLoading: false,
        isVideoLoading: false,
        error: null 
      }));
    } catch (err) {
      console.error(`❌ Player initialization failed (attempt ${retryCount + 1}/${maxRetries + 1}):`, err);
      
      // Reset initialization flag for potential retry
      initializationRef.current = false;
      
      // Clean up any partial initialization
      try {
        if (shakaPlayer && typeof shakaPlayer.destroy === 'function') {
          await shakaPlayer.detach();
          await shakaPlayer.destroy();
          setShakaPlayer(null);
        }
      } catch (cleanupError) {
        console.warn('Error during partial cleanup:', cleanupError);
      }
      
      // Retry logic for certain types of errors
      if (retryCount < maxRetries && err instanceof Error) {
        const isRetryableError = 
          err.message.includes('manifest') ||
          err.message.includes('not accessible') ||
          err.message.includes('timeout') ||
          err.message.includes('network');
          
        if (isRetryableError) {
          const retryDelay = Math.min(1000 * Math.pow(2, retryCount), 5000); // Exponential backoff
          console.log(`🔄 Retrying player initialization in ${retryDelay}ms...`);
          
          setTimeout(() => {
            initializePlayer(retryCount + 1);
          }, retryDelay);
          return;
        }
      }
      
      // Provide more specific error messages
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
  }, [
    videoElement,
    playbackDecision,
    config.debug,
    shakaPlayer,
    cleanupPlayer,
    setShakaPlayer,
    getShakaPlayerConfig,
    setPlayerInitialized,
    setLoadingState,
  ]);

  const loadNewManifest = useCallback(async (manifestUrl: string) => {
    if (!shakaPlayer) {
      console.warn('⚠️ No Shaka player available for manifest loading');
      return;
    }

    try {
      if (config.debug) console.log('🔄 Loading new manifest:', manifestUrl);
      
      await MediaService.waitForManifest(manifestUrl);
      await shakaPlayer.load(manifestUrl);
      
      if (config.debug) console.log('✅ New manifest loaded successfully');
    } catch (error) {
      console.error('❌ Failed to load new manifest:', error);
      throw error;
    }
  }, [shakaPlayer, config.debug]);

  useEffect(() => {
    if (playbackDecision && videoElement && !playerInitialized && !initializationRef.current) {
      if (config.debug) console.log('✅ Triggering player initialization');
      initializePlayer();
    }
  }, [playbackDecision, videoElement, playerInitialized, initializePlayer, config.debug]);

  useEffect(() => {
    return () => {
      cleanupPlayer();
    };
  }, [cleanupPlayer]);

  return {
    initializePlayer,
    cleanupPlayer,
    loadNewManifest,
    playerInitialized,
  };
};