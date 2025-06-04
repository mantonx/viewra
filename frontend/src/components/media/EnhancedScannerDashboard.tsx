import React, { useState, useEffect, useCallback } from 'react';
import { Monitor, Activity, Settings, RefreshCw, Pause, Play } from 'lucide-react';
import ScanProgressCard from './ScanProgressCard';
import ScanActivityFeed from './ScanActivityFeed';

interface Library {
  id: number;
  path: string;
  type: string;
  created_at: string;
}

interface ScanJob {
  id: number;
  library_id: number;
  status: 'running' | 'paused' | 'completed' | 'failed' | 'cancelled';
  progress: number;
  files_found: number;
  files_processed: number;
  bytes_processed: number;
  started_at: string;
  updated_at: string;
}

interface DetailedScanProgress {
  // Basic info
  job_id: number;
  files_processed: number;
  files_found: number;
  files_skipped: number;
  bytes_processed: number;
  errors_count: number;
  progress: number;
  eta: string;
  estimated_time_left: number;
  files_per_second: number;

  // Worker status
  active_workers: number;
  max_workers: number;
  min_workers: number;
  queue_length: number;

  // System metrics
  cpu_percent: number;
  memory_percent: number;
  io_wait_percent: number;
  load_average: number;
  network_mbps: number;

  // Throttling
  emergency_brake: boolean;
  current_batch_size: number;
  processing_delay_ms: number;
}

interface ScanActivity {
  id: string;
  timestamp: string;
  type:
    | 'scan.progress'
    | 'scan.started'
    | 'scan.completed'
    | 'scan.error'
    | 'plugin.enrichment'
    | 'plugin.asset'
    | 'system.health'
    | 'file.processed';
  source: string;
  title: string;
  message: string;
  severity: 'info' | 'success' | 'warning' | 'error';
  data?: {
    [key: string]: unknown;
  };
}

interface ScannerStats {
  active_scans: number;
  total_files_scanned: number;
  total_bytes_scanned: number;
}

const EnhancedScannerDashboard: React.FC = () => {
  const [libraries, setLibraries] = useState<Library[]>([]);
  const [scanJobs, setScanJobs] = useState<ScanJob[]>([]);
  const [scanProgress, setScanProgress] = useState<Map<number, DetailedScanProgress>>(new Map());
  const [scanActivities, setScanActivities] = useState<ScanActivity[]>([]);
  const [scannerStats, setScannerStats] = useState<ScannerStats | null>(null);
  const [isAutoRefresh, setIsAutoRefresh] = useState(true);
  const [refreshInterval, setRefreshInterval] = useState(2000); // 2 seconds
  const [selectedJobIds, setSelectedJobIds] = useState<Set<number>>(new Set());

  // Data fetching functions
  const loadLibraries = useCallback(async () => {
    try {
      const res = await fetch('/api/admin/libraries');
      if (res.ok) {
        const data = await res.json();
        setLibraries(data);
      }
    } catch (error) {
      console.error('Failed to load libraries:', error);
    }
  }, []);

  const loadScanJobs = useCallback(async () => {
    try {
      const res = await fetch('/api/admin/scanner/current-jobs');
      if (res.ok) {
        const data = await res.json();
        setScanJobs(data.jobs || []);

        // Auto-select running jobs for monitoring
        const runningJobs = data.jobs?.filter((job: ScanJob) => job.status === 'running') || [];
        if (runningJobs.length > 0) {
          setSelectedJobIds(new Set(runningJobs.map((job: ScanJob) => job.id)));
        }
      }
    } catch (error) {
      console.error('Failed to load scan jobs:', error);
    }
  }, []);

  const loadDetailedProgress = useCallback(async (jobId: number) => {
    try {
      const res = await fetch(`/api/admin/scanner/progress/${jobId}`);
      if (res.ok) {
        const data = await res.json();
        setScanProgress((prev) => {
          const newMap = new Map(prev);
          newMap.set(jobId, data);
          return newMap;
        });
      }
    } catch (error) {
      console.error(`Failed to load progress for job ${jobId}:`, error);
    }
  }, []);

  const loadScannerStats = useCallback(async () => {
    try {
      const res = await fetch('/api/admin/scanner/stats');
      if (res.ok) {
        const data = await res.json();
        setScannerStats(data);
      }
    } catch (error) {
      console.error('Failed to load scanner stats:', error);
    }
  }, []);

  // Simulate scan activities (in real implementation, this would come from WebSocket/SSE)
  const generateMockActivity = useCallback((): ScanActivity => {
    const activities: Array<{
      type:
        | 'scan.progress'
        | 'scan.started'
        | 'scan.completed'
        | 'scan.error'
        | 'plugin.enrichment'
        | 'plugin.asset'
        | 'system.health'
        | 'file.processed';
      title: string;
      message: string;
      severity: 'info' | 'success' | 'warning' | 'error';
      data?: { [key: string]: unknown };
    }> = [
      {
        type: 'scan.progress',
        title: 'Scan Progress Update',
        message: 'Processed 1,234 files (85% complete)',
        severity: 'info',
      },
      {
        type: 'plugin.enrichment',
        title: 'MusicBrainz Enrichment',
        message: 'Retrieved metadata for "Abbey Road" by The Beatles',
        severity: 'success',
        data: { plugin: 'musicbrainz', fileName: 'abbey_road.mp3' },
      },
      {
        type: 'plugin.asset',
        title: 'Asset Downloaded',
        message: 'Downloaded front cover artwork',
        severity: 'success',
        data: { plugin: 'musicbrainz', assetType: 'front_cover', fileName: 'cover.jpg' },
      },
      {
        type: 'system.health',
        title: 'System Performance',
        message: 'CPU usage: 45%, Memory: 62%',
        severity: 'info',
      },
      {
        type: 'scan.error',
        title: 'Processing Error',
        message: 'Failed to process corrupted file',
        severity: 'error',
        data: { fileName: 'corrupted_track.mp3', errorCount: 1 },
      },
    ];

    const activity = activities[Math.floor(Math.random() * activities.length)];

    return {
      id: `activity_${Date.now()}_${Math.random()}`,
      timestamp: new Date().toISOString(),
      source: 'scanner',
      ...activity,
    };
  }, []);

  // Auto-refresh effect
  useEffect(() => {
    if (!isAutoRefresh) return;

    const interval = setInterval(async () => {
      await Promise.all([
        loadScanJobs(),
        loadScannerStats(),
        ...Array.from(selectedJobIds).map((jobId) => loadDetailedProgress(jobId)),
      ]);

      // Simulate new activity
      if (Math.random() < 0.3) {
        // 30% chance of new activity
        setScanActivities((prev) => [generateMockActivity(), ...prev.slice(0, 49)]);
      }
    }, refreshInterval);

    return () => clearInterval(interval);
  }, [
    isAutoRefresh,
    refreshInterval,
    selectedJobIds,
    loadScanJobs,
    loadScannerStats,
    loadDetailedProgress,
    generateMockActivity,
  ]);

  // Initial load
  useEffect(() => {
    loadLibraries();
    loadScanJobs();
    loadScannerStats();
  }, [loadLibraries, loadScanJobs, loadScannerStats]);

  // Scanner control functions
  const startScan = async (libraryId: number) => {
    try {
      const res = await fetch(`/api/admin/scanner/start/${libraryId}`, {
        method: 'POST',
      });
      if (res.ok) {
        await loadScanJobs();
      }
    } catch (error) {
      console.error('Failed to start scan:', error);
    }
  };

  const pauseScan = async (libraryId: number) => {
    try {
      const res = await fetch(`/api/admin/scanner/pause/${libraryId}`, {
        method: 'POST',
      });
      if (res.ok) {
        await loadScanJobs();
      }
    } catch (error) {
      console.error('Failed to pause scan:', error);
    }
  };

  const resumeScan = async (libraryId: number) => {
    try {
      const res = await fetch(`/api/admin/scanner/resume/${libraryId}`, {
        method: 'POST',
      });
      if (res.ok) {
        await loadScanJobs();
      }
    } catch (error) {
      console.error('Failed to resume scan:', error);
    }
  };

  // Helper functions
  const formatBytes = (bytes: number): string => {
    if (bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
  };

  const detectCurrentPhase = (
    progress: DetailedScanProgress
  ): 'discovery' | 'processing' | 'enrichment' | 'finalization' => {
    if (progress.progress < 5) return 'discovery';
    if (progress.progress < 85) return 'processing';
    if (progress.progress < 98) return 'enrichment';
    return 'finalization';
  };

  const clearActivities = () => {
    setScanActivities([]);
  };

  const toggleJobMonitoring = (jobId: number) => {
    setSelectedJobIds((prev) => {
      const newSet = new Set(prev);
      if (newSet.has(jobId)) {
        newSet.delete(jobId);
      } else {
        newSet.add(jobId);
      }
      return newSet;
    });
  };

  return (
    <div className="space-y-6">
      {/* Dashboard Header */}
      <div className="bg-slate-800 rounded-lg p-6 border border-slate-700">
        <div className="flex items-center justify-between mb-4">
          <div className="flex items-center gap-3">
            <Monitor className="w-6 h-6 text-blue-400" />
            <h2 className="text-xl font-bold text-white">Enhanced Scanner Dashboard</h2>
          </div>

          <div className="flex items-center gap-3">
            <button
              onClick={() => setIsAutoRefresh(!isAutoRefresh)}
              className={`flex items-center gap-2 px-3 py-2 rounded transition-colors ${
                isAutoRefresh
                  ? 'bg-green-600 hover:bg-green-700 text-white'
                  : 'bg-slate-600 hover:bg-slate-500 text-slate-300'
              }`}
            >
              {isAutoRefresh ? <Pause className="w-4 h-4" /> : <Play className="w-4 h-4" />}
              {isAutoRefresh ? 'Auto-refresh ON' : 'Auto-refresh OFF'}
            </button>

            <select
              value={refreshInterval}
              onChange={(e) => setRefreshInterval(Number(e.target.value))}
              className="bg-slate-700 text-white px-3 py-2 rounded border border-slate-600"
              disabled={!isAutoRefresh}
            >
              <option value={1000}>1s</option>
              <option value={2000}>2s</option>
              <option value={5000}>5s</option>
              <option value={10000}>10s</option>
            </select>
          </div>
        </div>

        {/* System Overview */}
        {scannerStats && (
          <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
            <div className="bg-slate-700/50 rounded p-4 text-center">
              <Activity className="w-6 h-6 mx-auto mb-2 text-green-400" />
              <div className="text-2xl font-bold text-white">{scannerStats.active_scans}</div>
              <div className="text-sm text-slate-300">Active Scans</div>
            </div>

            <div className="bg-slate-700/50 rounded p-4 text-center">
              <RefreshCw className="w-6 h-6 mx-auto mb-2 text-blue-400" />
              <div className="text-2xl font-bold text-white">
                {scannerStats.total_files_scanned.toLocaleString()}
              </div>
              <div className="text-sm text-slate-300">Files Processed</div>
            </div>

            <div className="bg-slate-700/50 rounded p-4 text-center">
              <Settings className="w-6 h-6 mx-auto mb-2 text-purple-400" />
              <div className="text-2xl font-bold text-white">
                {formatBytes(scannerStats.total_bytes_scanned)}
              </div>
              <div className="text-sm text-slate-300">Data Processed</div>
            </div>
          </div>
        )}
      </div>

      {/* Active Scan Progress Cards */}
      {scanJobs
        .filter((job) => job.status === 'running' || job.status === 'paused')
        .map((job) => {
          const library = libraries.find((lib) => lib.id === job.library_id);
          const detailedProgress = scanProgress.get(job.id);

          if (!detailedProgress || !library) return null;

          const progressData = {
            jobId: job.id,
            libraryId: job.library_id,
            libraryPath: library.path,
            status: job.status,
            currentPhase: detectCurrentPhase(detailedProgress),
            phaseProgress: detailedProgress.progress,
            filesProcessed: detailedProgress.files_processed,
            filesFound: detailedProgress.files_found,
            totalFiles: detailedProgress.files_found,
            remainingFiles: detailedProgress.files_found - detailedProgress.files_processed,
            filesSkipped: detailedProgress.files_skipped,
            errorsCount: detailedProgress.errors_count,
            bytesProcessed: detailedProgress.bytes_processed,
            totalBytes: detailedProgress.bytes_processed, // Estimate
            filesPerSecond: detailedProgress.files_per_second,
            throughputMbps: detailedProgress.network_mbps,
            progress: detailedProgress.progress,
            eta: detailedProgress.eta,
            elapsedTime: 'N/A', // Would need to calculate
            estimatedTimeLeft: detailedProgress.estimated_time_left,
            activeWorkers: detailedProgress.active_workers,
            maxWorkers: detailedProgress.max_workers,
            minWorkers: detailedProgress.min_workers,
            queueDepth: detailedProgress.queue_length,
            cpuPercent: detailedProgress.cpu_percent,
            memoryPercent: detailedProgress.memory_percent,
            ioWaitPercent: detailedProgress.io_wait_percent,
            loadAverage: detailedProgress.load_average,
            networkMbps: detailedProgress.network_mbps,
            emergencyBrake: detailedProgress.emergency_brake,
            currentBatchSize: detailedProgress.current_batch_size,
            processingDelayMs: detailedProgress.processing_delay_ms,
            pluginStats: {
              musicbrainz: { processed: 125, errors: 2, assets: 98 },
              tmdb: { processed: 45, errors: 1, assets: 43 },
            },
            lastUpdate: new Date(),
          };

          return (
            <ScanProgressCard
              key={job.id}
              progress={progressData}
              onPause={() => pauseScan(job.library_id)}
              onResume={() => resumeScan(job.library_id)}
              onCancel={() => pauseScan(job.library_id)} // For now, cancel = pause
            />
          );
        })}

      {/* Grid Layout for Activity Feed and Controls */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Activity Feed - Takes 2/3 of space */}
        <div className="lg:col-span-2">
          <ScanActivityFeed
            activities={scanActivities}
            maxItems={50}
            showFilters={true}
            autoRefresh={isAutoRefresh}
            onClear={clearActivities}
          />
        </div>

        {/* Sidebar Controls */}
        <div className="space-y-4">
          {/* Library Controls */}
          <div className="bg-slate-800 rounded-lg p-4 border border-slate-700">
            <h3 className="font-semibold text-white mb-3">Library Management</h3>
            <div className="space-y-2">
              {libraries.map((library) => {
                const scanJob = scanJobs.find(
                  (job) =>
                    job.library_id === library.id &&
                    (job.status === 'running' || job.status === 'paused')
                );

                return (
                  <div
                    key={library.id}
                    className="flex items-center justify-between p-2 bg-slate-700/50 rounded"
                  >
                    <div className="flex-1 min-w-0">
                      <div className="text-sm font-medium text-white truncate">{library.path}</div>
                      <div className="text-xs text-slate-400">{library.type}</div>
                    </div>

                    <div className="flex items-center gap-2">
                      {scanJob ? (
                        <>
                          <span
                            className={`px-2 py-1 rounded text-xs ${
                              scanJob.status === 'running'
                                ? 'bg-green-600 text-white'
                                : 'bg-yellow-600 text-white'
                            }`}
                          >
                            {scanJob.status}
                          </span>
                          <button
                            onClick={() => toggleJobMonitoring(scanJob.id)}
                            className={`p-1 rounded text-xs transition-colors ${
                              selectedJobIds.has(scanJob.id)
                                ? 'bg-blue-600 text-white'
                                : 'bg-slate-600 text-slate-300 hover:bg-slate-500'
                            }`}
                            title={
                              selectedJobIds.has(scanJob.id)
                                ? 'Stop monitoring'
                                : 'Monitor progress'
                            }
                          >
                            <Monitor className="w-3 h-3" />
                          </button>
                        </>
                      ) : (
                        <button
                          onClick={() => startScan(library.id)}
                          className="px-2 py-1 bg-green-600 hover:bg-green-700 text-white rounded text-xs transition-colors"
                        >
                          Scan
                        </button>
                      )}
                    </div>
                  </div>
                );
              })}
            </div>
          </div>

          {/* Scanner Settings */}
          <div className="bg-slate-800 rounded-lg p-4 border border-slate-700">
            <h3 className="font-semibold text-white mb-3">Scanner Settings</h3>
            <div className="space-y-3 text-sm">
              <div className="flex items-center justify-between">
                <span className="text-slate-300">Auto-refresh Rate</span>
                <span className="text-white">{refreshInterval / 1000}s</span>
              </div>
              <div className="flex items-center justify-between">
                <span className="text-slate-300">Monitored Jobs</span>
                <span className="text-white">{selectedJobIds.size}</span>
              </div>
              <div className="flex items-center justify-between">
                <span className="text-slate-300">Total Activities</span>
                <span className="text-white">{scanActivities.length}</span>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
};

export default EnhancedScannerDashboard;
