import React, { forwardRef, useRef, useImperativeHandle, useCallback, useEffect } from 'react';
import { useAtom } from 'jotai';

import { videoElementAtom, shakaPlayerAtom, configAtom } from '@/atoms/mediaPlayer';
import type { VideoElementProps, VideoElementRef } from './types';
import { cn } from '@/utils/cn';

export const VideoElement = forwardRef<VideoElementRef, VideoElementProps>(({
  className,
  onLoadedMetadata,
  onLoadedData,
  onTimeUpdate,
  onPlay,
  onPause,
  onVolumeChange,
  onDurationChange,
  onCanPlay,
  onWaiting,
  onPlaying,
  onStalled,
  onDoubleClick,
  autoPlay = false,
  muted = false,
  preload = 'auto',
}, ref) => {
  const videoRef = useRef<HTMLVideoElement>(null);
  const [, setVideoElement] = useAtom(videoElementAtom);
  const [shakaPlayer, setShakaPlayer] = useAtom(shakaPlayerAtom);
  const [config] = useAtom(configAtom);
  
  const cleanupRef = useRef<(() => void) | null>(null);

  const loadManifest = useCallback(async (url: string) => {
    if (!shakaPlayer) {
      throw new Error('Shaka player not initialized');
    }
    
    try {
      await shakaPlayer.load(url);
      if (config.debug) console.log('✅ Manifest loaded:', url);
    } catch (error) {
      console.error('❌ Failed to load manifest:', error);
      throw error;
    }
  }, [shakaPlayer, config.debug]);

  const destroy = useCallback(() => {
    if (cleanupRef.current) {
      cleanupRef.current();
      cleanupRef.current = null;
    }
    
    if (shakaPlayer) {
      try {
        shakaPlayer.destroy();
        setShakaPlayer(null);
      } catch (error) {
        console.warn('Error destroying Shaka player:', error);
      }
    }
  }, [shakaPlayer, setShakaPlayer]);

  useImperativeHandle(ref, () => ({
    videoElement: videoRef.current,
    shakaPlayer,
    loadManifest,
    destroy,
  }), [shakaPlayer, loadManifest, destroy]);

  useEffect(() => {
    const video = videoRef.current;
    if (!video) return;

    setVideoElement(video);

    const eventHandlers = {
      loadedmetadata: onLoadedMetadata,
      loadeddata: onLoadedData,
      timeupdate: onTimeUpdate,
      play: onPlay,
      pause: onPause,
      volumechange: onVolumeChange,
      durationchange: onDurationChange,
      canplay: onCanPlay,
      waiting: onWaiting,
      playing: onPlaying,
      stalled: onStalled,
      dblclick: onDoubleClick,
    };

    const cleanup = () => {
      Object.entries(eventHandlers).forEach(([event, handler]) => {
        if (handler) {
          video.removeEventListener(event, handler);
        }
      });
    };

    Object.entries(eventHandlers).forEach(([event, handler]) => {
      if (handler) {
        video.addEventListener(event, handler);
      }
    });

    cleanupRef.current = cleanup;

    return () => {
      cleanup();
      setVideoElement(null);
    };
  }, [
    setVideoElement,
    onLoadedMetadata,
    onLoadedData,
    onTimeUpdate,
    onPlay,
    onPause,
    onVolumeChange,
    onDurationChange,
    onCanPlay,
    onWaiting,
    onPlaying,
    onStalled,
    onDoubleClick,
  ]);

  return (
    <video
      ref={videoRef}
      className={cn('w-full h-full object-contain', className)}
      playsInline
      preload={preload}
      autoPlay={autoPlay}
      muted={muted}
    />
  );
});

VideoElement.displayName = 'VideoElement';