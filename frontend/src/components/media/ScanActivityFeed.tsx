import React, { useState, useEffect } from 'react';
import {
  FileText,
  Download,
  AlertCircle,
  CheckCircle,
  Clock,
  Database,
  Zap,
  Cpu,
  Music,
  Film,
  Image,
  Filter,
  X,
} from 'lucide-react';

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
    jobId?: number;
    libraryId?: number;
    fileName?: string;
    plugin?: string;
    assetType?: string;
    filesProcessed?: number;
    errorCount?: number;
    [key: string]: unknown;
  };
}

interface ScanActivityFeedProps {
  activities: ScanActivity[];
  maxItems?: number;
  showFilters?: boolean;
  autoRefresh?: boolean;
  onClear?: () => void;
}

const ScanActivityFeed: React.FC<ScanActivityFeedProps> = ({
  activities,
  maxItems = 50,
  showFilters = true,
  autoRefresh = true,
  onClear,
}) => {
  const [filteredActivities, setFilteredActivities] = useState<ScanActivity[]>(activities);
  const [selectedFilters, setSelectedFilters] = useState<string[]>([]);
  const [showFilterPanel, setShowFilterPanel] = useState(false);

  const filterTypes = [
    { id: 'scan.progress', label: 'Scan Progress', icon: FileText, color: 'text-blue-400' },
    { id: 'scan.started', label: 'Scan Started', icon: CheckCircle, color: 'text-green-400' },
    { id: 'scan.completed', label: 'Scan Completed', icon: CheckCircle, color: 'text-green-400' },
    { id: 'scan.error', label: 'Scan Errors', icon: AlertCircle, color: 'text-red-400' },
    {
      id: 'plugin.enrichment',
      label: 'Plugin Enrichment',
      icon: Database,
      color: 'text-purple-400',
    },
    { id: 'plugin.asset', label: 'Asset Downloads', icon: Download, color: 'text-yellow-400' },
    { id: 'system.health', label: 'System Health', icon: Cpu, color: 'text-orange-400' },
    { id: 'file.processed', label: 'File Processing', icon: FileText, color: 'text-slate-400' },
  ];

  useEffect(() => {
    let filtered = activities;

    if (selectedFilters.length > 0) {
      filtered = activities.filter((activity) => selectedFilters.includes(activity.type));
    }

    // Sort by timestamp (newest first) and limit
    filtered = filtered
      .sort((a, b) => new Date(b.timestamp).getTime() - new Date(a.timestamp).getTime())
      .slice(0, maxItems);

    setFilteredActivities(filtered);
  }, [activities, selectedFilters, maxItems]);

  const getActivityIcon = (activity: ScanActivity) => {
    switch (activity.type) {
      case 'scan.progress':
        return FileText;
      case 'scan.started':
        return CheckCircle;
      case 'scan.completed':
        return CheckCircle;
      case 'scan.error':
        return AlertCircle;
      case 'plugin.enrichment':
        return Database;
      case 'plugin.asset':
        return Download;
      case 'system.health':
        return Cpu;
      case 'file.processed':
        return FileText;
      default:
        return FileText;
    }
  };

  const getActivityColor = (activity: ScanActivity) => {
    switch (activity.severity) {
      case 'success':
        return 'text-green-400 bg-green-900/20 border-green-500/30';
      case 'warning':
        return 'text-yellow-400 bg-yellow-900/20 border-yellow-500/30';
      case 'error':
        return 'text-red-400 bg-red-900/20 border-red-500/30';
      default:
        return 'text-blue-400 bg-blue-900/20 border-blue-500/30';
    }
  };

  const getMediaTypeIcon = (fileName: string) => {
    const ext = fileName.split('.').pop()?.toLowerCase();
    if (['mp3', 'flac', 'wav', 'm4a', 'aac'].includes(ext || '')) return Music;
    if (['mp4', 'mkv', 'avi', 'mov'].includes(ext || '')) return Film;
    if (['jpg', 'png', 'gif', 'webp'].includes(ext || '')) return Image;
    return FileText;
  };

  const formatTimestamp = (timestamp: string) => {
    const date = new Date(timestamp);
    return date.toLocaleTimeString();
  };

  const formatRelativeTime = (timestamp: string) => {
    const now = new Date();
    const activityTime = new Date(timestamp);
    const diffMs = now.getTime() - activityTime.getTime();
    const diffSeconds = Math.floor(diffMs / 1000);
    const diffMinutes = Math.floor(diffSeconds / 60);
    const diffHours = Math.floor(diffMinutes / 60);

    if (diffSeconds < 60) return 'just now';
    if (diffMinutes < 60) return `${diffMinutes}m ago`;
    if (diffHours < 24) return `${diffHours}h ago`;
    return activityTime.toLocaleDateString();
  };

  const toggleFilter = (filterType: string) => {
    setSelectedFilters((prev) =>
      prev.includes(filterType) ? prev.filter((f) => f !== filterType) : [...prev, filterType]
    );
  };

  const clearAllFilters = () => {
    setSelectedFilters([]);
  };

  return (
    <div className="bg-slate-800 rounded-lg border border-slate-700">
      {/* Header */}
      <div className="p-4 border-b border-slate-700">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            <Zap className="w-5 h-5 text-yellow-400" />
            <h3 className="font-semibold text-white">Scanner Activity Feed</h3>
            {autoRefresh && (
              <div className="flex items-center gap-1 text-xs text-green-400">
                <div className="w-2 h-2 bg-green-400 rounded-full animate-pulse" />
                Live
              </div>
            )}
          </div>

          <div className="flex items-center gap-2">
            {showFilters && (
              <button
                onClick={() => setShowFilterPanel(!showFilterPanel)}
                className="p-2 bg-slate-700 hover:bg-slate-600 text-white rounded transition-colors"
                title="Filter activities"
              >
                <Filter className="w-4 h-4" />
              </button>
            )}

            {onClear && (
              <button
                onClick={onClear}
                className="p-2 bg-red-600 hover:bg-red-700 text-white rounded transition-colors"
                title="Clear activity log"
              >
                <X className="w-4 h-4" />
              </button>
            )}
          </div>
        </div>

        {/* Filter Panel */}
        {showFilterPanel && (
          <div className="mt-4 p-3 bg-slate-700/50 rounded">
            <div className="flex items-center justify-between mb-2">
              <span className="text-sm font-medium text-white">Filter by type:</span>
              {selectedFilters.length > 0 && (
                <button
                  onClick={clearAllFilters}
                  className="text-xs text-slate-400 hover:text-white transition-colors"
                >
                  Clear all
                </button>
              )}
            </div>
            <div className="flex flex-wrap gap-2">
              {filterTypes.map((filter) => {
                const Icon = filter.icon;
                const isSelected = selectedFilters.includes(filter.id);
                return (
                  <button
                    key={filter.id}
                    onClick={() => toggleFilter(filter.id)}
                    className={`flex items-center gap-1 px-2 py-1 rounded text-xs transition-colors ${
                      isSelected
                        ? 'bg-blue-600 text-white'
                        : 'bg-slate-600 text-slate-300 hover:bg-slate-500'
                    }`}
                  >
                    <Icon className="w-3 h-3" />
                    {filter.label}
                  </button>
                );
              })}
            </div>
            {selectedFilters.length > 0 && (
              <div className="mt-2 text-xs text-slate-400">
                Showing {filteredActivities.length} of {activities.length} activities
              </div>
            )}
          </div>
        )}
      </div>

      {/* Activity List */}
      <div className="max-h-96 overflow-y-auto">
        {filteredActivities.length === 0 ? (
          <div className="p-8 text-center text-slate-400">
            <FileText className="w-8 h-8 mx-auto mb-2 text-slate-500" />
            <p>No activities to display</p>
            {selectedFilters.length > 0 && (
              <button
                onClick={clearAllFilters}
                className="mt-2 text-sm text-blue-400 hover:text-blue-300 transition-colors"
              >
                Clear filters to see all activities
              </button>
            )}
          </div>
        ) : (
          <div className="divide-y divide-slate-700">
            {filteredActivities.map((activity) => {
              const Icon = getActivityIcon(activity);
              const MediaIcon = activity.data?.fileName
                ? getMediaTypeIcon(activity.data.fileName)
                : null;

              return (
                <div
                  key={activity.id}
                  className={`p-4 hover:bg-slate-700/30 transition-colors border-l-2 ${getActivityColor(activity)}`}
                >
                  <div className="flex items-start gap-3">
                    <div className="flex-shrink-0 p-2 bg-slate-700/50 rounded">
                      <Icon className="w-4 h-4" />
                    </div>

                    <div className="flex-1 min-w-0">
                      <div className="flex items-center justify-between">
                        <h4 className="font-medium text-white truncate">{activity.title}</h4>
                        <div className="flex items-center gap-2 text-xs text-slate-400">
                          <Clock className="w-3 h-3" />
                          <span title={formatTimestamp(activity.timestamp)}>
                            {formatRelativeTime(activity.timestamp)}
                          </span>
                        </div>
                      </div>

                      <p className="text-sm text-slate-300 mt-1">{activity.message}</p>

                      {/* Additional Details */}
                      {activity.data && (
                        <div className="mt-2 flex flex-wrap gap-4 text-xs">
                          {activity.data.fileName && (
                            <div className="flex items-center gap-1 text-slate-400">
                              {MediaIcon && <MediaIcon className="w-3 h-3" />}
                              <span className="truncate max-w-32">{activity.data.fileName}</span>
                            </div>
                          )}

                          {activity.data.plugin && (
                            <div className="text-purple-400">Plugin: {activity.data.plugin}</div>
                          )}

                          {activity.data.assetType && (
                            <div className="text-yellow-400">Asset: {activity.data.assetType}</div>
                          )}

                          {activity.data.filesProcessed && (
                            <div className="text-blue-400">
                              Files: {activity.data.filesProcessed.toLocaleString()}
                            </div>
                          )}

                          {activity.data.errorCount && activity.data.errorCount > 0 && (
                            <div className="text-red-400">Errors: {activity.data.errorCount}</div>
                          )}

                          {activity.data.jobId && (
                            <div className="text-slate-500">Job #{activity.data.jobId}</div>
                          )}
                        </div>
                      )}
                    </div>
                  </div>
                </div>
              );
            })}
          </div>
        )}
      </div>

      {/* Footer Stats */}
      <div className="p-3 border-t border-slate-700 bg-slate-700/30">
        <div className="flex items-center justify-between text-xs text-slate-400">
          <span>
            {filteredActivities.length} of {activities.length} activities
            {selectedFilters.length > 0 && ` (filtered)`}
          </span>
          <span>Last updated: {new Date().toLocaleTimeString()}</span>
        </div>
      </div>
    </div>
  );
};

export default ScanActivityFeed;
