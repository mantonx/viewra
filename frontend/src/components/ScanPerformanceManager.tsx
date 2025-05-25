import React, { useState, useEffect } from 'react';
import type { ScanConfig, ScanPerformanceStats } from '../types/media.types';

const ScanPerformanceManager: React.FC = () => {
  const [config, setConfig] = useState<ScanConfig | null>(null);
  const [performanceStats, setPerformanceStats] = useState<ScanPerformanceStats[]>([]);
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

  // Load data on component mount
  useEffect(() => {
    const loadData = async () => {
      setLoading(true);
      await Promise.all([fetchConfig(), fetchPerformanceStats()]);
      setLoading(false);
    };
    loadData();
  }, []);

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

  // Format bytes for display
  const formatBytes = (bytes: number): string => {
    if (bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return `${parseFloat((bytes / Math.pow(k, i)).toFixed(2))} ${sizes[i]}`;
  };

  // Format duration for display
  const formatDuration = (seconds: number): string => {
    if (seconds < 60) return `${seconds}s`;
    const minutes = Math.floor(seconds / 60);
    const remainingSeconds = seconds % 60;
    return `${minutes}m ${remainingSeconds}s`;
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
      </div>
    </div>
  );
};

export default ScanPerformanceManager;
