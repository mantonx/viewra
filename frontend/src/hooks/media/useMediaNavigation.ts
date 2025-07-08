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
import { MediaPlaybackService } from '@/services/MediaPlaybackService';
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
      await MediaPlaybackService.stopAllSessions();
      
      setLoadingState(prev => ({ ...prev, isLoading: true, error: null }));

      const mediaFile = await MediaPlaybackService.getMediaFile(mediaId);
      if (!mediaFile || mediaFile.media_type !== mediaType) {
        const errorMsg = `No media file found for this ${mediaType}. ID: ${mediaId}`;
        console.error('❌ useMediaNavigation:', errorMsg);
        setLoadingState(prev => ({ ...prev, isLoading: false, error: errorMsg }));
        return;
      }

      setMediaFile(mediaFile);

      if (mediaFile.duration && mediaFile.duration > 0) {
        setProgressState(prev => ({
          ...prev,
          originalDuration: mediaFile.duration!,
        }));
      }

      const [decision, metadata] = await Promise.all([
        MediaPlaybackService.getPlaybackDecision(mediaFile.path, mediaFile.id),
        MediaPlaybackService.getMediaMetadata(mediaFile.id),
      ]);

      if (metadata) {
        setCurrentMedia(metadata);
      }

      // Check if remuxing or transcoding is needed
      const needsRemux = decision.method === 'remux';
      const needsTranscode = decision.method === 'transcode' || needsRemux;
      
      if (needsTranscode) {
        console.log('🎬 Processing required for:', mediaFile.path);
        console.log('📋 Reason:', decision.reason);
        console.log('🚧 Transcoding/remuxing implementation is pending');
        
        // For now, show an error since transcoding/remuxing isn't implemented
        const processType = needsRemux ? 'remuxing' : 'transcoding';
        const errorMsg = `This media requires ${processType} (${decision.reason}), which is not yet implemented.`;
        setLoadingState(prev => ({ ...prev, isLoading: false, error: errorMsg }));
        return;
        
        // TODO: Uncomment when transcoding is implemented
        // const sessionData = await MediaPlaybackService.startTranscodingSession(
        //   mediaFile.id,
        //   decision.transcode_params?.target_container || 'mp4',
        //   mediaFile.path
        // );
        // const updatedDecision = {
        //   ...decision,
        //   session_id: sessionData.id,
        //   stream_url: sessionData.stream_url || sessionData.manifest_url,
        // };
        // setPlaybackDecision(updatedDecision);
      } else {
        // For direct play, use direct stream URL
        const updatedDecision = {
          ...decision,
          stream_url: MediaPlaybackService.getStreamUrl(mediaFile.id),
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