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
        className="flex-1 flex items-center gap-1 px-3 py-2 bg-gray-50 rounded-md text-sm overflow-x-auto"
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
                {!isFirst && <span className="text-gray-400">/</span>}
                {isLast ? (
                  <span className="text-gray-700 font-medium">{segment}</span>
                ) : (
                  <button
                    onClick={() => onNavigate(segmentPath)}
                    disabled={isLoading}
                    className="text-blue-600 hover:text-blue-800 hover:underline disabled:text-gray-400 disabled:no-underline disabled:cursor-not-allowed font-medium"
                    title={`Navigate to ${segmentPath}`}
                  >
                    {segment}
                  </button>
                )}
              </div>
            )
          })
        ) : (
          <span className="text-gray-700 font-medium">/</span>
        )}
      </div>
    </nav>
  )
}

export { BreadcrumbNav }
