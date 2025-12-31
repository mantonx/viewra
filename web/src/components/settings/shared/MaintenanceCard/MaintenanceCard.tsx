import { useState } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { Card, CardHeader, CardContent, Button, Modal, ModalContent, ModalFooter } from '@/components/ui'
import { useToast } from '@/lib/hooks/useToast'
import {
  useGetApiAdminSystemMaintenance,
  usePostApiAdminSystemMaintenance,
  getGetApiAdminSystemMaintenanceQueryKey,
} from '@/lib/api/generated/system/system'
import { text } from '@/styles/semantic'
import { cn } from '@/lib/utils'
import { Wrench } from 'lucide-react'

/**
 * Card for managing maintenance mode.
 * Allows admins to enable/disable maintenance mode with an optional message.
 */
const MaintenanceCard = () => {
  const toast = useToast()
  const queryClient = useQueryClient()
  const [showEnableModal, setShowEnableModal] = useState(false)
  const [showDisableConfirm, setShowDisableConfirm] = useState(false)
  const [reason, setReason] = useState('')
  const [estimatedMinutes, setEstimatedMinutes] = useState('')

  const { data: maintenanceData } = useGetApiAdminSystemMaintenance()
  const maintenanceMutation = usePostApiAdminSystemMaintenance()

  const state = maintenanceData?.status === 200 ? maintenanceData.data : null
  const isEnabled = state?.enabled ?? false

  const invalidateMaintenanceQuery = () => {
    queryClient.invalidateQueries({ queryKey: getGetApiAdminSystemMaintenanceQueryKey() })
  }

  const handleEnable = async () => {
    try {
      const estimatedDuration = estimatedMinutes ? `${estimatedMinutes}m` : undefined
      await maintenanceMutation.mutateAsync({
        data: {
          enabled: true,
          reason: reason || 'Scheduled maintenance',
          estimatedDuration,
        },
      })
      invalidateMaintenanceQuery()
      toast.success('Maintenance mode enabled')
      setShowEnableModal(false)
      setReason('')
      setEstimatedMinutes('')
    } catch {
      toast.error('Failed to enable maintenance mode')
    }
  }

  const handleDisable = async () => {
    try {
      await maintenanceMutation.mutateAsync({
        data: {
          enabled: false,
        },
      })
      invalidateMaintenanceQuery()
      toast.success('Maintenance mode disabled')
      setShowDisableConfirm(false)
    } catch {
      toast.error('Failed to disable maintenance mode')
    }
  }

  const formatTime = (isoString?: string) => {
    if (!isoString) {
      return null
    }
    try {
      return new Date(isoString).toLocaleString()
    } catch {
      return null
    }
  }

  return (
    <>
      <Card variant="glass">
        <CardHeader className="border-b border-neutral-100 dark:border-neutral-800">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-3">
              <div
                className={cn(
                  'p-2 rounded-lg',
                  isEnabled
                    ? 'bg-amber-100 dark:bg-amber-900/50 text-amber-600 dark:text-amber-400'
                    : 'bg-neutral-100 dark:bg-neutral-800 text-neutral-600 dark:text-neutral-400'
                )}
              >
                <Wrench className="w-5 h-5" />
              </div>
              <div>
                <h2 className={cn('text-lg font-semibold', text.primary)}>Maintenance Mode</h2>
                <p className={cn('text-sm mt-0.5', text.secondary)}>
                  Block user access during maintenance operations
                </p>
              </div>
            </div>
            {isEnabled && (
              <div
                className={cn(
                  'flex items-center gap-1.5 px-2 py-1 rounded text-xs font-medium',
                  'bg-amber-100 dark:bg-amber-900/50 text-amber-700 dark:text-amber-400'
                )}
              >
                <span className="w-2 h-2 rounded-full bg-amber-500 animate-pulse" />
                Active
              </div>
            )}
          </div>
        </CardHeader>

        <CardContent className="py-4">
          {isEnabled && state && (
            <div className="mb-4 p-3 rounded-lg bg-amber-50 dark:bg-amber-900/20 border border-amber-200 dark:border-amber-700/50">
              <div className="space-y-1">
                {state.reason && (
                  <p className={cn('text-sm', text.primary)}>
                    <span className="font-medium">Reason:</span> {state.reason}
                  </p>
                )}
                {state.startedAt && (
                  <p className={cn('text-xs', text.secondary)}>
                    Started: {formatTime(state.startedAt)}
                  </p>
                )}
                {state.estimatedEnd && (
                  <p className={cn('text-xs', text.secondary)}>
                    Estimated end: {formatTime(state.estimatedEnd)}
                  </p>
                )}
              </div>
            </div>
          )}

          <div className="flex items-center justify-between">
            <div className="flex-1 mr-4">
              <p className={cn('text-sm', text.secondary)}>
                {isEnabled
                  ? 'Users cannot access the service while maintenance mode is active.'
                  : 'Enable maintenance mode to block user access during maintenance.'}
              </p>
            </div>
            {isEnabled ? (
              showDisableConfirm ? (
                <div className="flex gap-2 shrink-0">
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={() => setShowDisableConfirm(false)}
                    disabled={maintenanceMutation.isPending}
                  >
                    Cancel
                  </Button>
                  <Button
                    variant="primary"
                    size="sm"
                    onClick={handleDisable}
                    isLoading={maintenanceMutation.isPending}
                  >
                    Confirm
                  </Button>
                </div>
              ) : (
                <Button
                  variant="secondary"
                  size="sm"
                  onClick={() => setShowDisableConfirm(true)}
                  className="shrink-0"
                >
                  <Wrench className="w-4 h-4 mr-1.5" />
                  End Maintenance
                </Button>
              )
            ) : (
              <Button
                variant="secondary"
                size="sm"
                onClick={() => setShowEnableModal(true)}
                className="shrink-0"
              >
                <Wrench className="w-4 h-4 mr-1.5" />
                Enable Maintenance
              </Button>
            )}
          </div>
        </CardContent>
      </Card>

      {/* Enable Maintenance Modal */}
      <Modal
        isOpen={showEnableModal}
        onClose={() => setShowEnableModal(false)}
        title="Enable Maintenance Mode"
        size="sm"
      >
        <ModalContent>
          <div className="space-y-4">
            <p className={cn('text-sm', text.secondary)}>
              Enabling maintenance mode will block all user requests except health checks and admin
              endpoints. Users will see a maintenance message.
            </p>

            <div>
              <label htmlFor="reason" className={cn('block text-sm font-medium mb-1.5', text.primary)}>
                Reason (optional)
              </label>
              <input
                id="reason"
                type="text"
                value={reason}
                onChange={(e) => setReason(e.target.value)}
                placeholder="e.g., Database migration in progress"
                className={cn(
                  'w-full px-3 py-2 rounded-lg text-sm',
                  'bg-neutral-50 dark:bg-neutral-900',
                  'border border-neutral-200 dark:border-neutral-700',
                  'focus:outline-none focus:ring-2 focus:ring-primary-500/30',
                  text.primary
                )}
              />
            </div>

            <div>
              <label
                htmlFor="duration"
                className={cn('block text-sm font-medium mb-1.5', text.primary)}
              >
                Estimated duration in minutes (optional)
              </label>
              <input
                id="duration"
                type="number"
                min="1"
                value={estimatedMinutes}
                onChange={(e) => setEstimatedMinutes(e.target.value)}
                placeholder="e.g., 30"
                className={cn(
                  'w-full px-3 py-2 rounded-lg text-sm',
                  'bg-neutral-50 dark:bg-neutral-900',
                  'border border-neutral-200 dark:border-neutral-700',
                  'focus:outline-none focus:ring-2 focus:ring-primary-500/30',
                  text.primary
                )}
              />
            </div>
          </div>
        </ModalContent>
        <ModalFooter>
          <Button
            variant="ghost"
            onClick={() => setShowEnableModal(false)}
            disabled={maintenanceMutation.isPending}
          >
            Cancel
          </Button>
          <Button
            variant="primary"
            onClick={handleEnable}
            isLoading={maintenanceMutation.isPending}
          >
            Enable Maintenance Mode
          </Button>
        </ModalFooter>
      </Modal>
    </>
  )
}

export { MaintenanceCard }
