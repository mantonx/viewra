import { useState, useCallback } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { Card, CardContent, CardHeader, Button, Loading } from '@/components/ui'
import { useGetApiSystemInfo, getApiSystemInfo } from '@/lib/api/generated/system/system'
import { cn } from '@/lib/utils'
import { text } from '@/styles/semantic'
import { Database, ArrowRightLeft, Server, FileText } from 'lucide-react'
import { DatabaseMigrationWizard } from '@/views/settings/DatabaseMigration'
import type { DatabaseDriver } from '@/views/settings/DatabaseMigration'

/**
 * Wait for the server to come back online, then invalidate queries.
 * Polls the health endpoint until it responds successfully.
 */
const waitForServerAndRefresh = async (
  queryClient: ReturnType<typeof useQueryClient>,
  maxAttempts = 30,
  intervalMs = 1000
) => {
  for (let i = 0; i < maxAttempts; i++) {
    try {
      const response = await getApiSystemInfo()
      if (response.status === 200) {
        // Server is back, invalidate all queries to refresh data
        await queryClient.invalidateQueries()
        return
      }
    } catch {
      // Server not ready yet, wait and retry
    }
    await new Promise((resolve) => setTimeout(resolve, intervalMs))
  }
  // Max attempts reached, invalidate anyway to trigger refetch
  await queryClient.invalidateQueries()
}

/**
 * Card displaying database information with migration option.
 */
export const DatabaseCard = () => {
  const queryClient = useQueryClient()
  const [showMigrationWizard, setShowMigrationWizard] = useState(false)
  const { data: systemInfo, isLoading } = useGetApiSystemInfo()

  const handleWizardClose = useCallback(() => {
    setShowMigrationWizard(false)
    // Wait for server to come back online, then refresh all data
    waitForServerAndRefresh(queryClient)
  }, [queryClient])

  if (isLoading) {
    return (
      <Card variant="glass" className="relative overflow-hidden">
        <CardContent className="py-8">
          <Loading text="Loading database info..." />
        </CardContent>
      </Card>
    )
  }

  const info = systemInfo?.status === 200 ? systemInfo.data : null
  const databaseInfo = info?.database

  if (!databaseInfo) {
    return null
  }

  const isPostgres = databaseInfo.driver === 'postgres'
  const currentDriver: DatabaseDriver = isPostgres ? 'postgres' : 'sqlite'
  const targetDriver = isPostgres ? 'SQLite' : 'PostgreSQL'

  return (
    <>
      <Card variant="glass" className="relative overflow-hidden">
        <div className="absolute inset-0 bg-linear-to-br from-blue-500/5 via-transparent to-blue-500/10" />

        <CardHeader className="relative border-b border-neutral-100 dark:border-neutral-800">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-3">
              <div
                className={cn(
                  'p-2.5 rounded-xl',
                  'bg-linear-to-br from-blue-500 to-blue-600',
                  'text-white shadow-lg shadow-blue-500/25'
                )}
              >
                <Database className="w-5 h-5" />
              </div>
              <div>
                <h2 className={cn('text-lg font-semibold', text.primary)}>Database</h2>
                <p className={cn('text-sm mt-0.5', text.secondary)}>
                  Current database configuration
                </p>
              </div>
            </div>
          </div>
        </CardHeader>

        <CardContent className="relative">
          <div className="flex items-start gap-6 py-2">
            {/* Database icon and name */}
            <div className="flex items-center gap-4">
              <div
                className={cn(
                  'p-3 rounded-xl',
                  'bg-neutral-100 dark:bg-neutral-800',
                  text.secondary
                )}
              >
                {isPostgres ? (
                  <Server className="w-8 h-8" />
                ) : (
                  <FileText className="w-8 h-8" />
                )}
              </div>
              <div>
                <p className={cn('text-lg font-semibold', text.primary)}>
                  {isPostgres ? 'PostgreSQL' : 'SQLite'}
                </p>
                <p className={cn('text-sm', text.secondary)}>
                  {isPostgres
                    ? `${databaseInfo.host}:${databaseInfo.port}/${databaseInfo.databaseName}`
                    : databaseInfo.databaseName || 'viewra.db'}
                </p>
              </div>
            </div>

            {/* SSL Mode for postgres */}
            {isPostgres && databaseInfo.sslMode && (
              <div className="flex-1">
                <p className={cn('text-xs font-medium uppercase tracking-wider mb-1', text.tertiary)}>
                  SSL Mode
                </p>
                <p className={cn('text-sm', text.primary)}>
                  {databaseInfo.sslMode}
                </p>
              </div>
            )}
          </div>

          {/* Migrate button */}
          <div className="mt-4 pt-4 border-t border-neutral-100 dark:border-neutral-800">
            <Button
              variant="secondary"
              className="w-full"
              onClick={() => setShowMigrationWizard(true)}
            >
              <ArrowRightLeft className="w-4 h-4 mr-2" />
              Migrate to {targetDriver}...
            </Button>
          </div>
        </CardContent>
      </Card>

      <DatabaseMigrationWizard
        isOpen={showMigrationWizard}
        onClose={handleWizardClose}
        currentDriver={currentDriver}
      />
    </>
  )
}
