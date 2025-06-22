import React, { useState, useEffect, useCallback } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { ArrowLeft, Play, Calendar, Clock, Info } from 'lucide-react';
import type { TVShow, Season, Episode } from '@/types/tv.types';

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

interface GroupedSeason {
  season: Season;
  episodes: (Episode & { mediaFile?: MediaFile })[];
}

const TVShowDetail: React.FC = () => {
  const { showId } = useParams<{ showId: string }>();
  const navigate = useNavigate();

  const [tvShow, setTVShow] = useState<TVShow | null>(null);
  const [seasons, setSeasons] = useState<GroupedSeason[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [selectedSeason, setSelectedSeason] = useState<number>(1);

  const loadTVShowData = useCallback(async () => {
    if (!showId) return;

    setLoading(true);
    setError(null);

    try {
      // Get TV show details
      const showResponse = await fetch(`/api/media/tv-shows`);
      const showData = await showResponse.json();
      const show = showData.tv_shows?.find((s: TVShow) => s.id === showId);

      if (!show) {
        throw new Error('TV show not found');
      }

      setTVShow(show);

      // Get all media files to find episodes for this show
      const filesResponse = await fetch(`/api/media/files?limit=1000`);
      const filesData = await filesResponse.json();

      // Filter episode files for this show
      const episodeFiles =
        filesData.media_files?.filter((file: MediaFile) => file.media_type === 'episode') || [];

      // Get episode metadata for each file
      const episodePromises = episodeFiles.map(async (file: MediaFile) => {
        try {
          const metadataResponse = await fetch(`/api/media/files/${file.id}/metadata`);
          const metadataData = await metadataResponse.json();

          if (
            metadataData.metadata?.type === 'episode' &&
            metadataData.metadata.season?.tv_show?.id === showId
          ) {
            return {
              episode: {
                id: metadataData.metadata.episode_id,
                title: metadataData.metadata.title,
                episode_number: metadataData.metadata.episode_number,
                air_date: metadataData.metadata.air_date,
                description: metadataData.metadata.description,
                duration: metadataData.metadata.duration,
                still_image: metadataData.metadata.still_image,
                season_id: metadataData.metadata.season.id,
                created_at: '',
                updated_at: '',
                season: metadataData.metadata.season,
              } as Episode,
              mediaFile: file,
              seasonNumber: metadataData.metadata.season.season_number,
            };
          }
          return null;
        } catch (err) {
          console.warn(`Failed to get metadata for file ${file.id}:`, err);
          return null;
        }
      });

      const episodeResults = await Promise.all(episodePromises);
      const validEpisodes = episodeResults.filter((result) => result !== null);

      // Group episodes by season
      const seasonMap = new Map<number, GroupedSeason>();

      validEpisodes.forEach((result) => {
        if (!result) return;

        const { episode, mediaFile, seasonNumber } = result;

        if (!seasonMap.has(seasonNumber)) {
          seasonMap.set(seasonNumber, {
            season: {
              id: episode.season_id,
              tv_show_id: showId,
              season_number: seasonNumber,
              description: '',
              poster: '',
              air_date: undefined,
              created_at: '',
              updated_at: '',
            },
            episodes: [],
          });
        }

        const seasonData = seasonMap.get(seasonNumber)!;
        seasonData.episodes.push({
          ...episode,
          mediaFile,
        });
      });

      // Sort episodes within each season
      seasonMap.forEach((seasonData) => {
        seasonData.episodes.sort((a, b) => a.episode_number - b.episode_number);
      });

      // Convert to array and sort by season number
      const sortedSeasons = Array.from(seasonMap.values()).sort(
        (a, b) => a.season.season_number - b.season.season_number
      );

      setSeasons(sortedSeasons);

      // Set default selected season to the first available season
      if (sortedSeasons.length > 0) {
        setSelectedSeason(sortedSeasons[0].season.season_number);
      }
    } catch (err) {
      console.error('Failed to load TV show data:', err);
      setError(err instanceof Error ? err.message : 'Failed to load TV show data');
    } finally {
      setLoading(false);
    }
  }, [showId]);

  useEffect(() => {
    loadTVShowData();
  }, [loadTVShowData]);

  const handlePlayEpisode = (episodeId: string) => {
    navigate(`/player/episode/${episodeId}`);
  };

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
          <p className="text-slate-400">Loading TV show...</p>
        </div>
      </div>
    );
  }

  if (error || !tvShow) {
    return (
      <div className="min-h-screen bg-slate-900 flex items-center justify-center">
        <div className="text-center max-w-md">
          <div className="text-red-400 text-6xl mb-4">⚠️</div>
          <h2 className="text-xl font-bold text-white mb-2">Error</h2>
          <p className="text-slate-400 mb-4">{error || 'TV show not found'}</p>
          <button
            onClick={() => navigate('/tv-shows')}
            className="px-4 py-2 bg-purple-600 hover:bg-purple-700 text-white rounded transition-colors"
          >
            Back to TV Shows
          </button>
        </div>
      </div>
    );
  }

  const currentSeason = seasons.find((s) => s.season.season_number === selectedSeason);

  return (
    <div className="min-h-screen bg-slate-900">
      {/* Header */}
      <div className="bg-slate-800 p-4 flex items-center gap-4">
        <button
          onClick={() => navigate('/tv-shows')}
          className="p-2 hover:bg-slate-700 rounded-lg transition-colors"
        >
          <ArrowLeft className="w-5 h-5 text-white" />
        </button>

        <div className="flex items-center gap-4 flex-1">
          {tvShow.poster && (
            <img
              src={tvShow.poster}
              alt={tvShow.title}
              className="w-16 h-24 object-cover rounded"
              onError={(e) => {
                const target = e.target as HTMLImageElement;
                target.src = `/api/v1/assets/entity/tv_show/${tvShow.id}/preferred/poster/data`;
              }}
            />
          )}

          <div>
            <h1 className="text-white font-bold text-2xl">{tvShow.title}</h1>
            <div className="flex items-center gap-4 text-slate-400 text-sm mt-1">
              {tvShow.first_air_date && (
                <span className="flex items-center gap-1">
                  <Calendar className="w-4 h-4" />
                  {new Date(tvShow.first_air_date).getFullYear()}
                </span>
              )}
              {tvShow.status && (
                <span
                  className={`px-2 py-1 rounded text-xs ${
                    tvShow.status === 'Running'
                      ? 'bg-green-900 text-green-200'
                      : tvShow.status === 'Ended'
                        ? 'bg-red-900 text-red-200'
                        : 'bg-gray-900 text-gray-200'
                  }`}
                >
                  {tvShow.status}
                </span>
              )}
              <span>
                {seasons.length} Season{seasons.length !== 1 ? 's' : ''}
              </span>
            </div>
            {tvShow.description && (
              <p className="text-slate-300 text-sm mt-2 max-w-2xl line-clamp-3">
                {tvShow.description}
              </p>
            )}
          </div>
        </div>
      </div>

      <div className="p-6">
        {/* Season Selector */}
        {seasons.length > 1 && (
          <div className="mb-6">
            <div className="flex gap-2 flex-wrap">
              {seasons.map((season) => (
                <button
                  key={season.season.season_number}
                  onClick={() => setSelectedSeason(season.season.season_number)}
                  className={`px-4 py-2 rounded-lg transition-colors ${
                    selectedSeason === season.season.season_number
                      ? 'bg-purple-600 text-white'
                      : 'bg-slate-700 text-slate-300 hover:bg-slate-600'
                  }`}
                >
                  Season {season.season.season_number}
                  <span className="ml-2 text-xs opacity-75">
                    ({season.episodes.length} episodes)
                  </span>
                </button>
              ))}
            </div>
          </div>
        )}

        {/* Episodes List */}
        {currentSeason && (
          <div className="space-y-4">
            <h2 className="text-xl font-bold text-white mb-4">
              Season {currentSeason.season.season_number} Episodes
            </h2>

            {currentSeason.episodes.length === 0 ? (
              <div className="text-center py-8 text-slate-400">
                No episodes found for this season.
              </div>
            ) : (
              <div className="grid gap-4">
                {currentSeason.episodes.map((episode) => (
                  <div
                    key={episode.id}
                    className="bg-slate-800 rounded-lg p-4 hover:bg-slate-750 transition-colors"
                  >
                    <div className="flex items-start gap-4">
                      {/* Episode thumbnail */}
                      <div className="w-32 h-18 bg-slate-700 rounded flex-shrink-0 relative overflow-hidden">
                        {episode.still_image ? (
                          <img
                            src={episode.still_image}
                            alt={episode.title}
                            className="w-full h-full object-cover"
                          />
                        ) : tvShow.backdrop ? (
                          <img
                            src={tvShow.backdrop}
                            alt={episode.title}
                            className="w-full h-full object-cover opacity-50"
                          />
                        ) : (
                          <div className="w-full h-full flex items-center justify-center text-slate-500">
                            <Info className="w-6 h-6" />
                          </div>
                        )}

                        {/* Play button overlay */}
                        <button
                          onClick={() => handlePlayEpisode(episode.id)}
                          className="absolute inset-0 bg-black/50 flex items-center justify-center opacity-0 hover:opacity-100 transition-opacity"
                        >
                          <Play className="w-8 h-8 text-white" />
                        </button>
                      </div>

                      {/* Episode info */}
                      <div className="flex-1 min-w-0">
                        <div className="flex items-start justify-between gap-4">
                          <div className="flex-1 min-w-0">
                            <h3 className="text-white font-semibold text-lg">
                              {episode.episode_number}. {episode.title}
                            </h3>

                            <div className="flex items-center gap-4 text-slate-400 text-sm mt-1">
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
                                    <span>{episode.mediaFile.resolution}</span>
                                  )}
                                  {episode.mediaFile.video_codec && (
                                    <span>{episode.mediaFile.video_codec.toUpperCase()}</span>
                                  )}
                                  <span>{formatFileSize(episode.mediaFile.size_bytes)}</span>
                                </>
                              )}
                            </div>

                            {episode.description && (
                              <p className="text-slate-300 text-sm mt-2 line-clamp-2">
                                {episode.description}
                              </p>
                            )}
                          </div>

                          {/* Play button */}
                          <button
                            onClick={() => handlePlayEpisode(episode.id)}
                            className="flex items-center gap-2 px-4 py-2 bg-purple-600 hover:bg-purple-700 text-white rounded-lg transition-colors flex-shrink-0"
                          >
                            <Play className="w-4 h-4" />
                            Play
                          </button>
                        </div>
                      </div>
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>
        )}
      </div>
    </div>
  );
};

export default TVShowDetail;
