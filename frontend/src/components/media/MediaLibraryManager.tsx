import { useState, useEffect } from 'react';
import type { MediaLibrary, ScanJob, ScanStats, LibraryStats } from '@/types/media.types';
import type { ApiResponse } from '@/types/system.types';
import { Tooltip } from 'react-tooltip';

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
    totalFiles?: number;
    remainingFiles?: number;
    [key: string]: unknown;
  };
  timestamp: string;
}

// Add interface for monitoring status
interface MonitoredLibrary {
  id: number;
  path: string;
  type: string;
  last_scan_job_id: number;
  start_time: string;
  files_processed: number;
  status: string; // "monitoring", "processing", "error"
}

interface MonitoringStatus {
  [libraryId: number]: MonitoredLibrary;
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

  // File monitoring state
  const [monitoringStatus, setMonitoringStatus] = useState<MonitoringStatus>({});

  // Real-time scan progress tracking - enhanced with detailed stats
  const [scanProgress, setScanProgress] = useState<
    Map<
      number,
      {
        filesProcessed: number;
        bytesProcessed: number;
        progress: number;
        activeWorkers: number;
        maxWorkers?: number;
        minWorkers?: number;
        queueDepth: number;
        lastUpdate: Date;
        startTime?: Date;
        estimatedTimeLeft: number; // Now always provided by backend
        eta?: string;
        filesPerSecond?: number;
        throughputMbps?: number;
        elapsedTime?: string;
        // New fields for total files and bytes display
        totalFiles?: number;
        totalBytes?: number;
        filesFound?: number;
        remainingFiles?: number;
        discoveryComplete: boolean;
      }
    >
  >(new Map());

  // Load libraries on component mount
  useEffect(() => {
    loadLibraries();
    loadScanStats();
    loadCurrentJobs();
    loadLibraryStats();
    loadMonitoringStatus();
  }, []);

  // Periodic polling for detailed progress data
  useEffect(() => {
    const intervalId = setInterval(() => {
      // Refresh detailed progress for active jobs
      scanProgress.forEach((_, jobId) => {
        loadDetailedProgress(jobId);
      });

      // Also refresh monitoring status periodically
      loadMonitoringStatus();
    }, 2000); // Poll every 2 seconds

    return () => clearInterval(intervalId);
  }, [scanProgress]);

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
      const jobId = event.data.jobId;
      if (!jobId) return;

      const now = new Date();

      if (event.type === 'scan.progress') {
        setScanProgress((prev) => {
          const current = prev.get(jobId);
          const newProgress = {
            filesProcessed: (event.data.filesProcessed as number) || 0,
            bytesProcessed: (event.data.bytesProcessed as number) || 0,
            progress: (event.data.progress as number) || 0,
            activeWorkers: (event.data.activeWorkers as number) || 0,
            queueDepth: (event.data.queueDepth as number) || 0,
            lastUpdate: now,
            startTime: current?.startTime || now,
            estimatedTimeLeft:
              (event.data.estimatedTimeLeft as number) || current?.estimatedTimeLeft || 0,
            // Keep existing detailed data if available
            maxWorkers: current?.maxWorkers,
            minWorkers: current?.minWorkers,
            eta: current?.eta,
            filesPerSecond: (event.data.filesPerSecond as number) || current?.filesPerSecond,
            throughputMbps: (event.data.throughputMbps as number) || current?.throughputMbps,
            elapsedTime: current?.elapsedTime,

            // New fields for total files and bytes display
            totalFiles:
              (event.data.totalFiles as number) ||
              (event.data.filesFound as number) ||
              current?.totalFiles ||
              0,
            totalBytes: (event.data.totalBytes as number) || current?.totalBytes || 0,
            filesFound: (event.data.filesFound as number) || current?.filesFound || 0,
            remainingFiles: (() => {
              const totalFiles =
                (event.data.totalFiles as number) || (event.data.filesFound as number) || 0;
              const processed = (event.data.filesProcessed as number) || 0;
              return (event.data.remainingFiles as number) || Math.max(0, totalFiles - processed);
            })(),
            discoveryComplete: false,
          };

          const updatedMap = new Map(prev);
          updatedMap.set(jobId, newProgress);
          return updatedMap;
        });
      } else if (event.type === 'scan.discovery') {
        // Handle discovery events to show file discovery progress
        const filesInDir = (event.data.files_found_in_dir as number) || 0;
        const directory = (event.data.directory as string) || '';

        // Log discovery progress (could be enhanced with notifications later)
        console.log(`Discovering files: found ${filesInDir} in ${directory.split('/').pop()}`);
      } else if (event.type === 'scan.discovery_complete') {
        // Discovery phase is complete - now we have the final total
        const finalTotal = (event.data.final_total_files as number) || 0;
        const finalBytes = (event.data.final_total_bytes as number) || 0;

        console.log(
          `Discovery complete: ${finalTotal.toLocaleString()} files found (${formatBytes(finalBytes)}). Starting processing...`
        );

        // Update scan progress with final totals
        setScanProgress((prev) => {
          const current = prev.get(jobId);
          if (current) {
            const updatedProgress = {
              ...current,
              totalFiles: finalTotal,
              totalBytes: finalBytes,
              filesFound: finalTotal,
              discoveryComplete: true,
            };
            const updatedMap = new Map(prev);
            updatedMap.set(jobId, updatedProgress);
            return updatedMap;
          }
          return prev;
        });
      } else if (event.type === 'scan.started') {
        setScanProgress((prev) => {
          const updatedMap = new Map(prev);
          updatedMap.set(jobId, {
            filesProcessed: 0,
            bytesProcessed: 0,
            progress: 0,
            activeWorkers: 0,
            queueDepth: 0,
            lastUpdate: now,
            startTime: now,
            estimatedTimeLeft: 0,
            totalFiles: 0,
            totalBytes: 0,
            filesFound: 0,
            remainingFiles: 0,
            discoveryComplete: false,
          });
          return updatedMap;
        });
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
      handlePathChange(path.trim());
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

  // Helper function to detect library type based on path patterns
  const detectLibraryType = (path: string): string => {
    const lowerPath = path.toLowerCase();

    // Check for music indicators
    if (
      lowerPath.includes('music') ||
      lowerPath.includes('audio') ||
      lowerPath.includes('mp3') ||
      lowerPath.includes('songs') ||
      lowerPath.includes('albums') ||
      lowerPath.endsWith('/music') ||
      lowerPath.endsWith('/audio')
    ) {
      return 'music';
    }

    // Check for TV show indicators
    if (
      lowerPath.includes('tv') ||
      lowerPath.includes('shows') ||
      lowerPath.includes('series') ||
      lowerPath.includes('television') ||
      lowerPath.endsWith('/tv-shows') ||
      lowerPath.endsWith('/shows') ||
      lowerPath.endsWith('/series')
    ) {
      return 'tv';
    }

    // Check for movie indicators
    if (
      lowerPath.includes('movie') ||
      lowerPath.includes('film') ||
      lowerPath.includes('cinema') ||
      lowerPath.includes('video') ||
      lowerPath.endsWith('/movies') ||
      lowerPath.endsWith('/films') ||
      lowerPath.endsWith('/videos')
    ) {
      return 'movie';
    }

    // Default fallback - if path contains "media" or other generic terms, default to movies
    // since movies are often the most common media type
    return 'movie';
  };

  const handleQuickSelect = (path: string) => {
    const detectedType = detectLibraryType(path);
    setNewLibrary((prev) => ({
      ...prev,
      path,
      type: detectedType,
    }));
  };

  // Also add automatic type detection when path is manually entered
  const handlePathChange = (path: string) => {
    const detectedType = detectLibraryType(path);
    setNewLibrary((prev) => ({
      ...prev,
      path,
      type: detectedType,
    }));
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
            } else {
              // Handle scan start error - show user-friendly message
              let scanErrorMessage = `HTTP ${scanRes.status}: ${JSON.stringify(scanResult)}`;
              if (scanResult.error && scanResult.details) {
                // Check for specific error types and provide user-friendly messages
                if (scanResult.details.includes('scan already running for path')) {
                  const pathMatch = scanResult.details.match(/path '([^']+)'/);
                  const jobMatch = scanResult.details.match(/job ID: (\d+)/);
                  const libraryMatch = scanResult.details.match(/library ID: (\d+)/);

                  const path = pathMatch ? pathMatch[1] : 'this directory';
                  const jobId = jobMatch ? jobMatch[1] : 'unknown';
                  const conflictLibraryId = libraryMatch ? libraryMatch[1] : 'unknown';

                  scanErrorMessage = `Library created successfully, but automatic scan could not start: Another scan is already running on ${path} (Job ${jobId}, Library ${conflictLibraryId}). You can start the scan manually once the current scan completes.`;
                } else if (scanResult.details.includes('scan already running for library')) {
                  const jobMatch = scanResult.details.match(/job ID: (\d+)/);
                  const jobId = jobMatch ? jobMatch[1] : 'unknown';

                  scanErrorMessage = `Library created successfully, but automatic scan could not start: This library already has a running scan (Job ${jobId}).`;
                } else {
                  scanErrorMessage = `Library created successfully, but automatic scan failed: ${scanResult.details || scanResult.error}`;
                }
              } else if (scanResult.error) {
                scanErrorMessage = `Library created successfully, but automatic scan failed: ${scanResult.error}`;
              }

              // Update response to show the scan error
              setResponse({
                status: res.status, // Keep the successful library creation status
                data: {
                  library: result.library,
                  message: 'Library created successfully',
                  scan_error: scanErrorMessage,
                },
                error: scanErrorMessage,
              });
            }
          } catch (scanError) {
            console.error('Failed to start scan for new library:', scanError);
            // Update response to show the scan error
            setResponse({
              status: res.status, // Keep the successful library creation status
              data: {
                library: result.library,
                message: 'Library created successfully',
                scan_error: 'Failed to start automatic scan due to network error',
              },
              error: `Library created, but automatic scan failed: ${scanError instanceof Error ? scanError.message : 'Unknown error'}`,
            });
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

        // Fetch detailed progress for each active job
        for (const [, job] of Object.entries(extractedJobs)) {
          if (job.status === 'running') {
            loadDetailedProgress(job.id);
          }
        }
      }
    } catch (error) {
      console.error('Failed to load current jobs:', error);
    }
  };

  // New function to fetch detailed progress data
  const loadDetailedProgress = async (jobId: number) => {
    try {
      const res = await fetch(`/api/admin/scanner/progress/${jobId}`);
      const result = await res.json();

      if (res.ok) {
        const now = new Date();

        setScanProgress((prev) => {
          const current = prev.get(jobId);
          const newProgress = {
            filesProcessed: result.processed_files || result.files_processed || 0,
            bytesProcessed: result.processed_bytes || result.bytes_processed || 0,
            progress: result.progress || 0, // Backend already sends as percentage 0.0-100.0
            activeWorkers: result.active_workers || 0,
            maxWorkers: result.max_workers,
            minWorkers: result.min_workers,
            queueDepth: result.queue_length || result.queue_depth || 0,
            lastUpdate: now,
            startTime: current?.startTime || now,
            estimatedTimeLeft: result.estimated_time_left || 0,
            eta: result.eta || null,
            filesPerSecond: result.files_per_second || result.files_per_sec,
            throughputMbps: result.throughput_mbps,
            elapsedTime: result.elapsed_time,
            // New fields for total files and bytes display
            totalFiles: result.total_files || result.files_found || current?.totalFiles || 0,
            totalBytes: (result.total_bytes as number) || current?.totalBytes || 0,
            filesFound: result.files_found || current?.filesFound || 0,
            remainingFiles:
              result.remaining_files ||
              Math.max(
                0,
                (result.total_files || result.files_found || 0) -
                  (result.processed_files || result.files_processed || 0)
              ),
            discoveryComplete: false,
          };

          const newMap = new Map(prev);
          newMap.set(jobId, newProgress);
          return newMap;
        });

        // Load file monitoring status
        loadMonitoringStatus();
      }
    } catch (error) {
      console.error(`Failed to load detailed progress for job ${jobId}:`, error);
    }
  };

  // Load file monitoring status
  const loadMonitoringStatus = async () => {
    try {
      const res = await fetch('/api/scanner/monitoring');
      const result = await res.json();

      if (res.ok && result.monitoring_status) {
        setMonitoringStatus(result.monitoring_status);
      }
    } catch (error) {
      console.error('Failed to load monitoring status:', error);
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

        // Try to parse JSON error response for better error messages
        let errorMessage = `HTTP ${res.status}: ${errorText}`;
        try {
          const errorJson = JSON.parse(errorText);
          if (errorJson.error && errorJson.details) {
            // Check for specific error types and provide user-friendly messages
            if (errorJson.details.includes('scan already running for path')) {
              const pathMatch = errorJson.details.match(/path '([^']+)'/);
              const jobMatch = errorJson.details.match(/job ID: (\d+)/);
              const libraryMatch = errorJson.details.match(/library ID: (\d+)/);

              const path = pathMatch ? pathMatch[1] : 'this directory';
              const jobId = jobMatch ? jobMatch[1] : 'unknown';
              const conflictLibraryId = libraryMatch ? libraryMatch[1] : 'unknown';

              errorMessage = `Cannot start scan: Another scan is already running on ${path} (Job ${jobId}, Library ${conflictLibraryId}). Please wait for the current scan to complete or pause it before starting a new one.`;
            } else if (errorJson.details.includes('scan already running for library')) {
              const jobMatch = errorJson.details.match(/job ID: (\d+)/);
              const jobId = jobMatch ? jobMatch[1] : 'unknown';

              errorMessage = `Cannot start scan: This library already has a running scan (Job ${jobId}). Please wait for it to complete or pause it first.`;
            } else {
              errorMessage = errorJson.details || errorJson.error || errorMessage;
            }
          } else if (errorJson.error) {
            errorMessage = errorJson.error;
          }
        } catch {
          // If JSON parsing fails, use the original error text
          // But make it more user-friendly if it contains common patterns
          if (errorText.includes('scan already running')) {
            errorMessage =
              'Cannot start scan: A scan is already running. Please wait for it to complete or pause it first.';
          }
        }

        throw new Error(errorMessage);
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

  // New function to format elapsed time from backend
  const formatElapsedTime = (elapsedTimeStr: string) => {
    if (!elapsedTimeStr) return 'Unknown';

    // Parse duration string like "6m27.245602529s" or "1h30m45.123s"
    const timeMatch = elapsedTimeStr.match(/(?:(\d+)h)?(?:(\d+)m)?(?:(\d+(?:\.\d+)?)s)?/);
    if (!timeMatch) return elapsedTimeStr; // Return original if parsing fails

    const [, hours, minutes, seconds] = timeMatch;
    const parts = [];

    if (hours && parseInt(hours) > 0) {
      parts.push(`${hours}h`);
    }
    if (minutes && parseInt(minutes) > 0) {
      parts.push(`${minutes}m`);
    }
    if (seconds && parseFloat(seconds) > 0) {
      const secondsInt = Math.floor(parseFloat(seconds));
      parts.push(`${secondsInt}s`);
    }

    return parts.length > 0 ? parts.join(' ') : '0s';
  };

  // Helper function to get simple, clear progress info
  const getProgressInfo = (library: MediaLibrary, scanJob: ScanJob | undefined) => {
    const databaseFileCount = libraryStats[library.id]?.total_files || 0;
    const isActivelyScanning = scanJob?.status === 'running';
    const isPaused = scanJob?.status === 'paused';

    // Get real-time progress data if available
    const progressData = scanJob ? scanProgress.get(scanJob.id) : undefined;
    const filesProcessed = progressData?.filesProcessed || scanJob?.files_processed || 0;
    const totalFiles = progressData?.totalFiles || scanJob?.files_found || 0;
    const totalBytes = progressData?.totalBytes || 0;
    const bytesProcessed = progressData?.bytesProcessed || 0;
    const discoveryComplete = progressData?.discoveryComplete || false;

    // Simple progress: if scan is active, show estimated progress, otherwise 100% if files exist
    let progressPercent = 0;
    if (scanJob?.progress !== undefined) {
      progressPercent = scanJob.progress;
    } else if (progressData?.progress !== undefined) {
      progressPercent = progressData.progress;
    }

    let statusText = '';
    if (isActivelyScanning) {
      if (!discoveryComplete && totalFiles === 0) {
        // Discovery phase - no files found yet
        statusText = 'Discovering files...';
      } else if (!discoveryComplete && totalFiles > 0) {
        // Discovery phase - files being found
        statusText = `Discovering files... (${totalFiles.toLocaleString()} found)`;
      } else if (discoveryComplete && totalFiles > 0) {
        // Processing phase - stable total
        if (totalBytes > 0) {
          statusText = `Processing ${filesProcessed.toLocaleString()} of ${totalFiles.toLocaleString()} files (${formatBytes(bytesProcessed)} of ${formatBytes(totalBytes)})`;
        } else {
          statusText = `Processing ${filesProcessed.toLocaleString()} of ${totalFiles.toLocaleString()} files`;
        }
      } else {
        // Fallback
        statusText = 'Scanning in progress...';
      }
    } else if (isPaused) {
      if (totalFiles > 0) {
        if (totalBytes > 0) {
          statusText = `Paused at ${filesProcessed.toLocaleString()} of ${totalFiles.toLocaleString()} files (${formatBytes(bytesProcessed)} of ${formatBytes(totalBytes)})`;
        } else {
          statusText = `Paused at ${filesProcessed.toLocaleString()} of ${totalFiles.toLocaleString()} files`;
        }
      } else {
        statusText = 'Scan paused';
      }
    } else if (databaseFileCount > 0) {
      statusText = `${databaseFileCount.toLocaleString()} files scanned`;
    } else {
      statusText = 'No files scanned yet';
    }

    return {
      fileCount: Math.max(databaseFileCount, totalFiles),
      progressPercent,
      isActivelyScanning,
      isPaused,
      statusText,
      filesProcessed,
      totalFiles,
    };
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
          className="bg-green-600 hover:bg-green-700 text-white px-4 py-2 rounded text-sm transition-colors cursor-pointer"
          disabled={loading}
          data-tooltip-id="add-library-tooltip"
          data-tooltip-content={
            showAddForm ? 'Cancel adding new library' : 'Add a new media library'
          }
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
                  onChange={(e) => handlePathChange(e.target.value)}
                  placeholder="/path/to/your/media/directory"
                  className="flex-1 bg-slate-700 text-white px-3 py-2 rounded border border-slate-600 focus:border-blue-500 focus:outline-none"
                />
                <button
                  onClick={handleDirectorySelect}
                  className="bg-blue-600 hover:bg-blue-700 text-white px-4 py-2 rounded text-sm transition-colors cursor-pointer"
                  type="button"
                  data-tooltip-id="browse-button-tooltip"
                  data-tooltip-content="Browse and enter a custom directory path (type will be auto-detected)"
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
                        className="text-xs bg-slate-600 hover:bg-slate-500 text-slate-200 px-2 py-1 rounded transition-colors cursor-pointer"
                        type="button"
                        data-tooltip-id="quick-path-tooltip"
                        data-tooltip-content={`Select ${path} as library directory (type will be auto-detected)`}
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
              <p className="text-xs text-slate-400 mt-1">
                💡 Library type is automatically detected based on your path (e.g., "/media/music" →
                Music)
              </p>
            </div>

            <div className="flex gap-2">
              <button
                onClick={addLibrary}
                disabled={loading || !newLibrary.path}
                className="bg-green-600 hover:bg-green-700 disabled:bg-slate-600 text-white px-4 py-2 rounded text-sm transition-colors cursor-pointer"
                data-tooltip-id="add-library-form-tooltip"
                data-tooltip-content="Create new media library and start scanning"
              >
                Add Library
              </button>
              <button
                onClick={() => setShowAddForm(false)}
                className="bg-slate-600 hover:bg-slate-700 text-white px-4 py-2 rounded text-sm transition-colors cursor-pointer"
                data-tooltip-id="cancel-form-tooltip"
                data-tooltip-content="Cancel and close the form"
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

            // Get simple progress info
            const progressInfo = getProgressInfo(library, scanJob);
            const progressPercent = progressInfo.progressPercent;

            const isScanning = scanJob?.status === 'running';
            const isPaused = scanJob?.status === 'paused';
            const isFailed = scanJob?.status === 'failed';
            const isCompleted =
              scanJob?.status === 'completed' || (progressPercent >= 100 && !isScanning);
            const isNerdPanelExpanded = expandedNerdPanels.has(library.id);
            const progressData = scanJob ? scanProgress.get(Number(scanJob.id)) : null;

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
                      {monitoringStatus[library.id] && (
                        <span className="px-2 py-1 bg-blue-600 text-blue-100 rounded text-xs font-medium">
                          MONITORING
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
                        className="library-button bg-orange-600 hover:bg-orange-700 disabled:bg-slate-600 text-white px-3 py-1 rounded text-sm transition-colors cursor-pointer"
                        data-tooltip-id="pause-scan-tooltip"
                        data-tooltip-content="Pause the ongoing scan for this library"
                      >
                        {scanLoading.has(library.id) ? '...' : 'Pause Scan'}
                      </button>
                    ) : scanJob && (scanJob.status === 'paused' || scanJob.status === 'failed') ? (
                      <button
                        onClick={() => resumeScan(library.id)}
                        disabled={scanLoading.has(library.id)}
                        className="library-button bg-green-600 hover:bg-green-700 disabled:bg-slate-600 text-white px-3 py-1 rounded text-sm transition-colors cursor-pointer"
                        data-tooltip-id="resume-scan-tooltip"
                        data-tooltip-content="Resume the paused scan for this library"
                      >
                        {scanLoading.has(library.id) ? '...' : 'Resume Scan'}
                      </button>
                    ) : (
                      <button
                        onClick={() => startScan(library.id)}
                        disabled={scanLoading.has(library.id)}
                        className="library-button bg-blue-600 hover:bg-blue-700 disabled:bg-slate-600 text-white px-3 py-1 rounded text-sm transition-colors cursor-pointer"
                        data-tooltip-id="start-scan-tooltip"
                        data-tooltip-content="Start scanning this library for media files"
                      >
                        {scanLoading.has(library.id) ? '...' : 'Scan'}
                      </button>
                    )}

                    <button
                      onClick={() => removeLibrary(library.id)}
                      className="library-button bg-red-600 hover:bg-red-700 text-white px-3 py-1 rounded text-sm transition-colors cursor-pointer"
                      disabled={loading}
                      data-tooltip-id="remove-library-tooltip"
                      data-tooltip-content="Remove this library from the media server"
                    >
                      Remove
                    </button>
                  </div>
                </div>

                {/* Simplified Scan Progress Display */}
                {scanJob && (
                  <div className="mt-3 bg-slate-700 rounded p-3">
                    {/* Simple Progress Info */}
                    <div className="flex justify-between items-center mb-3">
                      <div className="flex items-center gap-3">
                        <span className="text-sm font-medium text-white">
                          {progressInfo.statusText}
                        </span>
                      </div>

                      <div className="flex items-center gap-3">
                        {/* Nerd Panel Toggle */}
                        <button
                          onClick={() => toggleNerdPanel(library.id)}
                          className="text-xs text-slate-400 hover:text-slate-200 transition-colors flex items-center gap-1 cursor-pointer"
                          title="Toggle detailed information"
                          data-tooltip-id="details-toggle-tooltip"
                          data-tooltip-content="Show/hide detailed scan progress information"
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

                    {/* Clean Modern Progress Bar with visible track */}
                    <div className="w-full bg-slate-700/50 rounded-full h-2 overflow-hidden relative">
                      {/* Full-height background track */}
                      <div className="absolute inset-0 bg-slate-600/40 rounded-full"></div>

                      <div
                        className={`h-2 rounded-full transition-all duration-700 ease-out relative z-10 ${
                          isFailed
                            ? 'bg-red-400'
                            : isPaused
                              ? 'bg-amber-400'
                              : isCompleted
                                ? 'bg-emerald-400'
                                : 'bg-blue-400'
                        }`}
                        style={{
                          width: `${progressPercent}%`,
                        }}
                      >
                        {/* Subtle glow for active scans only */}
                        {isScanning && (
                          <div className="absolute inset-0 bg-blue-300 rounded-full opacity-40 animate-pulse"></div>
                        )}
                      </div>
                    </div>

                    {/* Progress info below the bar */}
                    <div className="flex justify-between items-center mt-2 text-xs">
                      <div className="flex items-center gap-2">
                        <span className="text-slate-300">
                          {progressPercent > 0 ? `${progressPercent.toFixed(1)}%` : '0%'}
                        </span>
                      </div>
                      {progressInfo.isActivelyScanning && progressData?.estimatedTimeLeft && (
                        <span className="text-slate-400">
                          {formatHumanReadableETA(progressData.estimatedTimeLeft)}
                        </span>
                      )}
                    </div>

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

                        {/* Status and Error Messages */}
                        {scanJob.status_message && isPaused && (
                          <div className="mb-3 p-2 bg-blue-900/30 rounded text-sm">
                            <span className="text-blue-400">Status:</span>
                            <span className="text-slate-300 ml-2">{scanJob.status_message}</span>
                          </div>
                        )}
                        {scanJob.error_message && (
                          <div className="mb-3 p-2 bg-red-900/30 rounded text-sm">
                            <span className="text-red-400">Error:</span>
                            <span className="text-slate-300 ml-2">{scanJob.error_message}</span>
                          </div>
                        )}

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

                              {/* File Progress Information */}
                              {progressInfo.totalFiles > 0 && (
                                <>
                                  <div className="flex justify-between">
                                    <span className="text-slate-300">Files Processed:</span>
                                    <span className="text-white">
                                      {progressInfo.filesProcessed.toLocaleString()}
                                    </span>
                                  </div>
                                  <div className="flex justify-between">
                                    <span className="text-slate-300">Total Files:</span>
                                    <span className="text-white">
                                      {progressInfo.totalFiles.toLocaleString()}
                                    </span>
                                  </div>
                                  <div className="flex justify-between">
                                    <span className="text-slate-300">Remaining Files:</span>
                                    <span className="text-white">
                                      {(
                                        progressInfo.totalFiles - progressInfo.filesProcessed
                                      ).toLocaleString()}
                                    </span>
                                  </div>
                                  {progressData?.totalBytes && progressData.totalBytes > 0 && (
                                    <>
                                      <div className="flex justify-between">
                                        <span className="text-slate-300">Bytes Processed:</span>
                                        <span className="text-white">
                                          {formatBytes(progressData.bytesProcessed || 0)}
                                        </span>
                                      </div>
                                      <div className="flex justify-between">
                                        <span className="text-slate-300">Total Size:</span>
                                        <span className="text-white">
                                          {formatBytes(progressData.totalBytes)}
                                        </span>
                                      </div>
                                      <div className="flex justify-between">
                                        <span className="text-slate-300">Remaining Size:</span>
                                        <span className="text-white">
                                          {formatBytes(
                                            (progressData.totalBytes || 0) -
                                              (progressData.bytesProcessed || 0)
                                          )}
                                        </span>
                                      </div>
                                    </>
                                  )}

                                  <div className="flex justify-between">
                                    <span className="text-slate-300">Started:</span>
                                    <span className="text-white">
                                      {scanJob.started_at
                                        ? new Date(scanJob.started_at).toLocaleString()
                                        : 'N/A'}
                                    </span>
                                  </div>
                                </>
                              )}
                            </div>
                          </div>

                          {/* Performance Metrics */}
                          <div className="space-y-2">
                            <div className="text-slate-400 font-medium">Performance</div>
                            <div className="space-y-1">
                              <div className="flex justify-between">
                                <span className="text-slate-300">Data Processed:</span>
                                <span className="text-white">
                                  {formatBytes(
                                    progressData?.bytesProcessed || scanJob.bytes_processed || 0
                                  )}
                                </span>
                              </div>
                              {progressData && (
                                <>
                                  {/* Worker Information */}
                                  <div
                                    className="flex justify-between cursor-help"
                                    data-tooltip-id="active-workers-tooltip"
                                    data-tooltip-content="Number of parallel worker threads currently processing files"
                                  >
                                    <span className="text-slate-300">Active Workers:</span>
                                    <span className="text-white font-medium">
                                      {progressData.activeWorkers}
                                    </span>
                                  </div>
                                  {progressData.maxWorkers && (
                                    <div
                                      className="flex justify-between cursor-help"
                                      data-tooltip-id="worker-range-tooltip"
                                      data-tooltip-content="Scanner adapts worker count based on system load (min-max range)"
                                    >
                                      <span className="text-slate-300">Worker Range:</span>
                                      <span className="text-white">
                                        {progressData.minWorkers}-{progressData.maxWorkers}
                                      </span>
                                    </div>
                                  )}

                                  {/* Performance Metrics */}
                                  {progressData.filesPerSecond && (
                                    <div
                                      className="flex justify-between cursor-help"
                                      data-tooltip-id="files-per-sec-tooltip"
                                      data-tooltip-content="Files being processed per second"
                                    >
                                      <span className="text-slate-300">Files/sec:</span>
                                      <span className="text-white">
                                        {progressData.filesPerSecond.toFixed(2)}
                                      </span>
                                    </div>
                                  )}
                                  {progressData.throughputMbps && (
                                    <div
                                      className="flex justify-between cursor-help"
                                      data-tooltip-id="throughput-tooltip"
                                      data-tooltip-content="Data processing throughput in megabytes per second"
                                    >
                                      <span className="text-slate-300">Throughput:</span>
                                      <span className="text-white">
                                        {progressData.throughputMbps.toFixed(1)} MB/s
                                      </span>
                                    </div>
                                  )}

                                  {/* Queue and System Info */}
                                  <div
                                    className="flex justify-between cursor-help"
                                    data-tooltip-id="queue-depth-tooltip"
                                    data-tooltip-content="Number of files waiting to be processed"
                                  >
                                    <span className="text-slate-300">Queue Depth:</span>
                                    <span className="text-white">{progressData.queueDepth}</span>
                                  </div>
                                  {progressData.elapsedTime && (
                                    <div className="flex justify-between">
                                      <span className="text-slate-300">Elapsed:</span>
                                      <span className="text-white">
                                        {formatElapsedTime(progressData.elapsedTime)}
                                      </span>
                                    </div>
                                  )}

                                  {/* ETA Information */}
                                  {progressInfo.isActivelyScanning &&
                                    progressData?.estimatedTimeLeft && (
                                      <div className="flex justify-between">
                                        <span className="text-slate-300">Est. Time Left:</span>
                                        <span className="text-white">
                                          {formatHumanReadableETA(progressData.estimatedTimeLeft)}
                                        </span>
                                      </div>
                                    )}

                                  <div className="flex justify-between">
                                    <span className="text-slate-300">Last Update:</span>
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
                              {/* Consolidated ETA display - prefer backend ETA if available */}
                              {isScanning &&
                                (progressData?.eta || progressData?.estimatedTimeLeft) && (
                                  <div>
                                    <span className="text-green-400">Estimated completion:</span>{' '}
                                    {progressData.eta
                                      ? new Date(progressData.eta).toLocaleString()
                                      : progressData.estimatedTimeLeft
                                        ? new Date(
                                            Date.now() + progressData.estimatedTimeLeft * 1000
                                          ).toLocaleString()
                                        : 'Calculating...'}
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

      {/* Tooltips for all data-tooltip-id elements */}
      <Tooltip id="progress-tooltip" place="top" />
      <Tooltip id="progress-percent-tooltip" place="top" />
      <Tooltip id="eta-tooltip" place="top" />
      <Tooltip id="active-workers-tooltip" place="top" />
      <Tooltip id="worker-range-tooltip" place="top" />
      <Tooltip id="files-per-sec-tooltip" place="top" />
      <Tooltip id="throughput-tooltip" place="top" />
      <Tooltip id="queue-depth-tooltip" place="top" />

      {/* Button tooltips */}
      <Tooltip id="add-library-tooltip" place="bottom" />
      <Tooltip id="start-scan-tooltip" place="top" />
      <Tooltip id="pause-scan-tooltip" place="top" />
      <Tooltip id="resume-scan-tooltip" place="top" />
      <Tooltip id="remove-library-tooltip" place="top" />
      <Tooltip id="details-toggle-tooltip" place="top" />

      {/* Form tooltips */}
      <Tooltip id="browse-button-tooltip" place="top" />
      <Tooltip id="quick-path-tooltip" place="top" />
      <Tooltip id="add-library-form-tooltip" place="top" />
      <Tooltip id="cancel-form-tooltip" place="top" />

      {/* Minimal CSS for modern design */}
      <style>{`
        .text-shadow {
          text-shadow: 0 1px 2px rgba(0, 0, 0, 0.7);
        }
      `}</style>
    </div>
  );
};

export default MediaLibraryManager;
