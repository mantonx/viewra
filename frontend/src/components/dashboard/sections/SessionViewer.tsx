import React, { useState } from 'react';
import { 
  Activity, 
  Clock, 
  Monitor, 
  Zap, 
  Square, 
  RotateCcw,
  Users,
  TrendingUp,
  Server,
  Settings,
  Gauge
} from 'lucide-react';

interface TranscodeSessionSummary {
  id: string;
  input_filename: string;
  input_resolution: string;
  output_resolution: string;
  input_codec: string;
  output_codec: string;
  bitrate: string;
  duration: string;
  progress: number;
  transcoder_type: string; // "software", "nvenc", "vaapi", etc.
  client_ip: string;
  client_device: string;
  start_time: string;
  status: string;
  estimated_time_left: string;
  throughput_fps: number;
}

interface TranscoderEngineStatus {
  type: string;
  status: string;
  version: string;
  max_concurrent: number;
  active_sessions: number;
  queued_sessions: number;
  last_health_check: string;
  capabilities: string[];
}

interface TranscoderQuickStats {
  sessions_today: number;
  total_hours_today: number;
  average_speed: number;
  error_rate: number;
  current_throughput: string;
  peak_concurrent: number;
}

interface TranscoderMainData {
  active_sessions: TranscodeSessionSummary[];
  queued_sessions: TranscodeSessionSummary[];
  recent_sessions: TranscodeSessionSummary[];
  engine_status: TranscoderEngineStatus;
  quick_stats: TranscoderQuickStats;
}

interface SessionViewerProps {
  data: TranscoderMainData;
  isNerdMode: boolean;
  onToggleNerdMode: () => void;
  onRefresh: () => void;
  onSessionAction: (sessionId: string, action: 'stop' | 'restart' | 'prioritize') => void;
  onEngineAction: (action: 'clear_cache' | 'restart_service') => void;
}

const SessionViewer: React.FC<SessionViewerProps> = ({
  data,
  isNerdMode,
  onToggleNerdMode,
  onRefresh,
  onSessionAction,
  onEngineAction,
}) => {
  const [expandedSessions, setExpandedSessions] = useState<Record<string, boolean>>({});
  const [selectedTranscoderType, setSelectedTranscoderType] = useState<string>('all');

  const toggleSessionExpansion = (sessionId: string) => {
    setExpandedSessions(prev => ({
      ...prev,
      [sessionId]: !prev[sessionId]
    }));
  };

  const getStatusColor = (status: string) => {
    switch (status.toLowerCase()) {
      case 'running': return 'text-green-400 bg-green-400/20';
      case 'pending': return 'text-yellow-400 bg-yellow-400/20';
      case 'failed': return 'text-red-400 bg-red-400/20';
      case 'completed': return 'text-blue-400 bg-blue-400/20';
      default: return 'text-gray-400 bg-gray-400/20';
    }
  };

  const getTranscoderTypeColor = (type: string) => {
    switch (type.toLowerCase()) {
      case 'nvenc': return 'text-green-400 bg-green-400/20';
      case 'vaapi': return 'text-blue-400 bg-blue-400/20';
      case 'qsv': return 'text-purple-400 bg-purple-400/20';
      case 'software': return 'text-gray-400 bg-gray-400/20';
      default: return 'text-gray-400 bg-gray-400/20';
    }
  };

  const formatProgress = (progress: number) => {
    return `${(progress * 100).toFixed(1)}%`;
  };

  // Group sessions by transcoder type
  const sessionsByType = data.active_sessions.reduce((acc, session) => {
    const type = session.transcoder_type || 'software';
    if (!acc[type]) acc[type] = [];
    acc[type].push(session);
    return acc;
  }, {} as Record<string, TranscodeSessionSummary[]>);

  const filteredSessions = selectedTranscoderType === 'all' 
    ? data.active_sessions 
    : sessionsByType[selectedTranscoderType] || [];

  const transcoderTypes = Object.keys(sessionsByType);

  return (
    <div className="space-y-6">
      {/* Header with Engine Status */}
      <div className="bg-slate-800 border border-slate-700 rounded-lg p-6">
        <div className="flex items-center justify-between mb-4">
          <div className="flex items-center space-x-3">
            <div className={`w-3 h-3 rounded-full ${data.engine_status.status === 'healthy' ? 'bg-green-400' : 'bg-red-400'}`}></div>
            <h2 className="text-xl font-semibold text-white">
              FFmpeg Transcoder Engine
            </h2>
            <span className="text-sm text-slate-400">v{data.engine_status.version}</span>
          </div>
          <div className="flex items-center space-x-2">
            <button
              onClick={onToggleNerdMode}
              className={`px-3 py-1 text-xs rounded border transition-colors ${
                isNerdMode 
                  ? 'bg-purple-600/20 text-purple-400 border-purple-600' 
                  : 'bg-slate-700 text-slate-400 border-slate-600 hover:border-slate-500'
              }`}
            >
              {isNerdMode ? 'Simple View' : 'Nerd Panel'}
            </button>
            <button
              onClick={onRefresh}
              className="p-2 text-slate-400 hover:text-white bg-slate-700 rounded border border-slate-600 hover:border-slate-500 transition-colors"
            >
              <RotateCcw size={16} />
            </button>
          </div>
        </div>

        {/* Quick Stats Grid */}
        <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-6 gap-4">
          <div className="bg-slate-700/50 rounded-lg p-3">
            <div className="flex items-center justify-between mb-1">
              <Activity size={16} className="text-green-400" />
              <span className="text-lg font-bold text-white">{data.engine_status.active_sessions}</span>
            </div>
            <div className="text-xs text-slate-400">Active</div>
          </div>

          <div className="bg-slate-700/50 rounded-lg p-3">
            <div className="flex items-center justify-between mb-1">
              <Clock size={16} className="text-yellow-400" />
              <span className="text-lg font-bold text-white">{data.engine_status.queued_sessions}</span>
            </div>
            <div className="text-xs text-slate-400">Queued</div>
          </div>

          <div className="bg-slate-700/50 rounded-lg p-3">
            <div className="flex items-center justify-between mb-1">
              <TrendingUp size={16} className="text-blue-400" />
              <span className="text-lg font-bold text-white">{data.quick_stats.sessions_today}</span>
            </div>
            <div className="text-xs text-slate-400">Today</div>
          </div>

          <div className="bg-slate-700/50 rounded-lg p-3">
            <div className="flex items-center justify-between mb-1">
              <Gauge size={16} className="text-purple-400" />
              <span className="text-lg font-bold text-white">{data.quick_stats.average_speed.toFixed(1)}x</span>
            </div>
            <div className="text-xs text-slate-400">Avg Speed</div>
          </div>

          <div className="bg-slate-700/50 rounded-lg p-3">
            <div className="flex items-center justify-between mb-1">
              <Zap size={16} className="text-orange-400" />
              <span className="text-lg font-bold text-white">{data.quick_stats.current_throughput}</span>
            </div>
            <div className="text-xs text-slate-400">Throughput</div>
          </div>

          <div className="bg-slate-700/50 rounded-lg p-3">
            <div className="flex items-center justify-between mb-1">
              <Users size={16} className="text-cyan-400" />
              <span className="text-lg font-bold text-white">{data.quick_stats.peak_concurrent}</span>
            </div>
            <div className="text-xs text-slate-400">Peak</div>
          </div>
        </div>

        {/* Engine Actions */}
        <div className="mt-4 flex items-center justify-between pt-4 border-t border-slate-600">
          <div className="text-xs text-slate-400">
            {data.engine_status.max_concurrent} max concurrent • {data.engine_status.capabilities.join(', ')}
          </div>
          <div className="flex items-center space-x-2">
            <button
              onClick={() => onEngineAction('clear_cache')}
              className="px-3 py-1 text-xs bg-yellow-600/20 text-yellow-400 border border-yellow-600 rounded hover:bg-yellow-600/30 transition-colors"
            >
              Clear Cache
            </button>
            <button
              onClick={() => onEngineAction('restart_service')}
              className="px-3 py-1 text-xs bg-red-600/20 text-red-400 border border-red-600 rounded hover:bg-red-600/30 transition-colors"
            >
              Restart Service
            </button>
          </div>
        </div>
      </div>

      {/* Session Filters */}
      {transcoderTypes.length > 1 && (
        <div className="flex items-center space-x-2">
          <span className="text-sm text-slate-400">Filter by transcoder:</span>
          <select
            value={selectedTranscoderType}
            onChange={(e) => setSelectedTranscoderType(e.target.value)}
            className="px-3 py-1 bg-slate-800 border border-slate-700 rounded text-white text-sm focus:outline-none focus:ring-2 focus:ring-purple-500"
          >
            <option value="all">All Types ({data.active_sessions.length})</option>
            {transcoderTypes.map(type => (
              <option key={type} value={type}>
                {type.toUpperCase()} ({sessionsByType[type].length})
              </option>
            ))}
          </select>
        </div>
      )}

      {/* Active Sessions */}
      <div className="space-y-4">
        <div className="flex items-center justify-between">
          <h3 className="text-lg font-medium text-white">
            Active Transcoding Sessions
            <span className="ml-2 text-sm text-slate-400">({filteredSessions.length})</span>
          </h3>
        </div>

        {filteredSessions.length === 0 ? (
          <div className="bg-slate-800 border border-slate-700 rounded-lg p-8 text-center">
            <Server size={48} className="text-slate-600 mx-auto mb-4" />
            <div className="text-slate-400 mb-2">No active transcoding sessions</div>
            <p className="text-sm text-slate-500">
              Sessions will appear here when transcoding is in progress
            </p>
          </div>
        ) : (
          <div className="space-y-3">
            {filteredSessions.map((session) => (
              <div
                key={session.id}
                className="bg-slate-800 border border-slate-700 rounded-lg p-4 hover:border-slate-600 transition-colors"
              >
                {/* Session Header */}
                <div className="flex items-center justify-between mb-3">
                  <div className="flex items-center space-x-3">
                    <div className="flex items-center space-x-2">
                      <Monitor size={16} className="text-slate-400" />
                      <span className="font-medium text-white truncate max-w-md" title={session.input_filename}>
                        {session.input_filename}
                      </span>
                    </div>
                    <div className={`px-2 py-1 text-xs rounded-full ${getStatusColor(session.status)}`}>
                      {session.status}
                    </div>
                    <div className={`px-2 py-1 text-xs rounded-full ${getTranscoderTypeColor(session.transcoder_type)}`}>
                      {session.transcoder_type.toUpperCase()}
                    </div>
                  </div>
                  <div className="flex items-center space-x-2">
                    <button
                      onClick={() => toggleSessionExpansion(session.id)}
                      className="text-slate-400 hover:text-white transition-colors"
                    >
                      <Settings size={16} />
                    </button>
                  </div>
                </div>

                {/* Progress Bar */}
                <div className="mb-3">
                  <div className="flex items-center justify-between mb-1">
                    <span className="text-sm text-slate-400">Progress</span>
                    <span className="text-sm text-white">{formatProgress(session.progress)}</span>
                  </div>
                  <div className="w-full bg-slate-700 rounded-full h-2">
                    <div 
                      className="bg-gradient-to-r from-blue-500 to-purple-500 h-2 rounded-full transition-all duration-300" 
                      style={{ width: `${session.progress * 100}%` }}
                    />
                  </div>
                </div>

                {/* Session Summary */}
                <div className="grid grid-cols-2 md:grid-cols-4 gap-4 text-sm">
                  <div>
                    <div className="text-slate-400">Resolution</div>
                    <div className="text-white">{session.output_resolution || 'Auto'}</div>
                  </div>
                  <div>
                    <div className="text-slate-400">Codec</div>
                    <div className="text-white">{session.output_codec || 'Auto'}</div>
                  </div>
                  <div>
                    <div className="text-slate-400">Bitrate</div>
                    <div className="text-white">{session.bitrate || 'Auto'}</div>
                  </div>
                  <div>
                    <div className="text-slate-400">FPS</div>
                    <div className="text-white">{session.throughput_fps.toFixed(1)}</div>
                  </div>
                </div>

                {/* Expanded Session Details */}
                {expandedSessions[session.id] && (
                  <div className="mt-4 pt-4 border-t border-slate-700">
                    <div className="grid grid-cols-1 md:grid-cols-2 gap-4 mb-4">
                      <div className="space-y-2">
                        <h4 className="text-sm font-medium text-white">Client Information</h4>
                        <div className="text-sm space-y-1">
                          <div className="flex justify-between">
                            <span className="text-slate-400">IP Address:</span>
                            <span className="text-white">{session.client_ip || 'Unknown'}</span>
                          </div>
                          <div className="flex justify-between">
                            <span className="text-slate-400">Device:</span>
                            <span className="text-white">{session.client_device || 'Unknown'}</span>
                          </div>
                          <div className="flex justify-between">
                            <span className="text-slate-400">Started:</span>
                            <span className="text-white">{new Date(session.start_time).toLocaleTimeString()}</span>
                          </div>
                          <div className="flex justify-between">
                            <span className="text-slate-400">ETA:</span>
                            <span className="text-white">{session.estimated_time_left}</span>
                          </div>
                        </div>
                      </div>

                      <div className="space-y-2">
                        <h4 className="text-sm font-medium text-white">Session Actions</h4>
                        <div className="flex flex-wrap gap-2">
                          <button
                            onClick={() => onSessionAction(session.id, 'stop')}
                            className="flex items-center space-x-1 px-3 py-1 text-xs bg-red-600/20 text-red-400 border border-red-600 rounded hover:bg-red-600/30 transition-colors"
                          >
                            <Square size={12} />
                            <span>Stop</span>
                          </button>
                          <button
                            onClick={() => onSessionAction(session.id, 'restart')}
                            className="flex items-center space-x-1 px-3 py-1 text-xs bg-blue-600/20 text-blue-400 border border-blue-600 rounded hover:bg-blue-600/30 transition-colors"
                          >
                            <RotateCcw size={12} />
                            <span>Restart</span>
                          </button>
                          <button
                            onClick={() => onSessionAction(session.id, 'prioritize')}
                            className="flex items-center space-x-1 px-3 py-1 text-xs bg-yellow-600/20 text-yellow-400 border border-yellow-600 rounded hover:bg-yellow-600/30 transition-colors"
                          >
                            <Zap size={12} />
                            <span>Prioritize</span>
                          </button>
                        </div>
                      </div>
                    </div>
                  </div>
                )}
              </div>
            ))}
          </div>
        )}
      </div>

      {/* Queued Sessions (if any) */}
      {data.queued_sessions.length > 0 && (
        <div className="space-y-4">
          <h3 className="text-lg font-medium text-white">
            Queued Sessions
            <span className="ml-2 text-sm text-slate-400">({data.queued_sessions.length})</span>
          </h3>
          <div className="space-y-2">
            {data.queued_sessions.map((session, index) => (
              <div
                key={session.id}
                className="bg-slate-800 border border-slate-700 rounded-lg p-3 flex items-center justify-between"
              >
                <div className="flex items-center space-x-3">
                  <span className="text-sm text-slate-400">#{index + 1}</span>
                  <span className="text-white">{session.input_filename}</span>
                  <div className={`px-2 py-1 text-xs rounded-full ${getTranscoderTypeColor(session.transcoder_type)}`}>
                    {session.transcoder_type.toUpperCase()}
                  </div>
                </div>
                <div className="text-sm text-slate-400">
                  Waiting for available slot
                </div>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Nerd Panel */}
      {isNerdMode && (
        <div className="bg-slate-800 border border-slate-700 rounded-lg p-6">
          <h3 className="text-lg font-medium text-white mb-4">Advanced Diagnostics</h3>
          <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
            <div>
              <h4 className="text-sm font-medium text-white mb-2">System Resources</h4>
              <div className="space-y-2 text-sm">
                <div className="flex justify-between">
                  <span className="text-slate-400">CPU Usage:</span>
                  <span className="text-white">25.5%</span>
                </div>
                <div className="flex justify-between">
                  <span className="text-slate-400">Memory:</span>
                  <span className="text-white">8.2GB / 16GB</span>
                </div>
                <div className="flex justify-between">
                  <span className="text-slate-400">GPU Util:</span>
                  <span className="text-white">N/A (Software)</span>
                </div>
              </div>
            </div>
            <div>
              <h4 className="text-sm font-medium text-white mb-2">Performance Metrics</h4>
              <div className="space-y-2 text-sm">
                <div className="flex justify-between">
                  <span className="text-slate-400">Avg Quality:</span>
                  <span className="text-white">92%</span>
                </div>
                <div className="flex justify-between">
                  <span className="text-slate-400">Error Rate:</span>
                  <span className="text-white">{(data.quick_stats.error_rate * 100).toFixed(2)}%</span>
                </div>
                <div className="flex justify-between">
                  <span className="text-slate-400">Uptime:</span>
                  <span className="text-white">1h 23m</span>
                </div>
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  );
};

export default SessionViewer; 