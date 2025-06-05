import React from 'react';
import { MusicLibrary } from '../components';

const Music: React.FC = () => {
  return (
    <div>
      <div className="mb-8">
        <h1 className="text-3xl font-bold text-white mb-4">🎵 Music Library</h1>
        <p className="text-slate-300">
          Browse and play your music collection with advanced filtering and controls.
        </p>
      </div>

      <MusicLibrary />
    </div>
  );
};

export default Music;
