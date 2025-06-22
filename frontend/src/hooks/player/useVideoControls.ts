import { useCallback } from 'react';
import { useAtom } from 'jotai';
import { clampTime, isValidTime } from '../../utils/time';
import {
  videoElementAtom,
  playerStateAtom,
  isPlayingAtom,
  volumeAtom,
  isMutedAtom,
  isFullscreenAtom,
  durationAtom,
  currentTimeAtom,
} from '../../atoms/mediaPlayer';
import { useSessionManager } from '../session/useSessionManager';

export const useVideoControls = () => {
  const [videoElement] = useAtom(videoElementAtom);
  const [playerState, setPlayerState] = useAtom(playerStateAtom);
  const [isPlaying, setIsPlaying] = useAtom(isPlayingAtom);
  const [volume, setVolume] = useAtom(volumeAtom);
  const [isMuted, setIsMuted] = useAtom(isMutedAtom);
  const [isFullscreen, setIsFullscreen] = useAtom(isFullscreenAtom);
  const [duration] = useAtom(durationAtom);
  const [currentTime, setCurrentTime] = useAtom(currentTimeAtom);
  
  const { stopAllSessions } = useSessionManager();

  const togglePlayPause = useCallback(() => {
    if (videoElement) {
      if (isPlaying) {
        videoElement.pause();
      } else {
        videoElement.play();
      }
    }
  }, [videoElement, isPlaying]);

  const play = useCallback(() => {
    if (videoElement) {
      videoElement.play();
    }
  }, [videoElement]);

  const pause = useCallback(() => {
    if (videoElement) {
      videoElement.pause();
    }
  }, [videoElement]);

  const stop = useCallback(() => {
    if (videoElement) {
      videoElement.pause();
      videoElement.currentTime = 0;
      setCurrentTime(0);
    }
  }, [videoElement, setCurrentTime]);

  const restartFromBeginning = useCallback(async () => {
    if (videoElement) {
      console.log('🔄 Restarting video from beginning, cleaning up sessions...');
      
      await stopAllSessions();
      
      videoElement.currentTime = 0;
      setCurrentTime(0);
      videoElement.play();
      console.log('✅ Video restarted from beginning, sessions cleaned up');
    }
  }, [videoElement, setCurrentTime, stopAllSessions]);

  const seek = useCallback((time: number) => {
    if (videoElement && duration > 0 && isValidTime(time)) {
      const clampedTime = clampTime(time, duration);
      videoElement.currentTime = clampedTime;
      setCurrentTime(clampedTime);
    }
  }, [videoElement, duration, setCurrentTime]);

  const seekByProgress = useCallback((progress: number) => {
    if (duration > 0) {
      const time = progress * duration;
      seek(time);
    }
  }, [duration, seek]);

  const skipBackward = useCallback((seconds: number = 10) => {
    const newTime = Math.max(0, currentTime - seconds);
    seek(newTime);
  }, [currentTime, seek]);

  const skipForward = useCallback((seconds: number = 10) => {
    const newTime = Math.min(duration, currentTime + seconds);
    seek(newTime);
  }, [currentTime, duration, seek]);

  const setVideoVolume = useCallback((newVolume: number) => {
    if (videoElement && isValidTime(newVolume)) {
      const clampedVolume = clampTime(newVolume, 1); // Volume is 0-1
      videoElement.volume = clampedVolume;
      setVolume(clampedVolume);
    }
  }, [videoElement, setVolume]);

  const toggleMute = useCallback(() => {
    if (videoElement) {
      videoElement.muted = !videoElement.muted;
      setIsMuted(videoElement.muted);
    }
  }, [videoElement, setIsMuted]);

  const mute = useCallback(() => {
    if (videoElement) {
      videoElement.muted = true;
      setIsMuted(true);
    }
  }, [videoElement, setIsMuted]);

  const unmute = useCallback(() => {
    if (videoElement) {
      videoElement.muted = false;
      setIsMuted(false);
    }
  }, [videoElement, setIsMuted]);

  const toggleFullscreen = useCallback(() => {
    if (!document.fullscreenElement) {
      videoElement?.parentElement?.requestFullscreen();
    } else {
      document.exitFullscreen();
    }
  }, [videoElement]);

  const enterFullscreen = useCallback(() => {
    if (!document.fullscreenElement && videoElement?.parentElement) {
      videoElement.parentElement.requestFullscreen();
    }
  }, [videoElement]);

  const exitFullscreen = useCallback(() => {
    if (document.fullscreenElement) {
      document.exitFullscreen();
    }
  }, []);

  return {
    // Playback controls
    togglePlayPause,
    play,
    pause,
    stop,
    restartFromBeginning,
    
    // Seeking controls
    seek,
    seekByProgress,
    skipBackward,
    skipForward,
    
    // Volume controls
    setVolume: setVideoVolume,
    toggleMute,
    mute,
    unmute,
    
    // Fullscreen controls
    toggleFullscreen,
    enterFullscreen,
    exitFullscreen,
    
    // State
    isPlaying,
    volume,
    isMuted,
    isFullscreen,
    duration,
    currentTime,
  };
};