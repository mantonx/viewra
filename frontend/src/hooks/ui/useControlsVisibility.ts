import { useCallback, useEffect, useRef } from 'react';
import { useAtom } from 'jotai';
import { showControlsAtom } from '../../atoms/mediaPlayer';

interface UseControlsVisibilityProps {
  autoHideDelay?: number;
  containerRef?: React.RefObject<HTMLElement | null>;
  enabled?: boolean;
}

export const useControlsVisibility = ({
  autoHideDelay = 3000,
  containerRef,
  enabled = true,
}: UseControlsVisibilityProps = {}) => {
  const [showControls, setShowControls] = useAtom(showControlsAtom);
  const hideTimeoutRef = useRef<NodeJS.Timeout>();

  const showControlsTemporarily = useCallback(() => {
    if (!enabled) return;
    
    setShowControls(true);
    
    if (hideTimeoutRef.current) {
      clearTimeout(hideTimeoutRef.current);
    }
    
    hideTimeoutRef.current = setTimeout(() => {
      setShowControls(false);
    }, autoHideDelay);
  }, [enabled, autoHideDelay, setShowControls]);

  const hideControlsImmediately = useCallback(() => {
    if (hideTimeoutRef.current) {
      clearTimeout(hideTimeoutRef.current);
    }
    setShowControls(false);
  }, [setShowControls]);

  const showControlsPermanently = useCallback(() => {
    if (hideTimeoutRef.current) {
      clearTimeout(hideTimeoutRef.current);
    }
    setShowControls(true);
  }, [setShowControls]);

  const handleMouseMove = useCallback(() => {
    if (enabled) {
      showControlsTemporarily();
    }
  }, [enabled, showControlsTemporarily]);

  const handleMouseLeave = useCallback(() => {
    if (enabled) {
      hideControlsImmediately();
    }
  }, [enabled, hideControlsImmediately]);

  const handleMouseEnter = useCallback(() => {
    if (enabled) {
      showControlsPermanently();
    }
  }, [enabled, showControlsPermanently]);

  useEffect(() => {
    if (!enabled) return;

    const container = containerRef?.current;
    if (container) {
      container.addEventListener('mousemove', handleMouseMove);
      container.addEventListener('mouseleave', handleMouseLeave);
      
      // Show controls initially
      showControlsTemporarily();

      return () => {
        container.removeEventListener('mousemove', handleMouseMove);
        container.removeEventListener('mouseleave', handleMouseLeave);
      };
    }
  }, [enabled, containerRef, handleMouseMove, handleMouseLeave, showControlsTemporarily]);

  useEffect(() => {
    return () => {
      if (hideTimeoutRef.current) {
        clearTimeout(hideTimeoutRef.current);
      }
    };
  }, []);

  return {
    showControls,
    showControlsTemporarily,
    hideControlsImmediately,
    showControlsPermanently,
    handleMouseMove,
    handleMouseLeave,
    handleMouseEnter,
  };
};