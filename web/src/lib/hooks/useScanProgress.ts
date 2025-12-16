import { useState, useCallback, useEffect } from 'react'
import { useSSE, type SSEConnectionState } from './useSSE'

/**
 * Scan progress data from SSE events
 */
export interface ScanProgressData {
  job_id: number
  status: string
  progress: number
  phase: string
  files_found: number
  files_processed: number
  error_count: number
  warning_count: number
  estimated_total: number
  discovery_done: boolean
  discovery_errors?: number
  discovery_warnings?: number
  dirs_scanned?: number
  dirs_skipped?: number
  files_skipped?: number
  eta_seconds?: number
  error_message?: string
}

/**
 * Scan progress state with computed fields
 */
export interface ScanProgressState {
  /** Library ID */
  libraryId: number
  /** Scan job ID */
  jobId: number
  /** Scan status: pending, running, paused, completed, failed */
  status: string
  /** Progress percentage (0-100) */
  progress: number
  /** Current scan phase: discovering, processing, completed */
  phase: string
  /** Total files discovered */
  filesFound: number
  /** Files processed so far */
  filesProcessed: number
  /** Number of errors encountered */
  errorCount: number
  /** Number of warnings encountered */
  warningCount: number
  /** Estimated total files (from previous scan) */
  estimatedTotal: number
  /** Whether file discovery is complete */
  discoveryDone: boolean
  /** ETA in seconds (if available) */
  etaSeconds?: number
  /** Error message if failed */
  errorMessage?: string
  /** Discovery health metrics */
  discoveryErrors: number
  discoveryWarnings: number
  dirsScanned: number
  dirsSkipped: number
  filesSkipped: number
  /** Whether a scan is actively running */
  isScanning: boolean
  /** Whether scan is paused */
  isPaused: boolean
  /** Whether scan is completed */
  isCompleted: boolean
  /** Whether scan failed */
  isFailed: boolean
}

export interface UseScanProgressOptions {
  /** Enable/disable the SSE connection */
  enabled?: boolean
  /** Called when progress updates */
  onProgress?: (state: ScanProgressState) => void
  /** Called when scan completes */
  onComplete?: () => void
  /** Called when scan fails */
  onError?: (errorMessage: string) => void
}

export interface UseScanProgressReturn {
  /** Current scan progress state */
  scanStatus: ScanProgressState | null
  /** SSE connection state */
  connectionState: SSEConnectionState
  /** Any connection error */
  error: Error | null
  /** Whether a scan is actively running */
  isScanning: boolean
  /** Whether scan is paused */
  isPaused: boolean
  /** Manually reconnect SSE */
  reconnect: () => void
}

/**
 * Hook to stream scan progress for a library via SSE.
 *
 * Provides real-time updates on scan status including files discovered,
 * files processed, errors, and ETA.
 *
 * @example
 * ```tsx
 * const { scanStatus, isScanning, connectionState } = useScanProgress(libraryId, {
 *   enabled: true,
 *   onComplete: () => toast.success('Scan complete!'),
 * })
 *
 * if (isScanning && scanStatus) {
 *   return <div>Scanning: {scanStatus.filesProcessed}/{scanStatus.filesFound}</div>
 * }
 * ```
 */
export const useScanProgress = (
  libraryId: number,
  options: UseScanProgressOptions = {}
): UseScanProgressReturn => {
  const { enabled = true, onProgress, onComplete, onError } = options

  const [scanStatus, setScanStatus] = useState<ScanProgressState | null>(null)
  const [wasScanning, setWasScanning] = useState(false)

  const handleEvent = useCallback(
    (data: ScanProgressData) => {
      const status = data.status ?? 'unknown'
      const isScanning = status === 'running'
      const isPaused = status === 'paused'
      const isCompleted = status === 'completed'
      const isFailed = status === 'failed'

      const state: ScanProgressState = {
        libraryId,
        jobId: data.job_id,
        status,
        progress: data.progress ?? 0,
        phase: data.phase ?? '',
        filesFound: data.files_found ?? 0,
        filesProcessed: data.files_processed ?? 0,
        errorCount: data.error_count ?? 0,
        warningCount: data.warning_count ?? 0,
        estimatedTotal: data.estimated_total ?? 0,
        discoveryDone: data.discovery_done ?? false,
        etaSeconds: data.eta_seconds,
        errorMessage: data.error_message,
        discoveryErrors: data.discovery_errors ?? 0,
        discoveryWarnings: data.discovery_warnings ?? 0,
        dirsScanned: data.dirs_scanned ?? 0,
        dirsSkipped: data.dirs_skipped ?? 0,
        filesSkipped: data.files_skipped ?? 0,
        isScanning,
        isPaused,
        isCompleted,
        isFailed,
      }

      setScanStatus(state)
      onProgress?.(state)

      // Track completion/failure transitions
      if (wasScanning && isCompleted) {
        onComplete?.()
      }
      if (wasScanning && isFailed && data.error_message) {
        onError?.(data.error_message)
      }

      setWasScanning(isScanning)
    },
    [libraryId, onProgress, onComplete, onError, wasScanning]
  )

  const { connectionState, error, reconnect } = useSSE<ScanProgressData>(
    `/api/libraries/${libraryId}/scan/stream`,
    {
      enabled: enabled && libraryId > 0,
      onEvent: handleEvent,
      eventTypes: ['init', 'update', 'complete'],
      reconnectDelay: 5000,
      maxReconnectAttempts: 10,
    }
  )

  // Reset state when disabled or library changes
  useEffect(() => {
    if (!enabled || libraryId <= 0) {
      setScanStatus(null)
      setWasScanning(false)
    }
  }, [enabled, libraryId])

  return {
    scanStatus,
    connectionState,
    error,
    isScanning: scanStatus?.isScanning ?? false,
    isPaused: scanStatus?.isPaused ?? false,
    reconnect,
  }
}

export default useScanProgress
