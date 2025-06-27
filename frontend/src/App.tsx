import { Routes, Route } from 'react-router-dom';
import { Provider } from 'jotai';
import { Header } from './components';
import Home from './pages/Home';
import Admin from './pages/Admin';
import Music from './pages/Music';
import TVShows from './pages/TVShows';
import EnrichmentDashboard from './pages/EnrichmentDashboard';

import TVShowDetail from './components/tv/TVShowDetail';
import EpisodePlayer from './pages/player/EpisodePlayer';
import MoviePlayer from './pages/player/MoviePlayer';
import MediaPlayerTest from './pages/MediaPlayerTest';
import { ThemeProvider } from './providers/ThemeProvider';

const App = () => {
  return (
    <Provider>
      <ThemeProvider defaultTheme="system">
        <div className="min-h-screen bg-background text-foreground">
          <Header />
          <main className="container mx-auto px-4 py-8">
            {/* Docker Compose Development Environment */}
            <Routes>
              <Route path="/" element={<Home />} />
              <Route path="/music" element={<Music />} />
              <Route path="/tv-shows" element={<TVShows />} />
              <Route path="/tv-shows/:showId" element={<TVShowDetail />} />
              
              {/* MediaPlayer routes */}
              <Route path="/player/episode/:episodeId" element={<EpisodePlayer />} />
              <Route path="/player/movie/:movieId" element={<MoviePlayer />} />
              
              {/* Test routes */}
              <Route path="/test/media-player" element={<MediaPlayerTest />} />
              
              <Route path="/admin" element={<Admin />} />
              <Route path="/enrichment-dashboard" element={<EnrichmentDashboard />} />
              
              {/* Future routes for media manager:
                  <Route path="/library" element={<Library />} />
                  <Route path="/media/:id" element={<MediaDetail />} />
                  <Route path="/settings" element={<Settings />} />
              */}
            </Routes>
          </main>
        </div>
      </ThemeProvider>
    </Provider>
  );
};

export default App;
