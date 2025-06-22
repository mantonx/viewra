import { useCallback, useEffect } from 'react';
import { useAtom } from 'jotai';
import { activeSessionsAtom, sessionStateAtom, playbackDecisionAtom } from '../../atoms/mediaPlayer';
import { MediaService } from '../../services/MediaService';
import { API_ENDPOINTS, buildApiUrl } from '../../constants/api';

export const useSessionManager = () => {
  const [activeSessions, setActiveSessions] = useAtom(activeSessionsAtom);
  const [sessionState, setSessionState] = useAtom(sessionStateAtom);
  const [playbackDecision] = useAtom(playbackDecisionAtom);

  const isValidSessionId = useCallback((sessionId: string) => {
    return MediaService.isValidSessionId(sessionId);
  }, []);

  const stopTranscodingSession = useCallback(async (sessionId: string) => {
    if (!sessionId || sessionState.isStoppingSession) return;
    
    if (!isValidSessionId(sessionId)) {
      console.warn('⚠️ Skipping cleanup for invalid/old session ID format:', sessionId);
      setActiveSessions(prev => {
        const newSet = new Set(prev);
        newSet.delete(sessionId);
        return newSet;
      });
      return;
    }
    
    setSessionState(prev => ({ ...prev, isStoppingSession: true }));
    console.log('🛑 Stopping transcoding session (UUID-based):', sessionId);
    
    try {
      await MediaService.stopTranscodingSession(sessionId);
      console.log('✅ Successfully stopped transcoding session:', sessionId);
      setActiveSessions(prev => {
        const newSet = new Set(prev);
        newSet.delete(sessionId);
        return newSet;
      });
    } catch (error) {
      console.error('❌ Error stopping transcoding session:', sessionId, error);
    } finally {
      setSessionState(prev => ({ ...prev, isStoppingSession: false }));
    }
  }, [sessionState.isStoppingSession, isValidSessionId, setSessionState, setActiveSessions]);

  const stopAllSessions = useCallback(async () => {
    const sessions = Array.from(activeSessions);
    if (sessions.length === 0) return;
    
    console.log('🛑 Stopping all active sessions:', sessions);
    await Promise.all(sessions.map(sessionId => stopTranscodingSession(sessionId)));
  }, [activeSessions, stopTranscodingSession]);

  const addSession = useCallback((sessionId: string) => {
    if (isValidSessionId(sessionId)) {
      console.log('📝 Tracking new UUID-based session:', sessionId);
      setActiveSessions(prev => new Set(prev).add(sessionId));
    } else {
      console.warn('⚠️ Received session with invalid/old format, not tracking:', sessionId);
    }
  }, [isValidSessionId, setActiveSessions]);

  const removeSession = useCallback((sessionId: string) => {
    setActiveSessions(prev => {
      const newSet = new Set(prev);
      newSet.delete(sessionId);
      return newSet;
    });
  }, [setActiveSessions]);

  useEffect(() => {
    if (playbackDecision?.session_id) {
      addSession(playbackDecision.session_id);
    }
  }, [playbackDecision?.session_id, addSession]);

  useEffect(() => {
    const handleBeforeUnload = () => {
      const currentSessions = Array.from(activeSessions);
      currentSessions.forEach(sessionId => {
        if (isValidSessionId(sessionId)) {
          const url = buildApiUrl(API_ENDPOINTS.PLAYBACK.SESSION.path(sessionId));
          navigator.sendBeacon(url, 
            JSON.stringify({ method: 'DELETE' }));
        }
      });
    };

    window.addEventListener('beforeunload', handleBeforeUnload);
    return () => window.removeEventListener('beforeunload', handleBeforeUnload);
  }, [activeSessions, isValidSessionId]);

  return {
    activeSessions,
    sessionState,
    stopTranscodingSession,
    stopAllSessions,
    addSession,
    removeSession,
    isValidSessionId,
  };
};