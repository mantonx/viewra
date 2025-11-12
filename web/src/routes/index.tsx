import { createFileRoute, Link } from '@tanstack/react-router'

const Index = () => {
  return (
    <div className="p-8">
      <h1 className="text-4xl font-bold mb-4">Welcome to ViewRA</h1>
      <p className="text-gray-600 mb-4">
        Your self-hosted media server for movies, TV shows, and music.
      </p>
      <div className="flex gap-4">
        <Link
          to="/libraries"
          className="px-4 py-2 bg-blue-600 text-white rounded hover:bg-blue-700"
        >
          Manage Libraries
        </Link>
        <Link to="/media" className="px-4 py-2 bg-gray-600 text-white rounded hover:bg-gray-700">
          Browse Media
        </Link>
      </div>
    </div>
  )
}

export const Route = createFileRoute('/')({
  component: Index,
})
