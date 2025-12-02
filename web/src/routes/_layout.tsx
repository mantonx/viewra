import { createFileRoute, Link, Outlet, useNavigate } from '@tanstack/react-router'
import { useEffect } from 'react'
import { AudioPlayerProvider } from '@/lib/contexts/AudioPlayerContext'
import { AudioPlayer } from '@/components/music'
import { useAuth } from '@/contexts'
import { Home, Library, Film, Tv, Music, Clock, Eye, LogOut, User, Users, KeyRound } from 'lucide-react'

const Layout = () => {
  const navigate = useNavigate()
  const { isAuthenticated, isLoading, needsSetup, user, logout } = useAuth()

  // Redirect to login/setup if not authenticated
  useEffect(() => {
    if (!isLoading) {
      if (needsSetup) {
        navigate({ to: '/setup' })
      } else if (!isAuthenticated) {
        navigate({ to: '/login' })
      }
    }
  }, [isAuthenticated, isLoading, needsSetup, navigate])

  // Show loading while checking auth
  if (isLoading) {
    return (
      <div className="min-h-screen flex items-center justify-center app-background">
        <div className="animate-pulse text-neutral-500 dark:text-neutral-400">Loading...</div>
      </div>
    )
  }

  // Don't render layout if not authenticated
  if (!isAuthenticated) {
    return null
  }

  const handleLogout = async () => {
    await logout()
    navigate({ to: '/login' })
  }

  return (
    <AudioPlayerProvider>
      <div className="flex h-screen app-background">
        {/* Sidebar */}
        <aside className="w-64 glass-sidebar text-neutral-900 dark:text-white flex flex-col relative overflow-hidden">
          {/* Subtle ambient glow at top */}
          <div className="sidebar-glow" />

          <div className="p-4 border-b border-neutral-200/50 dark:border-white/5 relative">
            <h1 className="text-2xl font-semibold text-neutral-900 dark:text-neutral-50 font-display tracking-tight">ViewRA</h1>
            <p className="text-sm text-neutral-500 dark:text-neutral-400">Media Server</p>
          </div>

          <nav className="flex-1 p-4 space-y-1 relative">
            <Link
              to="/"
              className="flex items-center gap-3 px-4 py-2.5 rounded-lg hover:bg-neutral-100 dark:hover:bg-white/5 transition-colors"
              activeProps={{ className: 'bg-neutral-100 dark:bg-white/10' }}
            >
              <Home className="w-5 h-5" />
              <span>Dashboard</span>
            </Link>
            <Link
              to="/libraries"
              className="flex items-center gap-3 px-4 py-2.5 rounded-lg hover:bg-neutral-100 dark:hover:bg-white/5 transition-colors"
              activeProps={{ className: 'bg-neutral-100 dark:bg-white/10' }}
            >
              <Library className="w-5 h-5" />
              <span>Libraries</span>
            </Link>
            <Link
              to="/movies"
              search={{ id: undefined, t: undefined, q: undefined, sort: undefined, genres: undefined, yearMin: undefined, yearMax: undefined, qualities: undefined, watched: undefined, view: undefined }}
              className="flex items-center gap-3 px-4 py-2.5 rounded-lg hover:bg-neutral-100 dark:hover:bg-white/5 transition-colors"
              activeProps={{ className: 'bg-neutral-100 dark:bg-white/10' }}
            >
              <Film className="w-5 h-5" />
              <span>Movies</span>
            </Link>
            <Link
              to="/tv"
              search={{ q: undefined, sort: undefined, genres: undefined, yearMin: undefined, yearMax: undefined, watched: undefined, view: undefined }}
              className="flex items-center gap-3 px-4 py-2.5 rounded-lg hover:bg-neutral-100 dark:hover:bg-white/5 transition-colors"
              activeProps={{ className: 'bg-neutral-100 dark:bg-white/10' }}
            >
              <Tv className="w-5 h-5" />
              <span>TV Shows</span>
            </Link>
            <Link
              to="/music"
              search={{ q: undefined, sort: undefined, view: undefined }}
              className="flex items-center gap-3 px-4 py-2.5 rounded-lg hover:bg-neutral-100 dark:hover:bg-white/5 transition-colors"
              activeProps={{ className: 'bg-neutral-100 dark:bg-white/10' }}
            >
              <Music className="w-5 h-5" />
              <span>Music</span>
            </Link>

            <div className="mt-8 pt-4 border-t border-neutral-200/50 dark:border-white/5">
              <p className="px-4 text-xs text-neutral-500 dark:text-neutral-400 uppercase tracking-wider font-medium mb-2">
                Settings
              </p>
              <Link
                to="/settings/display"
                className="flex items-center gap-3 px-4 py-2.5 rounded-lg hover:bg-neutral-100 dark:hover:bg-white/5 transition-colors"
                activeProps={{ className: 'bg-neutral-100 dark:bg-white/10' }}
              >
                <Eye className="w-5 h-5" />
                <span>Display</span>
              </Link>
              <Link
                to="/settings/scheduler"
                className="flex items-center gap-3 px-4 py-2.5 rounded-lg hover:bg-neutral-100 dark:hover:bg-white/5 transition-colors"
                activeProps={{ className: 'bg-neutral-100 dark:bg-white/10' }}
              >
                <Clock className="w-5 h-5" />
                <span>Scheduler</span>
              </Link>
              <Link
                to="/settings/account"
                className="flex items-center gap-3 px-4 py-2.5 rounded-lg hover:bg-neutral-100 dark:hover:bg-white/5 transition-colors"
                activeProps={{ className: 'bg-neutral-100 dark:bg-white/10' }}
              >
                <KeyRound className="w-5 h-5" />
                <span>Account</span>
              </Link>
              {user?.is_admin && (
                <Link
                  to="/settings/users"
                  className="flex items-center gap-3 px-4 py-2.5 rounded-lg hover:bg-neutral-100 dark:hover:bg-white/5 transition-colors"
                  activeProps={{ className: 'bg-neutral-100 dark:bg-white/10' }}
                >
                  <Users className="w-5 h-5" />
                  <span>Users</span>
                </Link>
              )}
            </div>
          </nav>

          <div className="p-4 border-t border-neutral-200/50 dark:border-white/5">
            <div className="flex items-center justify-between mb-3">
              <div className="flex items-center gap-2 min-w-0">
                <User className="w-4 h-4 shrink-0 text-neutral-500 dark:text-neutral-400" />
                <span className="text-sm text-neutral-700 dark:text-neutral-300 truncate">
                  {user?.display_name || user?.username}
                </span>
              </div>
              <button
                onClick={handleLogout}
                className="p-1.5 rounded hover:bg-neutral-100 dark:hover:bg-white/5 transition-colors"
                title="Sign out"
              >
                <LogOut className="w-4 h-4 text-neutral-500 dark:text-neutral-400" />
              </button>
            </div>
            <div className="text-xs text-neutral-500 dark:text-neutral-400">
              <p>Version 0.0.1</p>
            </div>
          </div>
        </aside>

        {/* Main content */}
        <main className="flex-1 overflow-hidden">
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
