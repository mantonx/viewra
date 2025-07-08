import { useCallback } from 'react';
import { useAtom } from 'jotai';
import { seekAheadStateAtom } from '../../atoms/mediaPlayer';

/**
 * useSeekAhead - Simplified hook for seek operations
 * 
 * With direct MP4 playback, we no longer need complex seek-ahead functionality.
 * This hook is kept for compatibility but simplified to just track seek state.
 */
export const useSeekAhead = () => {
  const [seekAheadState, setSeekAheadState] = useAtom(seekAheadStateAtom);

  const requestSeekAhead = useCallback(async (seekTime: number) => {
    // With direct MP4 playback, seek is handled by the browser's native
    // range request support. No special handling needed.
    console.log('🎯 Seeking to:', seekTime);
    return Promise.resolve();
  }, []);

  const isSeekAheadNeeded = useCallback((seekTime: number): boolean => {
    // Direct MP4 playback handles seeking efficiently with range requests
    return false;
  }, []);

  const getSeekOffset = useCallback(() => {
    return seekAheadState.seekOffset;
  }, [seekAheadState]);

  const resetSeekOffset = useCallback(() => {
    setSeekAheadState({ isSeekingAhead: false, seekOffset: 0 });
  }, [setSeekAheadState]);

  return {
    requestSeekAhead,
    isSeekAheadNeeded,
    seekAheadState,
    getSeekOffset,
    resetSeekOffset,
  };
};