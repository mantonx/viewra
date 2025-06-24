import { useCallback, useRef } from 'react';
import { MediaService } from '@/services/MediaService';
import type { MediaFile, TranscodingSession } from '@/components/MediaPlayer/types';

interface PreTranscodeOptions {
  onStart?: (sessionId: string) => void;
  onError?: (error: Error) => void;
  delay?: number; // Delay before starting pre-transcode (ms)
}

export const usePreTranscode = (options: PreTranscodeOptions = {}) => {
  const { onStart, onError, delay = 500 } = options;
  const sessionRef = useRef<string | null>(null);
  const timeoutRef = useRef<NodeJS.Timeout | null>(null);

  const startPreTranscode = useCallback(async (mediaFile: MediaFile) => {
    // Clear any existing timeout
    if (timeoutRef.current) {
      clearTimeout(timeoutRef.current);
      timeoutRef.current = null;
    }

    // Set a delay to avoid starting on quick hovers
    timeoutRef.current = setTimeout(async () => {
      try {
        // Don't start if we already have a session for this file
        if (sessionRef.current) {
          return;
        }

        console.log('🚀 Starting pre-transcode for:', mediaFile.file_path);

        // Get playback decision first
        const decision = await MediaService.getPlaybackDecision(
          mediaFile.file_path,
          mediaFile.id
        );

        // Only pre-transcode if transcoding is needed
        if (!decision.should_transcode) {
          console.log('✅ Direct play available, no pre-transcode needed');
          return;
        }

        // Start transcoding with lowest quality first for fast startup
        const session = await MediaService.startTranscodingSession(
          mediaFile.id,
          'dash',
          'h264',
          'aac',
          30, // Lower quality for pre-transcode
          'fastest'
        );

        sessionRef.current = session.session_id;
        console.log('✅ Pre-transcode started:', session.session_id);
        
        if (onStart) {
          onStart(session.session_id);
        }

        // Keep the session warm but don't fully transcode
        // Just get the first few segments ready
        setTimeout(() => {
          if (sessionRef.current === session.session_id) {
            console.log('⏸️ Pre-transcode reached warm state');
            // Session stays active, ready for immediate playback
          }
        }, 5000); // 5 seconds to prepare initial segments

      } catch (error) {
        console.error('❌ Pre-transcode failed:', error);
        if (onError) {
          onError(error as Error);
        }
      }
    }, delay);
  }, [delay, onStart, onError]);

  const cancelPreTranscode = useCallback(() => {
    // Clear timeout if hover ends quickly
    if (timeoutRef.current) {
      clearTimeout(timeoutRef.current);
      timeoutRef.current = null;
    }

    // Don't stop the session - keep it warm for potential playback
    console.log('🛑 Pre-transcode hover ended, keeping session warm');
  }, []);

  const stopPreTranscode = useCallback(async () => {
    // Actually stop the transcoding session
    if (sessionRef.current) {
      try {
        await MediaService.stopTranscodingSession(sessionRef.current);
        console.log('🛑 Pre-transcode session stopped:', sessionRef.current);
      } catch (error) {
        console.error('Failed to stop pre-transcode:', error);
      }
      sessionRef.current = null;
    }
  }, []);

  const getSessionId = useCallback(() => {
    return sessionRef.current;
  }, []);

  return {
    startPreTranscode,
    cancelPreTranscode,
    stopPreTranscode,
    getSessionId,
  };
};