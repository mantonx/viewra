import React, { useState, useEffect, useRef } from 'react';
import { ArrowLeft } from 'lucide-react';
import { cn } from '@/utils/cn';
import { VideoControls } from './components/VideoControls';
import { StatusOverlay } from './components/StatusOverlay';
import { MediaInfoOverlay } from './components/MediaInfoOverlay';
import '@/styles/player-theme.css';
import type { MediaType, Episode, Movie } from './types';

interface MediaPlayerDemoProps {
  mediaType: MediaType;
  className?: string;
  autoplay?: boolean;
  onBack?: () => void;
  showBuffering?: boolean;
  showError?: boolean;
  errorMessage?: string;
  initialTime?: number;
}

// Mock media data
const mockEpisode: Episode = {
  id: 'ep-123',
  type: 'episode',
  title: 'The Beginning',
  episode_number: 1,
  season_number: 1,
  description: 'In the series premiere, our heroes embark on an epic journey.',
  duration: 2700, // 45 minutes
  air_date: '2024-01-15',
  series: {
    id: 'series-456',
    title: 'Epic Adventure Series',
    description: 'An amazing series about adventure and discovery',
  },
};

const mockMovie: Movie = {
  id: 'movie-789',
  type: 'movie',
  title: 'The Great Adventure',
  description: 'An epic tale of courage, friendship, and discovery.',
  duration: 7200, // 2 hours
  release_date: '2023-12-25',
  runtime: 120,
};

export const MediaPlayerDemo: React.FC<MediaPlayerDemoProps> = ({
  mediaType,
  className,
  autoplay = false,
  onBack,
  showBuffering = false,
  showError = false,
  errorMessage = 'Failed to load video',
  initialTime = 0,
}) => {
  const containerRef = useRef<HTMLDivElement>(null);
  const [isPlaying, setIsPlaying] = useState(autoplay);
  const [currentTime, setCurrentTime] = useState(initialTime);
  const [volume, setVolume] = useState(0.7);
  const [isMuted, setIsMuted] = useState(false);
  const [isFullscreen, setIsFullscreen] = useState(false);
  const [showControls, setShowControls] = useState(true);
  const [isBuffering] = useState(showBuffering);

  const media = mediaType === 'episode' ? mockEpisode : mockMovie;
  const duration = media.duration || 3600;

  // Mock buffered ranges
  const bufferedRanges = [
    { start: 0, end: Math.min(currentTime + 300, duration) },
  ];

  const handlePlayPause = () => setIsPlaying(!isPlaying);
  const handleSeek = (progress: number) => setCurrentTime(progress * duration);
  const handleVolumeChange = (newVolume: number) => {
    setVolume(newVolume);
    setIsMuted(newVolume === 0);
  };
  const handleToggleMute = () => setIsMuted(!isMuted);
  const handleToggleFullscreen = () => setIsFullscreen(!isFullscreen);
  const handleSkipBackward = () => setCurrentTime(Math.max(0, currentTime - 10));
  const handleSkipForward = () => setCurrentTime(Math.min(duration, currentTime + 10));
  const handleStop = () => {
    setIsPlaying(false);
    setCurrentTime(0);
  };
  const handleRestart = () => {
    setCurrentTime(0);
    setIsPlaying(true);
  };

  // Simulate playback
  useEffect(() => {
    if (isPlaying && currentTime < duration && !showBuffering) {
      const interval = setInterval(() => {
        setCurrentTime(prev => Math.min(prev + 1, duration));
      }, 1000);
      return () => clearInterval(interval);
    }
  }, [isPlaying, currentTime, duration, showBuffering]);

  // Auto-hide controls
  useEffect(() => {
    let timeout: NodeJS.Timeout;
    
    const handleMouseMove = () => {
      setShowControls(true);
      clearTimeout(timeout);
      if (isPlaying) {
        timeout = setTimeout(() => setShowControls(false), 3000);
      }
    };

    const container = containerRef.current;
    if (container) {
      container.addEventListener('mousemove', handleMouseMove);
      return () => {
        clearTimeout(timeout);
        container.removeEventListener('mousemove', handleMouseMove);
      };
    }
  }, [isPlaying]);

  if (showError) {
    return (
      <div className="flex items-center justify-center h-screen player-gradient text-white">
        <div className="text-center max-w-md">
          <h2 className="text-xl font-bold mb-4">Playback Error</h2>
          <p className="text-red-400 mb-4">{errorMessage}</p>
          <button
            onClick={() => window.location.reload()}
            className="px-4 py-2 rounded transition-all duration-200 player-accent-gradient text-white hover:brightness-110 hover:scale-105 active:scale-95"
          >
            Reload Player
          </button>
        </div>
      </div>
    );
  }

  return (
    <div ref={containerRef} className={cn('relative h-screen player-gradient overflow-hidden', className)}>
      {/* Back button */}
      <button
        onClick={onBack || (() => console.log('Back clicked'))}
        className="absolute top-4 left-4 z-50 p-2 rounded-full text-white transition-all player-control-button"
        style={{
          backgroundColor: `rgb(var(--player-surface-overlay))`,
          boxShadow: 'var(--player-shadow-md)',
          backdropFilter: 'blur(12px)',
          transitionDuration: 'var(--player-transition-fast)'
        }}
        title="Go back"
      >
        <ArrowLeft className="w-6 h-6" />
      </button>

      {/* Video container */}
      <div className="relative w-full h-full">
        {/* Fake video background */}
        <div className="w-full h-full bg-gradient-to-br from-slate-900 to-slate-800 flex items-center justify-center">
          <div className="text-8xl text-white/10">
            {isPlaying ? '▶️' : '⏸️'}
          </div>
        </div>

        {/* Status overlays */}
        <StatusOverlay
          isBuffering={isBuffering}
          isSeekingAhead={false}
          isLoading={false}
          error={null}
        />

        {/* Media info overlay */}
        <MediaInfoOverlay
          media={media}
          position="top-left"
          autoHide
          autoHideDelay={5000}
        />

        {/* Video controls */}
        <div
          className={cn(
            'absolute bottom-0 left-0 right-0 p-6 transition-opacity',
            showControls ? 'opacity-100' : 'opacity-0'
          )}
          style={{
            background: `linear-gradient(to top, rgb(var(--player-surface-backdrop)), transparent)`,
            transitionDuration: 'var(--player-transition-slow)'
          }}
        >
          <VideoControls
            isPlaying={isPlaying}
            currentTime={currentTime}
            duration={duration}
            volume={volume}
            isMuted={isMuted}
            isFullscreen={isFullscreen}
            bufferedRanges={bufferedRanges}
            isSeekingAhead={false}
            onPlayPause={handlePlayPause}
            onStop={handleStop}
            onRestart={handleRestart}
            onSeek={handleSeek}
            onSkipBackward={handleSkipBackward}
            onSkipForward={handleSkipForward}
            onVolumeChange={handleVolumeChange}
            onToggleMute={handleToggleMute}
            onToggleFullscreen={handleToggleFullscreen}
            showStopButton
            showSkipButtons
            showVolumeControl
            showFullscreenButton
            showTimeDisplay
            skipSeconds={10}
          />
        </div>
      </div>
    </div>
  );
};