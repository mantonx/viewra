import { Button } from '@/components/ui/Button/Button'
import type { BreadcrumbNavProps } from './BreadcrumbNav.types'

const BreadcrumbNav = ({ currentPath, onNavigate, onNavigateUp, canNavigateUp, isLoading }: BreadcrumbNavProps) => {
  const pathSegments = currentPath?.split('/').filter(Boolean) || []

  return (
    <nav className="mb-4 flex items-center gap-2" aria-label="Breadcrumb navigation">
      <Button
        variant="secondary"
        size="sm"
        onClick={onNavigateUp}
        disabled={!canNavigateUp || isLoading}
        aria-label="Navigate to parent directory"
        title="Go up one level (Backspace)"
      >
        ↑ Up
      </Button>
      <div
        className="flex-1 flex items-center gap-1 px-3 py-2 bg-neutral-50 dark:bg-neutral-950 rounded-md text-sm overflow-x-auto"
        role="navigation"
        aria-label="Current path"
        title={currentPath || '/'}
      >
        {pathSegments.length > 0 ? (
          pathSegments.map((segment, index, array) => {
            const segmentPath = `/${array.slice(0, index + 1).join('/')}`
            const isLast = index === array.length - 1
            const isFirst = index === 0

            return (
              <div key={segmentPath} className="flex items-center gap-1 shrink-0">
                {!isFirst && <span className="text-neutral-400 dark:text-neutral-600">/</span>}
                {isLast ? (
                  <span className="text-neutral-700 dark:text-neutral-300 font-medium">{segment}</span>
                ) : (
                  <button
                    onClick={() => onNavigate(segmentPath)}
                    disabled={isLoading}
                    className="text-blue-600 dark:text-blue-400 hover:text-blue-800 dark:hover:text-blue-300 hover:underline disabled:text-neutral-400 dark:disabled:text-neutral-600 disabled:no-underline disabled:cursor-not-allowed font-medium"
                    title={`Navigate to ${segmentPath}`}
                  >
                    {segment}
                  </button>
                )}
              </div>
            )
          })
        ) : (
          <span className="text-neutral-700 dark:text-neutral-300 font-medium">/</span>
        )}
      </div>
    </nav>
  )
}

export { BreadcrumbNav }
