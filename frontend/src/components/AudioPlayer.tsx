import React from 'react';

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
  format: string;
  has_artwork: boolean;
}

interface MusicFile {
  id: number;
  path: string;
  size: number;
  hash: string;
  library_id: number;
  last_seen: string;
  created_at: string;
  updated_at: string;
  music_metadata: MusicMetadata;
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
  formatTime: (seconds: number) => string;
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
  formatTime,
}) => {
  if (!currentTrack) return null;

  return (
    <div className="fixed bottom-0 left-0 right-0 bg-slate-800 p-4 shadow-lg border-t border-slate-700 z-50">
      <div className="max-w-6xl mx-auto">
        <div className="flex items-center gap-4">
          {/* Track Artwork */}
          <div className="w-16 h-16 flex-shrink-0">
            {currentTrack?.music_metadata?.has_artwork ? (
              <img
                src={`/api/media/${currentTrack.id}/artwork`}
                alt={currentTrack.music_metadata?.album || 'Album Artwork'}
                className="w-full h-full object-cover rounded"
              />
            ) : (
              <div className="w-full h-full bg-slate-700 flex items-center justify-center rounded">
                <span className="text-2xl">🎵</span>
              </div>
            )}
          </div>

          {/* Track Info */}
          <div className="flex-1 min-w-0">
            <h3 className="text-white font-medium truncate">
              {currentTrack?.music_metadata?.title || currentTrack?.path.split('/').pop()}
            </h3>
            <p className="text-slate-400 text-sm truncate">
              {currentTrack?.music_metadata?.artist || 'Unknown Artist'}
            </p>

            {/* Progress Bar */}
            <div className="flex items-center gap-2 mt-2">
              <span className="text-xs text-slate-500 w-10">{formatTime(currentTime)}</span>
              <input
                type="range"
                min="0"
                max={duration || 100}
                value={currentTime || 0}
                onChange={onSeek}
                className="flex-1 h-1 bg-slate-600 rounded-lg appearance-none cursor-pointer"
              />
              <span className="text-xs text-slate-500 w-10">{formatTime(duration)}</span>
            </div>
          </div>

          {/* Playback Controls */}
          <div className="flex items-center gap-4">
            <button
              onClick={onPlayPause}
              className="bg-purple-600 hover:bg-purple-700 rounded-full p-3 text-white"
            >
              {isPlaying ? (
                <span className="block w-4 h-4">⏸</span>
              ) : (
                <span className="block w-4 h-4">▶️</span>
              )}
            </button>

            {/* Playback Speed */}
            <div className="flex items-center gap-1">
              <button
                onClick={() => onPlaybackRateChange(0.5)}
                className={`px-2 py-1 text-xs rounded ${
                  playbackRate === 0.5 ? 'bg-purple-600 text-white' : 'bg-slate-700 text-slate-300'
                }`}
              >
                0.5x
              </button>
              <button
                onClick={() => onPlaybackRateChange(1)}
                className={`px-2 py-1 text-xs rounded ${
                  playbackRate === 1 ? 'bg-purple-600 text-white' : 'bg-slate-700 text-slate-300'
                }`}
              >
                1x
              </button>
              <button
                onClick={() => onPlaybackRateChange(1.5)}
                className={`px-2 py-1 text-xs rounded ${
                  playbackRate === 1.5 ? 'bg-purple-600 text-white' : 'bg-slate-700 text-slate-300'
                }`}
              >
                1.5x
              </button>
            </div>

            {/* Volume Control */}
            <div className="flex items-center gap-2">
              <span className="text-white">🔊</span>
              <input
                type="range"
                min="0"
                max="1"
                step="0.01"
                value={volume}
                onChange={onVolumeChange}
                className="w-24"
              />
            </div>
          </div>

          <audio
            ref={audioRef}
            src={currentTrack ? `/api/media/${currentTrack.id}/stream` : undefined}
            onEnded={() => {}}
          />
        </div>
      </div>
    </div>
  );
};

export default AudioPlayer;
