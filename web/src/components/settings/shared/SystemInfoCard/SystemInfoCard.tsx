import { Card, CardContent, CardHeader, Loading } from '@/components/ui'
import { useGetApiSystemInfo } from '@/lib/api/generated/system/system'
import { cn } from '@/lib/utils'
import { text } from '@/styles/semantic'
import { Cpu, HardDrive, MemoryStick, MonitorPlay } from 'lucide-react'

/**
 * Card displaying detected system hardware information.
 * Shows CPU, memory, and GPU details.
 */
export const SystemInfoCard = () => {
  const { data: systemInfo, isLoading } = useGetApiSystemInfo()

  if (isLoading) {
    return (
      <Card variant="glass" className="relative overflow-hidden">
        <div className="absolute inset-0 bg-linear-to-br from-primary-500/5 via-transparent to-primary-500/10" />
        <CardContent className="py-8">
          <Loading text="Detecting hardware..." />
        </CardContent>
      </Card>
    )
  }

  const info = systemInfo?.status === 200 ? systemInfo.data : null
  if (!info) {
    return null
  }

  const cpuInfo = info.cpu
  const memoryInfo = info.memory
  const gpuInfo = info.gpu

  return (
    <Card variant="glass" className="relative overflow-hidden">
      <div className="absolute inset-0 bg-linear-to-br from-primary-500/5 via-transparent to-primary-500/10" />

      <CardHeader className="relative border-b border-neutral-100 dark:border-neutral-800">
        <div className="flex items-center gap-3">
          <div
            className={cn(
              'p-2.5 rounded-xl',
              'bg-linear-to-br from-primary-500 to-primary-600',
              'text-white shadow-lg shadow-primary-500/25'
            )}
          >
            <MonitorPlay className="w-5 h-5" />
          </div>
          <div>
            <h2 className={cn('text-lg font-semibold', text.primary)}>System Hardware</h2>
            <p className={cn('text-sm mt-0.5', text.secondary)}>Detected hardware capabilities</p>
          </div>
        </div>
      </CardHeader>

      <CardContent className="relative">
        <div className="grid grid-cols-1 md:grid-cols-3 gap-6 py-2">
          {/* CPU Info */}
          <div className="space-y-3">
            <div className="flex items-center gap-2">
              <Cpu className={cn('w-4 h-4', text.secondary)} />
              <span className={cn('text-sm font-medium', text.secondary)}>CPU</span>
            </div>
            <div className="space-y-1">
              <p className={cn('text-sm font-medium', text.primary)}>
                {cpuInfo?.model || 'Unknown'}
              </p>
              <p className={cn('text-xs', text.tertiary)}>
                {cpuInfo?.physicalCpus || 0} cores / {cpuInfo?.logicalCpus || 0} threads
              </p>
            </div>
          </div>

          {/* Memory Info */}
          <div className="space-y-3">
            <div className="flex items-center gap-2">
              <MemoryStick className={cn('w-4 h-4', text.secondary)} />
              <span className={cn('text-sm font-medium', text.secondary)}>Memory</span>
            </div>
            <div className="space-y-1">
              <p className={cn('text-sm font-medium', text.primary)}>
                {memoryInfo?.totalFormatted || 'Unknown'}
              </p>
              <p className={cn('text-xs', text.tertiary)}>System RAM</p>
            </div>
          </div>

          {/* GPU Info */}
          <div className="space-y-3">
            <div className="flex items-center gap-2">
              <HardDrive className={cn('w-4 h-4', text.secondary)} />
              <span className={cn('text-sm font-medium', text.secondary)}>GPU</span>
            </div>
            <div className="space-y-1">
              {gpuInfo?.available ? (
                <>
                  <p className={cn('text-sm font-medium', text.primary)}>
                    {gpuInfo.deviceNames?.[0] || gpuInfo.type || 'GPU Detected'}
                  </p>
                  <div className="flex items-center gap-2">
                    <span
                      className={cn(
                        'inline-flex items-center px-1.5 py-0.5 rounded text-xs font-medium',
                        'bg-emerald-100 dark:bg-emerald-900/50 text-emerald-700 dark:text-emerald-400'
                      )}
                    >
                      {gpuInfo.hwAccelType || gpuInfo.type}
                    </span>
                    {gpuInfo.hasVaapi && (
                      <span
                        className={cn(
                          'inline-flex items-center px-1.5 py-0.5 rounded text-xs',
                          'bg-blue-100 dark:bg-blue-900/50 text-blue-700 dark:text-blue-400'
                        )}
                      >
                        VAAPI
                      </span>
                    )}
                  </div>
                </>
              ) : (
                <>
                  <p className={cn('text-sm font-medium', text.primary)}>No GPU Detected</p>
                  <p className={cn('text-xs', text.tertiary)}>Software encoding only</p>
                </>
              )}
            </div>
          </div>
        </div>
      </CardContent>
    </Card>
  )
}
