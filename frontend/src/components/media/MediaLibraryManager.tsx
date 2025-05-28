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

  // Nerd panel state
  const [expandedNerdPanels, setExpandedNerdPanels] = useState<Set<number>>(new Set());

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

  const formatHumanReadableETA = (seconds: number) => {
    if (seconds < 60) return `${seconds} seconds remaining`;
    if (seconds < 3600) {
      const minutes = Math.floor(seconds / 60);
      return `${minutes} minute${minutes !== 1 ? 's' : ''} remaining`;
    }

    const hours = Math.floor(seconds / 3600);
    const minutes = Math.floor((seconds % 3600) / 60);

    if (hours < 24) {
      if (minutes === 0) {
        return `${hours} hour${hours !== 1 ? 's' : ''} remaining`;
      }
      return `${hours}h ${minutes}m remaining`;
    }

    const days = Math.floor(hours / 24);
    const remainingHours = hours % 24;
    if (remainingHours === 0) {
      return `${days} day${days !== 1 ? 's' : ''} remaining`;
    }
    return `${days}d ${remainingHours}h remaining`;
  };

  const toggleNerdPanel = (libraryId: number) => {
    setExpandedNerdPanels((prev) => {
      const newSet = new Set(prev);
      if (newSet.has(libraryId)) {
        newSet.delete(libraryId);
      } else {
        newSet.add(libraryId);
      }
      return newSet;
    });
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

            // Calculate progress percentage first since status variables depend on it
            // Use the backend's progress field if available, otherwise calculate from files
            const progressPercent = scanJob
              ? scanJob.progress !== undefined && scanJob.progress !== null
                ? Math.min(scanJob.progress, 100) // Cap at 100%
                : scanJob.files_found > 0
                  ? Math.min(Math.round((scanJob.files_processed / scanJob.files_found) * 100), 100)
                  : 0
              : 0;

            const isScanning = scanJob?.status === 'running' && progressPercent < 100;
            const isPaused = scanJob?.status === 'paused';
            const isFailed = scanJob?.status === 'failed';
            const isCompleted = scanJob?.status === 'completed' || progressPercent >= 100;
            const isNerdPanelExpanded = expandedNerdPanels.has(library.id);
            const progressData = scanJob ? scanProgress.get(Number(scanJob.id)) : null;

            // DEBUGGING: Log job status for specific library or all
            // Replace YOUR_PROBLEMATIC_LIBRARY_ID with the actual ID if known, otherwise logs for all
            // if (library.id === YOUR_PROBLEMATIC_LIBRARY_ID) {
            /* console.log(
              `[Debug] LibID: ${library.id}, JobID: ${scanJob?.id}, Status: ${scanJob?.status}, isCompleted: ${isCompleted}, Files: ${scanJob?.files_processed}/${scanJob?.files_found}, Progress: ${scanJob?.progress}%`
            );
            */
            // console.log('Full scanJob for LibID ' + library.id + ':', JSON.stringify(scanJob));
            // }

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
                        <span className="scanning-badge px-2 py-1 bg-yellow-600 text-yellow-100 rounded text-xs font-medium animate-pulse">
                          SCANNING
                        </span>
                      )}
                      {isPaused && (
                        <span className="px-2 py-1 bg-amber-600 text-amber-100 rounded text-xs font-medium">
                          PAUSED
                        </span>
                      )}
                      {isFailed && (
                        <span className="px-2 py-1 bg-red-600 text-red-100 rounded text-xs font-medium">
                          FAILED
                        </span>
                      )}
                      {isCompleted && (
                        <span className="px-2 py-1 bg-green-600 text-green-100 rounded text-xs font-medium">
                          COMPLETED
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

                {/* Simplified Scan Progress Display */}
                {scanJob && (
                  <div className="mt-3 bg-slate-700 rounded p-3">
                    {/* Main Progress Info */}
                    <div className="flex justify-between items-center mb-3">
                      <div className="flex items-center gap-3">
                        <span className="text-sm font-medium text-white">
                          {scanJob.files_processed.toLocaleString()} /{' '}
                          {(isCompleted && scanJob.files_processed > scanJob.files_found
                            ? scanJob.files_processed
                            : scanJob.files_found
                          ).toLocaleString()}{' '}
                          files
                        </span>
                        <span className="text-xs text-slate-300">({progressPercent}%)</span>
                      </div>

                      <div className="flex items-center gap-3">
                        {/* Human-readable ETA */}
                        {isScanning && progressData?.estimatedTimeLeft && (
                          <span className="text-xs text-blue-400 font-medium">
                            {formatHumanReadableETA(progressData.estimatedTimeLeft)}
                          </span>
                        )}

                        {/* Nerd Panel Toggle */}
                        <button
                          onClick={() => toggleNerdPanel(library.id)}
                          className="text-xs text-slate-400 hover:text-slate-200 transition-colors flex items-center gap-1"
                          title="Toggle detailed information"
                        >
                          <span>Details</span>
                          <svg
                            className={`w-3 h-3 transition-transform duration-200 ${isNerdPanelExpanded ? 'rotate-180' : ''}`}
                            fill="none"
                            stroke="currentColor"
                            viewBox="0 0 24 24"
                          >
                            <path
                              strokeLinecap="round"
                              strokeLinejoin="round"
                              strokeWidth={2}
                              d="M19 9l-7 7-7-7"
                            />
                          </svg>
                        </button>
                      </div>
                    </div>

                    {/* Animated Progress Bar */}
                    <div className="w-full bg-slate-600 rounded-full h-3 overflow-hidden shadow-inner">
                      <div
                        className={`h-3 rounded-full transition-all duration-500 ease-out relative overflow-hidden ${
                          isFailed
                            ? 'bg-red-500 shadow-red-500/20'
                            : isPaused
                              ? 'bg-amber-500 shadow-amber-500/20'
                              : isCompleted
                                ? 'bg-green-500 shadow-green-500/20'
                                : 'bg-gradient-to-r from-blue-500 to-blue-400'
                        } ${isScanning ? 'shadow-lg shadow-blue-500/30' : 'shadow-md'}`}
                        style={{
                          width: `${progressPercent}%`,
                          animation: isScanning ? 'progress-glow 3s ease-in-out infinite' : 'none',
                        }}
                      >
                        {/* Subtle shimmer effect for active scans */}
                        {isScanning && (
                          <div
                            className="absolute top-0 left-0 h-full bg-gradient-to-r from-transparent via-blue-200 to-transparent opacity-20"
                            style={{
                              width: '60px',
                              animation: 'shimmer-slide 3s ease-in-out infinite',
                            }}
                          ></div>
                        )}
                      </div>
                    </div>

                    {/* Error Message (if any) */}
                    {scanJob.error_message && (
                      <div className="mt-2 text-sm">
                        <span className="text-red-400">Error:</span>
                        <span className="text-slate-300 ml-2">{scanJob.error_message}</span>
                      </div>
                    )}

                    {/* Nerd Panel - Detailed Information */}
                    <div
                      className={`overflow-hidden transition-all duration-300 ease-in-out ${
                        isNerdPanelExpanded ? 'max-h-96 opacity-100 mt-4' : 'max-h-0 opacity-0'
                      }`}
                    >
                      <div className="border-t border-slate-600 pt-4">
                        <h4 className="text-sm font-medium text-white mb-3 flex items-center gap-2">
                          🤓 Detailed Information
                        </h4>

                        <div className="grid grid-cols-2 gap-4 text-xs">
                          {/* Job Information */}
                          <div className="space-y-2">
                            <div className="text-slate-400 font-medium">Job Details</div>
                            <div className="space-y-1">
                              <div className="flex justify-between">
                                <span className="text-slate-300">Job ID:</span>
                                <span className="text-white font-mono">#{scanJob.id}</span>
                              </div>
                              <div className="flex justify-between">
                                <span className="text-slate-300">Status:</span>
                                <span className="text-white">{scanJob.status}</span>
                              </div>
                              <div className="flex justify-between">
                                <span className="text-slate-300">Started:</span>
                                <span className="text-white">
                                  {scanJob.started_at
                                    ? new Date(scanJob.started_at).toLocaleString()
                                    : 'N/A'}
                                </span>
                              </div>
                              <div className="flex justify-between">
                                <span className="text-slate-300">Last Update:</span>
                                <span className="text-white">
                                  {scanJob.updated_at
                                    ? new Date(scanJob.updated_at).toLocaleString()
                                    : 'N/A'}
                                </span>
                              </div>
                            </div>
                          </div>

                          {/* Performance Metrics */}
                          <div className="space-y-2">
                            <div className="text-slate-400 font-medium">Performance</div>
                            <div className="space-y-1">
                              <div className="flex justify-between">
                                <span className="text-slate-300">Data Processed:</span>
                                <span className="text-white">
                                  {formatBytes(scanJob.bytes_processed)}
                                </span>
                              </div>
                              {progressData && (
                                <>
                                  <div className="flex justify-between">
                                    <span className="text-slate-300">Active Workers:</span>
                                    <span className="text-white">{progressData.activeWorkers}</span>
                                  </div>
                                  <div className="flex justify-between">
                                    <span className="text-slate-300">Queue Depth:</span>
                                    <span className="text-white">{progressData.queueDepth}</span>
                                  </div>
                                  <div className="flex justify-between">
                                    <span className="text-slate-300">Last Seen:</span>
                                    <span className="text-white">
                                      {progressData.lastUpdate.toLocaleTimeString()}
                                    </span>
                                  </div>
                                </>
                              )}
                            </div>
                          </div>
                        </div>

                        {/* Progress Timeline */}
                        {scanJob.started_at && (
                          <div className="mt-4 pt-3 border-t border-slate-600">
                            <div className="text-slate-400 font-medium text-xs mb-2">Timeline</div>
                            <div className="text-xs text-slate-300">
                              <div>Started: {new Date(scanJob.started_at).toLocaleString()}</div>
                              {scanJob.completed_at && (
                                <div>
                                  Completed: {new Date(scanJob.completed_at).toLocaleString()}
                                </div>
                              )}
                              {isScanning && progressData?.estimatedTimeLeft && (
                                <div>
                                  Estimated completion:{' '}
                                  {new Date(
                                    Date.now() + progressData.estimatedTimeLeft * 1000
                                  ).toLocaleString()}
                                </div>
                              )}
                            </div>
                          </div>
                        )}
                      </div>
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

      {/* Add custom CSS for smooth shimmer animation */}
      <style>{`
        @keyframes shimmer-slide {
          0% {
            transform: translateX(-60px);
            opacity: 0;
          }
          30% {
            opacity: 0.5;
          }
          70% {
            opacity: 0.5;
          }
          100% {
            transform: translateX(calc(100% + 60px));
            opacity: 0;
          }
        }
        
        @keyframes progress-glow {
          0%, 100% {
            box-shadow: 0 0 5px rgba(59, 130, 246, 0.3);
          }
          50% {
            box-shadow: 0 0 15px rgba(59, 130, 246, 0.5);
          }
        }
      `}</style>
    </div>
  );
};

export default MediaLibraryManager;
