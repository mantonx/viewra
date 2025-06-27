import { useCallback, useEffect } from 'react';
import { useNavigate, useParams, useSearchParams } from 'react-router-dom';
import { useAtom } from 'jotai';
import { getSavedPosition, savePosition, clearSavedPosition } from '@/utils/storage';
import {
  currentMediaAtom,
  mediaFileAtom,
  playbackDecisionAtom,
  configAtom,
  loadingStateAtom,
  progressStateAtom,
} from '@/atoms/mediaPlayer';
import { MediaService } from '@/services/MediaService';
import { buildApiUrl } from '@/constants/api';
import { useSessionManager } from '../session/useSessionManager';
import type { MediaIdentifier } from '@/components/MediaPlayer';

export const useMediaNavigation = (mediaIdentifier: MediaIdentifier) => {
  const navigate = useNavigate();
  const { episodeId, movieId } = useParams<{ episodeId?: string; movieId?: string }>();
  const [searchParams] = useSearchParams();
  
  const [currentMedia, setCurrentMedia] = useAtom(currentMediaAtom);
  const [mediaFile, setMediaFile] = useAtom(mediaFileAtom);
  const [, setPlaybackDecision] = useAtom(playbackDecisionAtom);
  const [config, setConfig] = useAtom(configAtom);
  const [loadingState, setLoadingState] = useAtom(loadingStateAtom);
  const [, setProgressState] = useAtom(progressStateAtom);
  
  const { stopAllSessions } = useSessionManager();

  const mediaType = mediaIdentifier.type;
  const mediaId = mediaType === 'episode' ? episodeId : movieId;

  const handleBack = useCallback(async () => {
    console.log('🚪 Navigating away, cleaning up sessions...');
    await stopAllSessions();
    navigate(-1);
  }, [navigate, stopAllSessions]);

  const loadMediaData = useCallback(async () => {
    if (!mediaId) return;

    try {
      // Clean up any existing sessions first
      console.log('🧹 Cleaning up existing sessions before loading new media...');
      await stopAllSessions();
      
      setLoadingState(prev => ({ ...prev, isLoading: true, error: null }));

      const mediaFile = await MediaService.getMediaFiles(mediaId, mediaType);
      if (!mediaFile) {
        throw new Error(`No media file found for this ${mediaType}`);
      }

      setMediaFile(mediaFile);

      if (mediaFile.duration && mediaFile.duration > 0) {
        setProgressState(prev => ({
          ...prev,
          originalDuration: mediaFile.duration!,
        }));
      }

      const deviceProfile = MediaService.getDefaultDeviceProfile();
      
      const [decision, metadata] = await Promise.all([
        MediaService.getPlaybackDecision(mediaFile.path, mediaFile.id, deviceProfile),
        MediaService.getMediaMetadata(mediaId, mediaFile.id),
      ]);

      if (metadata) {
        setCurrentMedia(metadata);
      }

      if (decision.should_transcode) {
        console.log('🎬 Starting transcoding session for:', mediaFile.path);
        
        const sessionData = await MediaService.startTranscodingSession(
          mediaFile.id,  // Use media file ID instead of path
          decision.transcode_params?.target_container || 'dash',
          decision.transcode_params?.target_codec || 'h264'
        );

        console.log('✅ Transcoding session started:', sessionData.id);

        // Update decision with transcoding session info
        // Use content-addressable URLs if available
        let manifestUrl = sessionData.manifest_url;
        let streamUrl = sessionData.manifest_url;
        
        if (sessionData.content_hash) {
          // Use content-addressable storage URLs
          const container = decision.transcode_params?.target_container || 'dash';
          if (container === 'hls') {
            manifestUrl = buildApiUrl(`/v1/content/${sessionData.content_hash}/playlist.m3u8`);
          } else {
            manifestUrl = buildApiUrl(`/v1/content/${sessionData.content_hash}/manifest.mpd`);
          }
          streamUrl = manifestUrl;
        } else if (!manifestUrl) {
          // Fallback for sessions without manifest URL
          console.warn('Session has no manifest URL and no content hash, this should not happen');
          manifestUrl = '';
          streamUrl = '';
        }
        
        const updatedDecision = {
          ...decision,
          session_id: sessionData.id,
          manifest_url: manifestUrl,
          stream_url: streamUrl,
          content_hash: sessionData.content_hash,
          content_url: sessionData.content_url,
        };
        
        setPlaybackDecision(updatedDecision);
      } else {
        // For direct play, ensure stream_url is set
        const updatedDecision = {
          ...decision,
          stream_url: decision.direct_play_url || decision.stream_url || mediaFile.path,
        };
        setPlaybackDecision(updatedDecision);
      }

      setLoadingState(prev => ({ ...prev, isLoading: false }));

    } catch (err) {
      console.error(`❌ Failed to load ${mediaType} data:`, err);
      setLoadingState(prev => ({
        ...prev,
        error: err instanceof Error ? err.message : `Failed to load ${mediaType}`,
        isLoading: false,
      }));
    }
  }, [mediaId, mediaType, setLoadingState, setMediaFile, setProgressState, setPlaybackDecision, setCurrentMedia, stopAllSessions]);

  const updateConfig = useCallback(() => {
    const debug = searchParams.get('debug') === 'true' || searchParams.get('debug') === '1';
    const startTime = parseInt(searchParams.get('t') || '0', 10);
    const autoplay = searchParams.get('autoplay') !== 'false';

    setConfig({
      debug,
      startTime,
      autoplay,
    });
  }, [searchParams, setConfig]);

  useEffect(() => {
    updateConfig();
  }, [updateConfig]);

  useEffect(() => {
    if (mediaId) {
      // Reset state when media changes
      setPlaybackDecision(null);
      setCurrentMedia(null);
      setMediaFile(null);
      setLoadingState({
        isLoading: true,
        isVideoLoading: false,
        error: null,
      });
      
      loadMediaData();
    }
  }, [mediaId]); // Intentionally not including all deps to avoid infinite loops

  const getSavedPositionForMedia = useCallback((): number => {
    return mediaId ? getSavedPosition(mediaId) : 0;
  }, [mediaId]);

  const savePositionForMedia = useCallback((position: number) => {
    if (mediaId) {
      savePosition(mediaId, position);
    }
  }, [mediaId]);

  const clearSavedPositionForMedia = useCallback(() => {
    if (mediaId) {
      clearSavedPosition(mediaId);
    }
  }, [mediaId]);

  const getStartPosition = useCallback((): number => {
    if (config.startTime > 0) {
      return config.startTime;
    }
    
    const savedPosition = getSavedPositionForMedia();
    return savedPosition > 0 ? savedPosition : 0;
  }, [config.startTime, getSavedPositionForMedia]);

  return {
    mediaId,
    currentMedia,
    mediaFile,
    handleBack,
    loadMediaData,
    getSavedPosition: getSavedPositionForMedia,
    savePosition: savePositionForMedia,
    clearSavedPosition: clearSavedPositionForMedia,
    getStartPosition,
    config,
    loadingState,
  };
};