import React from 'react';
import type { TVShow } from '@/types/tv.types';

interface TVShowCardProps {
  show: TVShow;
  onClick?: (show: TVShow) => void;
}

const TVShowCard: React.FC<TVShowCardProps> = ({ show, onClick }) => {
  const handleClick = () => {
    if (onClick) {
      onClick(show);
    }
  };

  const formatYear = (dateString: string | undefined) => {
    if (!dateString) return '';
    return new Date(dateString).getFullYear().toString();
  };

  const getStatusColor = (status: string | undefined) => {
    switch (status) {
      case 'Running':
        return 'bg-green-900 text-green-200';
      case 'Ended':
        return 'bg-red-900 text-red-200';
      case 'Cancelled':
        return 'bg-gray-900 text-gray-200';
      default:
        return 'bg-slate-900 text-slate-200';
    }
  };

  return (
    <div
      className={`bg-slate-800 rounded-lg overflow-hidden shadow-lg hover:shadow-xl transition-all duration-200 hover:scale-105 ${
        onClick ? 'cursor-pointer' : ''
      }`}
      onClick={handleClick}
    >
      {/* Poster */}
      <div className="aspect-[2/3] relative bg-slate-700">
        <img
          src={
            show.poster
              ? show.poster
              : `/api/v1/assets/entity/tv_show/${show.id}/preferred/poster/data`
          }
          alt={`${show.title} poster`}
          className="w-full h-full object-cover"
          onError={(e) => {
            const target = e.target as HTMLImageElement;
            // If it's already the fallback, don't try again
            if (target.src.includes('data:image/svg+xml')) return;

            // Try the other source first (TMDB vs local asset)
            if (show.poster && !target.src.includes('tmdb.org')) {
              target.src = show.poster;
            } else if (!show.poster && !target.src.includes('/api/v1/assets/')) {
              target.src = `/api/v1/assets/entity/tv_show/${show.id}/preferred/poster/data`;
            } else {
              // Use SVG placeholder as final fallback
              target.src =
                'data:image/svg+xml;base64,PHN2ZyB3aWR0aD0iMjAwIiBoZWlnaHQ9IjMwMCIgdmlld0JveD0iMCAwIDIwMCAzMDAiIGZpbGw9Im5vbmUiIHhtbG5zPSJodHRwOi8vd3d3LnczLm9yZy8yMDAwL3N2ZyI+CjxyZWN0IHdpZHRoPSIyMDAiIGhlaWdodD0iMzAwIiBmaWxsPSIjMzc0MTUxIi8+Cjx0ZXh0IHg9IjEwMCIgeT0iMTYwIiBmb250LWZhbWlseT0ic2Fucy1zZXJpZiIgZm9udC1zaXplPSI0OCIgZmlsbD0iIzlDQTRBRiIgdGV4dC1hbmNob3I9Im1pZGRsZSI+8J+TujwvdGV4dD4KPC9zdmc+Cg==';
            }
          }}
        />

        {/* Status badge */}
        {show.status && (
          <div className="absolute top-2 right-2">
            <span
              className={`px-2 py-1 rounded text-xs font-medium ${getStatusColor(show.status)}`}
            >
              {show.status}
            </span>
          </div>
        )}
      </div>

      {/* Content */}
      <div className="p-4">
        <h3 className="text-white font-semibold text-sm mb-2 line-clamp-2 leading-tight">
          {show.title}
        </h3>

        <div className="flex items-center justify-between text-xs text-slate-400">
          <span>{formatYear(show.first_air_date)}</span>
          {show.tmdb_id && <span className="text-xs text-slate-500">TMDB</span>}
        </div>

        {show.description && (
          <p className="text-slate-400 text-xs mt-2 line-clamp-3 leading-relaxed">
            {show.description}
          </p>
        )}
      </div>
    </div>
  );
};

export default TVShowCard;
