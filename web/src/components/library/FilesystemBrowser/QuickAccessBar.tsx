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
        <span className="text-xs font-medium text-gray-500" id="quick-access-label">
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
          <span className="text-xs font-medium text-gray-500">Recent:</span>
          {recentPaths.map((path) => (
            <Button
              key={path}
              variant="secondary"
              size="sm"
              onClick={() => onNavigate(path)}
              disabled={isLoading}
              title={path}
              className="max-w-[200px] overflow-hidden text-ellipsis whitespace-nowrap"
            >
              {path}
            </Button>
          ))}
          <button
            onClick={onClearRecent}
            className="text-xs text-gray-500 hover:text-gray-700 underline ml-1"
            title="Clear recent paths"
          >
            Clear
          </button>
        </div>
      )}
    </div>
  )
}

export { QuickAccessBar }
