import React from 'react';
import { Grid, List, Disc } from '@/components/ui/icons';
// If you have alternative icons for sorting, import them here, e.g.:
// import { ArrowUp, ArrowDown } from '@/components/ui/icons';
import { cn } from '@/lib/utils';

interface ViewControlsProps {
  viewMode: 'grid' | 'list' | 'albums';
  filterText: string;
  filterGenre: string;
  sortField: string;
  sortDirection: 'asc' | 'desc';
  availableGenres: string[];
  onViewModeChange: (mode: 'grid' | 'list' | 'albums') => void;
  onFilterTextChange: (text: string) => void;
  onFilterGenreChange: (genre: string) => void;
  onSortChange: (field: string) => void;
  onSortDirectionToggle: () => void;
}

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
    <div className="bg-slate-800 rounded-lg p-4 mb-6">
      <div className="flex flex-col sm:flex-row sm:justify-between sm:items-center gap-4">
        {/* View Mode Buttons */}
        <div className="flex gap-2">
          <button
            onClick={() => onViewModeChange('grid')}
            className={cn(
              'flex-1 px-4 py-2 rounded-lg font-medium transition-all',
              viewMode === 'grid'
                ? 'bg-blue-600 text-white shadow-md'
                : 'bg-slate-700 text-slate-300 hover:bg-slate-600'
            )}
            type="button"
          >
            <Grid className="w-5 h-5 mr-2 inline-block" />
            Grid View
          </button>
          <button
            onClick={() => onViewModeChange('list')}
            className={cn(
              'flex-1 px-4 py-2 rounded-lg font-medium transition-all',
              viewMode === 'list'
                ? 'bg-blue-600 text-white shadow-md'
                : 'bg-slate-700 text-slate-300 hover:bg-slate-600'
            )}
            type="button"
          >
            <List className="w-5 h-5 mr-2 inline-block" />
            List View
          </button>
          <button
            onClick={() => onViewModeChange('albums')}
            className={cn(
              'flex-1 px-4 py-2 rounded-lg font-medium transition-all',
              viewMode === 'albums'
                ? 'bg-blue-600 text-white shadow-md'
                : 'bg-slate-700 text-slate-300 hover:bg-slate-600'
            )}
            type="button"
          >
            <Disc className="w-5 h-5 mr-2 inline-block" />
            Album View
          </button>
        </div>

        {/* Sort and Filter Controls */}
        <div className="flex flex-col sm:flex-row sm:items-center gap-4 w-full sm:w-auto">
          {/* Filter by Text */}
          <div className="flex-1">
            <input
              type="text"
              value={filterText}
              onChange={(e) => onFilterTextChange(e.target.value)}
              placeholder="Search..."
              className="w-full bg-slate-700 text-white px-3 py-2 rounded border border-slate-600 focus:border-blue-500 focus:outline-none"
            />
          </div>

          {/* Filter by Genre */}
          <div className="flex-1">
            <select
              value={filterGenre}
              onChange={(e) => onFilterGenreChange(e.target.value)}
              className="w-full bg-slate-700 text-white px-3 py-2 rounded border border-slate-600 focus:border-blue-500 focus:outline-none"
            >
              <option value="">All Genres</option>
              {availableGenres.map((genre) => (
                <option key={genre} value={genre}>
                  {genre}
                </option>
              ))}
            </select>
          </div>

          {/* Sort By */}
          <div className="flex-1">
            <select
              value={sortField}
              onChange={(e) => onSortChange(e.target.value)}
              className="w-full bg-slate-700 text-white px-3 py-2 rounded border border-slate-600 focus:border-blue-500 focus:outline-none"
            >
              <option value="artist">Artist</option>
              <option value="title">Title</option>
              <option value="album">Album</option>
              <option value="year">Year</option>
              <option value="genre">Genre</option>
            </select>
          </div>

          {/* Sort Direction Toggle */}
          <button
            onClick={onSortDirectionToggle}
            className="bg-slate-700 hover:bg-slate-600 text-white px-4 py-2 rounded-lg transition-colors flex items-center gap-2"
            type="button"
          >
            {/* Replace with available icons or remove icons if not available */}
            {sortDirection === 'asc' ? (
              <>
                <span className="w-5 h-5 inline-block">&#8593;</span>
                Ascending
              </>
            ) : (
              <>
                <span className="w-5 h-5 inline-block">&#8595;</span>
                Descending
              </>
            )}
          </button>
        </div>
      </div>
    </div>
  );
};

export default ViewControls;
