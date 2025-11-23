import { Button } from '@/components/ui/Button/Button'
import type { QuickAccessBarProps } from './QuickAccessBar.types'

const QUICK_ACCESS_PATHS = [
  { path: '/home/fictional', label: 'Home', ariaLabel: 'Navigate to home directory' },
  { path: '/media', label: '/media', ariaLabel: 'Navigate to /media directory' },
  { path: '/mnt', label: '/mnt', ariaLabel: 'Navigate to /mnt directory' },
  { path: '/cifs', label: '/cifs', ariaLabel: 'Navigate to /cifs directory' },
]

const QuickAccessBar = ({ onNavigate, recentPaths, onClearRecent, isLoading }: QuickAccessBarProps) => {
  return (
    <div className="mb-3 flex flex-col gap-2" role="region" aria-label="Quick access and recent paths">
      {/* Quick Access */}
      <div className="flex gap-2 flex-wrap items-center">
        <span className="text-xs font-medium text-neutral-500 dark:text-neutral-500" id="quick-access-label">
          Quick Access:
        </span>
        {QUICK_ACCESS_PATHS.map(({ path, label, ariaLabel }) => (
          <Button
            key={path}
            variant="secondary"
            size="sm"
            onClick={() => onNavigate(path)}
            disabled={isLoading}
            aria-label={ariaLabel}
          >
            {label}
          </Button>
        ))}
      </div>

      {/* Recent Paths */}
      {recentPaths.length > 0 && (
        <div className="flex gap-2 flex-wrap items-center">
          <span className="text-xs font-medium text-neutral-500 dark:text-neutral-500">Recent:</span>
          {recentPaths.map((path) => {
            // Show last 2-3 segments for better readability
            const segments = path.split('/').filter(Boolean)
            const displayPath = segments.length > 3
              ? `.../${segments.slice(-3).join('/')}`
              : path

            return (
              <Button
                key={path}
                variant="secondary"
                size="sm"
                onClick={() => onNavigate(path)}
                disabled={isLoading}
                title={`Navigate to ${path}`}
                className="max-w-[280px] overflow-hidden text-ellipsis whitespace-nowrap"
              >
                {displayPath}
              </Button>
            )
          })}
          <button
            onClick={onClearRecent}
            className="text-xs text-neutral-500 dark:text-neutral-500 hover:text-red-600 dark:hover:text-red-400 hover:underline ml-1 px-2 py-1 rounded transition-colors"
            title="Clear all recent paths"
            aria-label="Clear all recent paths"
          >
            Clear
          </button>
        </div>
      )}
    </div>
  )
}

export { QuickAccessBar }
