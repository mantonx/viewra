import React from 'react';
import { Tv, Film, Music, Database } from 'lucide-react';
import { EnrichmentProgressCard } from '@/components';

const EnrichmentDashboard: React.FC = () => {
  return (
    <div className="p-6 max-w-7xl mx-auto">
      {/* Header */}
      <div className="mb-6">
        <h1 className="text-2xl font-bold text-white mb-2">Enrichment Dashboard</h1>
        <p className="text-slate-400">
          Track metadata and artwork enrichment progress across all your media libraries.
        </p>
      </div>

      {/* Progress Cards Grid */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {/* Overall Progress */}
        <div className="lg:col-span-2">
          <EnrichmentProgressCard
            mediaType="all"
            title="Overall Enrichment Progress"
            icon={<Database className="w-6 h-6 text-blue-400" />}
          />
        </div>

        {/* TV Shows Progress */}
        <EnrichmentProgressCard
          mediaType="tv_shows"
          title="TV Show Enrichment"
          icon={<Tv className="w-6 h-6 text-green-400" />}
        />

        {/* Movies Progress */}
        <EnrichmentProgressCard
          mediaType="movies"
          title="Movie Enrichment"
          icon={<Film className="w-6 h-6 text-purple-400" />}
        />

        {/* Music Progress */}
        <div className="lg:col-span-2">
          <EnrichmentProgressCard
            mediaType="music"
            title="Music Enrichment"
            icon={<Music className="w-6 h-6 text-orange-400" />}
          />
        </div>
      </div>

      {/* Help Text */}
      <div className="mt-8 bg-slate-800 rounded-lg border border-slate-700 p-6">
        <h2 className="text-lg font-semibold text-white mb-3">About Enrichment Progress</h2>
        <div className="text-slate-300 space-y-2">
          <p>
            This dashboard shows the progress of metadata and artwork enrichment for your media
            library. Enrichment happens automatically after files are scanned and includes:
          </p>
          <ul className="list-disc list-inside ml-4 space-y-1">
            <li>
              <strong>Metadata:</strong> Title, description, air dates, genres, and more from TMDB
              and other sources
            </li>
            <li>
              <strong>Artwork:</strong> Posters, backdrops, banners, and thumbnails
            </li>
            <li>
              <strong>Quality Validation:</strong> Automatic detection and filtering of invalid
              content
            </li>
          </ul>
          <p className="mt-4">
            Progress updates automatically every 30 seconds. Items are considered "fully enriched"
            when they have both complete metadata and artwork.
          </p>
        </div>
      </div>
    </div>
  );
};

export default EnrichmentDashboard;
