import React, { useState, useEffect, useRef, useCallback } from 'react';
import {
  Volume2,
  VolumeX,
  SkipForward,
  SkipBack,
  Repeat,
  Shuffle,
  ChevronUp,
  ChevronDown,
  Info,
  X,
  Clock,
  FileAudio,
  HardDrive,
  Disc,
  Activity,
} from '@/components/ui/icons';
import AudioBadge from './AudioBadge';
import AlbumArtwork from './AlbumArtwork';
import AnimatedPlayPause from '@/components/ui/AnimatedPlayPause';
import { cn } from '@/lib/utils';
import IconButton from '@/components/ui/IconButton';
import { Tooltip } from 'react-tooltip';
import { MusicFile } from '@/components/media/music.types';
import { buildArtworkUrl } from '@/utils/api';

interface MusicMetadata {
  id: number;
  media_file_id: number;
  title: string;
  album: string;
  artist: string;
  album_artist: string;
  genre: string;
  year: number;
  track: number;
  track_total: number;
  disc: number;
  disc_total: number;
  duration: number;
  bitrate: number;
  sample_rate: number;
  channels: number;
  format: string;
  has_artwork: boolean;
}

interface AudioPlayerProps {
  currentTrack: MusicFile | null;
  isPlaying: boolean;
  currentTime: number;
  duration: number;
  volume: number;
  playbackRate: number;
  audioRef: React.RefObject<HTMLAudioElement>;
  onPlayPause: () => void;
  onSeek: (e: React.ChangeEvent<HTMLInputElement>) => void;
  onVolumeChange: (e: React.ChangeEvent<HTMLInputElement>) => void;
  onPlaybackRateChange: (rate: number) => void;
  onNext?: () => void;
  onPrevious?: () => void;
  onShuffle?: () => void;
  onRepeat?: () => void;
  formatTime: (seconds: number) => string;
  isShuffleOn?: boolean;
  isRepeatOn?: boolean;
}

const AudioPlayer: React.FC<AudioPlayerProps> = ({
  currentTrack,
  isPlaying,
  currentTime,
  duration,
  volume,
  playbackRate,
  audioRef,
  onPlayPause,
  onSeek,
  onVolumeChange,
  onPlaybackRateChange,
  onNext,
  onPrevious,
  onShuffle,
  onRepeat,
  formatTime,
  isShuffleOn = false,
  isRepeatOn = false,
}) => {
  const [isMinimized, setIsMinimized] = useState(false);
  const [isVisible, setIsVisible] = useState(false);
  const [userHasManuallyToggled, setUserHasManuallyToggled] = useState(false);
  const [showNerdInfo, setShowNerdInfo] = useState(false);

  // Track player visibility with slide-in animation
  useEffect(() => {
    if (currentTrack && !isVisible) {
      // Delay to ensure smooth slide-in animation
      const timer = setTimeout(() => {
        setIsVisible(true);
      }, 100);
      return () => clearTimeout(timer);
    } else if (!currentTrack && isVisible) {
      // Slide out when track is removed
      setIsVisible(false);
    }
  }, [currentTrack, isVisible]);

  // Handle manual toggle of minimized state
  const toggleMinimize = () => {
    setIsMinimized(!isMinimized);
    setUserHasManuallyToggled(true);
  };

  // Auto-minimize on scroll (only if user hasn't manually toggled)
  useEffect(() => {
    if (userHasManuallyToggled) return;

    const handleScroll = () => {
      const scrolledDown = window.scrollY > 100;
      setIsMinimized(scrolledDown);
    };

    window.addEventListener('scroll', handleScroll);
    return () => window.removeEventListener('scroll', handleScroll);
  }, [userHasManuallyToggled]);

  // Close nerd info panel when clicking outside or pressing Escape
  useEffect(() => {
    if (!showNerdInfo) return;

    const handleClickOutside = (event: MouseEvent) => {
      const target = event.target as Element;
      if (!target.closest('[data-nerd-info]')) {
        setShowNerdInfo(false);
      }
    };

    const handleEscape = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        setShowNerdInfo(false);
      }
    };

    document.addEventListener('mousedown', handleClickOutside);
    document.addEventListener('keydown', handleEscape);

    return () => {
      document.removeEventListener('mousedown', handleClickOutside);
      document.removeEventListener('keydown', handleEscape);
    };
  }, [showNerdInfo]);

  // Don't render anything if no track
  if (!currentTrack) return null;

  const audioFormat = currentTrack.music_metadata.format || 'Unknown';
  const audioBitrate = currentTrack.music_metadata.bitrate || 0;

  return (
    <div
      className={cn(
        'fixed bottom-0 left-0 right-0 bg-gradient-to-r from-slate-800 to-slate-900 shadow-2xl border-t border-slate-700 z-50 transform transition-all duration-500 ease-in-out backdrop-blur-sm',
        isVisible ? 'translate-y-0' : 'translate-y-full',
        isMinimized ? 'py-3 px-4' : 'py-6 px-8'
      )}
      style={{
        marginBottom: '0px',
        paddingBottom: '12px',
        bottom: '0px',
        position: 'fixed',
        boxShadow: '0 -10px 25px -5px rgba(0, 0, 0, 0.3)',
      }}
    >
      <div className="w-full max-w-none mx-auto">
        <div
          className={cn(
            'flex items-center gap-8 w-full',
            isMinimized ? 'justify-between' : 'justify-between'
          )}
        >
          {/* Left Section: Track Artwork & Info */}
          <div className="flex items-center gap-4 flex-1 min-w-0">
            {/* Track Artwork */}
            <AlbumArtwork
              artworkUrl={
                currentTrack?.music_metadata?.has_artwork
                  ? buildArtworkUrl(currentTrack.id)
                  : undefined
              }
              altText={currentTrack?.music_metadata?.album || 'Album Artwork'}
              isPlaying={isPlaying}
              isMinimized={isMinimized}
              className={isMinimized ? 'w-10 h-10' : 'w-16 h-16'}
              trackTitle={currentTrack?.music_metadata?.title}
              artistName={currentTrack?.music_metadata?.artist}
              albumName={currentTrack?.music_metadata?.album}
            />

            {/* Track Info */}
            <div className="flex-1 min-w-0">
              <h3 className="text-white font-medium truncate">
                {currentTrack?.music_metadata?.title || currentTrack?.path.split('/').pop()}
              </h3>
              <p className="text-slate-400 text-sm truncate">
                {currentTrack?.music_metadata?.artist || 'Unknown Artist'}
              </p>

              {/* Progress Bar (hidden when minimized) */}
              {!isMinimized && (
                <div className="flex items-center gap-2 mt-3">
                  <span className="text-xs text-slate-500 w-10 font-mono">
                    {formatTime(currentTime)}
                  </span>
                  <div className="relative flex-1 group">
                    <input
                      type="range"
                      min="0"
                      max={duration || 100}
                      value={currentTime || 0}
                      onChange={onSeek}
                      className="absolute inset-0 w-full h-full opacity-0 cursor-pointer z-10"
                    />
                    <div className="h-2 bg-slate-600 rounded-full overflow-hidden group-hover:h-3 transition-all">
                      <div
                        className="h-full bg-gradient-to-r from-purple-500 to-purple-600 rounded-full transition-all duration-200"
                        style={{
                          width: `${duration ? (currentTime / duration) * 100 : 0}%`,
                        }}
                      />
                    </div>
                  </div>
                  <span className="text-xs text-slate-500 w-10 font-mono">
                    {formatTime(duration)}
                  </span>
                </div>
              )}
            </div>
          </div>

          {/* Center Section: Playback Controls */}
          <div className="flex items-center justify-center gap-4 flex-shrink-0">
            {/* Previous Track Button */}
            <IconButton
              icon={<SkipBack size={isMinimized ? 14 : 16} />}
              variant="control"
              onClick={onPrevious}
              aria-label="Previous track"
              className={cn(
                'transition-all duration-200 hover:scale-105',
                !onPrevious && 'opacity-50 cursor-not-allowed'
              )}
              disabled={!onPrevious}
              data-tooltip-id="previous-tooltip"
              data-tooltip-content="Previous track"
            />

            {/* Play/Pause Button */}
            <IconButton
              icon={
                <AnimatedPlayPause
                  isPlaying={isPlaying}
                  size={isMinimized ? 16 : 18}
                  className="text-white"
                />
              }
              variant="primary"
              onClick={onPlayPause}
              className={cn(
                'transition-all duration-200 hover:scale-105 shadow-lg',
                isMinimized ? 'w-8 h-8' : 'w-12 h-12'
              )}
              aria-label={isPlaying ? 'Pause' : 'Play'}
              data-tooltip-id="play-pause-tooltip"
              data-tooltip-content={isPlaying ? 'Pause' : 'Play'}
            />

            {/* Next Track Button */}
            <IconButton
              icon={<SkipForward size={isMinimized ? 14 : 16} />}
              variant="control"
              onClick={onNext}
              aria-label="Next track"
              className={cn(
                'transition-all duration-200 hover:scale-105',
                !onNext && 'opacity-50 cursor-not-allowed'
              )}
              disabled={!onNext}
              data-tooltip-id="next-tooltip"
              data-tooltip-content="Next track"
            />
          </div>

          {/* Right Section: Additional Controls & Quality Badge */}
          <div className="flex items-center gap-6 flex-1 justify-end min-w-0">
            {/* Additional Controls (hidden when minimized) */}
            {!isMinimized && (
              <div className="flex items-center gap-6">
                {/* Playback Speed */}
                <div className="flex items-center gap-1">
                  {[0.5, 1, 1.5].map((rate) => (
                    <button
                      key={rate}
                      onClick={() => onPlaybackRateChange(rate)}
                      className={cn(
                        'px-3 py-1.5 text-xs rounded-lg transition-all duration-200 font-medium cursor-pointer',
                        playbackRate === rate
                          ? 'bg-purple-600 text-white shadow-lg scale-105'
                          : 'bg-slate-700 text-slate-300 hover:bg-slate-600 hover:scale-105'
                      )}
                    >
                      {rate}x
                    </button>
                  ))}
                </div>

                {/* Shuffle Button */}
                <IconButton
                  icon={<Shuffle size={16} />}
                  variant="ghost"
                  onClick={onShuffle}
                  className={cn(
                    'transition-all duration-200 hover:scale-105',
                    isShuffleOn && 'text-purple-500 bg-purple-500/10',
                    !onShuffle && 'opacity-50 cursor-not-allowed'
                  )}
                  aria-label="Shuffle"
                  disabled={!onShuffle}
                  data-tooltip-id="shuffle-tooltip"
                  data-tooltip-content={isShuffleOn ? 'Disable shuffle' : 'Enable shuffle'}
                />

                {/* Repeat Button */}
                <IconButton
                  icon={<Repeat size={16} />}
                  variant="ghost"
                  onClick={onRepeat}
                  className={cn(
                    'transition-all duration-200 hover:scale-105',
                    isRepeatOn && 'text-purple-500 bg-purple-500/10',
                    !onRepeat && 'opacity-50 cursor-not-allowed'
                  )}
                  aria-label="Repeat"
                  disabled={!onRepeat}
                  data-tooltip-id="repeat-tooltip"
                  data-tooltip-content={isRepeatOn ? 'Disable repeat' : 'Enable repeat'}
                />

                {/* Volume Control */}
                <div className="flex items-center gap-2">
                  <IconButton
                    icon={volume === 0 ? <VolumeX size={16} /> : <Volume2 size={16} />}
                    variant="ghost"
                    onClick={() => {
                      const newVolume = volume === 0 ? 0.7 : 0;
                      const event = {
                        target: {
                          value: newVolume,
                        },
                      } as unknown as React.ChangeEvent<HTMLInputElement>;
                      onVolumeChange(event);
                    }}
                    className="transition-all duration-200 hover:scale-105"
                    aria-label={volume === 0 ? 'Unmute' : 'Mute'}
                  />
                  <div className="relative w-24 group">
                    <input
                      type="range"
                      min="0"
                      max="1"
                      step="0.01"
                      value={volume}
                      onChange={onVolumeChange}
                      className="absolute inset-0 w-full h-full opacity-0 cursor-pointer z-10"
                    />
                    <div className="h-2 bg-slate-600 rounded-full overflow-hidden group-hover:h-3 transition-all">
                      <div
                        className="h-full bg-gradient-to-r from-purple-500 to-purple-600 rounded-full transition-all duration-200"
                        style={{ width: `${volume * 100}%` }}
                      />
                    </div>
                  </div>
                </div>
              </div>
            )}

            {/* Minimize/Expand Toggle */}
            <IconButton
              icon={isMinimized ? <ChevronUp size={18} /> : <ChevronDown size={18} />}
              variant="ghost"
              onClick={toggleMinimize}
              className="transition-all duration-200 hover:scale-105"
              aria-label={isMinimized ? 'Expand player' : 'Minimize player'}
              data-tooltip-id="minimize-tooltip"
              data-tooltip-content={isMinimized ? 'Expand player' : 'Minimize player'}
            />

            {/* Nerd Info Button */}
            {!isMinimized && (
              <IconButton
                icon={<Info size={18} />}
                variant="ghost"
                onClick={() => setShowNerdInfo(!showNerdInfo)}
                className={cn(
                  'transition-all duration-200 hover:scale-105',
                  showNerdInfo && 'text-purple-500 bg-purple-500/10'
                )}
                aria-label="Show technical information"
                data-tooltip-id="nerd-info-tooltip"
                data-tooltip-content="Show technical information"
              />
            )}

            {/* Quality Badge - Using new AudioBadge component */}
            <AudioBadge
              format={audioFormat}
              bitrate={audioBitrate}
              isMinimized={isMinimized}
              className="shadow-xl hover:scale-105 hover:shadow-2xl hover:brightness-110"
            />
          </div>
        </div>
      </div>

      {/* Nerd Info Panel */}
      {showNerdInfo && (
        <div
          data-nerd-info
          className="absolute bottom-full left-4 right-4 mb-2 bg-slate-900/95 backdrop-blur-sm border border-slate-600 rounded-lg shadow-2xl p-4 transform transition-all duration-300 ease-out"
        >
          <div className="flex items-center justify-between mb-3">
            <h3 className="text-white font-bold flex items-center gap-2">
              <Activity size={18} className="text-purple-400" />
              Technical Information
            </h3>
            <button
              onClick={() => setShowNerdInfo(false)}
              className="text-slate-400 hover:text-white transition-colors cursor-pointer"
              aria-label="Close technical information"
            >
              <X size={18} />
            </button>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 gap-4 text-sm">
            {/* Audio Format & Quality */}
            <div className="space-y-2">
              <h4 className="text-purple-400 font-semibold flex items-center gap-2">
                <FileAudio size={16} />
                Audio Format
              </h4>
              <div className="space-y-1 text-slate-300 font-mono">
                <div className="flex justify-between">
                  <span>Format:</span>
                  <span className="text-white">
                    {currentTrack.music_metadata?.format || 'Unknown'}
                  </span>
                </div>
                <div className="flex justify-between">
                  <span>Bitrate:</span>
                  <span className="text-white">
                    {currentTrack.music_metadata?.bitrate
                      ? `${Math.round(currentTrack.music_metadata.bitrate / 1000)}kbps`
                      : 'Unknown'}
                  </span>
                </div>
                <div className="flex justify-between">
                  <span>Sample Rate:</span>
                  <span className="text-white">
                    {currentTrack.music_metadata?.sample_rate
                      ? `${(currentTrack.music_metadata.sample_rate / 1000).toFixed(1)}kHz`
                      : 'Unknown'}
                  </span>
                </div>
                <div className="flex justify-between">
                  <span>Channels:</span>
                  <span className="text-white">
                    {currentTrack.music_metadata?.channels
                      ? `${currentTrack.music_metadata.channels} (${
                          currentTrack.music_metadata.channels === 1
                            ? 'Mono'
                            : currentTrack.music_metadata.channels === 2
                              ? 'Stereo'
                              : `${currentTrack.music_metadata.channels}ch`
                        })`
                      : 'Unknown'}
                  </span>
                </div>
                <div className="flex justify-between">
                  <span>Duration:</span>
                  <span className="text-white">
                    {formatTime((currentTrack.music_metadata?.duration || 0) / 1000000000)}
                  </span>
                </div>
              </div>
            </div>

            {/* File Information */}
            <div className="space-y-2">
              <h4 className="text-blue-400 font-semibold flex items-center gap-2">
                <HardDrive size={16} />
                File Information
              </h4>
              <div className="space-y-1 text-slate-300 font-mono">
                <div className="flex justify-between">
                  <span>Size:</span>
                  <span className="text-white">
                    {(currentTrack.size / (1024 * 1024)).toFixed(2)} MB
                  </span>
                </div>
                <div className="flex justify-between">
                  <span>Artwork:</span>
                  <span className="text-white">
                    {currentTrack.music_metadata?.has_artwork ? 'Yes' : 'No'}
                  </span>
                </div>
                <div className="flex justify-between">
                  <span>File ID:</span>
                  <span className="text-white">{currentTrack.id}</span>
                </div>
                <div className="flex justify-between">
                  <span>Hash:</span>
                  <span className="text-white text-xs" title={currentTrack.hash}>
                    {currentTrack.hash.substring(0, 12)}...
                  </span>
                </div>
              </div>
            </div>

            {/* Track Metadata */}
            <div className="space-y-2">
              <h4 className="text-green-400 font-semibold flex items-center gap-2">
                <Disc size={16} />
                Track Details
              </h4>
              <div className="space-y-1 text-slate-300 font-mono">
                <div className="flex justify-between">
                  <span>Track:</span>
                  <span className="text-white">
                    {currentTrack.music_metadata?.track || 'Unknown'} of{' '}
                    {currentTrack.music_metadata?.track_total || '?'}
                  </span>
                </div>
                <div className="flex justify-between">
                  <span>Disc:</span>
                  <span className="text-white">
                    {currentTrack.music_metadata?.disc || 'Unknown'} of{' '}
                    {currentTrack.music_metadata?.disc_total || '?'}
                  </span>
                </div>
                <div className="flex justify-between">
                  <span>Year:</span>
                  <span className="text-white">
                    {currentTrack.music_metadata?.year || 'Unknown'}
                  </span>
                </div>
                <div className="flex justify-between">
                  <span>Genre:</span>
                  <span className="text-white">
                    {currentTrack.music_metadata?.genre || 'Unknown'}
                  </span>
                </div>
              </div>
            </div>

            {/* Playback Information */}
            <div className="space-y-2">
              <h4 className="text-orange-400 font-semibold flex items-center gap-2">
                <Clock size={16} />
                Playback Status
              </h4>
              <div className="space-y-1 text-slate-300 font-mono">
                <div className="flex justify-between">
                  <span>Current Time:</span>
                  <span className="text-white">{formatTime(currentTime)}</span>
                </div>
                <div className="flex justify-between">
                  <span>Remaining:</span>
                  <span className="text-white">{formatTime(duration - currentTime)}</span>
                </div>
                <div className="flex justify-between">
                  <span>Speed:</span>
                  <span className="text-white">{playbackRate}x</span>
                </div>
                <div className="flex justify-between">
                  <span>Volume:</span>
                  <span className="text-white">{Math.round(volume * 100)}%</span>
                </div>
              </div>
            </div>
          </div>
        </div>
      )}

      {/* Hidden audio element */}
      <audio
        ref={audioRef}
        src={currentTrack ? `/api/media/${currentTrack.id}/stream` : undefined}
        onEnded={onNext}
        loop={isRepeatOn}
      />

      {/* Tooltips */}
      <Tooltip id="play-pause-tooltip" />
      <Tooltip id="previous-tooltip" />
      <Tooltip id="next-tooltip" />
      <Tooltip id="shuffle-tooltip" />
      <Tooltip id="repeat-tooltip" />
      <Tooltip id="minimize-tooltip" />
      <Tooltip id="nerd-info-tooltip" />
      <Tooltip id="audio-quality-tooltip" />
    </div>
  );
};

export default AudioPlayer;
