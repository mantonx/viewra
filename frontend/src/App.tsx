import { Routes, Route } from 'react-router-dom';
import { Provider } from 'jotai';
import { Header } from './components';
import Home from './pages/Home';
import Admin from './pages/Admin';

const App = () => {
  return (
    <Provider>
      <div className="min-h-screen bg-slate-950">
        <Header />
        <main className="container mx-auto px-4 py-8">
          {/* Docker Compose Development Environment */}
          <Routes>
            <Route path="/" element={<Home />} />
            <Route path="/admin" element={<Admin />} />
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
