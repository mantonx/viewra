import { createFileRoute, Link, Outlet } from '@tanstack/react-router'
import { AudioPlayerProvider } from '@/lib/contexts/AudioPlayerContext'
import { AudioPlayer } from '@/components/music'
import { Home, Library, Film, Tv, Music, Clock, Eye } from 'lucide-react'

const Layout = () => {

  return (
    <AudioPlayerProvider>
      <div className="flex h-screen bg-neutral-100 dark:bg-neutral-950">
        {/* Sidebar */}
        <aside className="w-64 bg-white dark:bg-neutral-900 text-neutral-900 dark:text-white flex flex-col border-r border-neutral-200 dark:border-neutral-800">
          <div className="p-4 border-b border-neutral-200 dark:border-neutral-800">
            <h1 className="text-2xl font-bold text-neutral-900 dark:text-neutral-50">ViewRA</h1>
            <p className="text-sm text-neutral-500 dark:text-neutral-400">Media Server</p>
          </div>

          <nav className="flex-1 p-4 space-y-2">
            <Link
              to="/"
              className="flex items-center gap-3 px-4 py-2 rounded hover:bg-neutral-100 dark:hover:bg-neutral-800 transition-colors"
              activeProps={{ className: 'bg-neutral-100 dark:bg-neutral-800' }}
            >
              <Home className="w-5 h-5" />
              <span>Dashboard</span>
            </Link>
            <Link
              to="/libraries"
              className="flex items-center gap-3 px-4 py-2 rounded hover:bg-neutral-100 dark:hover:bg-neutral-800 transition-colors"
              activeProps={{ className: 'bg-neutral-100 dark:bg-neutral-800' }}
            >
              <Library className="w-5 h-5" />
              <span>Libraries</span>
            </Link>
            <Link
              to="/movies"
              search={{ id: undefined, t: undefined, q: undefined, sort: undefined, genres: undefined, yearMin: undefined, yearMax: undefined, qualities: undefined, watched: undefined, view: undefined }}
              className="flex items-center gap-3 px-4 py-2 rounded hover:bg-neutral-100 dark:hover:bg-neutral-800 transition-colors"
              activeProps={{ className: 'bg-neutral-100 dark:bg-neutral-800' }}
            >
              <Film className="w-5 h-5" />
              <span>Movies</span>
            </Link>
            <Link
              to="/tv"
              search={{ q: undefined, sort: undefined, view: undefined }}
              className="flex items-center gap-3 px-4 py-2 rounded hover:bg-neutral-100 dark:hover:bg-neutral-800 transition-colors"
              activeProps={{ className: 'bg-neutral-100 dark:bg-neutral-800' }}
            >
              <Tv className="w-5 h-5" />
              <span>TV Shows</span>
            </Link>
            <Link
              to="/music"
              search={{ q: undefined, sort: undefined, view: undefined }}
              className="flex items-center gap-3 px-4 py-2 rounded hover:bg-neutral-100 dark:hover:bg-neutral-800 transition-colors"
              activeProps={{ className: 'bg-neutral-100 dark:bg-neutral-800' }}
            >
              <Music className="w-5 h-5" />
              <span>Music</span>
            </Link>

            <div className="mt-8 pt-4 border-t border-neutral-200 dark:border-neutral-800">
              <p className="px-4 text-xs text-neutral-500 dark:text-neutral-500 uppercase tracking-wider mb-2">
                Settings
              </p>
              <Link
                to="/settings/display"
                className="flex items-center gap-3 px-4 py-2 rounded hover:bg-neutral-100 dark:hover:bg-neutral-800 transition-colors"
                activeProps={{ className: 'bg-neutral-100 dark:bg-neutral-800' }}
              >
                <Eye className="w-5 h-5" />
                <span>Display</span>
              </Link>
              <Link
                to="/settings/scheduler"
                className="flex items-center gap-3 px-4 py-2 rounded hover:bg-neutral-100 dark:hover:bg-neutral-800 transition-colors"
                activeProps={{ className: 'bg-neutral-100 dark:bg-neutral-800' }}
              >
                <Clock className="w-5 h-5" />
                <span>Scheduler</span>
              </Link>
            </div>
          </nav>

          <div className="p-4 border-t border-neutral-200 dark:border-neutral-800">
            <div className="text-sm text-neutral-500 dark:text-neutral-400">
              <p>Version 0.0.1</p>
            </div>
          </div>
        </aside>

        {/* Main content */}
        <main className="flex-1 overflow-y-auto pb-32">
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
