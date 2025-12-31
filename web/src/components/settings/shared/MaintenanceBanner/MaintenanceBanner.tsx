import { cn } from '@/lib/utils'
import { Wrench } from 'lucide-react'
import { useGetApiAdminSystemMaintenance } from '@/lib/api/generated/system/system'

/**
 * Banner displayed when the server is in maintenance mode.
 * Shows the reason and estimated end time if available.
 * Uses sticky positioning to stay at top while scrolling.
 */
const MaintenanceBanner = () => {
  const { data: maintenanceData, isError } = useGetApiAdminSystemMaintenance({
    query: {
      refetchInterval: 10000, // Poll every 10 seconds
      staleTime: 5000,
      retry: false, // Don't retry on auth errors
    },
  })

  // Only show for successful responses where maintenance is enabled
  // If the request errored (e.g., 401/403 for non-admins), don't show banner
  const state = !isError && maintenanceData?.status === 200 ? maintenanceData.data : null

  // Don't show if not in maintenance mode or if request failed
  if (!state?.enabled) {
    return null
  }

  const formatEstimatedEnd = (isoString?: string) => {
    if (!isoString) {
      return null
    }
    try {
      const date = new Date(isoString)
      return date.toLocaleTimeString(undefined, {
        hour: '2-digit',
        minute: '2-digit',
      })
    } catch {
      return null
    }
  }

  const estimatedEnd = formatEstimatedEnd(state.estimatedEnd)

  return (
    <div
      className={cn(
        'sticky top-0 left-0 right-0 z-50',
        'bg-amber-500 text-white',
        'py-2 px-4',
        'shadow-lg'
      )}
    >
      <div className="flex items-center justify-center gap-3">
        <Wrench className="w-5 h-5 animate-pulse" />
        <div className="text-center">
          <span className="font-medium">Maintenance Mode Active</span>
          {state.reason && <span className="mx-2">-</span>}
          {state.reason && <span>{state.reason}</span>}
          {estimatedEnd && (
            <span className="ml-2 text-amber-100">
              (Expected completion: {estimatedEnd})
            </span>
          )}
        </div>
      </div>
    </div>
  )
}

export { MaintenanceBanner }
