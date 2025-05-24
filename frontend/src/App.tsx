import { Routes, Route } from 'react-router-dom';
import { Provider } from 'jotai';
import Header from './components/Header';
import Home from './pages/Home';

const App = () => {
  return (
    <Provider>
      <div className="min-h-screen bg-slate-950">
        <Header />
        <main className="container mx-auto px-4 py-8">
          <Routes>
            <Route path="/" element={<Home />} />
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
