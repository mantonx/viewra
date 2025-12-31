import { useState } from 'react'
import { Button } from '@/components/ui'
import { cn } from '@/lib/utils'
import { text } from '@/styles/semantic'
import { useToast } from '@/lib/hooks/useToast'
import { useServerStatus } from '@/contexts'
import {
  CheckCircle,
  XCircle,
  Loader2,
  Clock,
  RefreshCw,
  Database,
  Shield,
  Server,
  FileCheck,
  Copy,
  Settings,
  RotateCcw,
} from 'lucide-react'
import {
  useGetApiAdminSystemDatabaseMigrate,
  usePostApiAdminSystemRestartNow,
} from '@/lib/api/generated/system/system'

type Props = {
  migrationId?: string | null
  onComplete: () => void
  onFailed: () => void
  onForceClose: () => void
}

const phaseIcons: Record<string, React.ReactNode> = {
  maintenance_mode: <Shield className="w-4 h-4" />,
  backup: <Database className="w-4 h-4" />,
  connect_target: <Server className="w-4 h-4" />,
  create_schema: <FileCheck className="w-4 h-4" />,
  copying: <Copy className="w-4 h-4" />,
  verification: <CheckCircle className="w-4 h-4" />,
  update_config: <Settings className="w-4 h-4" />,
}

const phaseLabels: Record<string, string> = {
  maintenance_mode: 'Enable Maintenance Mode',
  backup: 'Create Backup',
  connect_target: 'Connect to Target',
  create_schema: 'Create Schema',
  copying: 'Copy Data',
  verification: 'Verify Migration',
  update_config: 'Update Configuration',
}

export const StepProgress = ({ onComplete, onFailed, onForceClose }: Props) => {
  const toast = useToast()
  const { setRestarting } = useServerStatus()
  const [showRestartConfirm, setShowRestartConfirm] = useState(false)
  const restartMutation = usePostApiAdminSystemRestartNow()

  const { data, refetch } = useGetApiAdminSystemDatabaseMigrate({
    query: {
      refetchInterval: (query) => {
        const state = query.state.data
        if (state?.status === 200) {
          const migrationStatus = state.data.status
          if (migrationStatus === 'completed' || migrationStatus === 'failed') {
            return false // Stop polling
          }
        }
        return 2000 // Poll every 2 seconds
      },
    },
  })

  const state = data?.status === 200 ? data.data : null

  // Note: We intentionally don't auto-close on completion or failure
  // so users can see the result and choose to restart the server

  const handleRestart = async () => {
    try {
      setRestarting() // Trigger full-screen overlay
      await restartMutation.mutateAsync()
      onComplete() // Close the modal
    } catch {
      toast.error('Failed to restart server')
      setShowRestartConfirm(false)
    }
  }

  const progress = state?.progress
  const phases = state?.phases ?? []
  const error = state?.error
  const result = state?.result

  const getPhaseStatus = (phaseName: string) => {
    const phase = phases.find((p) => p.name === phaseName)
    return phase?.status ?? 'pending'
  }

  const renderPhaseIcon = (status: string) => {
    switch (status) {
      case 'completed':
        return <CheckCircle className="w-4 h-4 text-emerald-500" />
      case 'in_progress':
        return <Loader2 className="w-4 h-4 text-primary-500 animate-spin" />
      case 'failed':
        return <XCircle className="w-4 h-4 text-red-500" />
      default:
        return <Clock className="w-4 h-4 text-neutral-400" />
    }
  }

  return (
    <div className="space-y-6">
      <div>
        <h3 className={cn('text-base font-semibold mb-1', text.primary)}>
          {state?.status === 'completed'
            ? 'Migration Complete'
            : state?.status === 'failed'
              ? 'Migration Failed'
              : 'Migration in Progress'}
        </h3>
        <p className={cn('text-sm', text.secondary)}>
          {state?.status === 'completed'
            ? 'Your data has been successfully migrated'
            : state?.status === 'failed'
              ? 'An error occurred during migration'
              : 'Please wait while your data is being migrated'}
        </p>
      </div>

      {/* Progress bar */}
      {state?.status === 'in_progress' && progress && (
        <div className="space-y-2">
          <div className="flex justify-between text-sm">
            <span className={text.secondary}>
              {progress.currentTable
                ? `Copying ${progress.currentTable}...`
                : 'Processing...'}
            </span>
            <span className={text.primary}>{progress.percentComplete ?? 0}%</span>
          </div>
          <div className="h-2 bg-neutral-200 dark:bg-neutral-700 rounded-full overflow-hidden">
            <div
              className="h-full bg-primary-500 transition-all duration-300"
              style={{ width: `${progress.percentComplete ?? 0}%` }}
            />
          </div>
          <div className="flex justify-between text-xs">
            <span className={text.tertiary}>
              {(progress.tablesCompleted ?? 0).toLocaleString()} /{' '}
              {(progress.tablesTotal ?? 0).toLocaleString()} tables
            </span>
            <span className={text.tertiary}>
              {(progress.rowsCopied ?? 0).toLocaleString()} /{' '}
              {(progress.rowsTotal ?? 0).toLocaleString()} rows
            </span>
          </div>
        </div>
      )}

      {/* Phase list */}
      <div className="space-y-2">
        {Object.keys(phaseLabels).map((phaseName) => {
          const status = getPhaseStatus(phaseName)
          return (
            <div
              key={phaseName}
              className={cn(
                'flex items-center gap-3 p-3 rounded-lg transition-colors',
                status === 'in_progress' && 'bg-primary-50 dark:bg-primary-500/10',
                status === 'completed' && 'bg-emerald-50 dark:bg-emerald-500/10',
                status === 'failed' && 'bg-red-50 dark:bg-red-500/10'
              )}
            >
              <div className={cn('p-1.5 rounded', text.tertiary)}>
                {phaseIcons[phaseName]}
              </div>
              <span
                className={cn(
                  'flex-1 text-sm',
                  status === 'in_progress' && 'font-medium text-primary-700 dark:text-primary-400',
                  status === 'completed' && 'text-emerald-700 dark:text-emerald-400',
                  status === 'failed' && 'text-red-700 dark:text-red-400',
                  status === 'pending' && text.tertiary
                )}
              >
                {phaseLabels[phaseName]}
              </span>
              {renderPhaseIcon(status)}
            </div>
          )
        })}
      </div>

      {/* Error display */}
      {error && (
        <div
          className={cn(
            'p-4 rounded-xl',
            'bg-red-50 dark:bg-red-500/10',
            'border border-red-200 dark:border-red-500/20'
          )}
        >
          <p className="text-sm font-medium text-red-700 dark:text-red-400">Error Details</p>
          <p className="text-sm mt-1 text-red-600 dark:text-red-500">{error.message}</p>
          {error.table && (
            <p className="text-xs mt-2 text-red-500 dark:text-red-600">Table: {error.table}</p>
          )}
        </div>
      )}

      {/* Success result */}
      {result && state?.status === 'completed' && (
        <div
          className={cn(
            'p-4 rounded-xl',
            'bg-emerald-50 dark:bg-emerald-500/10',
            'border border-emerald-200 dark:border-emerald-500/20'
          )}
        >
          <p className="text-sm font-medium text-emerald-700 dark:text-emerald-400">
            Migration Summary
          </p>
          <div className="mt-2 grid grid-cols-2 gap-2 text-sm text-emerald-600 dark:text-emerald-500">
            <div>Tables migrated: {result.tablesMigrated}</div>
            <div>Rows migrated: {(result.rowsMigrated ?? 0).toLocaleString()}</div>
            <div>Duration: {result.durationSeconds}s</div>
            <div>Verification: {result.verificationPassed ? 'Passed' : 'Failed'}</div>
          </div>
        </div>
      )}

      {/* Restart prompt */}
      {result?.requiresRestart && state?.status === 'completed' && (
        <div
          className={cn(
            'p-4 rounded-xl',
            'bg-amber-50 dark:bg-amber-500/10',
            'border border-amber-200 dark:border-amber-500/20'
          )}
        >
          <div className="flex items-start gap-3">
            <RotateCcw className="w-5 h-5 text-amber-600 dark:text-amber-400 mt-0.5 shrink-0" />
            <div className="flex-1">
              <p className="text-sm font-medium text-amber-700 dark:text-amber-400">
                Server restart required
              </p>
              <p className="text-sm mt-1 text-amber-600 dark:text-amber-500">
                {showRestartConfirm
                  ? 'Active streams will be interrupted. The server will be back online in a few seconds.'
                  : 'Restart the server to start using the new database.'}
              </p>
            </div>
          </div>
          <div className="mt-3 flex justify-end gap-2">
            {showRestartConfirm ? (
              <>
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={() => setShowRestartConfirm(false)}
                  disabled={restartMutation.isPending}
                >
                  Cancel
                </Button>
                <Button
                  variant="primary"
                  size="sm"
                  onClick={handleRestart}
                  isLoading={restartMutation.isPending}
                >
                  <RotateCcw className="w-4 h-4 mr-1.5" />
                  Confirm Restart
                </Button>
              </>
            ) : (
              <Button variant="primary" size="sm" onClick={() => setShowRestartConfirm(true)}>
                <RotateCcw className="w-4 h-4 mr-1.5" />
                Restart Now
              </Button>
            )}
          </div>
        </div>
      )}

      {/* Actions */}
      <div className="flex justify-between pt-4">
        <div>
          {state?.status === 'in_progress' && (
            <Button variant="ghost" size="sm" onClick={onForceClose}>
              Force Close
            </Button>
          )}
        </div>
        <div className="flex gap-3">
          {state?.status === 'in_progress' && (
            <Button variant="ghost" onClick={() => refetch()}>
              <RefreshCw className="w-4 h-4 mr-2" />
              Refresh
            </Button>
          )}
          {state?.status === 'completed' && <Button onClick={onComplete}>Close</Button>}
          {state?.status === 'failed' && <Button onClick={onFailed}>Close</Button>}
        </div>
      </div>
    </div>
  )
}
