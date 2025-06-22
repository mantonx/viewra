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
        MediaService.getPlaybackDecision(mediaFile.path, deviceProfile),
        MediaService.getMediaMetadata(mediaId, mediaFile.id),
      ]);

      setPlaybackDecision(decision);
      if (metadata) {
        setCurrentMedia(metadata);
      }

      if (decision.should_transcode) {
        console.log('🎬 Starting transcoding session for:', mediaFile.path);
        
        const sessionData = await MediaService.startTranscodingSession(
          mediaFile.path,
          decision.transcode_params?.target_container || 'dash',
          decision.transcode_params?.target_codec || 'h264'
        );

        console.log('✅ Transcoding session started:', sessionData.id);

        setPlaybackDecision(prev => ({
          ...prev!,
          session_id: sessionData.id,
          manifest_url: `/api/playback/stream/${sessionData.id}/manifest.mpd`,
        }));
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
  }, [mediaId, mediaType, setLoadingState, setMediaFile, setProgressState, setPlaybackDecision, setCurrentMedia]);

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
      loadMediaData();
    }
  }, [mediaId, loadMediaData]);

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