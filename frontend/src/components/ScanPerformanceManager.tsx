import React, { useState, useEffect } from 'react';
import type { ScanConfig, ScanPerformanceStats, ScanJob } from '../types/media.types';
import { formatBytes, formatDuration } from '../lib/utils';

interface ActiveScan {
  jobId: string;
  libraryId: number;
  progress: number;
  eta: string;
  filesPerSecond: number;
  filesProcessed: number;
  bytesProcessed: number;
  totalFiles: number;
  status: string;
}

const ScanPerformanceManager: React.FC = () => {
  const [config, setConfig] = useState<ScanConfig | null>(null);
  const [performanceStats, setPerformanceStats] = useState<ScanPerformanceStats[]>([]);
  const [activeScans, setActiveScans] = useState<ActiveScan[]>([]);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);

  // Fetch current configuration
  const fetchConfig = async () => {
    try {
      const response = await fetch('/api/admin/scanner/config');
      const data = await response.json();
      setConfig(data.config);
    } catch (error) {
      console.error('Failed to fetch scan config:', error);
    }
  };

  // Fetch performance statistics
  const fetchPerformanceStats = async () => {
    try {
      const response = await fetch('/api/admin/scanner/performance');
      const data = await response.json();
      setPerformanceStats(data.recent_scans || []);
    } catch (error) {
      console.error('Failed to fetch performance stats:', error);
    }
  };

  // Fetch active scans
  const fetchScanProgress = async () => {
    try {
      const response = await fetch('/api/scanner/status');
      const data = await response.json();

      // Fetch detailed progress for each active scan
      const progressPromises = data.jobs
        .filter((job: ScanJob) => job.status === 'running')
        .map(async (job: ScanJob) => {
          // Try the scanner module endpoint first, fall back to admin endpoint if needed
          let progressData;
          try {
            const progressResponse = await fetch(`/api/scanner/progress/${job.id}`);
            if (progressResponse.ok) {
              progressData = await progressResponse.json();
            } else {
              // Fall back to admin endpoint if module endpoint fails
              const adminProgressResponse = await fetch(`/api/admin/scanner/progress/${job.id}`);
              progressData = await adminProgressResponse.json();
            }
          } catch (err: unknown) {
            console.error(`Error fetching progress for job ${job.id}:`, err);
            progressData = { progress: 0, eta: 'Unknown', files_per_sec: 0, bytes_processed: 0 };
          }

          return {
            jobId: job.id,
            libraryId: job.library_id,
            progress: progressData.progress || 0,
            eta: progressData.eta || 'Unknown',
            filesPerSecond: progressData.files_per_sec || 0,
            filesProcessed: job.files_processed || 0,
            bytesProcessed: progressData.bytes_processed || 0,
            totalFiles: job.files_found || 0,
            status: job.status,
          };
        });

      const scans = await Promise.all(progressPromises);
      setActiveScans(scans);
    } catch (error) {
      console.error('Failed to fetch scan progress:', error);
    } finally {
      setLoading(false);
    }
  };

  // Load data on component mount
  useEffect(() => {
    const loadData = async () => {
      setLoading(true);
      await Promise.all([fetchConfig(), fetchPerformanceStats(), fetchScanProgress()]);
      setLoading(false);
    };
    loadData();
    
    // Set up polling for active scans (every 2 seconds)
    const pollingInterval = setInterval(() => {
      // Only fetch progress if we have active scans
      if (activeScans.length > 0) {
        fetchScanProgress();
      }
    }, 2000);
    
    // Clean up interval on component unmount
    return () => clearInterval(pollingInterval);
  }, [activeScans.length]);

  // Update configuration
  const updateConfig = async (updates: Partial<ScanConfig>) => {
    if (!config) return;

    setSaving(true);
    try {
      const response = await fetch('/api/admin/scanner/config', {
        method: 'PUT',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify(updates),
      });

      if (response.ok) {
        const data = await response.json();
        setConfig(data.config);
      }
    } catch (error) {
      console.error('Failed to update scan config:', error);
    } finally {
      setSaving(false);
    }
  };

  // Apply performance profile
  const applyProfile = async (profile: string) => {
    setSaving(true);
    try {
      const response = await fetch('/api/admin/scanner/config', {
        method: 'PUT',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ profile }),
      });

      if (response.ok) {
        const data = await response.json();
        setConfig(data.config);
      }
    } catch (error) {
      console.error('Failed to apply profile:', error);
    } finally {
      setSaving(false);
    }
  };

  const formatETA = (eta: string) => {
    if (eta === 'Unknown') return eta;

    try {
      const etaDate = new Date(eta);
      const now = new Date();
      const diff = etaDate.getTime() - now.getTime();

      if (diff <= 0) return 'Completing...';

      const minutes = Math.floor(diff / 60000);
      const hours = Math.floor(minutes / 60);
      const days = Math.floor(hours / 24);

      if (days > 0) return `${days}d ${hours % 24}h`;
      if (hours > 0) return `${hours}h ${minutes % 60}m`;
      return `${minutes}m`;
    } catch {
      return eta;
    }
  };

  if (loading) {
    return (
      <div className="bg-slate-900 rounded-lg p-6 shadow-xl">
        <div className="text-white">Loading scan performance settings...</div>
      </div>
    );
  }

  if (!config) {
    return (
      <div className="bg-slate-900 rounded-lg p-6 shadow-xl">
        <div className="text-red-400">Failed to load scan configuration.</div>
      </div>
    );
  }

  return (
    <div className="bg-slate-900 rounded-lg p-6 shadow-xl">
      <div className="flex justify-between items-center mb-6">
        <h2 className="text-2xl font-bold text-white">⚡ Scan Performance Settings</h2>
        <span
          className={`px-3 py-1 rounded text-sm font-medium ${
            config.parallel_scanning_enabled
              ? 'bg-green-600 text-green-100'
              : 'bg-blue-600 text-blue-100'
          }`}
        >
          {config.parallel_scanning_enabled ? 'Parallel Mode' : 'Sequential Mode'}
        </span>
      </div>

      <div className="space-y-6">
        {/* Performance Profiles */}
        <div className="bg-slate-800 rounded-lg p-4">
          <h3 className="text-lg font-medium text-white mb-3">Performance Profiles</h3>
          <div className="flex gap-2 mb-3">
            <button
              onClick={() => applyProfile('conservative')}
              disabled={saving}
              className="bg-blue-600 hover:bg-blue-700 disabled:bg-slate-600 text-white px-4 py-2 rounded text-sm transition-colors"
            >
              Conservative
            </button>
            <button
              onClick={() => applyProfile('default')}
              disabled={saving}
              className="bg-green-600 hover:bg-green-700 disabled:bg-slate-600 text-white px-4 py-2 rounded text-sm transition-colors"
            >
              Default
            </button>
            <button
              onClick={() => applyProfile('aggressive')}
              disabled={saving}
              className="bg-orange-600 hover:bg-orange-700 disabled:bg-slate-600 text-white px-4 py-2 rounded text-sm transition-colors"
            >
              Aggressive
            </button>
          </div>
          <p className="text-sm text-slate-400">
            <strong>Conservative:</strong> Lower resource usage for slower systems
            <br />
            <strong>Default:</strong> Balanced performance for most systems
            <br />
            <strong>Aggressive:</strong> Maximum performance for powerful systems
          </p>
        </div>

        {/* Configuration Settings */}
        <div className="bg-slate-800 rounded-lg p-4">
          <h3 className="text-lg font-medium text-white mb-3">Scanner Configuration</h3>
          <div className="space-y-4">
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              <div className="flex items-center justify-between">
                <label className="text-sm font-medium text-slate-300">Parallel Scanning</label>
                <label className="relative inline-flex items-center cursor-pointer">
                  <input
                    type="checkbox"
                    checked={config.parallel_scanning_enabled}
                    onChange={(e) => updateConfig({ parallel_scanning_enabled: e.target.checked })}
                    className="sr-only peer"
                  />
                  <div className="w-11 h-6 bg-slate-600 peer-focus:outline-none rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:rounded-full after:h-5 after:w-5 after:transition-all peer-checked:bg-blue-600"></div>
                </label>
              </div>

              <div className="flex items-center justify-between">
                <label className="text-sm font-medium text-slate-300">Smart Hashing</label>
                <label className="relative inline-flex items-center cursor-pointer">
                  <input
                    type="checkbox"
                    checked={config.smart_hash_enabled}
                    onChange={(e) => updateConfig({ smart_hash_enabled: e.target.checked })}
                    className="sr-only peer"
                  />
                  <div className="w-11 h-6 bg-slate-600 peer-focus:outline-none rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:rounded-full after:h-5 after:w-5 after:transition-all peer-checked:bg-blue-600"></div>
                </label>
              </div>

              <div className="flex items-center justify-between">
                <label className="text-sm font-medium text-slate-300">Async Metadata</label>
                <label className="relative inline-flex items-center cursor-pointer">
                  <input
                    type="checkbox"
                    checked={config.async_metadata_enabled}
                    onChange={(e) => updateConfig({ async_metadata_enabled: e.target.checked })}
                    className="sr-only peer"
                  />
                  <div className="w-11 h-6 bg-slate-600 peer-focus:outline-none rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:rounded-full after:h-5 after:w-5 after:transition-all peer-checked:bg-blue-600"></div>
                </label>
              </div>
            </div>

            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              <div>
                <label className="block text-sm font-medium text-slate-300 mb-1">
                  Worker Count
                </label>
                <select
                  value={config.worker_count.toString()}
                  onChange={(e) => updateConfig({ worker_count: parseInt(e.target.value) })}
                  className="w-full bg-slate-700 text-white px-3 py-2 rounded border border-slate-600 focus:border-blue-500 focus:outline-none"
                >
                  <option value="0">Auto (CPU Count)</option>
                  <option value="1">1</option>
                  <option value="2">2</option>
                  <option value="4">4</option>
                  <option value="8">8</option>
                </select>
              </div>

              <div>
                <label className="block text-sm font-medium text-slate-300 mb-1">Batch Size</label>
                <select
                  value={config.batch_size.toString()}
                  onChange={(e) => updateConfig({ batch_size: parseInt(e.target.value) })}
                  className="w-full bg-slate-700 text-white px-3 py-2 rounded border border-slate-600 focus:border-blue-500 focus:outline-none"
                >
                  <option value="10">10</option>
                  <option value="25">25</option>
                  <option value="50">50</option>
                  <option value="100">100</option>
                </select>
              </div>
            </div>
          </div>
        </div>

        {/* Performance Statistics */}
        <div className="bg-slate-800 rounded-lg p-4">
          <div className="flex justify-between items-center mb-3">
            <h3 className="text-lg font-medium text-white">Recent Scan Performance</h3>
            <button
              onClick={fetchPerformanceStats}
              className="bg-blue-600 hover:bg-blue-700 text-white px-3 py-1 rounded text-sm transition-colors"
            >
              Refresh Stats
            </button>
          </div>

          {performanceStats.length === 0 ? (
            <p className="text-slate-500">No completed scans yet.</p>
          ) : (
            <div className="space-y-2">
              {performanceStats.map((scan) => (
                <div
                  key={scan.id}
                  className="flex justify-between items-center p-3 bg-slate-700 rounded-lg"
                >
                  <div>
                    <div className="font-medium text-white">
                      Library {scan.library_id} - Scan #{scan.id}
                    </div>
                    <div className="text-sm text-slate-400">
                      {scan.files_processed} files • {formatBytes(scan.bytes_processed)}
                    </div>
                  </div>
                  <div className="text-right">
                    <div className="font-medium text-white">
                      {formatDuration(scan.duration_seconds)}
                    </div>
                    <div className="text-sm text-slate-400">
                      {scan.files_per_second.toFixed(1)} files/s • {scan.mb_per_second.toFixed(1)}{' '}
                      MB/s
                    </div>
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>

        {/* Active Scans */}
        {activeScans.length > 0 && (
          <div className="bg-slate-800 rounded-lg p-4">
            <div className="flex justify-between items-center mb-3">
              <h3 className="text-lg font-medium text-white">Active Scans</h3>
            </div>

            {activeScans.map((scan) => (
              <div key={scan.jobId} className="space-y-2 p-4 border border-slate-600 rounded-lg mb-4 bg-slate-700">
                <div className="flex justify-between items-center">
                  <div className="flex items-center gap-2">
                    <span className="bg-blue-600 text-blue-100 px-2 py-1 rounded text-xs font-medium">
                      Library #{scan.libraryId}
                    </span>
                    <span className="bg-slate-600 text-slate-200 px-2 py-1 rounded text-xs font-medium">
                      {scan.filesPerSecond.toFixed(1)} files/sec
                    </span>
                  </div>
                  <span className="text-sm text-slate-400">ETA: {formatETA(scan.eta)}</span>
                </div>

                <div className="w-full bg-slate-600 rounded-full h-2">
                  <div 
                    className="bg-blue-500 h-2 rounded-full transition-all duration-300" 
                    style={{ width: `${scan.progress}%` }}
                  />
                </div>

                <div className="flex justify-between text-sm text-slate-400">
                  <span>
                    {scan.filesProcessed.toLocaleString()} / {scan.totalFiles.toLocaleString()}{' '}
                    files
                  </span>
                  <span>{formatBytes(scan.bytesProcessed)} processed</span>
                </div>

                <div className="flex gap-2 mt-2">
                  <button
                    className="bg-slate-600 hover:bg-slate-500 text-white px-3 py-1 rounded text-sm transition-colors border border-slate-500"
                    onClick={() => fetch(`/api/scanner/pause/${scan.jobId}`, { method: 'POST' })}
                  >
                    Pause
                  </button>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
};

export default ScanPerformanceManager;
