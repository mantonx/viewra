import React, { useState } from 'react';
import {
  Play,
  Pause,
  Square,
  BarChart3,
  Cpu,
  Server,
  Wifi,
  AlertTriangle,
  Eye,
  EyeOff,
  ChevronDown,
  ChevronUp,
} from 'lucide-react';

interface ScanProgress {
  jobId: number;
  libraryId: number;
  libraryPath: string;
  status: 'running' | 'paused' | 'completed' | 'failed' | 'cancelled';

  // Phase Information
  currentPhase: 'discovery' | 'processing' | 'enrichment' | 'finalization';
  phaseProgress: number;

  // File Statistics
  filesProcessed: number;
  filesFound: number;
  totalFiles: number;
  remainingFiles: number;
  filesSkipped: number;
  errorsCount: number;

  // Data Statistics
  bytesProcessed: number;
  totalBytes: number;

  // Performance Metrics
  filesPerSecond: number;
  throughputMbps: number;
  progress: number;
  eta: string | null;
  elapsedTime: string;
  estimatedTimeLeft: number;

  // Worker Status
  activeWorkers: number;
  maxWorkers: number;
  minWorkers: number;
  queueDepth: number;

  // System Health
  cpuPercent: number;
  memoryPercent: number;
  ioWaitPercent: number;
  loadAverage: number;
  networkMbps: number;

  // Throttling
  emergencyBrake: boolean;
  currentBatchSize: number;
  processingDelayMs: number;

  // Plugin Enrichment
  pluginStats?: {
    musicbrainz?: { processed: number; errors: number; assets: number };
    tmdb?: { processed: number; errors: number; assets: number };
    [key: string]: { processed: number; errors: number; assets: number } | undefined;
  };

  lastUpdate: Date;
}

interface ScanProgressCardProps {
  progress: ScanProgress;
  onPause?: () => void;
  onResume?: () => void;
  onCancel?: () => void;
}

const ScanProgressCard: React.FC<ScanProgressCardProps> = ({
  progress,
  onPause,
  onResume,
  onCancel,
}) => {
  const [isExpanded, setIsExpanded] = useState(false);
  const [showTechnicalDetails, setShowTechnicalDetails] = useState(false);

  // Helper Functions
  const formatBytes = (bytes: number): string => {
    if (bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
  };

  const formatDuration = (seconds: number): string => {
    if (seconds < 60) return `${Math.round(seconds)}s`;
    if (seconds < 3600) return `${Math.floor(seconds / 60)}m ${Math.round(seconds % 60)}s`;
    const hours = Math.floor(seconds / 3600);
    const minutes = Math.floor((seconds % 3600) / 60);
    return `${hours}h ${minutes}m`;
  };

  const getPhaseIcon = (phase: string) => {
    switch (phase) {
      case 'discovery':
        return '📁';
      case 'processing':
        return '⚙️';
      case 'enrichment':
        return '🔍';
      case 'finalization':
        return '✅';
      default:
        return '📊';
    }
  };

  const getPhaseDescription = (phase: string) => {
    switch (phase) {
      case 'discovery':
        return 'Discovering files in library directories';
      case 'processing':
        return 'Extracting metadata and indexing media files';
      case 'enrichment':
        return 'Enriching metadata via plugins (MusicBrainz, TMDB, etc.)';
      case 'finalization':
        return 'Finalizing scan and updating library statistics';
      default:
        return 'Processing media library';
    }
  };

  const getStatusColor = (status: string) => {
    switch (status) {
      case 'running':
        return 'text-green-400 bg-green-900/20 border-green-500';
      case 'paused':
        return 'text-yellow-400 bg-yellow-900/20 border-yellow-500';
      case 'completed':
        return 'text-blue-400 bg-blue-900/20 border-blue-500';
      case 'failed':
        return 'text-red-400 bg-red-900/20 border-red-500';
      case 'cancelled':
        return 'text-gray-400 bg-gray-900/20 border-gray-500';
      default:
        return 'text-slate-400 bg-slate-900/20 border-slate-500';
    }
  };

  const isActive = progress.status === 'running';
  const isPaused = progress.status === 'paused';
  const isComplete = progress.status === 'completed';

  return (
    <div
      className={`bg-slate-800 rounded-lg border-2 transition-all duration-200 ${getStatusColor(progress.status)}`}
    >
      {/* Header */}
      <div className="p-4 border-b border-slate-700">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-3">
            <div className="text-2xl">{getPhaseIcon(progress.currentPhase)}</div>
            <div>
              <h3 className="font-semibold text-white">Library: {progress.libraryPath}</h3>
              <p className="text-sm text-slate-400">{getPhaseDescription(progress.currentPhase)}</p>
            </div>
          </div>

          <div className="flex items-center gap-2">
            {/* Status Badge */}
            <span
              className={`px-3 py-1 rounded-full text-xs font-medium border ${getStatusColor(progress.status)}`}
            >
              {progress.status.toUpperCase()}
            </span>

            {/* Controls */}
            {isActive && (
              <button
                onClick={onPause}
                className="p-2 bg-yellow-600 hover:bg-yellow-700 text-white rounded transition-colors"
                title="Pause scan"
              >
                <Pause className="w-4 h-4" />
              </button>
            )}

            {isPaused && (
              <button
                onClick={onResume}
                className="p-2 bg-green-600 hover:bg-green-700 text-white rounded transition-colors"
                title="Resume scan"
              >
                <Play className="w-4 h-4" />
              </button>
            )}

            {(isActive || isPaused) && (
              <button
                onClick={onCancel}
                className="p-2 bg-red-600 hover:bg-red-700 text-white rounded transition-colors"
                title="Cancel scan"
              >
                <Square className="w-4 h-4" />
              </button>
            )}

            <button
              onClick={() => setIsExpanded(!isExpanded)}
              className="p-2 bg-slate-600 hover:bg-slate-700 text-white rounded transition-colors"
              title={isExpanded ? 'Collapse details' : 'Expand details'}
            >
              {isExpanded ? <ChevronUp className="w-4 h-4" /> : <ChevronDown className="w-4 h-4" />}
            </button>
          </div>
        </div>
      </div>

      {/* Main Progress */}
      <div className="p-4">
        {/* Overall Progress Bar */}
        <div className="mb-4">
          <div className="flex justify-between text-sm text-slate-300 mb-2">
            <span>Overall Progress</span>
            <span>{progress.progress.toFixed(1)}%</span>
          </div>
          <div className="w-full bg-slate-700 rounded-full h-3">
            <div
              className={`h-3 rounded-full transition-all duration-500 ${
                isActive
                  ? 'bg-gradient-to-r from-green-500 to-green-400'
                  : isPaused
                    ? 'bg-gradient-to-r from-yellow-500 to-yellow-400'
                    : isComplete
                      ? 'bg-gradient-to-r from-blue-500 to-blue-400'
                      : 'bg-gradient-to-r from-gray-500 to-gray-400'
              }`}
              style={{ width: `${Math.min(progress.progress, 100)}%` }}
            />
          </div>
        </div>

        {/* Key Statistics */}
        <div className="grid grid-cols-2 md:grid-cols-4 gap-4 mb-4">
          <div className="bg-slate-700/50 rounded p-3 text-center">
            <div className="text-lg font-bold text-blue-400">
              {progress.filesProcessed.toLocaleString()}
            </div>
            <div className="text-xs text-slate-300">Files Processed</div>
          </div>

          <div className="bg-slate-700/50 rounded p-3 text-center">
            <div className="text-lg font-bold text-green-400">
              {formatBytes(progress.bytesProcessed)}
            </div>
            <div className="text-xs text-slate-300">Data Processed</div>
          </div>

          <div className="bg-slate-700/50 rounded p-3 text-center">
            <div className="text-lg font-bold text-purple-400">
              {progress.filesPerSecond.toFixed(1)}
            </div>
            <div className="text-xs text-slate-300">Files/sec</div>
          </div>

          <div className="bg-slate-700/50 rounded p-3 text-center">
            <div className="text-lg font-bold text-orange-400">
              {progress.eta ? formatDuration(progress.estimatedTimeLeft) : '---'}
            </div>
            <div className="text-xs text-slate-300">ETA</div>
          </div>
        </div>

        {/* Worker Status */}
        <div className="bg-slate-700/30 rounded p-3 mb-4">
          <div className="flex items-center justify-between mb-2">
            <div className="flex items-center gap-2">
              <Server className="w-4 h-4 text-blue-400" />
              <span className="text-sm font-medium text-white">Worker Pool</span>
            </div>
            <span className="text-xs text-slate-400">
              {progress.activeWorkers}/{progress.maxWorkers} workers active
            </span>
          </div>

          <div className="flex gap-2 items-center">
            <div className="flex-1 bg-slate-600 rounded-full h-2">
              <div
                className="bg-green-500 h-2 rounded-full transition-all duration-300"
                style={{ width: `${(progress.activeWorkers / progress.maxWorkers) * 100}%` }}
              />
            </div>
            <span className="text-xs text-slate-400 w-12">Q:{progress.queueDepth}</span>
          </div>
        </div>

        {/* System Health Indicators */}
        <div className="grid grid-cols-3 gap-2 mb-4">
          <div className="bg-slate-700/30 rounded p-2 text-center">
            <Cpu className="w-4 h-4 mx-auto mb-1 text-yellow-400" />
            <div className="text-xs text-slate-300">CPU</div>
            <div className="text-sm font-bold text-white">{progress.cpuPercent.toFixed(0)}%</div>
          </div>

          <div className="bg-slate-700/30 rounded p-2 text-center">
            <BarChart3 className="w-4 h-4 mx-auto mb-1 text-blue-400" />
            <div className="text-xs text-slate-300">Memory</div>
            <div className="text-sm font-bold text-white">{progress.memoryPercent.toFixed(0)}%</div>
          </div>

          <div className="bg-slate-700/30 rounded p-2 text-center">
            <Wifi className="w-4 h-4 mx-auto mb-1 text-green-400" />
            <div className="text-xs text-slate-300">Network</div>
            <div className="text-sm font-bold text-white">
              {progress.networkMbps.toFixed(1)}MB/s
            </div>
          </div>
        </div>

        {/* Emergency Brake Warning */}
        {progress.emergencyBrake && (
          <div className="bg-red-900/50 border border-red-500 rounded p-3 mb-4">
            <div className="flex items-center gap-2">
              <AlertTriangle className="w-5 h-5 text-red-400" />
              <span className="text-red-400 font-medium">Emergency Throttling Active</span>
            </div>
            <p className="text-sm text-red-300 mt-1">
              System resources are critically high. Scan has been automatically throttled.
            </p>
          </div>
        )}
      </div>

      {/* Expanded Details */}
      {isExpanded && (
        <div className="border-t border-slate-700 p-4">
          <div className="flex items-center justify-between mb-4">
            <h4 className="font-medium text-white">Detailed Statistics</h4>
            <button
              onClick={() => setShowTechnicalDetails(!showTechnicalDetails)}
              className="flex items-center gap-1 text-sm text-slate-400 hover:text-white transition-colors"
            >
              {showTechnicalDetails ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
              {showTechnicalDetails ? 'Hide' : 'Show'} Technical Details
            </button>
          </div>

          {/* File Statistics */}
          <div className="grid grid-cols-2 md:grid-cols-3 gap-4 mb-4">
            <div>
              <div className="text-sm text-slate-400">Total Files Found</div>
              <div className="text-lg font-bold text-white">
                {progress.totalFiles.toLocaleString()}
              </div>
            </div>
            <div>
              <div className="text-sm text-slate-400">Files Remaining</div>
              <div className="text-lg font-bold text-white">
                {progress.remainingFiles.toLocaleString()}
              </div>
            </div>
            <div>
              <div className="text-sm text-slate-400">Files Skipped</div>
              <div className="text-lg font-bold text-yellow-400">
                {progress.filesSkipped.toLocaleString()}
              </div>
            </div>
            <div>
              <div className="text-sm text-slate-400">Errors</div>
              <div className="text-lg font-bold text-red-400">
                {progress.errorsCount.toLocaleString()}
              </div>
            </div>
            <div>
              <div className="text-sm text-slate-400">Throughput</div>
              <div className="text-lg font-bold text-purple-400">
                {progress.throughputMbps.toFixed(2)} MB/s
              </div>
            </div>
            <div>
              <div className="text-sm text-slate-400">Elapsed Time</div>
              <div className="text-lg font-bold text-white">{progress.elapsedTime}</div>
            </div>
          </div>

          {/* Plugin Enrichment Status */}
          {progress.pluginStats && (
            <div className="mb-4">
              <h5 className="font-medium text-white mb-2">Plugin Enrichment Status</h5>
              <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
                {Object.entries(progress.pluginStats).map(([plugin, stats]) => (
                  <div key={plugin} className="bg-slate-700/50 rounded p-3">
                    <div className="flex items-center justify-between mb-2">
                      <span className="font-medium text-white capitalize">{plugin}</span>
                      <span className="text-xs text-slate-400">
                        {stats?.processed || 0} processed
                      </span>
                    </div>
                    <div className="grid grid-cols-3 gap-2 text-sm">
                      <div className="text-center">
                        <div className="text-green-400 font-bold">{stats?.assets || 0}</div>
                        <div className="text-slate-400 text-xs">Assets</div>
                      </div>
                      <div className="text-center">
                        <div className="text-blue-400 font-bold">{stats?.processed || 0}</div>
                        <div className="text-slate-400 text-xs">Processed</div>
                      </div>
                      <div className="text-center">
                        <div className="text-red-400 font-bold">{stats?.errors || 0}</div>
                        <div className="text-slate-400 text-xs">Errors</div>
                      </div>
                    </div>
                  </div>
                ))}
              </div>
            </div>
          )}

          {/* Technical Details */}
          {showTechnicalDetails && (
            <div className="bg-slate-900/50 rounded p-4">
              <h5 className="font-medium text-white mb-3">Technical Details</h5>
              <div className="grid grid-cols-2 md:grid-cols-3 gap-4 text-sm">
                <div>
                  <span className="text-slate-400">Job ID:</span>
                  <span className="text-white ml-2">{progress.jobId}</span>
                </div>
                <div>
                  <span className="text-slate-400">Library ID:</span>
                  <span className="text-white ml-2">{progress.libraryId}</span>
                </div>
                <div>
                  <span className="text-slate-400">Current Phase:</span>
                  <span className="text-white ml-2 capitalize">{progress.currentPhase}</span>
                </div>
                <div>
                  <span className="text-slate-400">Batch Size:</span>
                  <span className="text-white ml-2">{progress.currentBatchSize}</span>
                </div>
                <div>
                  <span className="text-slate-400">Processing Delay:</span>
                  <span className="text-white ml-2">{progress.processingDelayMs}ms</span>
                </div>
                <div>
                  <span className="text-slate-400">I/O Wait:</span>
                  <span className="text-white ml-2">{progress.ioWaitPercent.toFixed(1)}%</span>
                </div>
                <div>
                  <span className="text-slate-400">Load Average:</span>
                  <span className="text-white ml-2">{progress.loadAverage.toFixed(2)}</span>
                </div>
                <div>
                  <span className="text-slate-400">Last Update:</span>
                  <span className="text-white ml-2">
                    {progress.lastUpdate.toLocaleTimeString()}
                  </span>
                </div>
              </div>
            </div>
          )}
        </div>
      )}
    </div>
  );
};

export default ScanProgressCard;
