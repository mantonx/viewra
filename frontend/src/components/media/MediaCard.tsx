import React from 'react';
import type { MusicFile, Album, SimpleAlbum } from '@/types/music.types';
import { buildArtworkUrl } from '@/utils/api';

interface MediaCardProps {
  variant: 'album' | 'track';
  item: MusicFile | Album | SimpleAlbum;
  isCurrentTrack?: boolean;
  isPlaying?: boolean;
  onPlay: (track: MusicFile) => void;
}

const MediaCard: React.FC<MediaCardProps> = ({
  variant,
  item,
  isCurrentTrack = false,
  isPlaying = false,
  onPlay,
}) => {
  // Type guard to check if item is an Album or SimpleAlbum
  const isAlbum = (item: MusicFile | Album | SimpleAlbum): item is Album => {
    return 'tracks' in item && 'id' in item;
  };

  const isSimpleAlbum = (item: MusicFile | Album | SimpleAlbum): item is SimpleAlbum => {
    return 'tracks' in item && !('id' in item);
  };

  const isMusicFile = (item: MusicFile | Album | SimpleAlbum): item is MusicFile => {
    return 'path' in item;
  };

  if (variant === 'album' && (isAlbum(item) || isSimpleAlbum(item))) {
    const album = item;
    return (
      <div className="bg-slate-800 rounded-lg overflow-hidden hover:bg-slate-750 transition-colors group">
        {/* Album Cover */}
        <div className="relative aspect-square overflow-hidden bg-slate-700">
          {album.artwork ? (
            <img
              src={buildArtworkUrl(
                album.artwork.includes('/files/')
                  ? album.artwork.split('/files/')[1].split('/artwork')[0]
                  : album.artwork.replace('/api/media/', '').replace('/artwork', '')
              )}
              alt={album.title}
              className="w-full h-full object-cover group-hover:scale-105 transition-transform duration-300"
            />
          ) : (
            <div className="w-full h-full flex items-center justify-center">
              <span className="text-5xl">💿</span>
            </div>
          )}

          {/* Play Album Button */}
          <div className="absolute inset-0 flex items-center justify-center opacity-0 bg-black/60 group-hover:opacity-100 transition-opacity">
            <button
              onClick={() => {
                if (album.tracks.length > 0) {
                  if (isSimpleAlbum(album)) {
                    // SimpleAlbum has MusicFile tracks, use directly
                    onPlay(album.tracks[0]);
                  } else {
                    // Album has Track objects, convert to MusicFile
                    const track = album.tracks[0];
                    const musicFile: MusicFile = {
                      id: track.id,
                      path: `${track.title}.mp3`, // Fallback path
                      size_bytes: 0,
                      hash: '',
                      library_id: 0,
                      last_seen: track.updated_at,
                      created_at: track.created_at,
                      updated_at: track.updated_at,
                      track: {
                        id: track.id,
                        title: track.title,
                        album: album.title,
                        artist: '', // Would need artist data
                        album_artist: '',
                        track_number: track.track_number,
                        duration: track.duration,
                      }
                    };
                    onPlay(musicFile);
                  }
                }
              }}
              className="bg-purple-600 hover:bg-purple-700 rounded-full p-4 text-white"
            >
              <span className="block w-8 h-8">▶️</span>
            </button>
          </div>
        </div>

        {/* Album Info */}
        <div className="p-4">
          <h4 className="text-white font-medium truncate">{album.title}</h4>
          <p className="text-slate-400 text-sm mt-1">
            {album.tracks.length} tracks {album.year ? `• ${album.year}` : ''}
          </p>
        </div>
      </div>
    );
  }

  // Track variant
  if (variant === 'track' && isMusicFile(item)) {
    const track = item;
    return (
      <div
        className="bg-slate-800 rounded-lg p-4 hover:bg-slate-700 transition-colors cursor-pointer"
        onClick={() => onPlay(track)}
      >
        <div className="flex flex-col items-center">
          {/* Track Artwork */}
          <div className="relative w-full aspect-square mb-3">
            {track.track ? (
              <img
                src={buildArtworkUrl(track.id)}
                alt={track.track?.album || 'Album Artwork'}
                className="w-full h-full object-cover rounded-md"
              />
            ) : (
              <div className="w-full h-full bg-slate-700 flex items-center justify-center rounded-md">
                <span className="text-4xl">🎵</span>
              </div>
            )}

            {/* Play Indicator */}
            {isCurrentTrack && (
              <div className="absolute top-2 right-2 bg-purple-600 rounded-full p-1">
                <span className="block w-3 h-3 text-white">{isPlaying ? '⏸' : '▶️'}</span>
              </div>
            )}
          </div>

          {/* Track Info */}
          <h3 className="text-white font-medium text-center truncate w-full">
            {track.track?.title || track.path.split('/').pop()}
          </h3>
          <p className="text-slate-400 text-sm text-center truncate w-full">
            {track.track?.artist || 'Unknown Artist'}
          </p>
          <p className="text-slate-500 text-xs text-center mt-1 truncate w-full">
            {track.track?.album || 'Unknown Album'}
          </p>
        </div>
      </div>
    );
  }

  // Fallback for invalid variant/item combinations
  return null;
};

export default MediaCard;
