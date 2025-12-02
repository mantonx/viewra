import { formatDate } from '@/lib/utils'
import { Skeleton } from '@/components/ui/Skeleton/Skeleton'
import { Button } from '@/components/ui/Button/Button'
import type { DirectoryTableProps } from './DirectoryTable.types'

const DirectoryTable = ({
  directories,
  isLoading,
  sortBy,
  sortOrder,
  onSort,
  onNavigate,
  onSelectCurrent,
  selectedIndex,
  onSelectIndex,
  canNavigateUp,
  onNavigateUp,
}: DirectoryTableProps) => {
  // Loading skeleton
  if (isLoading) {
    return (
      <div className="flex-1 overflow-y-auto border border-neutral-200/50 dark:border-white/10 rounded-xl bg-white/50 dark:bg-white/5 backdrop-blur-sm">
        <table className="min-w-full divide-y divide-neutral-200/50 dark:divide-white/5">
          <thead className="bg-neutral-50/80 dark:bg-white/5 sticky top-0 backdrop-blur-sm">
            <tr>
              <th className="px-4 py-3 text-left text-xs font-medium text-neutral-500 dark:text-neutral-400 uppercase tracking-wider">
                Name
              </th>
              <th className="px-4 py-3 text-left text-xs font-medium text-neutral-500 dark:text-neutral-400 uppercase tracking-wider">
                Modified
              </th>
            </tr>
          </thead>
          <tbody className="divide-y divide-neutral-200/50 dark:divide-white/5">
            {[...Array(8)].map((_, i) => (
              <tr key={i}>
                <td className="px-4 py-3 whitespace-nowrap">
                  <div className="flex items-center">
                    <Skeleton className="h-5 w-5 mr-2 rounded" />
                    <Skeleton className="h-4 w-48 rounded" />
                  </div>
                </td>
                <td className="px-4 py-3 whitespace-nowrap">
                  <Skeleton className="h-4 w-32 rounded" />
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    )
  }

  // Empty state
  if (directories.length === 0) {
    return (
      <div className="flex-1 overflow-y-auto border border-neutral-200/50 dark:border-white/10 rounded-xl bg-white/50 dark:bg-white/5 backdrop-blur-sm">
        <div className="p-8 text-center">
          <svg
            className="mx-auto h-12 w-12 text-neutral-400 dark:text-neutral-500 mb-4"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
            aria-hidden="true"
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth={2}
              d="M3 7v10a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-6l-2-2H5a2 2 0 00-2 2z"
            />
          </svg>
          <p className="text-sm font-medium text-neutral-700 dark:text-neutral-300 mb-2">No subdirectories found</p>
          <p className="text-xs text-neutral-500 dark:text-neutral-400 mb-4">
            This folder has no subdirectories. You can still select this folder, or navigate up to choose a different location.
          </p>
          <div className="flex gap-2 justify-center">
            {canNavigateUp && (
              <Button variant="secondary" size="sm" onClick={onNavigateUp}>
                ← Go Up
              </Button>
            )}
            <Button variant="secondary" size="sm" onClick={onSelectCurrent}>
              Select This Folder
            </Button>
          </div>
        </div>
      </div>
    )
  }

  // Directory table
  return (
    <div className="flex-1 overflow-y-auto border border-neutral-200/50 dark:border-white/10 rounded-xl bg-white/50 dark:bg-white/5 backdrop-blur-sm">
      <table className="min-w-full divide-y divide-neutral-200/50 dark:divide-white/5" role="table" aria-label="Directory listing">
        <thead className="bg-neutral-50/80 dark:bg-white/5 sticky top-0 backdrop-blur-sm">
          <tr>
            <th className="px-4 py-3 text-left text-xs font-medium text-neutral-500 dark:text-neutral-400 uppercase tracking-wider" scope="col">
              <button
                onClick={() => onSort('name')}
                className="flex items-center gap-1.5 hover:text-neutral-700 dark:hover:text-neutral-200 transition-colors cursor-pointer"
                aria-label={`Sort by name ${sortBy === 'name' ? (sortOrder === 'asc' ? 'descending' : 'ascending') : 'ascending'}`}
                title={`Click to sort by name ${sortBy === 'name' ? (sortOrder === 'asc' ? 'descending' : 'ascending') : ''}`}
              >
                Name
                <span className={`text-xs ${sortBy === 'name' ? 'text-primary-600 dark:text-primary-400' : 'text-neutral-400 dark:text-neutral-500'}`} aria-hidden="true">
                  {sortBy === 'name' ? (sortOrder === 'asc' ? '↑' : '↓') : '⇅'}
                </span>
              </button>
            </th>
            <th className="px-4 py-3 text-left text-xs font-medium text-neutral-500 dark:text-neutral-400 uppercase tracking-wider" scope="col">
              <button
                onClick={() => onSort('modified')}
                className="flex items-center gap-1.5 hover:text-neutral-700 dark:hover:text-neutral-200 transition-colors cursor-pointer"
                aria-label={`Sort by modified date ${sortBy === 'modified' ? (sortOrder === 'asc' ? 'descending' : 'ascending') : 'ascending'}`}
                title={`Click to sort by date modified ${sortBy === 'modified' ? (sortOrder === 'asc' ? 'descending' : 'ascending') : ''}`}
              >
                Modified
                <span className={`text-xs ${sortBy === 'modified' ? 'text-primary-600 dark:text-primary-400' : 'text-neutral-400 dark:text-neutral-500'}`} aria-hidden="true">
                  {sortBy === 'modified' ? (sortOrder === 'asc' ? '↑' : '↓') : '⇅'}
                </span>
              </button>
            </th>
          </tr>
        </thead>
        <tbody className="divide-y divide-neutral-200/50 dark:divide-white/5">
          {directories.map((dir, index) => (
            <tr
              key={dir.path}
              onClick={() => dir.readable && onNavigate(dir.path || '')}
              onMouseEnter={() => onSelectIndex(index)}
              className={`
                ${dir.readable ? 'hover:bg-primary-50/50 dark:hover:bg-primary-500/10 cursor-pointer' : 'opacity-50 cursor-not-allowed'}
                ${selectedIndex === index ? 'bg-primary-50 dark:bg-primary-500/15 ring-2 ring-inset ring-primary-400/50 dark:ring-primary-500/30' : ''}
                transition-all duration-150
              `}
              role="button"
              tabIndex={dir.readable ? 0 : -1}
              aria-label={`${dir.name}${!dir.readable ? ' (not readable)' : ''}`}
              aria-disabled={!dir.readable}
              onKeyDown={(e) => {
                if ((e.key === 'Enter' || e.key === ' ') && dir.readable && dir.path) {
                  e.preventDefault()
                  onNavigate(dir.path)
                }
              }}
            >
              <td className="px-4 py-3 whitespace-nowrap">
                <div className="flex items-center">
                  <svg
                    className={`h-5 w-5 mr-2 ${dir.readable ? 'text-primary-500 dark:text-primary-400' : 'text-neutral-400 dark:text-neutral-600'}`}
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                    aria-hidden="true"
                  >
                    <path
                      strokeLinecap="round"
                      strokeLinejoin="round"
                      strokeWidth={2}
                      d="M3 7v10a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-6l-2-2H5a2 2 0 00-2 2z"
                    />
                  </svg>
                  <span className="text-sm font-medium text-neutral-900 dark:text-neutral-50">{dir.name}</span>
                </div>
              </td>
              <td className="px-4 py-3 whitespace-nowrap text-sm text-neutral-500 dark:text-neutral-400">
                {dir.modified_at ? formatDate(dir.modified_at) : '-'}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}

export { DirectoryTable }
