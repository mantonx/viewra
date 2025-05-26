import { useState, useEffect } from 'react';
import type { MediaLibrary, ScanJob, ScanStats, LibraryStats } from '@/types/media.types';
import type { ApiResponse } from '@/types/system.types';

// Add interface for scan events
interface ScanEvent {
  id: string;
  type: string;
  source: string;
  title: string;
  message: string;
  data: {
    jobId?: number;
    libraryId?: number;
    filesProcessed?: number;
    bytesProcessed?: number;
    progress?: number;
    activeWorkers?: number;
    queueDepth?: number;
    filesFound?: number;
    [key: string]: unknown;
  };
  timestamp: string;
}

const MediaLibraryManager = () => {
  const [libraries, setLibraries] = useState<MediaLibrary[]>([]);
  const [loading, setLoading] = useState(false);
  const [showAddForm, setShowAddForm] = useState(false);
  const [newLibrary, setNewLibrary] = useState({
    path: '',
    type: 'movie',
  });
  const [response, setResponse] = useState<ApiResponse | null>(null);

  // Scanner state - simplified to use current jobs per library
  const [currentJobs, setCurrentJobs] = useState<Record<number, ScanJob>>({});
  const [scanStats, setScanStats] = useState<ScanStats | null>(null);
  const [libraryStats, setLibraryStats] = useState<LibraryStats>({});
  const [scanLoading, setScanLoading] = useState<Set<number>>(new Set());

  // Real-time scan progress tracking
  const [scanProgress, setScanProgress] = useState<
    Map<
      number,
      {
        filesProcessed: number;
        bytesProcessed: number;
        progress: number;
        activeWorkers: number;
        queueDepth: number;
        lastUpdate: Date;
        startTime?: Date;
        estimatedTimeLeft?: number;
      }
    >
  >(new Map());

  // Load libraries on component mount
  useEffect(() => {
    loadLibraries();
    loadScanStats();
    loadCurrentJobs();
    loadLibraryStats();
  }, []);

  // Real-time event streaming for scan updates
  useEffect(() => {
    let eventSource: EventSource | null = null;

    const connectToScanEvents = () => {
      try {
        // Connect to event stream with scan event filters
        const streamUrl =
          '/api/events/stream?types=scan.started,scan.progress,scan.completed,scan.failed,scan.resumed,scan.paused,library.deleted';
        console.log('Connecting to scan event stream:', streamUrl);
        eventSource = new EventSource(streamUrl);

        eventSource.onopen = () => {
          console.log('Connected to scan event stream');
        };

        eventSource.onmessage = (event) => {
          try {
            const data = JSON.parse(event.data);

            if (data.type === 'connected') {
              console.log('Scan event stream connected:', data.message);
              return;
            }

            if (data.type === 'event' && data.data) {
              const scanEvent: ScanEvent = data.data;
              handleScanEvent(scanEvent);
            }
          } catch (err) {
            console.error('Error parsing scan event data:', err);
          }
        };

        eventSource.onerror = (error) => {
          console.error('Scan EventSource error:', error);
          // Attempt to reconnect after a delay
          setTimeout(() => {
            if (eventSource) {
              eventSource.close();
            }
            connectToScanEvents();
          }, 5000);
        };
      } catch (err) {
        console.error('Failed to connect to scan event stream:', err);
      }
    };

    const handleScanEvent = (event: ScanEvent) => {
      console.log('Received scan event:', event.type, event.data);

      // Handle library deletion events
      if (event.type === 'library.deleted' && event.data.libraryId) {
        const libraryId = event.data.libraryId;

        // Add removal animation before updating state
        const libraryElement = document.querySelector(`[data-library-id="${libraryId}"]`);
        if (libraryElement) {
          libraryElement.classList.add('library-removing');
          setTimeout(() => {
            // Clean up state after animation
            setCurrentJobs((prev) => {
              const newJobs = { ...prev };
              delete newJobs[libraryId];
              return newJobs;
            });
            setLibraryStats((prev) => {
              const newStats = { ...prev };
              delete newStats[libraryId];
              return newStats;
            });
            setScanProgress((prev) => {
              const newMap = new Map(prev);
              // Remove progress tracking for jobs related to this library
              const libraryJob = currentJobs[libraryId];
              if (libraryJob) {
                newMap.delete(Number(libraryJob.id));
              }
              return newMap;
            });

            // Reload libraries to get updated list
            loadLibraries();
          }, 300); // Match CSS animation duration
        } else {
          // No animation element found, update immediately
          setCurrentJobs((prev) => {
            const newJobs = { ...prev };
            delete newJobs[libraryId];
            return newJobs;
          });
          setLibraryStats((prev) => {
            const newStats = { ...prev };
            delete newStats[libraryId];
            return newStats;
          });
          loadLibraries();
        }
        return;
      }

      // Update scan jobs and stats when scan events occur
      if (event.type.startsWith('scan.')) {
        // Reload current jobs and stats
        loadCurrentJobs();
        loadScanStats();
        loadLibraryStats();

        // Handle progress events specifically
        if (event.type === 'scan.progress' && event.data.jobId) {
          const jobId = event.data.jobId;
          const now = new Date();

          setScanProgress((prev) => {
            const current = prev.get(jobId);
            const newProgress = {
              filesProcessed: event.data.filesProcessed || 0,
              bytesProcessed: event.data.bytesProcessed || 0,
              progress: event.data.progress || 0,
              activeWorkers: event.data.activeWorkers || 0,
              queueDepth: event.data.queueDepth || 0,
              lastUpdate: now,
              startTime: current?.startTime || now,
              estimatedTimeLeft: current?.estimatedTimeLeft,
            };

            // Calculate estimated time left
            if (current && newProgress.filesProcessed > current.filesProcessed) {
              const timeElapsed = now.getTime() - (current.startTime || now).getTime();
              const filesProcessed = newProgress.filesProcessed;
              const totalFiles = event.data.filesFound || 1;
              const filesRemaining = totalFiles - filesProcessed;

              if (filesProcessed > 0 && filesRemaining > 0) {
                const avgTimePerFile = timeElapsed / filesProcessed;
                newProgress.estimatedTimeLeft = Math.round(
                  (avgTimePerFile * filesRemaining) / 1000
                ); // in seconds
              }
            }

            const newMap = new Map(prev);
            newMap.set(jobId, newProgress);
            return newMap;
          });
        }

        // Handle scan start events
        if (
          event.type === 'scan.started' &&
          event.data.scanJobId &&
          typeof event.data.scanJobId === 'number'
        ) {
          const jobId = event.data.scanJobId;
          setScanProgress((prev) => {
            const newMap = new Map(prev);
            newMap.set(jobId, {
              filesProcessed: 0,
              bytesProcessed: 0,
              progress: 0,
              activeWorkers: 0,
              queueDepth: 0,
              lastUpdate: new Date(),
              startTime: new Date(),
            });
            return newMap;
          });
        }

        // Clean up progress tracking for completed/failed scans
        if (
          (event.type === 'scan.completed' || event.type === 'scan.failed') &&
          event.data.scanJobId &&
          typeof event.data.scanJobId === 'number'
        ) {
          const jobId = event.data.scanJobId;
          setScanProgress((prev) => {
            const newMap = new Map(prev);
            newMap.delete(jobId);
            return newMap;
          });
        }
      }
    };

    connectToScanEvents();

    return () => {
      if (eventSource) {
        eventSource.close();
      }
    };
  }, [currentJobs]); // Add currentJobs as dependency

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
                loadCurrentJobs();
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

  const loadCurrentJobs = async () => {
    try {
      const res = await fetch('/api/admin/scanner/current-jobs');
      const result = await res.json();

      if (res.ok && result.current_jobs) {
        // Extract job data from nested structure: {libraryId: {job: actualJobData}}
        const extractedJobs: Record<number, ScanJob> = {};

        for (const [libraryId, jobWrapper] of Object.entries(result.current_jobs)) {
          if (jobWrapper && typeof jobWrapper === 'object' && 'job' in jobWrapper) {
            const wrapper = jobWrapper as { job: ScanJob };
            extractedJobs[Number(libraryId)] = wrapper.job;
          }
        }

        setCurrentJobs(extractedJobs);
      }
    } catch (error) {
      console.error('Failed to load current jobs:', error);
    }
  };

  const startScan = async (libraryId: number) => {
    setScanLoading((prev) => new Set([...prev, libraryId]));
    try {
      const res = await fetch(`/api/admin/scanner/start/${libraryId}`, {
        method: 'POST',
      });

      // Check if response is ok before trying to parse JSON
      if (!res.ok) {
        const errorText = await res.text();
        throw new Error(`HTTP ${res.status}: ${errorText}`);
      }

      const result = await res.json();

      setResponse({
        status: res.status,
        data: result,
      });

      // Reload scan status after starting
      setTimeout(() => {
        loadCurrentJobs();
        loadScanStats();
      }, 1000);
    } catch (error) {
      console.error('Start scan error:', error);
      setResponse({
        status: 0,
        data: null,
        error: error instanceof Error ? error.message : 'Failed to start scan',
      });
    } finally {
      setScanLoading((prev) => {
        const newSet = new Set(prev);
        newSet.delete(libraryId);
        return newSet;
      });
    }
  };

  const pauseScan = async (libraryId: number) => {
    setScanLoading((prev) => new Set([...prev, libraryId]));
    try {
      const res = await fetch(`/api/admin/scanner/pause/${libraryId}`, {
        method: 'POST',
      });

      // Check if response is ok before trying to parse JSON
      if (!res.ok) {
        const errorText = await res.text();
        throw new Error(`HTTP ${res.status}: ${errorText}`);
      }

      const result = await res.json();

      setResponse({
        status: res.status,
        data: result,
      });

      // Reload scan status after pausing
      setTimeout(() => {
        loadCurrentJobs();
        loadScanStats();
      }, 1000);
    } catch (error) {
      console.error('Pause scan error:', error);
      setResponse({
        status: 0,
        data: null,
        error: error instanceof Error ? error.message : 'Failed to pause scan',
      });
    } finally {
      setScanLoading((prev) => {
        const newSet = new Set(prev);
        newSet.delete(libraryId);
        return newSet;
      });
    }
  };

  const resumeScan = async (libraryId: number) => {
    setScanLoading((prev) => new Set([...prev, libraryId]));
    try {
      // Use the admin resume endpoint which handles finding the right job
      const res = await fetch(`/api/admin/scanner/resume/${libraryId}`, {
        method: 'POST',
      });

      // Check if response is ok before trying to parse JSON
      if (!res.ok) {
        const errorText = await res.text();
        throw new Error(`HTTP ${res.status}: ${errorText}`);
      }

      const result = await res.json();

      setResponse({
        status: res.status,
        data: result,
      });

      // Reload scan status after resuming
      setTimeout(() => {
        loadCurrentJobs();
        loadScanStats();
      }, 1000);
    } catch (error) {
      console.error('Resume scan error:', error);
      setResponse({
        status: 0,
        data: null,
        error: error instanceof Error ? error.message : 'Failed to resume scan',
      });
    } finally {
      setScanLoading((prev) => {
        const newSet = new Set(prev);
        newSet.delete(libraryId);
        return newSet;
      });
    }
  };

  const formatBytes = (bytes: number) => {
    if (bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
  };

  const formatTimeLeft = (seconds: number) => {
    if (seconds < 60) return `${seconds}s`;
    if (seconds < 3600) return `${Math.floor(seconds / 60)}m ${seconds % 60}s`;
    const hours = Math.floor(seconds / 3600);
    const minutes = Math.floor((seconds % 3600) / 60);
    return `${hours}h ${minutes}m`;
  };

  const getScanJobForLibrary = (libraryId: number) => {
    // Simply return the current job for this library
    return currentJobs[libraryId];
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
          <div className="empty-state text-slate-400 text-center py-8">
            No media libraries configured yet. Add one to get started!
          </div>
        ) : (
          libraries.map((library) => {
            const scanJob = getScanJobForLibrary(library.id);
            const isScanning = scanJob?.status === 'running';

            return (
              <div
                key={library.id}
                data-library-id={library.id}
                className="library-item bg-slate-800 rounded-lg p-4"
              >
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
                        <span className="scanning-badge px-2 py-1 bg-yellow-600 text-yellow-100 rounded text-xs font-medium">
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
                        disabled={scanLoading.has(library.id)}
                        className="library-button bg-orange-600 hover:bg-orange-700 disabled:bg-slate-600 text-white px-3 py-1 rounded text-sm transition-colors"
                      >
                        {scanLoading.has(library.id) ? '...' : 'Pause Scan'}
                      </button>
                    ) : scanJob && (scanJob.status === 'paused' || scanJob.status === 'failed') ? (
                      <button
                        onClick={() => resumeScan(library.id)}
                        disabled={scanLoading.has(library.id)}
                        className="library-button bg-green-600 hover:bg-green-700 disabled:bg-slate-600 text-white px-3 py-1 rounded text-sm transition-colors"
                      >
                        {scanLoading.has(library.id) ? '...' : 'Resume Scan'}
                      </button>
                    ) : (
                      <button
                        onClick={() => startScan(library.id)}
                        disabled={scanLoading.has(library.id)}
                        className="library-button bg-blue-600 hover:bg-blue-700 disabled:bg-slate-600 text-white px-3 py-1 rounded text-sm transition-colors"
                      >
                        {scanLoading.has(library.id) ? '...' : 'Scan'}
                      </button>
                    )}

                    <button
                      onClick={() => removeLibrary(library.id)}
                      className="library-button bg-red-600 hover:bg-red-700 text-white px-3 py-1 rounded text-sm transition-colors"
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
                      <div className="flex gap-4 text-xs text-slate-300">
                        {scanJob && scanJob.started_at && (
                          <span>Started: {new Date(scanJob.started_at).toLocaleTimeString()}</span>
                        )}
                        {scanJob && scanProgress.get(Number(scanJob.id))?.estimatedTimeLeft && (
                          <span className="text-blue-400">
                            ETA:{' '}
                            {formatTimeLeft(
                              scanProgress.get(Number(scanJob.id))!.estimatedTimeLeft!
                            )}
                          </span>
                        )}
                      </div>
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

                      {/* Real-time worker information */}
                      {scanJob && scanProgress.get(Number(scanJob.id)) && (
                        <div className="flex justify-between text-sm">
                          <span className="text-slate-300">Workers:</span>
                          <span className="text-white">
                            {scanProgress.get(Number(scanJob.id))!.activeWorkers} active
                            {scanProgress.get(Number(scanJob.id))!.queueDepth > 0 && (
                              <span className="text-slate-400 ml-2">
                                ({scanProgress.get(Number(scanJob.id))!.queueDepth} queued)
                              </span>
                            )}
                          </span>
                        </div>
                      )}

                      {scanJob && scanJob.error_message && (
                        <div className="text-sm">
                          <span className="text-red-400">Error:</span>
                          <span className="text-slate-300 ml-2">{scanJob.error_message}</span>
                        </div>
                      )}

                      {/* Display progress bar */}
                      {((scanJob && scanJob.files_found > 0) ||
                        (libraryStats[library.id] && libraryStats[library.id].files_found)) && (
                        <div className="w-full bg-slate-600 rounded-full h-2">
                          <div
                            className={`progress-bar h-2 rounded-full transition-all duration-300 ${
                              (scanJob &&
                                (scanJob.status === 'paused' || scanJob.status === 'failed')) ||
                              (libraryStats[library.id] &&
                                (libraryStats[library.id].scan_status === 'paused' ||
                                  libraryStats[library.id].scan_status === 'failed'))
                                ? scanJob?.status === 'failed' ||
                                  libraryStats[library.id]?.scan_status === 'failed'
                                  ? 'bg-red-500'
                                  : 'bg-amber-500'
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

      {/* Real-time updates - no refresh needed */}

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
