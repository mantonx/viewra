import React from 'react';
import type { MusicFile, Album } from '@/types/music.types';

interface MediaCardProps {
  variant: 'track' | 'album';
  item: MusicFile | Album;
  isCurrentTrack?: boolean;
  isPlaying?: boolean;
  onPlay: (track: MusicFile) => void;
}

const MediaCard: React.FC<MediaCardProps> = ({
  variant,
  item,
  isCurrentTrack,
  isPlaying,
  onPlay,
}) => {
  if (variant === 'album') {
    const album = item as Album;
    return (
      <div className="bg-slate-800 rounded-lg p-4 hover:bg-slate-700 transition-colors">
        {/* Album card implementation */}
        <div className="aspect-square bg-slate-700 rounded-lg mb-3">
          {album.artwork && (
            <img
              src={album.artwork}
              alt={album.title}
              className="w-full h-full object-cover rounded-lg"
            />
          )}
        </div>
        <h4 className="text-white font-medium truncate">{album.title}</h4>
        <p className="text-slate-400 text-sm">{album.year || 'Unknown'}</p>
        <p className="text-slate-500 text-xs">{album.tracks.length} tracks</p>
      </div>
    );
  }

  const track = item as MusicFile;
  return (
    <div
      className={`bg-slate-800 rounded-lg p-4 hover:bg-slate-700 transition-colors cursor-pointer ${
        isCurrentTrack ? 'ring-2 ring-purple-500' : ''
      }`}
      onClick={() => onPlay(track)}
    >
      {/* Track card implementation */}
      <h4 className="text-white font-medium truncate">
        {track.music_metadata?.title || 'Unknown'}
      </h4>
      <p className="text-slate-400 text-sm truncate">
        {track.music_metadata?.artist || 'Unknown Artist'}
      </p>
      {isCurrentTrack && (
        <div className="mt-2 text-purple-400 text-sm">{isPlaying ? '▶️ Playing' : '⏸ Paused'}</div>
      )}
    </div>
  );
};

export default MediaCard;
