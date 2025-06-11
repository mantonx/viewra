import React, { useState, useEffect } from 'react';
import { Download, AlertTriangle, Eye, EyeOff, ChevronDown, ChevronUp } from 'lucide-react';

interface FieldProgress {
  field_name: string;
  total: number;
  populated: number;
  percentage: number;
  quality: 'excellent' | 'good' | 'poor' | 'missing';
  sources?: string[];
}

interface ArtworkProgress {
  artwork_type: string;
  total: number;
  available: number;
  percentage: number;
  resolution: 'hd' | 'sd' | 'mixed';
}

interface CategoryProgress {
  total: number;
  with_metadata: number;
  with_artwork: number;
  fully_enriched: number;
  pending_jobs: number;
  failed_jobs: number;
  metadata_fields: Record<string, FieldProgress>;
  artwork_types: Record<string, ArtworkProgress>;
  quality_score: number;
  last_enrichment?: string;
  estimated_time?: number;
}

interface EnrichmentItem {
  media_id: string;
  media_type: string;
  title: string;
  source: string;
  action: string;
  timestamp: string;
  fields: string[];
  quality: 'high' | 'medium' | 'low';
}

interface EnrichmentProgress {
  media_type: string;
  total_items: number;
  enriched_items: number;
  pending_items: number;
  failed_items: number;
  progress_percentage: number;
  estimated_completion?: string;
  last_update: string;
  field_progress: Record<string, number>;
  recent_activity: EnrichmentItem[];
  media_breakdown: {
    tv_shows?: CategoryProgress;
    movies?: CategoryProgress;
    music?: CategoryProgress;
    episodes?: CategoryProgress;
  };
}

interface EnrichmentProgressCardProps {
  mediaType: 'all' | 'tv_shows' | 'movies' | 'music';
  title: string;
  icon: React.ReactNode;
}

const EnrichmentProgressCard: React.FC<EnrichmentProgressCardProps> = ({
  mediaType,
  title,
  icon,
}) => {
  const [progress, setProgress] = useState<EnrichmentProgress | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [isExpanded, setIsExpanded] = useState(false);
  const [showDetails, setShowDetails] = useState(false);
  const [lastRefresh, setLastRefresh] = useState<Date>(new Date());

  const fetchProgress = async () => {
    try {
      setLoading(true);
      const endpoint =
        mediaType === 'all'
          ? '/api/enrichment/progress'
          : `/api/enrichment/progress/${mediaType.replace('_', '-')}`;

      const response = await fetch(endpoint);
      const data = await response.json();

      if (response.ok) {
        setProgress(data.progress);
        setError(null);
      } else {
        setError(data.error || 'Failed to fetch enrichment progress');
      }
    } catch (err) {
      setError('Network error while fetching progress');
      console.error('Enrichment progress fetch error:', err);
    } finally {
      setLoading(false);
      setLastRefresh(new Date());
    }
  };

  useEffect(() => {
    fetchProgress();

    // Refresh every 30 seconds
    const interval = setInterval(fetchProgress, 30000);
    return () => clearInterval(interval);
  }, [mediaType]);

  const formatDuration = (hours: number): string => {
    if (hours < 1) return `${Math.round(hours * 60)}m`;
    if (hours < 24) return `${Math.round(hours)}h`;
    const days = Math.floor(hours / 24);
    const remainingHours = Math.round(hours % 24);
    return `${days}d ${remainingHours}h`;
  };

  const formatTimeAgo = (timestamp: string): string => {
    const date = new Date(timestamp);
    const now = new Date();
    const diffMs = now.getTime() - date.getTime();
    const diffMins = Math.floor(diffMs / 60000);

    if (diffMins < 1) return 'just now';
    if (diffMins < 60) return `${diffMins}m ago`;
    const diffHours = Math.floor(diffMins / 60);
    if (diffHours < 24) return `${diffHours}h ago`;
    const diffDays = Math.floor(diffHours / 24);
    return `${diffDays}d ago`;
  };

  if (loading && !progress) {
    return (
      <div className="bg-slate-800 rounded-lg border border-slate-700 p-6">
        <div className="flex items-center gap-3 mb-4">
          {icon}
          <h3 className="text-lg font-semibold text-white">{title}</h3>
        </div>
        <div className="animate-pulse">
          <div className="h-4 bg-slate-700 rounded mb-3"></div>
          <div className="h-8 bg-slate-700 rounded mb-4"></div>
          <div className="grid grid-cols-3 gap-4">
            <div className="h-16 bg-slate-700 rounded"></div>
            <div className="h-16 bg-slate-700 rounded"></div>
            <div className="h-16 bg-slate-700 rounded"></div>
          </div>
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="bg-slate-800 rounded-lg border border-red-500 p-6">
        <div className="flex items-center gap-3 mb-4">
          <AlertTriangle className="w-6 h-6 text-red-400" />
          <h3 className="text-lg font-semibold text-white">{title}</h3>
        </div>
        <p className="text-red-400">{error}</p>
        <button
          onClick={fetchProgress}
          className="mt-3 px-4 py-2 bg-red-600 hover:bg-red-700 text-white rounded transition-colors"
        >
          Retry
        </button>
      </div>
    );
  }

  if (!progress) return null;

  const isActive = progress.pending_items > 0;
  const progressPercent = progress.progress_percentage || 0;

  return (
    <div className="bg-slate-800 rounded-lg border border-slate-700 transition-all duration-200">
      {/* Header */}
      <div className="p-4 border-b border-slate-700">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-3">
            {icon}
            <div>
              <h3 className="font-semibold text-white">{title}</h3>
              <p className="text-sm text-slate-400">
                {progress.enriched_items.toLocaleString()} of{' '}
                {progress.total_items.toLocaleString()} items enriched
                {progress.pending_items > 0 &&
                  ` • ${progress.pending_items.toLocaleString()} pending`}
              </p>
            </div>
          </div>

          <div className="flex items-center gap-2">
            {/* Status Indicators */}
            {isActive && (
              <div className="flex items-center gap-1 text-green-400">
                <Download className="w-4 h-4 animate-pulse" />
                <span className="text-xs">Active</span>
              </div>
            )}

            {progress.failed_items > 0 && (
              <div className="flex items-center gap-1 text-red-400">
                <AlertTriangle className="w-4 h-4" />
                <span className="text-xs">{progress.failed_items} failed</span>
              </div>
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
            <span>Enrichment Progress</span>
            <span>{progressPercent.toFixed(1)}%</span>
          </div>
          <div className="w-full bg-slate-700 rounded-full h-3">
            <div
              className={`h-3 rounded-full transition-all duration-500 ${
                isActive
                  ? 'bg-gradient-to-r from-green-500 to-green-400'
                  : progressPercent >= 100
                    ? 'bg-gradient-to-r from-blue-500 to-blue-400'
                    : 'bg-gradient-to-r from-purple-500 to-purple-400'
              }`}
              style={{ width: `${Math.min(progressPercent, 100)}%` }}
            />
          </div>
        </div>

        {/* Key Statistics */}
        <div className="grid grid-cols-2 md:grid-cols-4 gap-4 mb-4">
          <div className="bg-slate-700/50 rounded p-3 text-center">
            <div className="text-lg font-bold text-blue-400">
              {progress.total_items.toLocaleString()}
            </div>
            <div className="text-xs text-slate-300">Total Items</div>
          </div>

          <div className="bg-slate-700/50 rounded p-3 text-center">
            <div className="text-lg font-bold text-green-400">
              {progress.enriched_items.toLocaleString()}
            </div>
            <div className="text-xs text-slate-300">Enriched</div>
          </div>

          <div className="bg-slate-700/50 rounded p-3 text-center">
            <div className="text-lg font-bold text-yellow-400">
              {progress.pending_items.toLocaleString()}
            </div>
            <div className="text-xs text-slate-300">Pending</div>
          </div>

          <div className="bg-slate-700/50 rounded p-3 text-center">
            <div className="text-lg font-bold text-orange-400">
              {progress.estimated_completion
                ? formatDuration(
                    Math.abs(new Date(progress.estimated_completion).getTime() - Date.now()) /
                      (1000 * 60 * 60)
                  )
                : '---'}
            </div>
            <div className="text-xs text-slate-300">ETA</div>
          </div>
        </div>

        {/* Field Progress (if expanded) */}
        {isExpanded && Object.keys(progress.field_progress).length > 0 && (
          <div className="mb-4">
            <div className="flex items-center justify-between mb-3">
              <h4 className="font-medium text-white">Field Progress</h4>
              <button
                onClick={() => setShowDetails(!showDetails)}
                className="flex items-center gap-1 text-xs text-slate-400 hover:text-slate-200"
              >
                {showDetails ? <EyeOff className="w-3 h-3" /> : <Eye className="w-3 h-3" />}
                {showDetails ? 'Hide' : 'Show'} Details
              </button>
            </div>
            <div className="grid grid-cols-2 md:grid-cols-3 gap-3">
              {Object.entries(progress.field_progress).map(([field, percentage]) => (
                <div key={field} className="bg-slate-700/30 rounded p-3">
                  <div className="flex justify-between items-center mb-1">
                    <span className="text-sm text-slate-300 capitalize">
                      {field.replace('_', ' ')}
                    </span>
                    <span className="text-xs text-slate-400">{percentage.toFixed(0)}%</span>
                  </div>
                  <div className="w-full bg-slate-600 rounded-full h-2">
                    <div
                      className={`h-2 rounded-full ${
                        percentage >= 90
                          ? 'bg-green-400'
                          : percentage >= 70
                            ? 'bg-blue-400'
                            : percentage >= 30
                              ? 'bg-yellow-400'
                              : 'bg-red-400'
                      }`}
                      style={{ width: `${percentage}%` }}
                    />
                  </div>
                </div>
              ))}
            </div>
          </div>
        )}

        {/* Recent Activity */}
        {isExpanded && progress.recent_activity && progress.recent_activity.length > 0 && (
          <div className="mb-4">
            <h4 className="font-medium text-white mb-3">Recent Activity</h4>
            <div className="space-y-2 max-h-40 overflow-y-auto">
              {progress.recent_activity.slice(0, 5).map((item, index) => (
                <div key={index} className="flex items-center gap-3 bg-slate-700/30 rounded p-2">
                  <div
                    className={`w-2 h-2 rounded-full ${
                      item.quality === 'high'
                        ? 'bg-green-400'
                        : item.quality === 'medium'
                          ? 'bg-blue-400'
                          : 'bg-yellow-400'
                    }`}
                  />
                  <div className="flex-1 min-w-0">
                    <p className="text-sm text-white truncate">{item.title || 'Unknown'}</p>
                    <p className="text-xs text-slate-400">
                      {item.action} • {formatTimeAgo(item.timestamp)}
                    </p>
                  </div>
                </div>
              ))}
            </div>
          </div>
        )}

        {/* Last Update */}
        <div className="text-xs text-slate-500 text-center">
          Last updated: {lastRefresh.toLocaleTimeString()}
        </div>
      </div>
    </div>
  );
};

export default EnrichmentProgressCard;
