import React from 'react';
import { TVShowLibrary } from '../components';

const TVShows: React.FC = () => {
  return (
    <div>
      <div className="mb-8">
        <h1 className="text-3xl font-bold text-white mb-4">📺 TV Shows</h1>
        <p className="text-slate-300">
          Browse your TV show collection with poster views and episode tracking.
        </p>
      </div>

      <TVShowLibrary />
    </div>
  );
};

export default TVShows;
