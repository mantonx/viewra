import { Button, Loading } from '@/components/ui'
import { cn } from '@/lib/utils'
import { text } from '@/styles/semantic'
import { AlertTriangle, Clock, Database, HardDrive, Table } from 'lucide-react'
import { usePostApiAdminSystemDatabaseEstimate } from '@/lib/api/generated/system/system'
import { useEffect } from 'react'
import type {
  GithubComMantonxViewraInternalApplicationSystemMigrationPostgresConfig as PostgresConfig,
  GithubComMantonxViewraInternalApplicationSystemMigrationSQLiteConfig as SQLiteConfig,
} from '@/lib/api/generated/models'
import type { DatabaseDriver } from '../types'

type Props = {
  sourceDriver: DatabaseDriver
  targetDriver: DatabaseDriver
  postgresConfig: PostgresConfig
  sqliteConfig: SQLiteConfig
  onBack: () => void
  onStart: () => void
  isStarting: boolean
}

const formatBytes = (bytes: number): string => {
  if (bytes < 1024) {
    return `${bytes} B`
  }
  if (bytes < 1024 * 1024) {
    return `${(bytes / 1024).toFixed(1)} KB`
  }
  if (bytes < 1024 * 1024 * 1024) {
    return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
  }
  return `${(bytes / (1024 * 1024 * 1024)).toFixed(2)} GB`
}

export const StepReview = ({
  sourceDriver,
  targetDriver,
  postgresConfig,
  sqliteConfig,
  onBack,
  onStart,
  isStarting,
}: Props) => {
  const estimate = usePostApiAdminSystemDatabaseEstimate()

  useEffect(() => {
    estimate.mutate({ data: { targetDriver } })
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [targetDriver])

  const estimateData = estimate.data?.status === 200 ? estimate.data.data : null

  const getTargetDescription = () => {
    if (targetDriver === 'postgres') {
      return `${postgresConfig.host}:${postgresConfig.port}/${postgresConfig.database}`
    }
    return sqliteConfig.path
  }

  return (
    <div className="space-y-6">
      <div>
        <h3 className={cn('text-base font-semibold mb-1', text.primary)}>Review Migration</h3>
        <p className={cn('text-sm', text.secondary)}>
          Please review the migration details before starting
        </p>
      </div>

      {/* Source -> Target */}
      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        <div className={cn('p-4 rounded-xl', 'bg-neutral-50 dark:bg-neutral-800/50')}>
          <p className={cn('text-xs font-medium uppercase tracking-wider mb-2', text.tertiary)}>
            From
          </p>
          <p className={cn('text-sm font-medium', text.primary)}>
            {sourceDriver === 'postgres' ? 'PostgreSQL' : 'SQLite'}
          </p>
          <p className={cn('text-xs mt-1', text.secondary)}>Current database</p>
        </div>
        <div className={cn('p-4 rounded-xl', 'bg-primary-50 dark:bg-primary-500/10')}>
          <p className={cn('text-xs font-medium uppercase tracking-wider mb-2', text.tertiary)}>
            To
          </p>
          <p className={cn('text-sm font-medium', text.primary)}>
            {targetDriver === 'postgres' ? 'PostgreSQL' : 'SQLite'}
          </p>
          <p className={cn('text-xs mt-1', text.secondary)}>{getTargetDescription()}</p>
        </div>
      </div>

      {/* Estimation */}
      {estimate.isPending ? (
        <Loading text="Calculating migration estimate..." />
      ) : estimateData ? (
        <div
          className={cn(
            'p-4 rounded-xl border',
            'bg-neutral-50 dark:bg-neutral-800/30',
            'border-neutral-200 dark:border-neutral-700'
          )}
        >
          <h4 className={cn('text-sm font-semibold mb-4', text.primary)}>Estimation</h4>
          <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
            <div className="flex items-center gap-3">
              <HardDrive className={cn('w-4 h-4', text.tertiary)} />
              <div>
                <p className={cn('text-xs', text.tertiary)}>Data size</p>
                <p className={cn('text-sm font-medium', text.primary)}>
                  {formatBytes(estimateData.source?.sizeBytes ?? 0)}
                </p>
              </div>
            </div>
            <div className="flex items-center gap-3">
              <Table className={cn('w-4 h-4', text.tertiary)} />
              <div>
                <p className={cn('text-xs', text.tertiary)}>Tables</p>
                <p className={cn('text-sm font-medium', text.primary)}>
                  {estimateData.source?.tableCount ?? 0}
                </p>
              </div>
            </div>
            <div className="flex items-center gap-3">
              <Database className={cn('w-4 h-4', text.tertiary)} />
              <div>
                <p className={cn('text-xs', text.tertiary)}>Rows</p>
                <p className={cn('text-sm font-medium', text.primary)}>
                  {(estimateData.source?.totalRows ?? 0).toLocaleString()}
                </p>
              </div>
            </div>
            <div className="flex items-center gap-3">
              <Clock className={cn('w-4 h-4', text.tertiary)} />
              <div>
                <p className={cn('text-xs', text.tertiary)}>Est. time</p>
                <p className={cn('text-sm font-medium', text.primary)}>
                  {estimateData.estimate?.durationHuman ?? 'Unknown'}
                </p>
              </div>
            </div>
          </div>
        </div>
      ) : null}

      {/* Warnings */}
      {estimateData?.warnings && estimateData.warnings.length > 0 && (
        <div
          className={cn(
            'p-4 rounded-xl',
            'bg-amber-50 dark:bg-amber-500/10',
            'border border-amber-200 dark:border-amber-500/20'
          )}
        >
          <div className="flex items-start gap-3">
            <AlertTriangle className="w-5 h-5 text-amber-500 shrink-0" />
            <div>
              <p className="text-sm font-medium text-amber-700 dark:text-amber-400">Warnings</p>
              <ul className="mt-2 space-y-1">
                {estimateData.warnings.map((warning, i) => (
                  <li key={i} className="text-sm text-amber-600 dark:text-amber-500">
                    {warning}
                  </li>
                ))}
              </ul>
            </div>
          </div>
        </div>
      )}

      {/* Important notice */}
      <div
        className={cn(
          'p-4 rounded-xl',
          'bg-blue-50 dark:bg-blue-500/10',
          'border border-blue-200 dark:border-blue-500/20'
        )}
      >
        <p className="text-sm text-blue-700 dark:text-blue-400">
          <strong>Important:</strong> The server will enter maintenance mode during migration. All
          users will be temporarily unable to access the server. A restart will be required after
          migration completes.
        </p>
      </div>

      <div className="flex justify-between pt-4">
        <Button variant="ghost" onClick={onBack} disabled={isStarting}>
          Back
        </Button>
        <Button onClick={onStart} disabled={isStarting || estimate.isPending}>
          {isStarting ? 'Starting...' : 'Start Migration'}
        </Button>
      </div>
    </div>
  )
}
