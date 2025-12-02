import { createFileRoute, Link } from '@tanstack/react-router'
import { ContinueWatching } from '@/components/home'

const Index = () => {
  return (
    <div className="p-8">
      <h1 className="text-4xl font-bold mb-4 text-neutral-900 dark:text-neutral-50">Welcome to ViewRA</h1>
      <p className="text-neutral-600 dark:text-neutral-400 mb-6">
        Your self-hosted media server for movies, TV shows, and music.
      </p>

      {/* Continue Watching Section */}
      <ContinueWatching />

      {/* Quick Actions */}
      <section className="mt-8">
        <h2 className="text-2xl font-bold mb-4 text-neutral-900 dark:text-neutral-50">Quick Actions</h2>
        <div className="flex gap-4">
          <Link
            to="/libraries"
            className="px-4 py-2 bg-blue-600 text-white rounded hover:bg-blue-700 transition-colors"
          >
            Manage Libraries
          </Link>
          <Link
            to="/movies"
            search={{ id: undefined, t: undefined, q: undefined, sort: undefined, genres: undefined, yearMin: undefined, yearMax: undefined, qualities: undefined, watched: undefined, view: undefined }}
            className="px-4 py-2 bg-neutral-600 dark:bg-neutral-700 text-white rounded hover:bg-neutral-700 dark:hover:bg-neutral-600 transition-colors"
          >
            Browse Movies
          </Link>
          <Link
            to="/tv"
            search={{ q: undefined, sort: undefined, genres: undefined, yearMin: undefined, yearMax: undefined, watched: undefined, view: undefined }}
            className="px-4 py-2 bg-neutral-600 dark:bg-neutral-700 text-white rounded hover:bg-neutral-700 dark:hover:bg-neutral-600 transition-colors"
          >
            Browse TV Shows
          </Link>
          <Link
            to="/music"
            search={{ q: undefined, sort: undefined, view: undefined }}
            className="px-4 py-2 bg-neutral-600 dark:bg-neutral-700 text-white rounded hover:bg-neutral-700 dark:hover:bg-neutral-600 transition-colors"
          >
            Browse Music
          </Link>
        </div>
      </section>
    </div>
  )
}

export const Route = createFileRoute('/_layout/')({
  component: Index,
})
