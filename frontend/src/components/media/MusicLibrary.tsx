import { useState, useEffect, useRef, useCallback } from 'react';
import MediaCard from './MediaCard';
import AudioPlayer from '@/components/audio/AudioPlayer';
import type { MusicFile, GroupedMusicFile, SortField, SortDirection } from '@/types/music.types';
import type { ApiResponse } from '@/types/api.types';

const MusicLibrary = () => {
  const [musicFiles, setMusicFiles] = useState<MusicFile[]>([]);
  const [groupedFiles, setGroupedFiles] = useState<GroupedMusicFile[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [limit] = useState(48);

  // Audio player state
  const [currentTrack, setCurrentTrack] = useState<MusicFile | null>(null);
  const [isPlaying, setIsPlaying] = useState(false);
  const [currentTime, setCurrentTime] = useState(0);
  const [duration, setDuration] = useState(0);
  const [volume, setVolume] = useState(0.7);
  const [playbackRate, setPlaybackRate] = useState(1);
  const audioRef = useRef<HTMLAudioElement>(null);

  // New state for loop and shuffle functionality
  const [isLoopOn, setIsLoopOn] = useState(false);
  const [isShuffleOn, setIsShuffleOn] = useState(false);
  const [currentPlaylist, setCurrentPlaylist] = useState<MusicFile[]>([]);
  const [currentPlaylistIndex, setCurrentPlaylistIndex] = useState(-1);

  // Sorting and filtering
  const [sortField, setSortField] = useState<SortField>('artist');
  const [sortDirection, setSortDirection] = useState<SortDirection>('asc');
  const [filterText, setFilterText] = useState('');
  const [filterGenre, setFilterGenre] = useState<string>('');
  const [availableGenres, setAvailableGenres] = useState<string[]>([]);
  const [viewMode, setViewMode] = useState<'grid' | 'list' | 'albums'>('albums');

  const groupMusicFiles = useCallback(
    (files: MusicFile[]) => {
      // Apply filtering
      let filteredFiles = files;
      if (filterText) {
        const searchTerm = filterText.toLowerCase();
        filteredFiles = files.filter(
          (file) =>
            file.music_metadata?.title?.toLowerCase().includes(searchTerm) ||
            file.music_metadata?.artist?.toLowerCase().includes(searchTerm) ||
            file.music_metadata?.album?.toLowerCase().includes(searchTerm) ||
            file.music_metadata?.genre?.toLowerCase().includes(searchTerm)
        );
      }

      if (filterGenre) {
        filteredFiles = filteredFiles.filter((file) => file.music_metadata?.genre === filterGenre);
      }

      // Apply sorting to the list view
      const sortedFiles = [...filteredFiles].sort((a, b) => {
        const fieldA = a.music_metadata?.[sortField] || '';
        const fieldB = b.music_metadata?.[sortField] || '';

        if (typeof fieldA === 'string' && typeof fieldB === 'string') {
          return sortDirection === 'asc'
            ? fieldA.localeCompare(fieldB)
            : fieldB.localeCompare(fieldA);
        } else {
          // For numeric fields
          return sortDirection === 'asc'
            ? Number(fieldA) - Number(fieldB)
            : Number(fieldB) - Number(fieldA);
        }
      });

      // Group by artist and album
      const artistMap = new Map<string, Map<string, MusicFile[]>>();

      sortedFiles.forEach((file) => {
        const artist = file.music_metadata?.artist || 'Unknown Artist';
        const album = file.music_metadata?.album || 'Unknown Album';

        if (!artistMap.has(artist)) {
          artistMap.set(artist, new Map<string, MusicFile[]>());
        }

        const artistAlbums = artistMap.get(artist)!;
        if (!artistAlbums.has(album)) {
          artistAlbums.set(album, []);
        }

        artistAlbums.get(album)!.push(file);
      });

      // Convert map to our grouped structure
      const grouped: GroupedMusicFile[] = [];

      artistMap.forEach((albumsMap, artist) => {
        const artistGroup: GroupedMusicFile = {
          artist,
          albums: [],
        };

        albumsMap.forEach((tracks, albumTitle) => {
          // Sort tracks by disc number and track number
          const sortedTracks = [...tracks].sort((a, b) => {
            const discA = a.music_metadata?.disc || 0;
            const discB = b.music_metadata?.disc || 0;

            if (discA !== discB) return discA - discB;

            const trackA = a.music_metadata?.track || 0;
            const trackB = b.music_metadata?.track || 0;
            return trackA - trackB;
          });

          // Find album artwork from the first track that has it
          const trackWithArtwork = tracks.find((t) => t.music_metadata?.has_artwork);
          const artworkUrl = trackWithArtwork
            ? `/api/media/${trackWithArtwork.id}/artwork`
            : undefined;

          artistGroup.albums.push({
            title: albumTitle,
            year: tracks[0].music_metadata?.year,
            artwork: artworkUrl,
            tracks: sortedTracks,
          });
        });

        // Sort albums by year
        artistGroup.albums.sort((a, b) => {
          if (!a.year) return 1;
          if (!b.year) return -1;
          return a.year - b.year;
        });

        grouped.push(artistGroup);
      });

      setGroupedFiles(grouped);
    },
    [filterText, filterGenre, sortField, sortDirection]
  );

  const loadMusicFiles = useCallback(async () => {
    setLoading(true);
    try {
      const offset = (page - 1) * limit;
      const response = await fetch(`/api/media/music?limit=${limit}&offset=${offset}`);
      const data: ApiResponse = await response.json();

      setMusicFiles(data.music_files ?? []);
      setTotal(data.total ?? 0);

      // Extract unique genres
      const genres = Array.from(
        new Set(
          (data.music_files ?? [])
            .filter((file) => file.music_metadata?.genre)
            .map((file) => file.music_metadata.genre)
        )
      ).sort();

      setAvailableGenres(genres);

      // Group files by artist and album
      groupMusicFiles(data.music_files ?? []);
    } catch (err) {
      console.error('Failed to load music files:', err);
      setError('Failed to load music files. Please try again later.');
    } finally {
      setLoading(false);
    }
  }, [page, limit, groupMusicFiles]);

  useEffect(() => {
    loadMusicFiles();
  }, [loadMusicFiles]);

  useEffect(() => {
    if (audioRef.current) {
      if (isPlaying) {
        audioRef.current.play().catch((err) => {
          console.error('Failed to play audio:', err);
          setIsPlaying(false);
        });
      } else {
        audioRef.current.pause();
      }
    }
  }, [isPlaying, currentTrack]);

  // Audio event listeners
  useEffect(() => {
    const audio = audioRef.current;

    if (!audio) return;

    const handleTimeUpdate = () => {
      setCurrentTime(audio.currentTime);
    };

    const handleDurationChange = () => {
      setDuration(audio.duration);
    };

    const handleVolumeChange = () => {
      setVolume(audio.volume);
    };

    const handleRateChange = () => {
      setPlaybackRate(audio.playbackRate);
    };

    // Add event listeners
    audio.addEventListener('timeupdate', handleTimeUpdate);
    audio.addEventListener('durationchange', handleDurationChange);
    audio.addEventListener('volumechange', handleVolumeChange);
    audio.addEventListener('ratechange', handleRateChange);

    // Set initial values
    audio.volume = volume;
    audio.playbackRate = playbackRate;

    // Clean up
    return () => {
      audio.removeEventListener('timeupdate', handleTimeUpdate);
      audio.removeEventListener('durationchange', handleDurationChange);
      audio.removeEventListener('volumechange', handleVolumeChange);
      audio.removeEventListener('ratechange', handleRateChange);
    };
  }, [currentTrack, volume, playbackRate]);

  const formatDuration = (durationInNs: number) => {
    if (!durationInNs) return '00:00';

    const seconds = Math.floor(durationInNs / 1000000000);
    const minutes = Math.floor(seconds / 60);
    const remainingSeconds = seconds % 60;

    return `${minutes.toString().padStart(2, '0')}:${remainingSeconds.toString().padStart(2, '0')}`;
  };

  const formatTime = (seconds: number) => {
    if (!seconds || isNaN(seconds)) return '0:00';
    const minutes = Math.floor(seconds / 60);
    const remainingSeconds = Math.floor(seconds % 60);
    return `${minutes}:${remainingSeconds.toString().padStart(2, '0')}`;
  };

  // New methods for track navigation and loop functionality
  const playNext = () => {
    if (!currentPlaylist.length || currentPlaylistIndex === -1) return;

    // If shuffle is on, pick a random track
    if (isShuffleOn) {
      const randomIndex = Math.floor(Math.random() * currentPlaylist.length);
      setCurrentPlaylistIndex(randomIndex);
      setCurrentTrack(currentPlaylist[randomIndex]);
      setIsPlaying(true);
      return;
    }

    // Normal sequential playback
    const nextIndex = (currentPlaylistIndex + 1) % currentPlaylist.length;
    setCurrentPlaylistIndex(nextIndex);
    setCurrentTrack(currentPlaylist[nextIndex]);
    setIsPlaying(true);
  };

  const playPrevious = () => {
    if (!currentPlaylist.length || currentPlaylistIndex === -1) return;

    // If at the start or less than 3 seconds into the track, go to previous track
    // Otherwise restart the current track
    if (currentTime > 3) {
      if (audioRef.current) {
        audioRef.current.currentTime = 0;
      }
      return;
    }

    const prevIndex = (currentPlaylistIndex - 1 + currentPlaylist.length) % currentPlaylist.length;
    setCurrentPlaylistIndex(prevIndex);
    setCurrentTrack(currentPlaylist[prevIndex]);
    setIsPlaying(true);
  };

  const toggleLoop = () => {
    setIsLoopOn(!isLoopOn);
    if (audioRef.current) {
      audioRef.current.loop = !isLoopOn;
    }
  };

  const toggleShuffle = () => {
    setIsShuffleOn(!isShuffleOn);
  };

  const handlePlayPause = (track: MusicFile) => {
    if (currentTrack && currentTrack.id === track.id) {
      setIsPlaying(!isPlaying);
    } else {
      // Find which view mode we're in and set the appropriate playlist
      let playlist: MusicFile[] = [];
      let trackIndex = -1;

      if (viewMode === 'albums') {
        // Find the album this track belongs to
        for (const artistGroup of groupedFiles) {
          for (const album of artistGroup.albums) {
            const index = album.tracks.findIndex((t) => t.id === track.id);
            if (index !== -1) {
              playlist = album.tracks;
              trackIndex = index;
              break;
            }
          }
          if (trackIndex !== -1) break;
        }
      } else {
        // In grid or list view, use the filtered and sorted musicFiles
        playlist = musicFiles;
        trackIndex = playlist.findIndex((t) => t.id === track.id);
      }

      setCurrentPlaylist(playlist);
      setCurrentPlaylistIndex(trackIndex);
      setCurrentTrack(track);
      setIsPlaying(true);
    }
  };

  // Seek functionality
  const handleSeek = (e: React.ChangeEvent<HTMLInputElement>) => {
    const newTime = Number(e.target.value);
    setCurrentTime(newTime);
    if (audioRef.current) {
      audioRef.current.currentTime = newTime;
    }
  };

  // Volume control
  const handleVolumeChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const newVolume = Number(e.target.value);
    setVolume(newVolume);
    if (audioRef.current) {
      audioRef.current.volume = newVolume;
    }
  };

  // Playback rate control
  const handlePlaybackRateChange = (rate: number) => {
    setPlaybackRate(rate);
    if (audioRef.current) {
      audioRef.current.playbackRate = rate;
    }
  };

  // View mode toggle
  const toggleViewMode = (mode: 'grid' | 'list' | 'albums') => {
    setViewMode(mode);
  };

  // Sorting functionality
  const handleSortChange = (field: SortField) => {
    if (sortField === field) {
      setSortDirection(sortDirection === 'asc' ? 'desc' : 'asc');
    } else {
      setSortField(field);
      setSortDirection('asc');
    }
  };

  return (
    <div className="bg-slate-900 rounded-lg p-6 shadow-xl">
      <h2 className="text-xl font-semibold text-white mb-4">🎵 Music Library</h2>

      {loading && (
        <div className="text-center py-8">
          <div className="animate-spin rounded-full h-10 w-10 border-t-2 border-b-2 border-purple-500 mx-auto"></div>
          <p className="text-slate-400 mt-4">Loading music library...</p>
        </div>
      )}

      {error && <div className="bg-red-900/50 text-red-100 p-4 rounded-lg mb-4">{error}</div>}

      {!loading && musicFiles.length === 0 && (
        <div className="text-slate-400 text-center py-12">
          No music files found in your library.
          <br />
          <br />
          <span className="block text-sm">
            This could be because:
            <ul className="list-disc list-inside mt-2 text-slate-500">
              <li>Your scan is still in progress</li>
              <li>No music files were found</li>
              <li>Your music files don't have extractable metadata</li>
            </ul>
          </span>
        </div>
      )}

      {!loading && musicFiles.length > 0 && (
        <div className="space-y-6">
          {/* Audio Player */}
          {currentTrack && (
            <AudioPlayer
              currentTrack={currentTrack}
              isPlaying={isPlaying}
              currentTime={currentTime}
              duration={duration}
              volume={volume}
              playbackRate={playbackRate}
              audioRef={audioRef as React.RefObject<HTMLAudioElement>}
              onPlayPause={() => setIsPlaying(!isPlaying)}
              onSeek={handleSeek}
              onVolumeChange={handleVolumeChange}
              onPlaybackRateChange={handlePlaybackRateChange}
              onNext={currentPlaylist.length > 1 ? playNext : undefined}
              onPrevious={currentPlaylist.length > 1 ? playPrevious : undefined}
              onRepeat={toggleLoop}
              onShuffle={currentPlaylist.length > 1 ? toggleShuffle : undefined}
              isRepeatOn={isLoopOn}
              isShuffleOn={isShuffleOn}
              formatTime={formatTime}
            />
          )}

          {/* Controls and Filters */}
          <div className="bg-slate-800 rounded-lg p-4 mb-4">
            <div className="flex flex-wrap gap-4 justify-between items-center">
              {/* View Mode Selector */}
              <div className="flex rounded-lg overflow-hidden border border-slate-700">
                <button
                  onClick={() => toggleViewMode('grid')}
                  className={`px-3 py-2 ${viewMode === 'grid' ? 'bg-slate-600 text-white' : 'bg-slate-800 text-slate-400'}`}
                >
                  Grid
                </button>
                <button
                  onClick={() => toggleViewMode('list')}
                  className={`px-3 py-2 ${viewMode === 'list' ? 'bg-slate-600 text-white' : 'bg-slate-800 text-slate-400'}`}
                >
                  List
                </button>
                <button
                  onClick={() => toggleViewMode('albums')}
                  className={`px-3 py-2 ${viewMode === 'albums' ? 'bg-slate-600 text-white' : 'bg-slate-800 text-slate-400'}`}
                >
                  Albums
                </button>
              </div>

              {/* Search */}
              <div className="flex-1 max-w-md">
                <input
                  type="text"
                  placeholder="Search by title, artist, album..."
                  value={filterText}
                  onChange={(e) => setFilterText(e.target.value)}
                  className="w-full px-4 py-2 bg-slate-700 border border-slate-600 rounded-lg text-white"
                />
              </div>

              {/* Genre Filter */}
              <div>
                <select
                  value={filterGenre}
                  onChange={(e) => setFilterGenre(e.target.value)}
                  className="px-4 py-2 bg-slate-700 border border-slate-600 rounded-lg text-white"
                >
                  <option value="">All Genres</option>
                  {availableGenres.map((genre) => (
                    <option key={genre} value={genre}>
                      {genre}
                    </option>
                  ))}
                </select>
              </div>

              {/* Sort Options */}
              <div>
                <select
                  value={sortField}
                  onChange={(e) => handleSortChange(e.target.value as SortField)}
                  className="px-4 py-2 bg-slate-700 border border-slate-600 rounded-lg text-white"
                >
                  <option value="title">Title</option>
                  <option value="artist">Artist</option>
                  <option value="album">Album</option>
                  <option value="year">Year</option>
                  <option value="genre">Genre</option>
                </select>
                <button
                  onClick={() => setSortDirection(sortDirection === 'asc' ? 'desc' : 'asc')}
                  className="ml-2 px-3 py-2 bg-slate-700 border border-slate-600 rounded-lg text-white"
                >
                  {sortDirection === 'asc' ? '↑' : '↓'}
                </button>
              </div>
            </div>
          </div>

          {/* Album View */}
          {viewMode === 'albums' && groupedFiles.length > 0 && (
            <div className="space-y-8">
              {groupedFiles.map((artistGroup, index) => (
                <div key={index} className="space-y-4">
                  <h3 className="text-xl font-semibold text-white border-b border-slate-700 pb-2">
                    {artistGroup.artist}
                  </h3>

                  <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4 gap-6">
                    {artistGroup.albums.map((album, albumIndex) => (
                      <MediaCard
                        key={albumIndex}
                        variant="album"
                        item={album}
                        onPlay={handlePlayPause}
                      />
                    ))}
                  </div>
                </div>
              ))}
            </div>
          )}

          {/* Grid View */}
          {viewMode === 'grid' && (
            <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4 gap-4">
              {musicFiles.map((file) => (
                <MediaCard
                  key={file.id}
                  variant="track"
                  item={file}
                  isCurrentTrack={currentTrack?.id === file.id}
                  isPlaying={isPlaying}
                  onPlay={handlePlayPause}
                />
              ))}
            </div>
          )}

          {/* List View */}
          {viewMode === 'list' && (
            <div className="bg-slate-800 rounded-lg overflow-hidden">
              <div className="grid grid-cols-12 gap-4 p-3 bg-slate-700 text-slate-300 text-sm font-medium">
                <div className="col-span-5">Title</div>
                <div className="col-span-2">Artist</div>
                <div className="col-span-2">Album</div>
                <div className="col-span-1">Year</div>
                <div className="col-span-1">Duration</div>
                <div className="col-span-1">Genre</div>
              </div>

              <div className="divide-y divide-slate-700">
                {musicFiles.map((file) => (
                  <div
                    key={file.id}
                    className={`grid grid-cols-12 gap-4 p-3 hover:bg-slate-750 cursor-pointer ${
                      currentTrack && currentTrack.id === file.id ? 'bg-slate-700' : ''
                    }`}
                    onClick={() => handlePlayPause(file)}
                  >
                    <div className="col-span-5 flex items-center gap-3">
                      {file.music_metadata?.has_artwork ? (
                        <img
                          src={`/api/media/${file.id}/artwork`}
                          alt={file.music_metadata?.album || 'Album Artwork'}
                          className="w-8 h-8 object-cover rounded"
                        />
                      ) : (
                        <div className="w-8 h-8 bg-slate-700 flex items-center justify-center rounded">
                          <span className="text-sm">🎵</span>
                        </div>
                      )}
                      <span className="text-white truncate">
                        {file.music_metadata?.title ||
                          file.path.split('/').pop() ||
                          'Unknown Track'}
                        {currentTrack && currentTrack.id === file.id && (
                          <span className="ml-2 text-purple-400">{isPlaying ? '▶️' : '⏸'}</span>
                        )}
                      </span>
                    </div>
                    <div className="col-span-2 text-slate-400 truncate self-center">
                      {file.music_metadata?.artist || 'Unknown Artist'}
                    </div>
                    <div className="col-span-2 text-slate-400 truncate self-center">
                      {file.music_metadata?.album || 'Unknown Album'}
                    </div>
                    <div className="col-span-1 text-slate-500 self-center">
                      {file.music_metadata?.year || '—'}
                    </div>
                    <div className="col-span-1 text-slate-500 self-center">
                      {formatDuration(file.music_metadata?.duration || 0)}
                    </div>
                    <div className="col-span-1 text-slate-500 truncate self-center">
                      {file.music_metadata?.genre || '—'}
                    </div>
                  </div>
                ))}
              </div>
            </div>
          )}

          {/* Pagination */}
          {total > limit && (
            <div className="flex justify-between items-center mt-6">
              <div className="text-slate-400 text-sm">
                Showing {(page - 1) * limit + 1}-{Math.min(page * limit, total)} of {total}
              </div>
              <div className="flex space-x-2">
                <button
                  onClick={() => setPage((prev) => Math.max(prev - 1, 1))}
                  disabled={page === 1}
                  className="px-3 py-1 bg-slate-700 rounded text-white disabled:bg-slate-800 disabled:text-slate-600"
                >
                  Previous
                </button>
                <button
                  onClick={() => setPage((prev) => prev + 1)}
                  disabled={page * limit >= total}
                  className="px-3 py-1 bg-slate-700 rounded text-white disabled:bg-slate-800 disabled:text-slate-600"
                >
                  Next
                </button>
              </div>
            </div>
          )}

          <div className="flex justify-center mt-6">
            <button
              onClick={loadMusicFiles}
              className="px-4 py-2 bg-purple-600 hover:bg-purple-700 text-white rounded transition-colors"
            >
              Refresh Music Library
            </button>
          </div>
        </div>
      )}
    </div>
  );
};

export default MusicLibrary;
