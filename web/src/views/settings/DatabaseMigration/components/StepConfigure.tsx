import { useState } from 'react'
import { Button, Input, Select } from '@/components/ui'
import { cn } from '@/lib/utils'
import { text } from '@/styles/semantic'
import { CheckCircle, XCircle, Loader2 } from 'lucide-react'
import { usePostApiAdminSystemDatabaseTestConnection } from '@/lib/api/generated/system/system'
import type {
  GithubComMantonxViewraInternalApplicationSystemMigrationPostgresConfig as PostgresConfig,
  GithubComMantonxViewraInternalApplicationSystemMigrationSQLiteConfig as SQLiteConfig,
} from '@/lib/api/generated/models'
import type { DatabaseDriver } from '../types'

type Props = {
  targetDriver: DatabaseDriver
  postgresConfig: PostgresConfig
  sqliteConfig: SQLiteConfig
  onPostgresConfigChange: (config: PostgresConfig) => void
  onSqliteConfigChange: (config: SQLiteConfig) => void
  onConnectionTested: (success: boolean) => void
  onBack: () => void
  onNext: () => void
}

const sslModeOptions = [
  { value: 'disable', label: 'Disable' },
  { value: 'require', label: 'Require' },
  { value: 'verify-ca', label: 'Verify CA' },
  { value: 'verify-full', label: 'Verify Full' },
]

export const StepConfigure = ({
  targetDriver,
  postgresConfig,
  sqliteConfig,
  onPostgresConfigChange,
  onSqliteConfigChange,
  onConnectionTested,
  onBack,
  onNext,
}: Props) => {
  const [testResult, setTestResult] = useState<{
    success: boolean
    message: string
    version?: string
  } | null>(null)

  const testConnection = usePostApiAdminSystemDatabaseTestConnection()

  const handleTestConnection = async () => {
    setTestResult(null)

    const config =
      targetDriver === 'postgres'
        ? { driver: 'postgres' as const, postgres: postgresConfig }
        : { driver: 'sqlite' as const, sqlite: sqliteConfig }

    try {
      const result = await testConnection.mutateAsync({ data: config })
      if (result.status === 200) {
        const data = result.data
        setTestResult({
          success: data.success ?? false,
          message: data.message ?? 'Connection test completed',
          version: data.details?.version,
        })
        onConnectionTested(data.success ?? false)
      }
    } catch {
      setTestResult({
        success: false,
        message: 'Failed to test connection',
      })
      onConnectionTested(false)
    }
  }

  return (
    <div className="space-y-6">
      <div>
        <h3 className={cn('text-base font-semibold mb-1', text.primary)}>
          Configure {targetDriver === 'postgres' ? 'PostgreSQL' : 'SQLite'} Connection
        </h3>
        <p className={cn('text-sm', text.secondary)}>
          {targetDriver === 'postgres'
            ? 'Enter your PostgreSQL server details'
            : 'Specify the path for the new SQLite database'}
        </p>
      </div>

      {targetDriver === 'postgres' ? (
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          <Input
            label="Host"
            value={postgresConfig.host}
            onChange={(e) => onPostgresConfigChange({ ...postgresConfig, host: e.target.value })}
            placeholder="localhost"
          />
          <Input
            label="Port"
            type="number"
            value={postgresConfig.port}
            onChange={(e) =>
              onPostgresConfigChange({ ...postgresConfig, port: parseInt(e.target.value) || 5432 })
            }
            placeholder="5432"
          />
          <Input
            label="Database"
            value={postgresConfig.database}
            onChange={(e) => onPostgresConfigChange({ ...postgresConfig, database: e.target.value })}
            placeholder="viewra"
          />
          <Input
            label="Username"
            value={postgresConfig.user}
            onChange={(e) => onPostgresConfigChange({ ...postgresConfig, user: e.target.value })}
            placeholder="viewra"
          />
          <Input
            label="Password"
            type="password"
            value={postgresConfig.password}
            onChange={(e) => onPostgresConfigChange({ ...postgresConfig, password: e.target.value })}
            placeholder="Enter password"
          />
          <Select
            label="SSL Mode"
            value={postgresConfig.sslMode}
            onChange={(e) => onPostgresConfigChange({ ...postgresConfig, sslMode: e.target.value })}
            options={sslModeOptions}
          />
        </div>
      ) : (
        <div className="space-y-4">
          <div>
            <Input
              label="Database Path"
              value={sqliteConfig.path}
              onChange={(e) => onSqliteConfigChange({ path: e.target.value })}
              placeholder="data/viewra.db"
            />
            <p className={cn('mt-1 text-xs', text.tertiary)}>
              Path relative to application data directory
            </p>
          </div>
          <div
            className={cn(
              'p-4 rounded-xl',
              'bg-blue-50 dark:bg-blue-500/10',
              'border border-blue-200 dark:border-blue-500/20',
              'text-blue-700 dark:text-blue-400 text-sm'
            )}
          >
            A new SQLite database will be created at this path.
          </div>
        </div>
      )}

      {/* Test Connection Result */}
      {testResult && (
        <div
          className={cn(
            'p-4 rounded-xl flex items-start gap-3',
            testResult.success
              ? 'bg-emerald-50 dark:bg-emerald-500/10 border border-emerald-200 dark:border-emerald-500/20'
              : 'bg-red-50 dark:bg-red-500/10 border border-red-200 dark:border-red-500/20'
          )}
        >
          {testResult.success ? (
            <CheckCircle className="w-5 h-5 text-emerald-500 shrink-0" />
          ) : (
            <XCircle className="w-5 h-5 text-red-500 shrink-0" />
          )}
          <div>
            <p
              className={cn(
                'text-sm font-medium',
                testResult.success
                  ? 'text-emerald-700 dark:text-emerald-400'
                  : 'text-red-700 dark:text-red-400'
              )}
            >
              {testResult.message}
            </p>
            {testResult.version && (
              <p
                className={cn(
                  'text-xs mt-1',
                  testResult.success
                    ? 'text-emerald-600 dark:text-emerald-500'
                    : 'text-red-600 dark:text-red-500'
                )}
              >
                Version: {testResult.version}
              </p>
            )}
          </div>
        </div>
      )}

      <div className="flex justify-between pt-4">
        <Button variant="ghost" onClick={onBack}>
          Back
        </Button>
        <div className="flex gap-3">
          <Button
            variant="secondary"
            onClick={handleTestConnection}
            disabled={testConnection.isPending}
          >
            {testConnection.isPending && <Loader2 className="w-4 h-4 mr-2 animate-spin" />}
            Test Connection
          </Button>
          <Button onClick={onNext} disabled={!testResult?.success}>
            Next
          </Button>
        </div>
      </div>
    </div>
  )
}
