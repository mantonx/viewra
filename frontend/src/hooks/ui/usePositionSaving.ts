import { useCallback, useEffect, useRef } from 'react';
import { useAtom } from 'jotai';
import { currentTimeAtom } from '../../atoms/mediaPlayer';
import { isValidTime } from '../../utils/time';

interface UsePositionSavingProps {
  mediaId: string;
  saveInterval?: number; // seconds
  enabled?: boolean;
}

export const usePositionSaving = ({
  mediaId,
  saveInterval = 5,
  enabled = true,
}: UsePositionSavingProps) => {
  const [currentTime] = useAtom(currentTimeAtom);
  const lastSavedTimeRef = useRef<number>(0);

  const savePosition = useCallback((position: number) => {
    if (!enabled || !mediaId || !isValidTime(position)) return;
    
    localStorage.setItem(`video-position-${mediaId}`, position.toString());
    lastSavedTimeRef.current = position;
  }, [enabled, mediaId]);

  const getSavedPosition = useCallback((): number => {
    if (!enabled || !mediaId) return 0;
    
    const saved = localStorage.getItem(`video-position-${mediaId}`);
    return saved ? parseFloat(saved) : 0;
  }, [enabled, mediaId]);

  const clearSavedPosition = useCallback(() => {
    if (!enabled || !mediaId) return;
    
    localStorage.removeItem(`video-position-${mediaId}`);
    lastSavedTimeRef.current = 0;
  }, [enabled, mediaId]);

  const hasSavedPosition = useCallback((): boolean => {
    if (!enabled || !mediaId) return false;
    
    const saved = localStorage.getItem(`video-position-${mediaId}`);
    return saved !== null && parseFloat(saved) > 0;
  }, [enabled, mediaId]);

  // Auto-save position at intervals
  useEffect(() => {
    if (!enabled || !mediaId || !currentTime) return;

    const timeFloor = Math.floor(currentTime);
    const shouldSave = timeFloor % saveInterval === 0 && timeFloor !== lastSavedTimeRef.current;

    if (shouldSave) {
      savePosition(currentTime);
    }
  }, [currentTime, enabled, mediaId, saveInterval, savePosition]);

  // Save position when component unmounts
  useEffect(() => {
    return () => {
      if (enabled && mediaId && currentTime > 0) {
        savePosition(currentTime);
      }
    };
  }, [enabled, mediaId, currentTime, savePosition]);

  return {
    savePosition,
    getSavedPosition,
    clearSavedPosition,
    hasSavedPosition,
    lastSavedTime: lastSavedTimeRef.current,
  };
};