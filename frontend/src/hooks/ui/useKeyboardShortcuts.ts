import { useCallback, useEffect } from 'react';
import { useAtom } from 'jotai';
import { durationAtom, currentTimeAtom } from '../../atoms/mediaPlayer';

interface UseKeyboardShortcutsProps {
  onSeek: (progress: number) => void;
  onTogglePlayPause?: () => void;
  onToggleMute?: () => void;
  onToggleFullscreen?: () => void;
  skipSeconds?: number;
  enabled?: boolean;
}

export const useKeyboardShortcuts = ({
  onSeek,
  onTogglePlayPause,
  onToggleMute,
  onToggleFullscreen,
  skipSeconds = 10,
  enabled = true,
}: UseKeyboardShortcutsProps) => {
  const [duration] = useAtom(durationAtom);
  const [currentTime] = useAtom(currentTimeAtom);

  const handleKeyDown = useCallback((e: KeyboardEvent) => {
    if (!enabled || duration <= 0) return;

    // Prevent default behavior for media keys
    switch (e.code) {
      case 'ArrowLeft':
        e.preventDefault();
        onSeek(Math.max(0, currentTime - skipSeconds) / duration);
        break;
      case 'ArrowRight':
        e.preventDefault();
        onSeek(Math.min(duration, currentTime + skipSeconds) / duration);
        break;
      case 'Home':
        e.preventDefault();
        onSeek(0);
        break;
      case 'End':
        e.preventDefault();
        onSeek(1);
        break;
      case 'Space':
        e.preventDefault();
        onTogglePlayPause?.();
        break;
      case 'KeyM':
        e.preventDefault();
        onToggleMute?.();
        break;
      case 'KeyF':
        e.preventDefault();
        onToggleFullscreen?.();
        break;
      case 'Escape':
        if (document.fullscreenElement) {
          e.preventDefault();
          document.exitFullscreen();
        }
        break;
    }
  }, [enabled, duration, currentTime, skipSeconds, onSeek, onTogglePlayPause, onToggleMute, onToggleFullscreen]);

  useEffect(() => {
    if (enabled) {
      document.addEventListener('keydown', handleKeyDown);
      return () => document.removeEventListener('keydown', handleKeyDown);
    }
  }, [handleKeyDown, enabled]);

  return {
    handleKeyDown,
  };
};