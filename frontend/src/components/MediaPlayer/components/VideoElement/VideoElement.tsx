import React, { forwardRef, useRef, useImperativeHandle, useCallback, useEffect, useState } from 'react';
import { useAtom } from 'jotai';
import { Play } from 'lucide-react';

import { videoElementAtom, shakaPlayerAtom, configAtom, playerStateAtom } from '@/atoms/mediaPlayer';
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
  const [playerState] = useAtom(playerStateAtom);
  const [hasVideo, setHasVideo] = useState(false);
  
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
        // Safely detach and destroy
        if (typeof shakaPlayer.detach === 'function') {
          shakaPlayer.detach();
        }
        if (typeof shakaPlayer.destroy === 'function') {
          shakaPlayer.destroy();
        }
        setShakaPlayer(null);
      } catch (error) {
        console.warn('Error destroying Shaka player:', error);
      }
    }
    
    // Clear video element source to prevent memory leaks
    if (videoRef.current) {
      videoRef.current.src = '';
      videoRef.current.load();
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

    const handleLoadedData = () => {
      console.log('🎥 Video loaded data event fired');
      setHasVideo(true);
      if (onLoadedData) onLoadedData();
    };
    
    const handleLoadedMetadata = () => {
      console.log('🎥 Video loaded metadata event fired');
      setHasVideo(true);
      if (onLoadedMetadata) onLoadedMetadata();
    };
    
    const handleCanPlay = () => {
      console.log('🎥 Video can play event fired');
      setHasVideo(true);
      if (onCanPlay) onCanPlay();
    };

    const eventHandlers = {
      loadedmetadata: handleLoadedMetadata,
      loadeddata: handleLoadedData,
      timeupdate: onTimeUpdate,
      play: onPlay,
      pause: onPause,
      volumechange: onVolumeChange,
      durationchange: onDurationChange,
      canplay: handleCanPlay,
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
    <div className="relative w-full h-full">
      <video
        ref={videoRef}
        className={cn('w-full h-full object-contain', className)}
        playsInline
        preload={preload}
        autoPlay={autoPlay}
        muted={muted}
      />
      
      {/* Placeholder when no video is loaded */}
      {!hasVideo && (
        <div className="absolute inset-0 flex items-center justify-center pointer-events-none">
          <div className="text-8xl text-white/10 animate-pulse">
            {playerState.isPlaying ? <Play className="w-32 h-32" fill="currentColor" /> : <Play className="w-32 h-32" />}
          </div>
        </div>
      )}
    </div>
  );
});

VideoElement.displayName = 'VideoElement';