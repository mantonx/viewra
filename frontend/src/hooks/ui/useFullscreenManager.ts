import { useCallback, useEffect } from 'react';
import { useAtom } from 'jotai';
import { isFullscreenAtom } from '../../atoms/mediaPlayer';

export const useFullscreenManager = () => {
  const [isFullscreen, setIsFullscreen] = useAtom(isFullscreenAtom);

  const enterFullscreen = useCallback((element?: HTMLElement) => {
    const target = element || document.documentElement;
    
    if (target.requestFullscreen) {
      target.requestFullscreen();
    }
  }, []);

  const exitFullscreen = useCallback(() => {
    if (document.exitFullscreen) {
      document.exitFullscreen();
    }
  }, []);

  const toggleFullscreen = useCallback((element?: HTMLElement) => {
    if (isFullscreen) {
      exitFullscreen();
    } else {
      enterFullscreen(element);
    }
  }, [isFullscreen, enterFullscreen, exitFullscreen]);

  const handleFullscreenChange = useCallback(() => {
    setIsFullscreen(!!document.fullscreenElement);
  }, [setIsFullscreen]);

  useEffect(() => {
    document.addEventListener('fullscreenchange', handleFullscreenChange);
    return () => document.removeEventListener('fullscreenchange', handleFullscreenChange);
  }, [handleFullscreenChange]);

  return {
    isFullscreen,
    enterFullscreen,
    exitFullscreen,
    toggleFullscreen,
  };
};