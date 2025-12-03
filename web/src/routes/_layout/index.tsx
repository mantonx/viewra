import { createFileRoute, Link } from '@tanstack/react-router'
import { useQuery } from '@tanstack/react-query'
import { getGetApiLibrariesQueryOptions } from '@/lib/api'
import { ContinueWatching } from '@/components/home'
import { Film, Tv, Music, Library, Play } from 'lucide-react'

const Index = () => {
  const { data: librariesData } = useQuery(getGetApiLibrariesQueryOptions())

  // Calculate library stats - handle union type from API response
  const libraries = (librariesData?.status === 200 && 'libraries' in librariesData.data)
    ? librariesData.data.libraries ?? []
    : []
  // Count libraries by type (item counts would need separate API calls)
  const movieCount = libraries.filter((l) => l.type === 'movie').length
  const tvCount = libraries.filter((l) => l.type === 'tv').length
  const musicCount = libraries.filter((l) => l.type === 'music').length

  return (
    <div className="h-full overflow-auto">
      <div className="page-enter">
        {/* Hero Section with cinematic gradient */}
        <div className="relative overflow-hidden">
          {/* Ambient glow background */}
          <div className="absolute inset-0 bg-linear-to-br from-primary-500/10 via-transparent to-purple-500/5 dark:from-primary-500/20 dark:via-transparent dark:to-purple-500/10" />
          <div className="absolute top-0 left-1/4 w-96 h-96 bg-primary-500/10 dark:bg-primary-500/20 rounded-full blur-3xl" />
          <div className="absolute bottom-0 right-1/4 w-64 h-64 bg-purple-500/5 dark:bg-purple-500/15 rounded-full blur-3xl" />

          <div className="relative p-8 pb-10">
            <div className="flex items-center gap-3 mb-4">
              <div className="p-2.5 rounded-xl bg-primary-500/10 dark:bg-primary-500/20">
                <Play className="w-6 h-6 text-primary-600 dark:text-primary-400" />
              </div>
              <span className="text-xs font-semibold uppercase tracking-wider text-primary-600 dark:text-primary-400">
                Media Server
              </span>
            </div>
            <h1 className="text-4xl sm:text-5xl font-semibold mb-4 text-neutral-900 dark:text-neutral-50 font-display tracking-tight">
              Welcome to ViewRA
            </h1>
            <p className="text-lg text-neutral-600 dark:text-neutral-400 leading-relaxed max-w-2xl">
              Your self-hosted media server for movies, TV shows, and music.
            </p>

            {/* Glass stat cards */}
            <div className="grid grid-cols-3 gap-4 mt-8 max-w-xl">
              <div className="glass-card rounded-xl p-4 text-center">
                <div className="text-2xl font-bold text-neutral-900 dark:text-white font-display">
                  {movieCount}
                </div>
                <div className="text-xs text-neutral-500 dark:text-neutral-400 uppercase tracking-wide mt-1">
                  Movies
                </div>
              </div>
              <div className="glass-card rounded-xl p-4 text-center">
                <div className="text-2xl font-bold text-neutral-900 dark:text-white font-display">
                  {tvCount}
                </div>
                <div className="text-xs text-neutral-500 dark:text-neutral-400 uppercase tracking-wide mt-1">
                  TV Shows
                </div>
              </div>
              <div className="glass-card rounded-xl p-4 text-center">
                <div className="text-2xl font-bold text-neutral-900 dark:text-white font-display">
                  {musicCount}
                </div>
                <div className="text-xs text-neutral-500 dark:text-neutral-400 uppercase tracking-wide mt-1">
                  Albums
                </div>
              </div>
            </div>
          </div>
        </div>

        {/* Content below hero */}
        <div className="p-8 pt-6 space-y-10">
          {/* Continue Watching Section */}
          <ContinueWatching />

          {/* Quick Actions */}
          <section>
            <h2 className="text-xl font-semibold mb-5 text-neutral-900 dark:text-neutral-50 font-display tracking-tight">
              Quick Actions
            </h2>
            <div className="grid grid-cols-2 sm:grid-cols-4 gap-4">
              <Link
                to="/libraries"
                className="flex flex-col items-center gap-3 p-5 rounded-xl bg-primary-600 text-white hover:bg-primary-500 hover:shadow-lg hover:shadow-primary-500/20 transition-all duration-200 ease-out hover:-translate-y-0.5"
              >
                <Library className="w-6 h-6" />
                <span className="text-sm font-medium">Libraries</span>
              </Link>
              <Link
                to="/movies"
                search={{ id: undefined, t: undefined, q: undefined, sort: undefined, genres: undefined, yearMin: undefined, yearMax: undefined, qualities: undefined, watched: undefined, view: undefined }}
                className="flex flex-col items-center gap-3 p-5 rounded-xl bg-neutral-100 dark:bg-white/10 text-neutral-700 dark:text-neutral-100 hover:bg-neutral-200 dark:hover:bg-white/15 transition-all duration-200 ease-out hover:-translate-y-0.5"
              >
                <Film className="w-6 h-6" />
                <span className="text-sm font-medium">Movies</span>
              </Link>
              <Link
                to="/tv"
                search={{ q: undefined, sort: undefined, genres: undefined, yearMin: undefined, yearMax: undefined, watched: undefined, view: undefined }}
                className="flex flex-col items-center gap-3 p-5 rounded-xl bg-neutral-100 dark:bg-white/10 text-neutral-700 dark:text-neutral-100 hover:bg-neutral-200 dark:hover:bg-white/15 transition-all duration-200 ease-out hover:-translate-y-0.5"
              >
                <Tv className="w-6 h-6" />
                <span className="text-sm font-medium">TV Shows</span>
              </Link>
              <Link
                to="/music"
                search={{ q: undefined, sort: undefined, view: undefined }}
                className="flex flex-col items-center gap-3 p-5 rounded-xl bg-neutral-100 dark:bg-white/10 text-neutral-700 dark:text-neutral-100 hover:bg-neutral-200 dark:hover:bg-white/15 transition-all duration-200 ease-out hover:-translate-y-0.5"
              >
                <Music className="w-6 h-6" />
                <span className="text-sm font-medium">Music</span>
              </Link>
            </div>
          </section>
        </div>
      </div>
    </div>
  )
}

export const Route = createFileRoute('/_layout/')({
  component: Index,
})
