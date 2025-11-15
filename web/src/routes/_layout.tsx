import { createFileRoute, Link, Outlet } from '@tanstack/react-router'
import { AudioPlayerProvider } from '@/lib/contexts/AudioPlayerContext'
import { AudioPlayer } from '@/components/music'

const Layout = () => {
  return (
    <AudioPlayerProvider>
      <div className="flex h-screen bg-gray-100">
        {/* Sidebar */}
        <aside className="w-64 bg-gray-900 text-white flex flex-col">
          <div className="p-4 border-b border-gray-700">
            <h1 className="text-2xl font-bold">ViewRA</h1>
            <p className="text-sm text-gray-400">Media Server</p>
          </div>

          <nav className="flex-1 p-4 space-y-2">
            <Link
              to="/"
              className="block px-4 py-2 rounded hover:bg-gray-800 transition-colors"
              activeProps={{ className: 'bg-gray-800' }}
            >
              🏠 Dashboard
            </Link>
            <Link
              to="/libraries"
              className="block px-4 py-2 rounded hover:bg-gray-800 transition-colors"
              activeProps={{ className: 'bg-gray-800' }}
            >
              📚 Libraries
            </Link>
            <Link
              to="/movies"
              search={{ id: undefined }}
              className="block px-4 py-2 rounded hover:bg-gray-800 transition-colors"
              activeProps={{ className: 'bg-gray-800' }}
            >
              🎬 Movies
            </Link>
            <Link
              to="/tv"
              className="block px-4 py-2 rounded hover:bg-gray-800 transition-colors"
              activeProps={{ className: 'bg-gray-800' }}
            >
              📺 TV Shows
            </Link>
            <Link
              to="/music"
              className="block px-4 py-2 rounded hover:bg-gray-800 transition-colors"
              activeProps={{ className: 'bg-gray-800' }}
            >
              🎵 Music
            </Link>
          </nav>

          <div className="p-4 border-t border-gray-700">
            <div className="text-sm text-gray-400">
              <p>Version 0.0.1</p>
              <p>Phase 1 MVP</p>
            </div>
          </div>
        </aside>

        {/* Main content */}
        <main className="flex-1 overflow-auto pb-32">
          <Outlet />
        </main>

        {/* Persistent audio player */}
        <AudioPlayer />
      </div>
    </AudioPlayerProvider>
  )
}

export const Route = createFileRoute('/_layout')({
  component: Layout,
})
