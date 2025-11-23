import type { DirectorySearchProps } from './DirectorySearch.types'

const DirectorySearch = ({ searchQuery, onSearchChange, resultsCount, totalCount }: DirectorySearchProps) => {
  return (
    <div className="mb-4" role="search">
      <div className="relative">
        <label htmlFor="directory-search" className="sr-only">
          Filter directories
        </label>
        <input
          id="directory-search"
          type="text"
          value={searchQuery}
          onChange={(e) => onSearchChange(e.target.value)}
          placeholder="Filter directories..."
          className="w-full px-3 py-2 pl-10 border border-neutral-300 dark:border-neutral-700 rounded-md text-sm bg-white dark:bg-neutral-900 text-neutral-900 dark:text-neutral-50 placeholder:text-neutral-500 dark:placeholder:text-neutral-500 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent"
          aria-label="Filter directories by name"
        />
        <svg
          className="absolute left-3 top-2.5 h-5 w-5 text-neutral-400 dark:text-neutral-600"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
          aria-hidden="true"
        >
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            strokeWidth={2}
            d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"
          />
        </svg>
        {searchQuery && (
          <button
            onClick={() => onSearchChange('')}
            className="absolute right-3 top-2.5 text-neutral-400 dark:text-neutral-600 hover:text-neutral-600 dark:hover:text-neutral-400"
            aria-label="Clear search"
          >
            <svg className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M6 18L18 6M6 6l12 12"
              />
            </svg>
          </button>
        )}
      </div>
      {searchQuery && (
        <p className="mt-1 text-xs text-neutral-500 dark:text-neutral-500">
          Showing {resultsCount} of {totalCount} directories
        </p>
      )}
    </div>
  )
}

export { DirectorySearch }
