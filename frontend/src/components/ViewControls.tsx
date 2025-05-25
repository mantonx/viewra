import React from 'react';
import type { ViewControlsProps } from '../types/ui.types';
import type { SortField } from '../types/music.types';

const ViewControls: React.FC<ViewControlsProps> = ({
  viewMode,
  filterText,
  filterGenre,
  sortField,
  sortDirection,
  availableGenres,
  onViewModeChange,
  onFilterTextChange,
  onFilterGenreChange,
  onSortChange,
  onSortDirectionToggle,
}) => {
  return (
    <div className="bg-slate-800 rounded-lg p-4 mb-4">
      <div className="flex flex-wrap gap-4 justify-between items-center">
        {/* View Mode Selector */}
        <div className="flex rounded-lg overflow-hidden border border-slate-700">
          <button
            onClick={() => onViewModeChange('grid')}
            className={`px-3 py-2 ${
              viewMode === 'grid' ? 'bg-slate-600 text-white' : 'bg-slate-800 text-slate-400'
            }`}
          >
            Grid
          </button>
          <button
            onClick={() => onViewModeChange('list')}
            className={`px-3 py-2 ${
              viewMode === 'list' ? 'bg-slate-600 text-white' : 'bg-slate-800 text-slate-400'
            }`}
          >
            List
          </button>
          <button
            onClick={() => onViewModeChange('albums')}
            className={`px-3 py-2 ${
              viewMode === 'albums' ? 'bg-slate-600 text-white' : 'bg-slate-800 text-slate-400'
            }`}
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
            onChange={(e) => onFilterTextChange(e.target.value)}
            className="w-full px-4 py-2 bg-slate-700 border border-slate-600 rounded-lg text-white"
          />
        </div>

        {/* Genre Filter */}
        <div>
          <select
            value={filterGenre}
            onChange={(e) => onFilterGenreChange(e.target.value)}
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
            onChange={(e) => onSortChange(e.target.value as SortField)}
            className="px-4 py-2 bg-slate-700 border border-slate-600 rounded-lg text-white"
          >
            <option value="title">Title</option>
            <option value="artist">Artist</option>
            <option value="album">Album</option>
            <option value="year">Year</option>
            <option value="genre">Genre</option>
          </select>
          <button
            onClick={onSortDirectionToggle}
            className="ml-2 px-3 py-2 bg-slate-700 border border-slate-600 rounded-lg text-white"
          >
            {sortDirection === 'asc' ? '↑' : '↓'}
          </button>
        </div>
      </div>
    </div>
  );
};

export default ViewControls;
