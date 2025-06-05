import React, { useState, useEffect, useCallback } from 'react';
import TVShowCard from './TVShowCard';
import type { TVShow, SortField, SortDirection } from '@/types/tv.types';
import type { ApiResponse } from '@/types/api.types';

const TVShowLibrary = () => {
  const [tvShows, setTVShows] = useState<TVShow[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [limit] = useState(24);

  // Sorting and filtering
  const [sortField, setSortField] = useState<SortField>('title');
  const [sortDirection, setSortDirection] = useState<SortDirection>('asc');
  const [filterText, setFilterText] = useState('');
  const [viewMode, setViewMode] = useState<'grid' | 'list'>('grid');

  const loadTVShows = useCallback(async () => {
    setLoading(true);
    try {
      const offset = (page - 1) * limit;
      let query = `/api/media/tv-shows?limit=${limit}&offset=${offset}`;

      // Add sorting parameters
      query += `&sort=${sortField}&order=${sortDirection}`;

      // Add search filter if provided
      if (filterText) {
        query += `&search=${encodeURIComponent(filterText)}`;
      }

      const response = await fetch(query);
      const data: ApiResponse = await response.json();

      setTVShows(data.tv_shows ?? []);
      setTotal(data.total ?? 0);
    } catch (err) {
      console.error('Failed to load TV shows:', err);
      setError('Failed to load TV shows. Please try again later.');
    } finally {
      setLoading(false);
    }
  }, [page, limit, sortField, sortDirection, filterText]);

  useEffect(() => {
    loadTVShows();
  }, [loadTVShows]);

  // Sorting functionality
  const handleSortChange = (field: SortField) => {
    if (sortField === field) {
      setSortDirection(sortDirection === 'asc' ? 'desc' : 'asc');
    } else {
      setSortField(field);
      setSortDirection('asc');
    }
  };

  const formatYear = (dateString: string | null) => {
    if (!dateString) return '';
    return new Date(dateString).getFullYear().toString();
  };

  return (
    <div className="bg-slate-900 rounded-lg p-6 shadow-xl">
      {loading && (
        <div className="text-center py-8">
          <div className="animate-spin rounded-full h-10 w-10 border-t-2 border-b-2 border-purple-500 mx-auto"></div>
          <p className="text-slate-400 mt-4">Loading TV shows...</p>
        </div>
      )}

      {error && <div className="bg-red-900/50 text-red-100 p-4 rounded-lg mb-4">{error}</div>}

      {!loading && tvShows.length === 0 && (
        <div className="text-slate-400 text-center py-12">
          No TV shows found in your library.
          <br />
          <br />
          <span className="block text-sm">
            This could be because:
            <ul className="list-disc list-inside mt-2 text-slate-500">
              <li>Your scan is still in progress</li>
              <li>No TV show files were found</li>
              <li>Your TV show files don't have extractable metadata</li>
            </ul>
          </span>
        </div>
      )}

      {!loading && tvShows.length > 0 && (
        <div className="space-y-6">
          {/* Controls and Filters */}
          <div className="bg-slate-800 rounded-lg p-4 mb-4">
            <div className="flex flex-wrap gap-4 justify-between items-center">
              {/* View Mode Selector */}
              <div className="flex rounded-lg overflow-hidden border border-slate-700">
                <button
                  onClick={() => setViewMode('grid')}
                  className={`px-3 py-2 ${viewMode === 'grid' ? 'bg-slate-600 text-white' : 'bg-slate-800 text-slate-400'}`}
                >
                  Grid
                </button>
                <button
                  onClick={() => setViewMode('list')}
                  className={`px-3 py-2 ${viewMode === 'list' ? 'bg-slate-600 text-white' : 'bg-slate-800 text-slate-400'}`}
                >
                  List
                </button>
              </div>

              {/* Search */}
              <div className="flex-1 max-w-md">
                <input
                  type="text"
                  placeholder="Search TV shows by title..."
                  value={filterText}
                  onChange={(e) => setFilterText(e.target.value)}
                  className="w-full px-4 py-2 bg-slate-700 border border-slate-600 rounded-lg text-white placeholder-slate-400"
                />
              </div>

              {/* Sort Options */}
              <div>
                <select
                  value={sortField}
                  onChange={(e) => handleSortChange(e.target.value as SortField)}
                  className="px-4 py-2 bg-slate-700 border border-slate-600 rounded-lg text-white"
                >
                  <option value="title">Title</option>
                  <option value="first_air_date">First Air Date</option>
                  <option value="status">Status</option>
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

          {/* Show count */}
          <div className="text-slate-400 text-sm">
            {total === 1 ? '1 TV Show' : `${total} TV Shows`}
          </div>

          {/* Grid View */}
          {viewMode === 'grid' && (
            <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5 2xl:grid-cols-6 gap-6">
              {tvShows.map((show) => (
                <TVShowCard key={show.id} show={show} />
              ))}
            </div>
          )}

          {/* List View */}
          {viewMode === 'list' && (
            <div className="bg-slate-800 rounded-lg overflow-hidden">
              <div className="grid grid-cols-12 gap-4 p-3 bg-slate-700 text-slate-300 text-sm font-medium">
                <div className="col-span-1">Poster</div>
                <div className="col-span-4">Title</div>
                <div className="col-span-2">First Aired</div>
                <div className="col-span-2">Status</div>
                <div className="col-span-3">Description</div>
              </div>

              <div className="divide-y divide-slate-700">
                {tvShows.map((show) => (
                  <div
                    key={show.id}
                    className="grid grid-cols-12 gap-4 p-3 hover:bg-slate-750 cursor-pointer"
                  >
                    <div className="col-span-1">
                      <img
                        src={`/api/v1/assets/entity/tv_show/${show.id}/preferred/poster`}
                        alt={`${show.title} poster`}
                        className="w-12 h-16 object-cover rounded"
                        onError={(e) => {
                          const target = e.target as HTMLImageElement;
                          target.src =
                            'data:image/svg+xml;base64,PHN2ZyB3aWR0aD0iNDAiIGhlaWdodD0iNDAiIHZpZXdCb3g9IjAgMCA0MCA0MCIgZmlsbD0ibm9uZSIgeG1sbnM9Imh0dHA6Ly93d3cudzMub3JnLzIwMDAvc3ZnIj4KPHJlY3Qgd2lkdGg9IjQwIiBoZWlnaHQ9IjQwIiBmaWxsPSIjMzc0MTUxIi8+Cjx0ZXh0IHg9IjIwIiB5PSIyNCIgZm9udC1mYW1pbHk9InNhbnMtc2VyaWYiIGZvbnQtc2l6ZT0iMjQiIGZpbGw9IiM5Q0E0QUYiIHRleHQtYW5jaG9yPSJtaWRkbGUiPvCfk7o8L3RleHQ+Cjwvc3ZnPgo=';
                        }}
                      />
                    </div>
                    <div className="col-span-4 text-white truncate self-center">
                      <div className="font-medium">{show.title}</div>
                      {show.tmdb_id && (
                        <div className="text-xs text-slate-500">TMDB: {show.tmdb_id}</div>
                      )}
                    </div>
                    <div className="col-span-2 text-slate-400 self-center">
                      {formatYear(show.first_air_date)}
                    </div>
                    <div className="col-span-2 text-slate-400 self-center">
                      <span
                        className={`px-2 py-1 rounded text-xs ${
                          show.status === 'Running'
                            ? 'bg-green-900 text-green-200'
                            : show.status === 'Ended'
                              ? 'bg-red-900 text-red-200'
                              : 'bg-gray-900 text-gray-200'
                        }`}
                      >
                        {show.status || 'Unknown'}
                      </span>
                    </div>
                    <div className="col-span-3 text-slate-500 text-sm truncate self-center">
                      {show.description || 'No description available'}
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
              onClick={loadTVShows}
              className="px-4 py-2 bg-purple-600 hover:bg-purple-700 text-white rounded transition-colors"
            >
              Refresh TV Shows
            </button>
          </div>
        </div>
      )}
    </div>
  );
};

export default TVShowLibrary;
