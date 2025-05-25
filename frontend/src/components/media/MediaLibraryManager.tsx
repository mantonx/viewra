import { useState, useEffect } from 'react';
import type { MediaLibrary, ScanJob, ScanStats, LibraryStats } from '@/types/media.types';
import type { ApiResponse } from '@/types/system.types';

const MediaLibraryManager = () => {
  const [libraries, setLibraries] = useState<MediaLibrary[]>([]);
  const [loading, setLoading] = useState(false);
  const [showAddForm, setShowAddForm] = useState(false);
  const [newLibrary, setNewLibrary] = useState({
    path: '',
    type: 'movie',
  });
  const [response, setResponse] = useState<ApiResponse | null>(null);

  // Scanner state
  const [scanJobs, setScanJobs] = useState<ScanJob[]>([]);
  const [scanStats, setScanStats] = useState<ScanStats | null>(null);
  const [libraryStats, setLibraryStats] = useState<LibraryStats>({});
  const [scanningLibraries, setScanningLibraries] = useState<Set<number>>(new Set());
  const [scanLoading, setScanLoading] = useState(false);

  // Load libraries on component mount
  useEffect(() => {
    loadLibraries();
    loadScanStats();
    loadScanJobs();
    loadLibraryStats();

    // Poll for scan updates every 3 seconds
    const interval = setInterval(() => {
      loadScanJobs();
      loadScanStats();
      loadLibraryStats();
    }, 3000);

    return () => clearInterval(interval);
  }, []);

  const loadLibraries = async () => {
    setLoading(true);
    try {
      const res = await fetch('/api/admin/media-libraries/');
      const result = await res.json();

      if (res.ok && result.libraries) {
        setLibraries(result.libraries);
      }

      setResponse({
        status: res.status,
        data: result,
      });
    } catch (error) {
      setResponse({
        status: 0,
        data: null,
        error: error instanceof Error ? error.message : 'Failed to load libraries',
      });
    } finally {
      setLoading(false);
    }
  };

  const handleDirectorySelect = () => {
    // For a media server, we want to select existing directories on the server filesystem
    // Provide common directory suggestions or let user type the path
    const path = prompt(
      'Enter the full path to your media directory on the server:\n\n' +
        'Examples:\n' +
        '• /media/movies\n' +
        '• /home/user/Videos\n' +
        '• /mnt/storage/media\n' +
        '• /srv/media/tv-shows\n' +
        '• /data/music'
    );

    if (path && path.trim()) {
      setNewLibrary((prev) => ({ ...prev, path: path.trim() }));
    }
  };

  const getCommonPaths = () => {
    return [
      '/media/movies',
      '/media/tv-shows',
      '/media/music',
      '/home/user/Videos',
      '/home/user/Music',
      '/mnt/storage/media',
      '/srv/media',
      '/data/media',
    ];
  };

  const handleQuickSelect = (path: string) => {
    setNewLibrary((prev) => ({ ...prev, path }));
  };

  const addLibrary = async () => {
    if (!newLibrary.path || !newLibrary.type) {
      alert('Please select a directory and type');
      return;
    }

    setLoading(true);
    try {
      const res = await fetch('/api/admin/media-libraries/', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify(newLibrary),
      });

      const result = await res.json();

      setResponse({
        status: res.status,
        data: result,
      });

      if (res.ok) {
        // Reload libraries and reset form
        await loadLibraries();
        const newLibraryId = result.library?.id;

        // Auto-start scanning for the newly added library
        if (newLibraryId) {
          try {
            const scanRes = await fetch(`/api/admin/scanner/start/${newLibraryId}`, {
              method: 'POST',
            });

            const scanResult = await scanRes.json();

            if (scanRes.ok) {
              setScanningLibraries((prev) => new Set([...prev, newLibraryId]));

              // Update response to include scan info
              setResponse({
                status: scanRes.status,
                data: {
                  library: result.library,
                  message: 'Library created and scan started automatically',
                  scan: scanResult,
                },
              });

              // Reload scan status
              setTimeout(() => {
                loadScanJobs();
                loadScanStats();
                loadLibraryStats();
              }, 1000);
            }
          } catch (scanError) {
            console.error('Failed to start scan for new library:', scanError);
          }
        }

        setNewLibrary({ path: '', type: 'movie' });
        setShowAddForm(false);
      }
    } catch (error) {
      setResponse({
        status: 0,
        data: null,
        error: error instanceof Error ? error.message : 'Failed to add library',
      });
    } finally {
      setLoading(false);
    }
  };

  const removeLibrary = async (id: number) => {
    if (!confirm('Are you sure you want to remove this media library?')) {
      return;
    }

    setLoading(true);
    try {
      // Note: This assumes a DELETE endpoint exists - we may need to add this to the backend
      const res = await fetch(`/api/admin/media-libraries/${id}`, {
        method: 'DELETE',
      });

      if (res.ok) {
        // Remove from local state
        setLibraries((prev) => prev.filter((lib) => lib.id !== id));
        setResponse({
          status: res.status,
          data: { message: 'Library removed successfully' },
        });
      } else {
        const result = await res.json();
        setResponse({
          status: res.status,
          data: result,
        });
      }
    } catch (error) {
      setResponse({
        status: 0,
        data: null,
        error: error instanceof Error ? error.message : 'Failed to remove library',
      });
    } finally {
      setLoading(false);
    }
  };

  // Scanner functions
  const loadScanStats = async () => {
    try {
      const res = await fetch('/api/admin/scanner/stats');
      const result = await res.json();

      if (res.ok) {
        setScanStats(result);
      }
    } catch (error) {
      console.error('Failed to load scan stats:', error);
    }
  };

  const loadLibraryStats = async () => {
    try {
      const res = await fetch('/api/admin/scanner/library-stats');
      const result = await res.json();

      if (res.ok && result.library_stats) {
        setLibraryStats(result.library_stats);
      }
    } catch (error) {
      console.error('Failed to load library stats:', error);
    }
  };

  const loadScanJobs = async () => {
    try {
      const res = await fetch('/api/admin/scanner/status');
      const result = await res.json();

      if (res.ok && result.jobs) {
        setScanJobs(result.jobs);
        const activeScanLibraries = new Set<number>(
          result.jobs
            .filter((job: ScanJob) => job.status === 'running')
            .map((job: ScanJob) => job.library_id)
        );
        setScanningLibraries(activeScanLibraries);
      }
    } catch (error) {
      console.error('Failed to load scan jobs:', error);
    }
  };

  const startScan = async (libraryId: number) => {
    setScanLoading(true);
    try {
      const res = await fetch(`/api/admin/scanner/start/${libraryId}`, {
        method: 'POST',
      });
      const result = await res.json();

      setResponse({
        status: res.status,
        data: result,
      });

      if (res.ok) {
        setScanningLibraries((prev) => new Set([...prev, libraryId]));
        // Reload scan status after starting
        setTimeout(() => {
          loadScanJobs();
          loadScanStats();
        }, 1000);
      }
    } catch (error) {
      setResponse({
        status: 0,
        data: null,
        error: error instanceof Error ? error.message : 'Failed to start scan',
      });
    } finally {
      setScanLoading(false);
    }
  };

  const pauseScan = async (libraryId: number) => {
    setScanLoading(true);
    try {
      const res = await fetch(`/api/admin/scanner/pause/${libraryId}`, {
        method: 'POST',
      });
      const result = await res.json();

      setResponse({
        status: res.status,
        data: result,
      });

      if (res.ok) {
        setScanningLibraries((prev) => {
          const newSet = new Set(prev);
          newSet.delete(libraryId);
          return newSet;
        });
        // Reload scan status after pausing
        setTimeout(() => {
          loadScanJobs();
          loadScanStats();
        }, 1000);
      }
    } catch (error) {
      setResponse({
        status: 0,
        data: null,
        error: error instanceof Error ? error.message : 'Failed to pause scan',
      });
    } finally {
      setScanLoading(false);
    }
  };

  const resumeScan = async (libraryId: number) => {
    setScanLoading(true);
    try {
      const res = await fetch(`/api/admin/scanner/resume/${libraryId}`, {
        method: 'POST',
      });
      const result = await res.json();

      setResponse({
        status: res.status,
        data: result,
      });

      if (res.ok) {
        setScanningLibraries((prev) => new Set([...prev, libraryId]));
        // Reload scan status after resuming
        setTimeout(() => {
          loadScanJobs();
          loadScanStats();
        }, 1000);
      }
    } catch (error) {
      setResponse({
        status: 0,
        data: null,
        error: error instanceof Error ? error.message : 'Failed to resume scan',
      });
    } finally {
      setScanLoading(false);
    }
  };

  const formatBytes = (bytes: number) => {
    if (bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
  };

  const getScanJobForLibrary = (libraryId: number) => {
    return scanJobs.find(
      (job) => job.library_id === libraryId && (job.status === 'running' || job.status === 'paused')
    );
  };

  // Helper to determine if a scan is paused - used in the UI logic

  return (
    <div className="bg-slate-900 rounded-lg p-6 shadow-xl">
      <div className="flex items-center justify-between mb-4">
        <h2 className="text-xl font-semibold text-white">📚 Media Libraries</h2>
        <button
          onClick={() => setShowAddForm(!showAddForm)}
          className="bg-green-600 hover:bg-green-700 text-white px-4 py-2 rounded text-sm transition-colors"
          disabled={loading}
        >
          {showAddForm ? 'Cancel' : 'Add Library'}
        </button>
      </div>

      {/* Scanner Stats - System-wide */}
      {scanStats && (
        <div className="bg-slate-800 rounded-lg p-4 mb-4">
          <h3 className="text-lg font-medium text-white mb-2 flex items-center gap-2">
            🔍 System Scanner Status
          </h3>
          <div className="grid grid-cols-3 gap-4 text-sm">
            <div className="text-center">
              <div className="text-2xl font-bold text-blue-400">{scanStats.active_scans}</div>
              <div className="text-slate-300">Active Scans</div>
            </div>
            <div className="text-center">
              <div className="text-2xl font-bold text-green-400">
                {scanStats.total_files_scanned.toLocaleString()}
              </div>
              <div className="text-slate-300">Total Files Scanned</div>
            </div>
            <div className="text-center">
              <div className="text-2xl font-bold text-purple-400">
                {formatBytes(scanStats.total_bytes_scanned)}
              </div>
              <div className="text-slate-300">Total Data Processed</div>
            </div>
          </div>
        </div>
      )}

      {/* Add Library Form */}
      {showAddForm && (
        <div className="bg-slate-800 rounded-lg p-4 mb-4">
          <h3 className="text-lg font-medium text-white mb-3">Add New Media Library</h3>

          <div className="space-y-3">
            <div>
              <label className="block text-sm font-medium text-slate-300 mb-1">
                Directory Path
              </label>
              <div className="flex gap-2 mb-2">
                <input
                  type="text"
                  value={newLibrary.path}
                  onChange={(e) => setNewLibrary((prev) => ({ ...prev, path: e.target.value }))}
                  placeholder="/path/to/your/media/directory"
                  className="flex-1 bg-slate-700 text-white px-3 py-2 rounded border border-slate-600 focus:border-blue-500 focus:outline-none"
                />
                <button
                  onClick={handleDirectorySelect}
                  className="bg-blue-600 hover:bg-blue-700 text-white px-4 py-2 rounded text-sm transition-colors"
                  type="button"
                >
                  Browse
                </button>
              </div>

              {/* Common Path Suggestions */}
              <div className="mb-2">
                <span className="text-xs text-slate-400 mb-1 block">
                  Quick select common paths:
                </span>
                <div className="flex flex-wrap gap-1">
                  {getCommonPaths()
                    .slice(0, 4)
                    .map((path) => (
                      <button
                        key={path}
                        onClick={() => handleQuickSelect(path)}
                        className="text-xs bg-slate-600 hover:bg-slate-500 text-slate-200 px-2 py-1 rounded transition-colors"
                        type="button"
                      >
                        {path}
                      </button>
                    ))}
                </div>
              </div>
            </div>

            <div>
              <label className="block text-sm font-medium text-slate-300 mb-1">Library Type</label>
              <select
                value={newLibrary.type}
                onChange={(e) => setNewLibrary((prev) => ({ ...prev, type: e.target.value }))}
                className="w-full bg-slate-700 text-white px-3 py-2 rounded border border-slate-600 focus:border-blue-500 focus:outline-none"
              >
                <option value="movie">Movies</option>
                <option value="tv">TV Shows</option>
                <option value="music">Music</option>
              </select>
            </div>

            <div className="flex gap-2">
              <button
                onClick={addLibrary}
                disabled={loading || !newLibrary.path}
                className="bg-green-600 hover:bg-green-700 disabled:bg-slate-600 text-white px-4 py-2 rounded text-sm transition-colors"
              >
                Add Library
              </button>
              <button
                onClick={() => setShowAddForm(false)}
                className="bg-slate-600 hover:bg-slate-700 text-white px-4 py-2 rounded text-sm transition-colors"
              >
                Cancel
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Libraries List */}
      <div className="space-y-3 mb-4">
        {libraries.length === 0 ? (
          <div className="text-slate-400 text-center py-8">
            No media libraries configured yet. Add one to get started!
          </div>
        ) : (
          libraries.map((library) => {
            const scanJob = getScanJobForLibrary(library.id);
            const isScanning = scanningLibraries.has(library.id);

            return (
              <div key={library.id} className="bg-slate-800 rounded-lg p-4">
                <div className="flex items-center justify-between mb-2">
                  <div className="flex-1">
                    <div className="flex items-center gap-2 mb-1">
                      <span className="text-white font-medium">{library.path}</span>
                      <span
                        className={`px-2 py-1 rounded text-xs font-medium ${
                          library.type === 'movie'
                            ? 'bg-blue-600 text-blue-100'
                            : library.type === 'tv'
                              ? 'bg-purple-600 text-purple-100'
                              : 'bg-green-600 text-green-100'
                        }`}
                      >
                        {library.type.toUpperCase()}
                      </span>
                      {isScanning && (
                        <span className="px-2 py-1 bg-yellow-600 text-yellow-100 rounded text-xs font-medium animate-pulse">
                          SCANNING
                        </span>
                      )}
                    </div>
                    <div className="text-slate-400 text-sm flex gap-6">
                      <span>Added: {new Date(library.created_at).toLocaleDateString()}</span>

                      {/* Show library stats if available */}
                      {libraryStats[library.id] && (
                        <>
                          <span>
                            Files: {libraryStats[library.id].total_files?.toLocaleString() || 0}
                          </span>
                          <span>Size: {formatBytes(libraryStats[library.id].total_size || 0)}</span>
                        </>
                      )}
                    </div>
                  </div>

                  <div className="flex gap-2 ml-4">
                    {isScanning ? (
                      <button
                        onClick={() => pauseScan(library.id)}
                        disabled={scanLoading}
                        className="bg-orange-600 hover:bg-orange-700 disabled:bg-slate-600 text-white px-3 py-1 rounded text-sm transition-colors"
                      >
                        {scanLoading ? '...' : 'Pause Scan'}
                      </button>
                    ) : scanJobs.some(
                        (job) => job.library_id === library.id && job.status === 'paused'
                      ) ? (
                      <button
                        onClick={() => resumeScan(library.id)}
                        disabled={scanLoading}
                        className="bg-green-600 hover:bg-green-700 disabled:bg-slate-600 text-white px-3 py-1 rounded text-sm transition-colors"
                      >
                        {scanLoading ? '...' : 'Resume Scan'}
                      </button>
                    ) : (
                      <button
                        onClick={() => startScan(library.id)}
                        disabled={scanLoading}
                        className="bg-blue-600 hover:bg-blue-700 disabled:bg-slate-600 text-white px-3 py-1 rounded text-sm transition-colors"
                      >
                        {scanLoading ? '...' : 'Scan'}
                      </button>
                    )}

                    <button
                      onClick={() => removeLibrary(library.id)}
                      className="bg-red-600 hover:bg-red-700 text-white px-3 py-1 rounded text-sm transition-colors"
                      disabled={loading}
                    >
                      Remove
                    </button>
                  </div>
                </div>

                {/* Scan Progress */}
                {(scanJob ||
                  (libraryStats[library.id] && libraryStats[library.id].scan_status)) && (
                  <div className="mt-3 bg-slate-700 rounded p-3">
                    <div className="flex justify-between items-center mb-2">
                      <span className="text-sm font-medium text-white">
                        Scan {scanJob ? scanJob.status : libraryStats[library.id]?.scan_status}
                      </span>
                      {scanJob && scanJob.started_at && (
                        <span className="text-xs text-slate-300">
                          Started: {new Date(scanJob.started_at).toLocaleTimeString()}
                        </span>
                      )}
                    </div>

                    <div className="space-y-2">
                      {/* Use either scanJob data or libraryStats data, giving preference to scanJob */}
                      {((scanJob && scanJob.files_found > 0) ||
                        (libraryStats[library.id] && libraryStats[library.id].files_found)) && (
                        <div className="flex justify-between text-sm">
                          <span className="text-slate-300">Progress:</span>
                          <span className="text-white">
                            {(scanJob && scanJob.files_processed) ||
                              (libraryStats[library.id] &&
                                libraryStats[library.id].files_processed) ||
                              0}{' '}
                            /
                            {(scanJob && scanJob.files_found) ||
                              (libraryStats[library.id] && libraryStats[library.id].files_found) ||
                              0}{' '}
                            files
                            {((scanJob && scanJob.files_found > 0) ||
                              (libraryStats[library.id] &&
                                libraryStats[library.id].files_found)) && (
                              <span className="text-slate-400 ml-2">
                                (
                                {Math.round(
                                  (((scanJob && scanJob.files_processed) ||
                                    (libraryStats[library.id] &&
                                      libraryStats[library.id].files_processed) ||
                                    0) /
                                    ((scanJob && scanJob.files_found) ||
                                      (libraryStats[library.id] &&
                                        libraryStats[library.id].files_found) ||
                                      1)) *
                                    100
                                )}
                                %)
                              </span>
                            )}
                          </span>
                        </div>
                      )}

                      {((scanJob && scanJob.bytes_processed > 0) ||
                        (libraryStats[library.id] && libraryStats[library.id].bytes_processed)) && (
                        <div className="flex justify-between text-sm">
                          <span className="text-slate-300">Data processed:</span>
                          <span className="text-white">
                            {formatBytes(
                              (scanJob && scanJob.bytes_processed) ||
                                (libraryStats[library.id] &&
                                  libraryStats[library.id].bytes_processed) ||
                                0
                            )}
                          </span>
                        </div>
                      )}

                      {scanJob && scanJob.errors && scanJob.errors.length > 0 && (
                        <div className="text-sm">
                          <span className="text-red-400">Errors: {scanJob.errors.length}</span>
                        </div>
                      )}

                      {/* Display progress bar */}
                      {((scanJob && scanJob.files_found > 0) ||
                        (libraryStats[library.id] && libraryStats[library.id].files_found)) && (
                        <div className="w-full bg-slate-600 rounded-full h-2">
                          <div
                            className={`h-2 rounded-full transition-all duration-300 ${
                              (scanJob && scanJob.status === 'paused') ||
                              (libraryStats[library.id] &&
                                libraryStats[library.id].scan_status === 'paused')
                                ? 'bg-amber-500'
                                : 'bg-blue-500'
                            }`}
                            style={{
                              width: `${Math.round(
                                (((scanJob && scanJob.files_processed) ||
                                  (libraryStats[library.id] &&
                                    libraryStats[library.id].files_processed) ||
                                  0) /
                                  ((scanJob && scanJob.files_found) ||
                                    (libraryStats[library.id] &&
                                      libraryStats[library.id].files_found) ||
                                    1)) *
                                  100
                              )}%`,
                            }}
                          />
                        </div>
                      )}
                    </div>
                  </div>
                )}
              </div>
            );
          })
        )}
      </div>

      {/* Refresh Button */}
      <div className="flex gap-2 mb-4">
        <button
          onClick={loadLibraries}
          className="bg-blue-600 hover:bg-blue-700 text-white px-4 py-2 rounded text-sm transition-colors"
          disabled={loading}
        >
          {loading ? '🔄 Loading...' : 'Refresh Libraries'}
        </button>
      </div>

      {/* Response Display */}
      {response && (
        <div className="bg-slate-800 rounded p-4">
          <div className="flex items-center gap-2 mb-2">
            <span className="text-sm font-medium text-white">Last Action:</span>
            <span
              className={`text-sm font-bold ${
                response.status >= 200 && response.status < 300
                  ? 'text-green-400'
                  : response.status >= 400
                    ? 'text-red-400'
                    : 'text-yellow-400'
              }`}
            >
              {response.status || 'ERROR'}
            </span>
          </div>

          {response.error && (
            <div className="mb-2">
              <span className="text-sm font-medium text-white">Error:</span>
              <div className="text-red-400 text-sm font-mono bg-slate-700 p-2 rounded mt-1">
                {response.error}
              </div>
            </div>
          )}

          <div>
            <span className="text-sm font-medium text-white">Response:</span>
            <pre className="text-slate-300 text-xs font-mono bg-slate-700 p-2 rounded mt-1 overflow-auto max-h-40">
              {typeof response.data === 'string'
                ? response.data
                : JSON.stringify(response.data, null, 2)}
            </pre>
          </div>
        </div>
      )}
    </div>
  );
};

export default MediaLibraryManager;
