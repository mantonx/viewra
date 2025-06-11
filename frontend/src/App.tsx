import { Routes, Route } from 'react-router-dom';
import { Provider } from 'jotai';
import { Header } from './components';
import Home from './pages/Home';
import Admin from './pages/Admin';
import Music from './pages/Music';
import TVShows from './pages/TVShows';
import EnrichmentDashboard from './pages/EnrichmentDashboard';
import VideoPlayer from './components/tv/VideoPlayer';
import TVShowDetail from './components/tv/TVShowDetail';
import VideoPlayerTest from './pages/VideoPlayerTest';

const App = () => {
  return (
    <Provider>
      <div className="min-h-screen bg-slate-950">
        <Header />
        <main className="container mx-auto px-4 py-8">
          {/* Docker Compose Development Environment */}
          <Routes>
            <Route path="/" element={<Home />} />
            <Route path="/music" element={<Music />} />
            <Route path="/tv-shows" element={<TVShows />} />
            <Route path="/tv-shows/:showId" element={<TVShowDetail />} />
            <Route path="/watch/episode/:episodeId" element={<VideoPlayer />} />
            <Route path="/video-test" element={<VideoPlayerTest />} />
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
    </Provider>
  );
};

export default App;
