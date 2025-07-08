import React, { useRef, useCallback, useEffect, useState } from 'react';
import { 
  Play, 
  Pause, 
  SkipBack, 
  SkipForward, 
  Volume2, 
  VolumeX,
  ChevronUp,
  ChevronDown,
  Repeat,
  Shuffle,
  List,
  X,
  Music
} from 'lucide-react';
import { MediaPlayer as VidstackPlayer, MediaProvider, type MediaPlayerInstance } from '@vidstack/react';
import { useMediaRemote, useMediaStore } from '@vidstack/react';
import { cn } from '@/utils/cn';
import { MediaPlaybackService } from '@/services/MediaPlaybackService';
import type { PlaybackDecision } from '@/services/MediaPlaybackService';
import { PlaybackSessionTracker } from '@/utils/analytics';
import { getDeviceProfile } from '@/utils/deviceProfile';

interface MusicPlayerProps {
  mediaFileId: string;
  title?: string;
  artist?: string;
  album?: string;
  coverUrl?: string;
  playlist?: Array<{
    id: string;
    title: string;
    artist?: string;
    duration?: number;
  }>;
  onNext?: () => void;
  onPrevious?: () => void;
  className?: string;
}

/**
 * MusicPlayer - Dedicated audio playback component using Vidstack
 * 
 * Features:
 * - Clean, modern UI optimized for music playback
 * - Uses Vidstack for consistent playback experience
 * - Supports all playback methods (direct, remux, transcode)
 * - Playlist support with queue management
 * - Repeat and shuffle modes
 * - Minimizable player mode
 * - Progressive audio streaming
 * - Analytics tracking
 */
export const MusicPlayer: React.FC<MusicPlayerProps> = ({
  mediaFileId,
  title = 'Unknown Track',
  artist = 'Unknown Artist',
  album,
  coverUrl,
  playlist = [],
  onNext,
  onPrevious,
  className
}) => {
  const playerRef = useRef<MediaPlayerInstance>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [isMinimized, setIsMinimized] = useState(false);
  const [showPlaylist, setShowPlaylist] = useState(false);
  const [repeatMode, setRepeatMode] = useState<'none' | 'one' | 'all'>('none');
  const [isShuffled, setIsShuffled] = useState(false);
  const [playbackDecision, setPlaybackDecision] = useState<PlaybackDecision | null>(null);
  const [sessionId, setSessionId] = useState<string | null>(null);
  const [playbackUrl, setPlaybackUrl] = useState<string>('');
  const [sessionTracker, setSessionTracker] = useState<PlaybackSessionTracker | null>(null);

  // Initialize playback
  useEffect(() => {
    let mounted = true;
    let currentSessionId: string | null = null;

    const initPlayback = async () => {
      try {
        setIsLoading(true);
        setError(null);

        // Get playback decision
        // Get the media file first to get its path
        const mediaFile = await MediaPlaybackService.getMediaFile(mediaFileId);
        if (!mediaFile) {
          throw new Error('Media file not found');
        }
        const decision = await MediaPlaybackService.getPlaybackDecision(mediaFile.path, mediaFileId);
        if (!mounted) return;
        
        setPlaybackDecision(decision);

        let audioUrl = '';

        // Start session if needed
        if (decision.should_transcode) {
          const session = await MediaPlaybackService.startTranscodingSession(mediaFileId);
          
          if (!mounted) return;
          
          currentSessionId = session.id;
          setSessionId(session.id);
          audioUrl = session.stream_url || session.manifest_url || '';
        } else {
          // Direct play
          audioUrl = MediaPlaybackService.getStreamUrl(mediaFileId);
        }

        setPlaybackUrl(audioUrl);

        // Initialize analytics
        const deviceProfile = await getDeviceProfile();
        const tracker = new PlaybackSessionTracker(
          currentSessionId || 'direct-play',
          mediaFileId,
          'music',
          deviceProfile
        );
        setSessionTracker(tracker);

        console.log('🎵 Music playback initialized:', {
          should_transcode: decision.should_transcode,
          reason: decision.reason,
          url: audioUrl,
          sessionId: currentSessionId
        });

      } catch (err) {
        if (!mounted) return;
        setError(err instanceof Error ? err.message : 'Failed to load audio');
      } finally {
        if (mounted) {
          setIsLoading(false);
        }
      }
    };

    initPlayback();

    // Cleanup
    return () => {
      mounted = false;
      if (currentSessionId) {
        MediaPlaybackService.stopSession(currentSessionId).catch(console.error);
      }
      if (sessionTracker) {
        sessionTracker.stopTracking();
      }
    };
  }, [mediaFileId]);

  const handleEnded = useCallback(() => {
    if (repeatMode === 'one' && playerRef.current) {
      playerRef.current.play();
    } else if (repeatMode === 'all' || onNext) {
      onNext?.();
    }
  }, [repeatMode, onNext]);

  const toggleRepeat = useCallback(() => {
    const modes: Array<'none' | 'one' | 'all'> = ['none', 'one', 'all'];
    const currentIndex = modes.indexOf(repeatMode);
    setRepeatMode(modes[(currentIndex + 1) % modes.length]);
  }, [repeatMode]);

  const formatTime = (seconds: number): string => {
    if (!seconds || !isFinite(seconds)) return '0:00';
    const mins = Math.floor(seconds / 60);
    const secs = Math.floor(seconds % 60);
    return `${mins}:${secs.toString().padStart(2, '0')}`;
  };

  if (error) {
    return (
      <div className={cn('bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-lg p-4', className)}>
        <p className="text-red-600 dark:text-red-400">Error: {error}</p>
      </div>
    );
  }

  return (
    <div className={cn(
      'bg-white dark:bg-gray-900 rounded-lg shadow-lg transition-all duration-300',
      isMinimized ? 'h-20' : 'h-auto',
      className
    )}>
      {/* Hidden Vidstack player */}
      <div className="hidden">
        <VidstackPlayer
          ref={playerRef}
          src={playbackUrl}
          onEnded={handleEnded}
          onError={(event) => {
            setError('Playback error occurred');
            sessionTracker?.trackEvent('error', { error: event });
          }}
          onLoadedMetadata={() => {
            console.log('🎵 Audio metadata loaded');
          }}
        >
          <MediaProvider />
        </VidstackPlayer>
      </div>
      
      {/* Main Player UI */}
      <MusicPlayerControls
        playerRef={playerRef}
        isLoading={isLoading}
        isMinimized={isMinimized}
        setIsMinimized={setIsMinimized}
        title={title}
        artist={artist}
        album={album}
        coverUrl={coverUrl}
        playbackDecision={playbackDecision}
        onPrevious={onPrevious}
        onNext={onNext}
        repeatMode={repeatMode}
        toggleRepeat={toggleRepeat}
        isShuffled={isShuffled}
        setIsShuffled={setIsShuffled}
        showPlaylist={showPlaylist}
        setShowPlaylist={setShowPlaylist}
        formatTime={formatTime}
        sessionTracker={sessionTracker}
      />

      {/* Playlist */}
      {showPlaylist && playlist.length > 0 && (
        <div className="border-t border-gray-200 dark:border-gray-700 max-h-64 overflow-y-auto">
          <div className="p-2">
            <div className="flex justify-between items-center mb-2 px-2">
              <h4 className="text-sm font-semibold">Playlist</h4>
              <button
                onClick={() => setShowPlaylist(false)}
                className="p-1 hover:bg-gray-100 dark:hover:bg-gray-800 rounded"
              >
                <X className="w-4 h-4" />
              </button>
            </div>
            {playlist.map((track, index) => (
              <div
                key={track.id}
                className={cn(
                  'flex items-center gap-3 p-2 rounded hover:bg-gray-100 dark:hover:bg-gray-800 cursor-pointer',
                  track.id === mediaFileId && 'bg-gray-100 dark:bg-gray-800'
                )}
              >
                <span className="text-sm text-gray-500 w-6">{index + 1}</span>
                <div className="flex-1 min-w-0">
                  <p className="text-sm font-medium truncate">{track.title}</p>
                  {track.artist && (
                    <p className="text-xs text-gray-600 dark:text-gray-400 truncate">
                      {track.artist}
                    </p>
                  )}
                </div>
                {track.duration && (
                  <span className="text-xs text-gray-500">
                    {formatTime(track.duration)}
                  </span>
                )}
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  );
};

// Separate component for controls to use Vidstack hooks
const MusicPlayerControls: React.FC<{
  playerRef: React.RefObject<MediaPlayerInstance>;
  isLoading: boolean;
  isMinimized: boolean;
  setIsMinimized: (value: boolean) => void;
  title: string;
  artist: string;
  album?: string;
  coverUrl?: string;
  playbackDecision: PlaybackDecision | null;
  onPrevious?: () => void;
  onNext?: () => void;
  repeatMode: 'none' | 'one' | 'all';
  toggleRepeat: () => void;
  isShuffled: boolean;
  setIsShuffled: (value: boolean) => void;
  showPlaylist: boolean;
  setShowPlaylist: (value: boolean) => void;
  formatTime: (seconds: number) => string;
  sessionTracker: PlaybackSessionTracker | null;
}> = ({
  playerRef,
  isLoading,
  isMinimized,
  setIsMinimized,
  title,
  artist,
  album,
  coverUrl,
  playbackDecision,
  onPrevious,
  onNext,
  repeatMode,
  toggleRepeat,
  isShuffled,
  setIsShuffled,
  showPlaylist,
  setShowPlaylist,
  formatTime,
  sessionTracker
}) => {
  const remote = useMediaRemote(playerRef);
  const store = useMediaStore(playerRef);
  
  const isPlaying = store.playing || false;
  const currentTime = store.currentTime || 0;
  const duration = store.duration || 0;
  const volume = store.volume || 1;
  const isMuted = store.muted || false;

  // Track analytics
  useEffect(() => {
    if (sessionTracker) {
      sessionTracker.updatePlaybackState(currentTime, duration);
    }
  }, [sessionTracker, currentTime, duration]);

  const togglePlayPause = useCallback(() => {
    if (!remote) return;
    
    if (isPlaying) {
      remote.pause();
      sessionTracker?.trackEvent('pause');
    } else {
      remote.play();
      sessionTracker?.trackEvent('play');
    }
  }, [remote, isPlaying, sessionTracker]);

  const handleSeek = useCallback((e: React.MouseEvent<HTMLDivElement>) => {
    if (!remote || !duration) return;

    const rect = e.currentTarget.getBoundingClientRect();
    const x = e.clientX - rect.left;
    const percentage = x / rect.width;
    const newTime = percentage * duration;

    remote.seek(newTime);
    sessionTracker?.trackEvent('seek', { seekTime: newTime });
  }, [remote, duration, sessionTracker]);

  const handleVolumeChange = useCallback((e: React.ChangeEvent<HTMLInputElement>) => {
    if (!remote) return;
    const newVolume = parseFloat(e.target.value);
    remote.setVolume(newVolume);
  }, [remote]);

  const toggleMute = useCallback(() => {
    if (!remote) return;
    remote.setMuted(!isMuted);
  }, [remote, isMuted]);

  return (
    <div className="p-6">
      {/* Header with minimize button */}
      <div className="flex justify-between items-start mb-4">
        <div className="flex-1">
          <h3 className="text-lg font-semibold text-gray-900 dark:text-white truncate">
            {title}
          </h3>
          <p className="text-sm text-gray-600 dark:text-gray-400 truncate">
            {artist} {album && `• ${album}`}
          </p>
        </div>
        <button
          onClick={() => setIsMinimized(!isMinimized)}
          className="p-1 hover:bg-gray-100 dark:hover:bg-gray-800 rounded"
        >
          {isMinimized ? <ChevronUp className="w-5 h-5" /> : <ChevronDown className="w-5 h-5" />}
        </button>
      </div>

      {!isMinimized && (
        <>
          {/* Album Cover */}
          {coverUrl ? (
            <div className="mb-6 mx-auto w-48 h-48 rounded-lg overflow-hidden shadow-md">
              <img
                src={coverUrl}
                alt={`${album || title} cover`}
                className="w-full h-full object-cover"
              />
            </div>
          ) : (
            <div className="mb-6 mx-auto w-48 h-48 rounded-lg bg-gray-200 dark:bg-gray-700 flex items-center justify-center">
              <Music className="w-20 h-20 text-gray-400 dark:text-gray-600" />
            </div>
          )}

          {/* Progress Bar */}
          <div className="mb-4">
            <div 
              className="relative h-2 bg-gray-200 dark:bg-gray-700 rounded-full cursor-pointer"
              onClick={handleSeek}
            >
              <div 
                className="absolute h-full bg-blue-500 rounded-full"
                style={{ width: `${(currentTime / duration) * 100 || 0}%` }}
              />
            </div>
            <div className="flex justify-between mt-1 text-xs text-gray-600 dark:text-gray-400">
              <span>{formatTime(currentTime)}</span>
              <span>{formatTime(duration)}</span>
            </div>
          </div>

          {/* Playback Info */}
          {playbackDecision && (
            <div className="mb-4 text-xs text-gray-500 text-center">
              {!playbackDecision.should_transcode && 'Direct Play'}
              {playbackDecision.should_transcode && playbackDecision.transcode_params?.remux_only && 'Remuxing'}
              {playbackDecision.should_transcode && !playbackDecision.transcode_params?.remux_only && 'Transcoding'}
            </div>
          )}
        </>
      )}

      {/* Controls */}
      <div className="flex items-center justify-center gap-4">
        {/* Shuffle */}
        <button
          onClick={() => setIsShuffled(!isShuffled)}
          className={cn(
            'p-2 rounded hover:bg-gray-100 dark:hover:bg-gray-800',
            isShuffled && 'text-blue-500'
          )}
          title="Shuffle"
        >
          <Shuffle className="w-5 h-5" />
        </button>

        {/* Previous */}
        <button
          onClick={onPrevious}
          disabled={!onPrevious}
          className="p-2 rounded hover:bg-gray-100 dark:hover:bg-gray-800 disabled:opacity-50"
          title="Previous"
        >
          <SkipBack className="w-5 h-5" />
        </button>

        {/* Play/Pause */}
        <button
          onClick={togglePlayPause}
          disabled={isLoading}
          className="p-3 bg-blue-500 text-white rounded-full hover:bg-blue-600 disabled:opacity-50"
          title={isPlaying ? 'Pause' : 'Play'}
        >
          {isLoading ? (
            <div className="w-6 h-6 border-2 border-white border-t-transparent rounded-full animate-spin" />
          ) : isPlaying ? (
            <Pause className="w-6 h-6" />
          ) : (
            <Play className="w-6 h-6 ml-0.5" />
          )}
        </button>

        {/* Next */}
        <button
          onClick={onNext}
          disabled={!onNext}
          className="p-2 rounded hover:bg-gray-100 dark:hover:bg-gray-800 disabled:opacity-50"
          title="Next"
        >
          <SkipForward className="w-5 h-5" />
        </button>

        {/* Repeat */}
        <button
          onClick={toggleRepeat}
          className={cn(
            'p-2 rounded hover:bg-gray-100 dark:hover:bg-gray-800 relative',
            repeatMode !== 'none' && 'text-blue-500'
          )}
          title={`Repeat: ${repeatMode}`}
        >
          <Repeat className="w-5 h-5" />
          {repeatMode === 'one' && (
            <span className="absolute -top-1 -right-1 text-xs bg-blue-500 text-white rounded-full w-4 h-4 flex items-center justify-center">
              1
            </span>
          )}
        </button>
      </div>

      {/* Volume and Playlist controls */}
      {!isMinimized && (
        <div className="flex items-center justify-between mt-6">
          {/* Volume */}
          <div className="flex items-center gap-2 flex-1">
            <button onClick={toggleMute} className="p-1">
              {isMuted ? <VolumeX className="w-5 h-5" /> : <Volume2 className="w-5 h-5" />}
            </button>
            <input
              type="range"
              min="0"
              max="1"
              step="0.01"
              value={isMuted ? 0 : volume}
              onChange={handleVolumeChange}
              className="w-24"
            />
          </div>

          {/* Playlist toggle */}
          <button
            onClick={() => setShowPlaylist(!showPlaylist)}
            className="p-2 rounded hover:bg-gray-100 dark:hover:bg-gray-800"
            title="Show playlist"
          >
            <List className="w-5 h-5" />
          </button>
        </div>
      )}
    </div>
  );
};