import React, { useState, useEffect } from 'react';
import { Link } from 'react-router-dom';
import { Play, Film, Tv, Calendar, Clock, HardDrive, FileVideo, AlertCircle } from 'lucide-react';
import type { MediaFile, Episode, Movie, MediaItem } from './MediaPlayerTest.types';

const MediaPlayerTest: React.FC = () => {
  const [mediaItems, setMediaItems] = useState<MediaItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [showOnlyWithFiles, setShowOnlyWithFiles] = useState(true);

  useEffect(() => {
    const loadMedia = async () => {
      setLoading(true);
      setError(null);

      try {
        // Get all media files
        const filesResponse = await fetch(`/api/media/files?limit=1000`);
        const filesData = await filesResponse.json();

        const allMediaItems: MediaItem[] = [];

        // Process episodes
        const episodeFiles = filesData.media_files?.filter((file: MediaFile) => file.media_type === 'episode') || [];
        
        // Randomly select up to 10 episodes
        const selectedEpisodeFiles = episodeFiles
          .sort(() => Math.random() - 0.5)
          .slice(0, 10);

        for (const file of selectedEpisodeFiles) {
          try {
            const metadataResponse = await fetch(`/api/media/files/${file.id}/metadata`);
            const metadataData = await metadataResponse.json();

            if (metadataData.metadata?.type === 'episode') {
              const episode: Episode = {
                id: metadataData.metadata.episode_id,
                title: metadataData.metadata.title,
                episode_number: metadataData.metadata.episode_number,
                air_date: metadataData.metadata.air_date,
                description: metadataData.metadata.description,
                duration: metadataData.metadata.duration,
                still_image: metadataData.metadata.still_image,
                season: metadataData.metadata.season,
                mediaFile: file,
              };
              allMediaItems.push({ type: 'episode', data: episode });
            }
          } catch (err) {
            console.warn(`Failed to get metadata for episode file ${file.id}:`, err);
          }
        }

        // Process movies
        const movieFiles = filesData.media_files?.filter((file: MediaFile) => file.media_type === 'movie') || [];
        
        // Randomly select up to 5 movies
        const selectedMovieFiles = movieFiles
          .sort(() => Math.random() - 0.5)
          .slice(0, 5);

        for (const file of selectedMovieFiles) {
          try {
            const metadataResponse = await fetch(`/api/media/files/${file.id}/metadata`);
            const metadataData = await metadataResponse.json();

            if (metadataData.metadata?.type === 'movie') {
              const movie: Movie = {
                id: metadataData.metadata.movie_id,
                title: metadataData.metadata.title,
                release_date: metadataData.metadata.release_date,
                description: metadataData.metadata.description,
                duration: metadataData.metadata.duration,
                poster: metadataData.metadata.poster,
                backdrop: metadataData.metadata.backdrop,
                tmdb_id: metadataData.metadata.tmdb_id,
                mediaFile: file,
              };
              allMediaItems.push({ type: 'movie', data: movie });
            }
          } catch (err) {
            console.warn(`Failed to get metadata for movie file ${file.id}:`, err);
          }
        }

        // Sort by type (episodes first) then randomize within each type
        allMediaItems.sort((a, b) => {
          if (a.type !== b.type) {
            return a.type === 'episode' ? -1 : 1;
          }
          return Math.random() - 0.5;
        });

        setMediaItems(allMediaItems);
      } catch (err) {
        console.error('Failed to load media:', err);
        setError(err instanceof Error ? err.message : 'Failed to load media');
      } finally {
        setLoading(false);
      }
    };

    loadMedia();
  }, []);

  const formatDuration = (seconds?: number) => {
    if (!seconds) return '';
    const hours = Math.floor(seconds / 3600);
    const minutes = Math.floor((seconds % 3600) / 60);

    if (hours > 0) {
      return `${hours}h ${minutes}m`;
    }
    return `${minutes}m`;
  };

  const formatFileSize = (bytes: number) => {
    const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
    if (bytes === 0) return '0 B';
    const i = Math.floor(Math.log(bytes) / Math.log(1024));
    return Math.round((bytes / Math.pow(1024, i)) * 100) / 100 + ' ' + sizes[i];
  };

  const formatBitrate = (bitrate?: number) => {
    if (!bitrate) return '';
    if (bitrate > 1000000) {
      return `${(bitrate / 1000000).toFixed(1)} Mbps`;
    }
    return `${Math.round(bitrate / 1000)} kbps`;
  };

  const getCodecInfo = (file?: MediaFile) => {
    if (!file) return '';
    const parts = [];
    if (file.video_codec) parts.push(file.video_codec.toUpperCase());
    if (file.audio_codec) parts.push(file.audio_codec.toUpperCase());
    return parts.join(' / ');
  };

  if (loading) {
    return (
      <div className="min-h-screen bg-slate-900 flex items-center justify-center">
        <div className="text-center">
          <div className="animate-spin rounded-full h-12 w-12 border-t-2 border-b-2 border-purple-500 mx-auto mb-4"></div>
          <p className="text-slate-400">Loading test media...</p>
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="min-h-screen bg-slate-900 flex items-center justify-center">
        <div className="text-center max-w-md">
          <AlertCircle className="w-16 h-16 text-red-400 mx-auto mb-4" />
          <h2 className="text-xl font-bold text-white mb-2">Error</h2>
          <p className="text-slate-400 mb-4">{error}</p>
        </div>
      </div>
    );
  }

  const filteredItems = showOnlyWithFiles 
    ? mediaItems.filter(item => item.type === 'episode' ? item.data.mediaFile : item.data.mediaFile)
    : mediaItems;

  return (
    <div className="min-h-screen bg-slate-900">
      <div className="container mx-auto px-4 py-8">
        <div className="mb-8">
          <h1 className="text-3xl font-bold text-white mb-4 flex items-center gap-3">
            <FileVideo className="w-8 h-8" />
            Media Player Test
          </h1>
          <p className="text-slate-300 mb-4">
            Test the media player with various file types and codecs. This page shows a random selection of your media library.
          </p>
          <div className="flex items-center gap-4">
            <label className="flex items-center gap-2 text-slate-300">
              <input
                type="checkbox"
                checked={showOnlyWithFiles}
                onChange={(e) => setShowOnlyWithFiles(e.target.checked)}
                className="rounded"
              />
              Show only items with media files
            </label>
            <button
              onClick={() => window.location.reload()}
              className="px-4 py-2 bg-purple-600 hover:bg-purple-700 text-white rounded-lg transition-colors"
            >
              Refresh Selection
            </button>
          </div>
        </div>

        {filteredItems.length === 0 ? (
          <div className="text-center py-12">
            <FileVideo className="w-16 h-16 text-slate-600 mx-auto mb-4" />
            <h3 className="text-xl font-semibold text-white mb-2">No Media Found</h3>
            <p className="text-slate-400 mb-4">
              No media files were found in your library. Make sure you have:
            </p>
            <ul className="text-slate-500 text-sm list-disc list-inside space-y-1">
              <li>Added media libraries in the admin panel</li>
              <li>Completed a library scan</li>
              <li>Media files with proper metadata</li>
            </ul>
          </div>
        ) : (
          <div className="space-y-6">
            <div className="text-slate-400 text-sm">
              Showing {filteredItems.length} random items from your library
            </div>

            <div className="grid gap-4">
              {filteredItems.map((item) => {
                const isEpisode = item.type === 'episode';
                const data = item.data;
                const mediaFile = data.mediaFile;
                const thumbnail = isEpisode 
                  ? data.still_image || data.season.tv_show.backdrop
                  : data.poster || data.backdrop;

                return (
                  <div
                    key={`${item.type}-${data.id}`}
                    className="bg-slate-800 rounded-lg p-4 hover:bg-slate-750 transition-colors"
                  >
                    <div className="flex items-start gap-4">
                      {/* Thumbnail */}
                      <div className="w-48 h-28 bg-slate-700 rounded flex-shrink-0 relative overflow-hidden">
                        {thumbnail ? (
                          <img
                            src={thumbnail}
                            alt={data.title}
                            className="w-full h-full object-cover"
                          />
                        ) : (
                          <div className="w-full h-full flex items-center justify-center text-slate-500">
                            {isEpisode ? <Tv className="w-8 h-8" /> : <Film className="w-8 h-8" />}
                          </div>
                        )}

                        {/* Type badge */}
                        <div className={`absolute top-2 left-2 px-2 py-1 rounded text-xs font-medium ${
                          isEpisode ? 'bg-blue-600 text-white' : 'bg-purple-600 text-white'
                        }`}>
                          {isEpisode ? 'Episode' : 'Movie'}
                        </div>

                        {/* Play button overlay */}
                        {mediaFile && (
                          <Link
                            to={isEpisode 
                              ? `/player/episode/${mediaFile.id}`
                              : `/player/movie/${mediaFile.id}`
                            }
                            className="absolute inset-0 bg-black/50 flex items-center justify-center opacity-0 hover:opacity-100 transition-opacity"
                          >
                            <Play className="w-10 h-10 text-white" />
                          </Link>
                        )}
                      </div>

                      {/* Info */}
                      <div className="flex-1 min-w-0">
                        <div className="flex items-start justify-between gap-4">
                          <div className="flex-1 min-w-0">
                            {/* Title */}
                            <h3 className="text-white font-semibold text-lg">
                              {isEpisode ? data.season.tv_show.title : data.title}
                            </h3>
                            {isEpisode && (
                              <h4 className="text-slate-300 font-medium">
                                S{data.season.season_number}E{data.episode_number}: {data.title}
                              </h4>
                            )}

                            {/* Metadata */}
                            <div className="flex items-center gap-4 text-slate-400 text-sm mt-2">
                              {(isEpisode ? data.air_date : data.release_date) && (
                                <span className="flex items-center gap-1">
                                  <Calendar className="w-4 h-4" />
                                  {new Date(isEpisode ? data.air_date! : data.release_date!).toLocaleDateString()}
                                </span>
                              )}
                              {data.duration && (
                                <span className="flex items-center gap-1">
                                  <Clock className="w-4 h-4" />
                                  {formatDuration(data.duration)}
                                </span>
                              )}
                            </div>

                            {/* File info */}
                            {mediaFile && (
                              <div className="mt-3 space-y-2">
                                <div className="flex flex-wrap items-center gap-3 text-xs">
                                  {mediaFile.resolution && (
                                    <span className="bg-slate-700 px-2 py-1 rounded">
                                      {mediaFile.resolution}
                                    </span>
                                  )}
                                  {getCodecInfo(mediaFile) && (
                                    <span className="bg-slate-700 px-2 py-1 rounded">
                                      {getCodecInfo(mediaFile)}
                                    </span>
                                  )}
                                  {mediaFile.container && (
                                    <span className="bg-slate-700 px-2 py-1 rounded">
                                      {mediaFile.container.toUpperCase()}
                                    </span>
                                  )}
                                  {mediaFile.framerate && (
                                    <span className="bg-slate-700 px-2 py-1 rounded">
                                      {Math.round(mediaFile.framerate)}fps
                                    </span>
                                  )}
                                  {mediaFile.bitrate && (
                                    <span className="bg-slate-700 px-2 py-1 rounded">
                                      {formatBitrate(mediaFile.bitrate)}
                                    </span>
                                  )}
                                  <span className="flex items-center gap-1">
                                    <HardDrive className="w-3 h-3" />
                                    {formatFileSize(mediaFile.size_bytes)}
                                  </span>
                                </div>
                                
                                <div className="text-xs text-slate-500">
                                  {mediaFile.path.split('/').pop()}
                                </div>
                              </div>
                            )}

                            {/* Description */}
                            {data.description && (
                              <p className="text-slate-300 text-sm mt-2 line-clamp-2">
                                {data.description}
                              </p>
                            )}
                          </div>

                          {/* Play button */}
                          {mediaFile && (
                            <Link
                              to={isEpisode 
                                ? `/player/episode/${mediaFile.id}`
                                : `/player/movie/${mediaFile.id}`
                              }
                              className="flex items-center gap-2 px-6 py-3 bg-purple-600 hover:bg-purple-700 text-white rounded-lg transition-colors flex-shrink-0"
                            >
                              <Play className="w-5 h-5" />
                              Watch Now
                            </Link>
                          )}
                        </div>
                      </div>
                    </div>
                  </div>
                );
              })}
            </div>
          </div>
        )}
      </div>
    </div>
  );
};

export default MediaPlayerTest;