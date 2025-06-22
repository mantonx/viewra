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
      jumpLargeGaps: true,
      forceTransmuxTS: true,
      forceHTTPS: false,
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
        shakaPlayer.destroy();
      } catch (e) {
        console.warn('Error destroying player:', e);
      }
      setShakaPlayer(null);
    }

    setPlayerInitialized(false);
    initializationRef.current = false;
  }, [shakaUI, shakaPlayer, setShakaUI, setShakaPlayer, setPlayerInitialized]);

  const initializePlayer = useCallback(async () => {
    if (!videoElement || !playbackDecision || initializationRef.current) {
      return;
    }

    initializationRef.current = true;

    try {
      if (config.debug) console.log('🎬 Initializing Shaka Player...');

      if (shaka.polyfill) {
        shaka.polyfill.installAll();
      }

      if (!shaka.Player.isBrowserSupported()) {
        throw new Error('Browser not supported');
      }

      const player = new shaka.Player();
      setShakaPlayer(player);
      
      await player.attach(videoElement);

      const shakaConfig = getShakaPlayerConfig();
      player.configure(shakaConfig as any);

      const manifestUrl = playbackDecision.manifest_url || playbackDecision.stream_url;
      if (config.debug) console.log('🎬 Loading manifest/stream:', manifestUrl);

      if (playbackDecision.manifest_url && playbackDecision.should_transcode) {
        if (config.debug) console.log('⏳ Waiting for DASH manifest to be generated...');
        await MediaService.waitForManifest(manifestUrl);
      }

      await player.load(manifestUrl);
      if (config.debug) console.log('✅ Player loaded successfully');

      setPlayerInitialized(true);
      setLoadingState(prev => ({ ...prev, isLoading: false }));
    } catch (err) {
      console.error('❌ Player initialization failed:', err);
      setLoadingState(prev => ({
        ...prev,
        error: `Player initialization failed: ${err instanceof Error ? err.message : 'Unknown error'}`,
        isLoading: false,
      }));
      initializationRef.current = false;
    }
  }, [
    videoElement,
    playbackDecision,
    config.debug,
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