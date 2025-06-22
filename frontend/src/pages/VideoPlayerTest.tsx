import React, { useState, useEffect } from 'react';
import { Link } from 'react-router-dom';
import { Play, Calendar, Clock, Tv } from 'lucide-react';

interface MediaFile {
  id: string;
  media_id: string;
  media_type: string;
  path: string;
  container?: string;
  video_codec?: string;
  audio_codec?: string;
  resolution?: string;
  duration?: number;
  size_bytes: number;
}

interface Episode {
  id: string;
  title: string;
  episode_number: number;
  air_date?: string;
  description?: string;
  duration?: number;
  still_image?: string;
  season: {
    id: string;
    season_number: number;
    tv_show: {
      id: string;
      title: string;
      description?: string;
      poster?: string;
      backdrop?: string;
      tmdb_id?: string;
    };
  };
  mediaFile?: MediaFile;
}

const VideoPlayerTest: React.FC = () => {
  const [episodes, setEpisodes] = useState<Episode[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const loadEpisodes = async () => {
      setLoading(true);
      setError(null);

      try {
        // Get all media files
        const filesResponse = await fetch(`/api/media/files?limit=1000`);
        const filesData = await filesResponse.json();

        // Filter episode files
        const episodeFiles =
          filesData.media_files?.filter((file: MediaFile) => file.media_type === 'episode') || [];

        // Get episode metadata for each file
        const episodePromises = episodeFiles.slice(0, 20).map(async (file: MediaFile) => {
          try {
            const metadataResponse = await fetch(`/api/media/files/${file.id}/metadata`);
            const metadataData = await metadataResponse.json();

            if (metadataData.metadata?.type === 'episode') {
              return {
                id: metadataData.metadata.episode_id,
                title: metadataData.metadata.title,
                episode_number: metadataData.metadata.episode_number,
                air_date: metadataData.metadata.air_date,
                description: metadataData.metadata.description,
                duration: metadataData.metadata.duration,
                still_image: metadataData.metadata.still_image,
                season: metadataData.metadata.season,
                mediaFile: file,
              } as Episode;
            }
            return null;
          } catch (err) {
            console.warn(`Failed to get metadata for file ${file.id}:`, err);
            return null;
          }
        });

        const episodeResults = await Promise.all(episodePromises);
        const validEpisodes = episodeResults.filter((episode) => episode !== null);

        // Sort by show title, season, then episode number
        validEpisodes.sort((a, b) => {
          if (a.season.tv_show.title !== b.season.tv_show.title) {
            return a.season.tv_show.title.localeCompare(b.season.tv_show.title);
          }
          if (a.season.season_number !== b.season.season_number) {
            return a.season.season_number - b.season.season_number;
          }
          return a.episode_number - b.episode_number;
        });

        setEpisodes(validEpisodes);
      } catch (err) {
        console.error('Failed to load episodes:', err);
        setError(err instanceof Error ? err.message : 'Failed to load episodes');
      } finally {
        setLoading(false);
      }
    };

    loadEpisodes();
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

  if (loading) {
    return (
      <div className="min-h-screen bg-slate-900 flex items-center justify-center">
        <div className="text-center">
          <div className="animate-spin rounded-full h-12 w-12 border-t-2 border-b-2 border-purple-500 mx-auto mb-4"></div>
          <p className="text-slate-400">Loading episodes...</p>
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="min-h-screen bg-slate-900 flex items-center justify-center">
        <div className="text-center max-w-md">
          <div className="text-red-400 text-6xl mb-4">⚠️</div>
          <h2 className="text-xl font-bold text-white mb-2">Error</h2>
          <p className="text-slate-400 mb-4">{error}</p>
        </div>
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-slate-900">
      <div className="container mx-auto px-4 py-8">
        <div className="mb-8">
          <h1 className="text-3xl font-bold text-white mb-4 flex items-center gap-3">
            <Tv className="w-8 h-8" />
            Video Player Test
          </h1>
          <p className="text-slate-300">
            Test the Shaka Player video player with your TV show episodes. Click any episode to
            start watching.
          </p>
        </div>

        {episodes.length === 0 ? (
          <div className="text-center py-12">
            <Tv className="w-16 h-16 text-slate-600 mx-auto mb-4" />
            <h3 className="text-xl font-semibold text-white mb-2">No Episodes Found</h3>
            <p className="text-slate-400 mb-4">
              No TV show episodes were found in your library. Make sure you have:
            </p>
            <ul className="text-slate-500 text-sm list-disc list-inside space-y-1">
              <li>Added TV show libraries in the admin panel</li>
              <li>Completed a library scan</li>
              <li>TV show files with proper metadata</li>
            </ul>
          </div>
        ) : (
          <div className="space-y-6">
            <div className="text-slate-400 text-sm">
              Found {episodes.length} episode{episodes.length !== 1 ? 's' : ''} available for
              testing
            </div>

            <div className="grid gap-4">
              {episodes.map((episode) => (
                <div
                  key={episode.id}
                  className="bg-slate-800 rounded-lg p-4 hover:bg-slate-750 transition-colors"
                >
                  <div className="flex items-start gap-4">
                    {/* Episode thumbnail */}
                    <div className="w-40 h-24 bg-slate-700 rounded flex-shrink-0 relative overflow-hidden">
                      {episode.still_image ? (
                        <img
                          src={episode.still_image}
                          alt={episode.title}
                          className="w-full h-full object-cover"
                        />
                      ) : episode.season.tv_show.backdrop ? (
                        <img
                          src={episode.season.tv_show.backdrop}
                          alt={episode.title}
                          className="w-full h-full object-cover opacity-50"
                        />
                      ) : (
                        <div className="w-full h-full flex items-center justify-center text-slate-500">
                          <Play className="w-8 h-8" />
                        </div>
                      )}

                      {/* Play button overlay */}
                      <Link
                        to={`/player/episode/${episode.id}`}
                        className="absolute inset-0 bg-black/50 flex items-center justify-center opacity-0 hover:opacity-100 transition-opacity"
                      >
                        <Play className="w-10 h-10 text-white" />
                      </Link>
                    </div>

                    {/* Episode info */}
                    <div className="flex-1 min-w-0">
                      <div className="flex items-start justify-between gap-4">
                        <div className="flex-1 min-w-0">
                          <h3 className="text-white font-semibold text-lg">
                            {episode.season.tv_show.title}
                          </h3>
                          <h4 className="text-slate-300 font-medium">
                            S{episode.season.season_number}E{episode.episode_number}:{' '}
                            {episode.title}
                          </h4>

                          <div className="flex items-center gap-4 text-slate-400 text-sm mt-2">
                            {episode.air_date && (
                              <span className="flex items-center gap-1">
                                <Calendar className="w-4 h-4" />
                                {new Date(episode.air_date).toLocaleDateString()}
                              </span>
                            )}
                            {episode.duration && (
                              <span className="flex items-center gap-1">
                                <Clock className="w-4 h-4" />
                                {formatDuration(episode.duration)}
                              </span>
                            )}
                            {episode.mediaFile && (
                              <>
                                {episode.mediaFile.resolution && (
                                  <span className="bg-slate-700 px-2 py-1 rounded text-xs">
                                    {episode.mediaFile.resolution}
                                  </span>
                                )}
                                {episode.mediaFile.video_codec && (
                                  <span className="bg-slate-700 px-2 py-1 rounded text-xs">
                                    {episode.mediaFile.video_codec.toUpperCase()}
                                  </span>
                                )}
                                <span className="text-xs">
                                  {formatFileSize(episode.mediaFile.size_bytes)}
                                </span>
                              </>
                            )}
                          </div>

                          {episode.description && (
                            <p className="text-slate-300 text-sm mt-2 line-clamp-2">
                              {episode.description}
                            </p>
                          )}

                          {episode.mediaFile && (
                            <div className="text-xs text-slate-500 mt-2">
                              File: {episode.mediaFile.path.split('/').pop()}
                            </div>
                          )}
                        </div>

                        {/* Play button */}
                        <Link
                          to={`/player/episode/${episode.id}`}
                          className="flex items-center gap-2 px-6 py-3 bg-purple-600 hover:bg-purple-700 text-white rounded-lg transition-colors flex-shrink-0"
                        >
                          <Play className="w-5 h-5" />
                          Watch Now
                        </Link>
                      </div>
                    </div>
                  </div>
                </div>
              ))}
            </div>
          </div>
        )}
      </div>
    </div>
  );
};

export default VideoPlayerTest;
