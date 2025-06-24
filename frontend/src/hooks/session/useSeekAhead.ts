import { useCallback, useRef } from 'react';
import { useAtom } from 'jotai';
import {
  playbackDecisionAtom,
  mediaFileAtom,
  seekAheadStateAtom,
  progressStateAtom,
  videoElementAtom,
  isBufferingAtom,
  configAtom,
} from '../../atoms/mediaPlayer';
import { MediaService } from '../../services/MediaService';
import { useSessionManager } from './useSessionManager';
import { useMediaPlayer } from '../player/useMediaPlayer';

export const useSeekAhead = () => {
  const [playbackDecision, setPlaybackDecision] = useAtom(playbackDecisionAtom);
  const [mediaFile] = useAtom(mediaFileAtom);
  const [seekAheadState, setSeekAheadState] = useAtom(seekAheadStateAtom);
  const [progressState] = useAtom(progressStateAtom);
  const [videoElement] = useAtom(videoElementAtom);
  const [, setIsBuffering] = useAtom(isBufferingAtom);
  const [config] = useAtom(configAtom);

  const { stopTranscodingSession, addSession } = useSessionManager();
  const { loadNewManifest } = useMediaPlayer();

  const seekOffsetRef = useRef(0);

  const requestSeekAhead = useCallback(async (seekTime: number) => {
    if (!playbackDecision || !mediaFile) {
      console.warn('⚠️ Cannot request seek-ahead: missing playback decision or media file');
      return;
    }

    try {
      console.log('🚀 Requesting seek-ahead to time:', seekTime);
      
      let sessionId = playbackDecision.session_id;
      if (!sessionId && playbackDecision.manifest_url) {
        const urlMatch = playbackDecision.manifest_url.match(/\/stream\/([^/]+)\//);
        if (urlMatch) {
          sessionId = urlMatch[1];
        }
      }

      if (!sessionId) {
        console.warn('⚠️ No session ID available for seek-ahead');
        return;
      }

      // Stop the current session before creating a new seek-ahead session
      // This prevents multiple FFmpeg processes from running simultaneously
      console.log('🛑 Stopping current session before seek-ahead:', sessionId);
      await stopTranscodingSession(sessionId);

      setSeekAheadState(prev => ({ ...prev, isSeekingAhead: true }));
      setIsBuffering(true);

      const seekResponse = await MediaService.requestSeekAhead({
        session_id: sessionId,
        seek_position: Math.floor(seekTime),
      });

      console.log('✅ Seek-ahead transcoding started:', seekResponse);
      
      seekOffsetRef.current = Math.floor(seekTime);
      setSeekAheadState(prev => ({
        ...prev,
        seekOffset: Math.floor(seekTime),
      }));

      if (seekResponse.session_id) {
        addSession(seekResponse.session_id);
      }

      if (seekResponse.manifest_url) {
        console.log('🔄 Switching to new manifest URL:', seekResponse.manifest_url);
        
        setPlaybackDecision(prev => ({
          ...prev!,
          manifest_url: seekResponse.manifest_url,
          session_id: seekResponse.session_id,
        }));

        await loadNewManifest(seekResponse.manifest_url);
        
        if (videoElement) {
          const setupEventListeners = () => {
            const onCanPlay = () => {
              console.log('✅ Seek-ahead content is ready to play (canplay event)');
              setIsBuffering(false);
              setSeekAheadState(prev => ({ ...prev, isSeekingAhead: false }));
              
              if (videoElement) {
                videoElement.play().catch(err => {
                  console.warn('⚠️ Auto-play failed after seek-ahead:', err);
                });
              }
              
              cleanup();
            };
            
            const onLoadedData = () => {
              console.log('✅ Seek-ahead data loaded (loadeddata event)');
              setIsBuffering(false);
              setSeekAheadState(prev => ({ ...prev, isSeekingAhead: false }));
            };
            
            const onProgress = () => {
              if (videoElement && videoElement.buffered.length > 0) {
                const bufferedEnd = videoElement.buffered.end(videoElement.buffered.length - 1);
                if (bufferedEnd > 1) {
                  console.log('✅ Sufficient data buffered, clearing buffering state');
                  setIsBuffering(false);
                  setSeekAheadState(prev => ({ ...prev, isSeekingAhead: false }));
                  cleanup();
                }
              }
            };
            
            const cleanup = () => {
              if (videoElement) {
                videoElement.removeEventListener('canplay', onCanPlay);
                videoElement.removeEventListener('loadeddata', onLoadedData);
                videoElement.removeEventListener('progress', onProgress);
              }
            };
            
            videoElement.addEventListener('canplay', onCanPlay);
            videoElement.addEventListener('loadeddata', onLoadedData);
            videoElement.addEventListener('progress', onProgress);
            
            return cleanup;
          };
          
          const cleanup = setupEventListeners();
          
          setTimeout(() => {
            console.log('⏰ Seek-ahead timeout fallback - clearing buffering state');
            setSeekAheadState(prev => ({ ...prev, isSeekingAhead: false }));
            setIsBuffering(false);
            cleanup();
          }, 10000);
        }
      }
      
    } catch (error) {
      console.error('❌ Failed to request seek-ahead:', error);
      setIsBuffering(false);
      setSeekAheadState(prev => ({ ...prev, isSeekingAhead: false }));
    }
  }, [
    playbackDecision,
    mediaFile,
    setSeekAheadState,
    setIsBuffering,
    setPlaybackDecision,
    addSession,
    stopTranscodingSession,
    loadNewManifest,
    videoElement,
  ]);

  const isSeekAheadNeeded = useCallback((seekTime: number): boolean => {
    if (!videoElement || !playbackDecision) return false;

    const actualBufferedEnd = videoElement.buffered.length > 0 
      ? videoElement.buffered.end(videoElement.buffered.length - 1)
      : 0;

    const isDashOrHls = playbackDecision?.manifest_url && 
      (playbackDecision.manifest_url.includes('.mpd') || playbackDecision.manifest_url.includes('.m3u8'));
    
    return isDashOrHls && seekTime > actualBufferedEnd + 30;
  }, [videoElement, playbackDecision]);

  const getSeekOffset = useCallback(() => {
    return seekOffsetRef.current;
  }, []);

  const resetSeekOffset = useCallback(() => {
    seekOffsetRef.current = 0;
    setSeekAheadState(prev => ({ ...prev, seekOffset: 0 }));
  }, [setSeekAheadState]);

  return {
    requestSeekAhead,
    isSeekAheadNeeded,
    seekAheadState,
    getSeekOffset,
    resetSeekOffset,
  };
};