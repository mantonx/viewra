import React, { useState, useEffect } from 'react';
import { Link } from 'react-router-dom';
import { Play, Film, Tv, Calendar, Clock, HardDrive, FileVideo, AlertCircle, Shuffle, Filter, RefreshCw } from 'lucide-react';
import type { MediaFile, Episode, Movie, MediaItem } from './MediaPlayerTest.types';
import { MediaPlaybackService } from '../../services/MediaPlaybackService';

// TV show categories for filtering
const TV_SHOW_CATEGORIES = {
  animation: ['rick and morty', 'family guy', 'south park', 'the simpsons', 'archer'],
  comedy: ['the office', 'parks and recreation', 'brooklyn nine-nine', 'community', 'arrested development'],
  drama: ['breaking bad', 'the wire', 'the sopranos', 'mad men', 'house of cards'],
  crime: ['true detective', 'mindhunter', 'the wire', 'breaking bad', 'better call saul'],
  scifi: ['the expanse', 'star trek', 'doctor who', 'westworld', 'black mirror'],
  action: ['24', 'jack ryan', 'the punisher', 'daredevil', 'the mandalorian'],
  classic: ['friends', 'seinfeld', 'cheers', 'frasier', 'the golden girls']
};

interface TestFilters {
  category: string;
  resolution: string;
  codec: string;
  playbackMethod: string; // 'all', 'direct', 'remux', 'transcode'
  showMovies: boolean;
  showEpisodes: boolean;
}

const MediaPlayerTest: React.FC = () => {
  const [mediaItems, setMediaItems] = useState<MediaItem[]>([]);
  const [allMediaItems, setAllMediaItems] = useState<MediaItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [filters, setFilters] = useState<TestFilters>({
    category: 'all',
    resolution: 'all',
    codec: 'all',
    playbackMethod: 'all',
    showMovies: true,
    showEpisodes: true
  });

  const loadMedia = async () => {
    setLoading(true);
    setError(null);

    try {
      // Build query parameters based on filters
      const params = new URLSearchParams();
      params.append('limit', '50'); // Reduced for better performance
      
      if (filters.resolution !== 'all') {
        params.append('resolution', filters.resolution);
      }
      if (filters.codec !== 'all') {
        params.append('video_codec', filters.codec);
      }
      if (filters.playbackMethod !== 'all') {
        params.append('playback_method', filters.playbackMethod);
      }
      
      // Get media files with filters
      const filesResponse = await fetch(`/api/media/files?${params.toString()}`);
        const filesData = await filesResponse.json();

        const allItems: MediaItem[] = [];

        // Process episodes - backend filtering provides the variety
        const episodeFiles = filesData.files?.filter((file: MediaFile) => file.media_type === 'episode') || [];
        
        // Process all episode files returned by backend
        for (const file of episodeFiles) {
          try {
            // Extract show information from file path
            const pathMatch = file.path.match(/\/tv\/([^\/]+)\/Season\s+(\d+)\/.*?S(\d+)E(\d+)/i);
            if (pathMatch) {
              const [, showName, seasonNum, , episodeNum] = pathMatch;
              
              // Create episode metadata from file info
              const episode: Episode = {
                id: file.media_id || file.id,
                title: `Episode ${episodeNum}`,
                episode_number: parseInt(episodeNum, 10),
                air_date: undefined,
                description: `An episode of ${showName.replace(/[._-]/g, ' ')}`,
                duration: file.duration,
                still_image: undefined,
                season: {
                  id: `season-${seasonNum}`,
                  season_number: parseInt(seasonNum, 10),
                  tv_show: {
                    id: `show-${showName}`,
                    title: showName.replace(/[._-]/g, ' '),
                    description: `${showName.replace(/[._-]/g, ' ')} TV series`,
                    poster: undefined,
                    backdrop: undefined,
                    tmdb_id: undefined,
                  },
                },
                mediaFile: file,
              };
              allItems.push({ type: 'episode', data: episode });
            }
          } catch (err) {
            console.warn(`Failed to process episode file ${file.id}:`, err);
          }
        }

        // Process movies - backend filtering provides the variety
        const movieFiles = filesData.files?.filter((file: MediaFile) => file.media_type === 'movie') || [];
        
        // Process all movie files returned by backend
        for (const file of movieFiles) {
          try {
            // Extract movie title from file path
            const titleMatch = file.path.match(/\/movies\/([^\/]+)\/.*?\.(mkv|mp4|avi)$/i) || 
                              file.path.match(/([^\/]+)\.(mkv|mp4|avi)$/i);
            
            let movieTitle = 'Unknown Movie';
            if (titleMatch) {
              movieTitle = titleMatch[1]
                .replace(/\.(mkv|mp4|avi)$/i, '')
                .replace(/[._-]/g, ' ')
                .replace(/\s+(19[0-9]{2}|20[0-9]{2})\s*$/, ' ($1)') // Only match years 1900-2099 at end
                .trim();
            }
            
            const movie: Movie = {
              id: file.media_id || file.id,
              title: movieTitle,
              release_date: undefined,
              description: `A movie: ${movieTitle}`,
              duration: file.duration,
              poster: undefined,
              backdrop: undefined,
              tmdb_id: undefined,
              mediaFile: file,
            };
            allItems.push({ type: 'movie', data: movie });
          } catch (err) {
            console.warn(`Failed to process movie file ${file.id}:`, err);
          }
        }

        setAllMediaItems(allItems);
        setMediaItems(allItems);
      } catch (err) {
        console.error('Failed to load media:', err);
        setError(err instanceof Error ? err.message : 'Failed to load media');
      } finally {
        setLoading(false);
      }
    };

  // Load media on mount and when backend filters change
  useEffect(() => {
    loadMedia();
  }, [filters.resolution, filters.codec, filters.playbackMethod]);

  // Apply filters whenever filters or allMediaItems change
  useEffect(() => {
    let filtered = [...allMediaItems];

    // Filter by type
    if (!filters.showMovies) {
      filtered = filtered.filter(item => item.type !== 'movie');
    }
    if (!filters.showEpisodes) {
      filtered = filtered.filter(item => item.type !== 'episode');
    }

    // Filter by category (for TV shows)
    if (filters.category !== 'all') {
      filtered = filtered.filter(item => {
        if (item.type !== 'episode') return true;
        
        const showTitle = item.data.season.tv_show.title.toLowerCase();
        const categoryShows = TV_SHOW_CATEGORIES[filters.category as keyof typeof TV_SHOW_CATEGORIES] || [];
        
        return categoryShows.some(categoryShow => 
          showTitle.includes(categoryShow) || categoryShow.includes(showTitle.replace(/[^\w\s]/g, ''))
        );
      });
    }

    // Resolution, codec, and playback method filtering is now handled by backend

    setMediaItems(filtered);
  }, [filters, allMediaItems]);


  const getPlaybackMethodLabel = (method: 'direct' | 'remux' | 'transcode'): string => {
    switch (method) {
      case 'direct': return '🟢 Direct Play';
      case 'remux': return '🟡 Remux Required';
      case 'transcode': return '🔴 Transcode Required';
    }
  };

  const getPlaybackMethodColor = (method: 'direct' | 'remux' | 'transcode'): string => {
    switch (method) {
      case 'direct': return 'text-green-600 bg-green-50';
      case 'remux': return 'text-yellow-600 bg-yellow-50';
      case 'transcode': return 'text-red-600 bg-red-50';
    }
  };


  const getRandomSelection = () => {
    const shuffled = [...allMediaItems].sort(() => Math.random() - 0.5);
    setMediaItems(shuffled);
  };

  const resetFilters = () => {
    setFilters({
      category: 'all',
      resolution: 'all',
      codec: 'all',
      playbackMethod: 'all',
      showMovies: true,
      showEpisodes: true
    });
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

  const formatBitrate = (bitrateKbps?: number) => {
    if (!bitrateKbps) return '';
    if (bitrateKbps > 1000) {
      return `${(bitrateKbps / 1000).toFixed(1)} Mbps`;
    }
    return `${Math.round(bitrateKbps)} kbps`;
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
          <p className="text-slate-400">Loading diverse test media...</p>
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
          <button
            onClick={() => window.location.reload()}
            className="px-4 py-2 bg-purple-600 hover:bg-purple-700 text-white rounded-lg transition-colors"
          >
            Try Again
          </button>
        </div>
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-slate-900">
      <div className="container mx-auto px-4 py-8">
        <div className="mb-8">
          <h1 className="text-3xl font-bold text-white mb-6 flex items-center gap-3">
            <FileVideo className="w-8 h-8" />
            Media Player Test
          </h1>

          {/* Filters */}
          <div className="bg-slate-800 rounded-lg p-6 mb-6">
            <h3 className="text-lg font-semibold text-white mb-4 flex items-center gap-2">
              <Filter className="w-5 h-5" />
              Test Filters
            </h3>
            
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-5 gap-4 mb-4">
              {/* Content Type */}
              <div>
                <label className="block text-sm font-medium text-slate-300 mb-2">Content Type</label>
                <div className="space-y-2">
                  <label className="flex items-center gap-2">
                    <input
                      type="checkbox"
                      checked={filters.showEpisodes}
                      onChange={(e) => setFilters({...filters, showEpisodes: e.target.checked})}
                      className="rounded"
                    />
                    <span className="text-slate-300">TV Episodes</span>
                  </label>
                  <label className="flex items-center gap-2">
                    <input
                      type="checkbox"
                      checked={filters.showMovies}
                      onChange={(e) => setFilters({...filters, showMovies: e.target.checked})}
                      className="rounded"
                    />
                    <span className="text-slate-300">Movies</span>
                  </label>
                </div>
              </div>

              {/* TV Show Category */}
              <div>
                <label className="block text-sm font-medium text-slate-300 mb-2">TV Show Category</label>
                <select
                  value={filters.category}
                  onChange={(e) => setFilters({...filters, category: e.target.value})}
                  className="w-full bg-slate-700 text-white rounded-md px-3 py-2 text-sm"
                >
                  <option value="all">All Categories</option>
                  <option value="animation">Animation</option>
                  <option value="comedy">Comedy</option>
                  <option value="drama">Drama</option>
                  <option value="crime">Crime</option>
                  <option value="scifi">Sci-Fi</option>
                  <option value="action">Action</option>
                  <option value="classic">Classic</option>
                </select>
              </div>

              {/* Resolution */}
              <div>
                <label className="block text-sm font-medium text-slate-300 mb-2">Resolution</label>
                <select
                  value={filters.resolution}
                  onChange={(e) => setFilters({...filters, resolution: e.target.value})}
                  className="w-full bg-slate-700 text-white rounded-md px-3 py-2 text-sm"
                >
                  <option value="all">All Resolutions</option>
                  <option value="4k">4K/2160p</option>
                  <option value="1080p">1080p</option>
                  <option value="720p">720p</option>
                  <option value="480p">480p/SD</option>
                </select>
              </div>

              {/* Codec */}
              <div>
                <label className="block text-sm font-medium text-slate-300 mb-2">Video Codec</label>
                <select
                  value={filters.codec}
                  onChange={(e) => setFilters({...filters, codec: e.target.value})}
                  className="w-full bg-slate-700 text-white rounded-md px-3 py-2 text-sm"
                >
                  <option value="all">All Codecs</option>
                  <option value="h265">H.265/HEVC</option>
                  <option value="h264">H.264</option>
                  <option value="av1">AV1</option>
                  <option value="mpeg2">MPEG2</option>
                </select>
              </div>
              
              {/* Playback Method */}
              <div>
                <label className="block text-sm font-medium text-slate-300 mb-2">Playback Method</label>
                <select
                  value={filters.playbackMethod}
                  onChange={(e) => setFilters({...filters, playbackMethod: e.target.value})}
                  className="w-full bg-slate-700 text-white rounded-md px-3 py-2 text-sm"
                >
                  <option value="all">All Methods</option>
                  <option value="direct">🟢 Direct Play</option>
                  <option value="remux">🟡 Remux Required</option>
                  <option value="transcode">🔴 Transcode Required</option>
                </select>
              </div>
            </div>

            {/* Action Buttons */}
            <div className="flex flex-wrap gap-3">
              <button
                onClick={resetFilters}
                className="flex items-center gap-2 px-4 py-2 bg-slate-700 hover:bg-slate-600 text-white rounded-lg transition-colors"
              >
                <RefreshCw className="w-4 h-4" />
                Reset Filters
              </button>
              <button
                onClick={getRandomSelection}
                className="flex items-center gap-2 px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white rounded-lg transition-colors"
              >
                <Shuffle className="w-4 h-4" />
                Random Selection
              </button>
              <button
                onClick={() => window.location.reload()}
                className="flex items-center gap-2 px-4 py-2 bg-purple-600 hover:bg-purple-700 text-white rounded-lg transition-colors"
              >
                <RefreshCw className="w-4 h-4" />
                Reload All Media
              </button>
            </div>
          </div>

          {/* Simple stats */}
          <div className="flex items-center gap-4 text-sm text-slate-400 mb-6">
            <span>{mediaItems.length} items</span>
            <span>•</span>
            <span>{mediaItems.filter(item => item.type === 'episode').length} episodes</span>
            <span>•</span>
            <span>{mediaItems.filter(item => item.type === 'movie').length} movies</span>
            {filters.playbackMethod !== 'all' && (
              <>
                <span>•</span>
                <span className="text-slate-300">Showing {filters.playbackMethod} files</span>
              </>
            )}
          </div>
        </div>

        {mediaItems.length === 0 ? (
          <div className="text-center py-12">
            <p className="text-slate-400 mb-4">No media files match your filters.</p>
            <button
              onClick={resetFilters}
              className="px-4 py-2 bg-purple-600 hover:bg-purple-700 text-white rounded-lg transition-colors"
            >
              Reset Filters
            </button>
          </div>
        ) : (
          <div className="space-y-4">
            {mediaItems.map((item) => {
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

                      {/* Type and playback method badges */}
                      <div className="absolute top-2 left-2 flex flex-col gap-1">
                        <div className={`px-2 py-1 rounded text-xs font-medium ${
                          isEpisode ? 'bg-blue-600 text-white' : 'bg-purple-600 text-white'
                        }`}>
                          {isEpisode ? 'Episode' : 'Movie'}
                        </div>
                        {mediaFile && filters.playbackMethod !== 'all' && (
                          <div className={`px-2 py-1 rounded text-xs font-medium ${
                            getPlaybackMethodColor(filters.playbackMethod as 'direct' | 'remux' | 'transcode')
                          }`}>
                            {getPlaybackMethodLabel(filters.playbackMethod as 'direct' | 'remux' | 'transcode')}
                          </div>
                        )}
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
                                {mediaFile.video_framerate && (
                                  <span className="bg-slate-700 px-2 py-1 rounded">
                                    {parseFloat(mediaFile.video_framerate.split('/')[0]) / parseFloat(mediaFile.video_framerate.split('/')[1] || '1')}fps
                                  </span>
                                )}
                                {mediaFile.bitrate_kbps && (
                                  <span className="bg-slate-700 px-2 py-1 rounded">
                                    {formatBitrate(mediaFile.bitrate_kbps)}
                                  </span>
                                )}
                                <span className="flex items-center gap-1">
                                  <HardDrive className="w-3 h-3" />
                                  {formatFileSize(mediaFile.size_bytes)}
                                </span>
                              </div>
                              
                              <div className="text-xs text-slate-500 truncate">
                                {mediaFile.path.split('/').pop()}
                              </div>
                            </div>
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
                            Test Play
                          </Link>
                        )}
                      </div>
                    </div>
                  </div>
                </div>
              );
            })}
          </div>
        )}
      </div>
    </div>
  );
};

export default MediaPlayerTest;