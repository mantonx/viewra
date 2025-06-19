import React, { useState, useEffect, useCallback } from 'react';
import { 
  Activity, 
  Clock, 
  AlertTriangle, 
  Monitor 
} from 'lucide-react';
import { useDashboardWebSocket } from '@/hooks';
import TranscoderSectionRenderer from './sections/TranscoderSectionRenderer';
import type { DashboardSection } from '@/types/dashboard.types';

interface PluginDashboardProps {
  className?: string;
}

interface TranscoderMainData {
  active_sessions: TranscodeSessionSummary[];
  queued_sessions: TranscodeSessionSummary[];
  recent_sessions: TranscodeSessionSummary[];
  engine_status: TranscoderEngineStatus;
  quick_stats: TranscoderQuickStats;
}

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
  transcoder_type: string; // Now shows the actual transcoder (ffmpeg, nvenc, etc.)
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

interface DashboardData {
  [sectionId: string]: TranscoderMainData | Record<string, unknown>;
}

interface NerdModeState {
  [sectionId: string]: boolean;
}

interface WebSocketMessage {
  type: string;
  section_id?: string;
  data?: TranscoderMainData | Record<string, unknown>;
  data_type?: string;
  timestamp?: number;
  error?: string;
}

const isTranscoderMainData = (data: TranscoderMainData | Record<string, unknown>): data is TranscoderMainData => {
  return data && typeof data === 'object' && 'active_sessions' in data && 'engine_status' in data;
};

const PluginDashboard: React.FC<PluginDashboardProps> = ({ className }) => {
  const [sections, setSections] = useState<DashboardSection[]>([]);
  const [sectionData, setSectionData] = useState<DashboardData>({});
  const [nerdData, setNerdData] = useState<DashboardData>({});
  const [nerdMode, setNerdMode] = useState<NerdModeState>({});
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [lastUpdate, setLastUpdate] = useState<Date>(new Date());

  // WebSocket connection for real-time updates
  const handleWebSocketMessage = useCallback((message: WebSocketMessage) => {
    if (message.type === 'section_data' && message.section_id && message.data) {
      const sectionId = message.section_id;
      if (message.data_type === 'main') {
        setSectionData(prev => ({
          ...prev,
          [sectionId]: message.data!
        }));
        setLastUpdate(new Date());
      } else if (message.data_type === 'nerd') {
        setNerdData(prev => ({
          ...prev,
          [sectionId]: message.data!
        }));
      }
    } else if (message.type === 'section_update') {
      // Refresh sections list
      loadSections();
    }
  }, []);

  const { connected: wsConnected, error: wsError } = useDashboardWebSocket(handleWebSocketMessage, {
    enabled: true
  });

  // Load sections from API (initial load only)
  const loadSections = useCallback(async () => {
    try {
      const response = await fetch('/api/v1/dashboard/sections');
      if (response.ok) {
        const result = await response.json();
        if (result.success) {
          setSections(result.data || []);
        } else {
          setError('Failed to load dashboard sections');
        }
      } else {
        setError('Failed to load dashboard sections');
      }
    } catch (err) {
      console.error('Failed to load sections:', err);
      setError('Failed to load dashboard sections');
    }
  }, []);

  // Load nerd data when nerd mode is toggled
  const loadNerdData = useCallback(async (sectionId: string) => {
    try {
      const response = await fetch(`/api/v1/dashboard/sections/${sectionId}/data/nerd`);
      if (response.ok) {
        const result = await response.json();
        if (result.success) {
          setNerdData(prev => ({
            ...prev,
            [sectionId]: result.data
          }));
        }
      }
    } catch (err) {
      console.error(`Failed to load nerd data for section ${sectionId}:`, err);
    }
  }, []);

  // Initial load
  useEffect(() => {
    const initialize = async () => {
      setLoading(true);
      await loadSections();
      setLoading(false);
    };
    
    initialize();
  }, [loadSections]);

  // Handle WebSocket errors
  useEffect(() => {
    if (wsError) {
      setError(`WebSocket error: ${wsError}`);
    }
  }, [wsError]);

  const toggleNerdMode = (sectionId: string) => {
    const newNerdMode = !nerdMode[sectionId];
    setNerdMode(prev => ({
      ...prev,
      [sectionId]: newNerdMode
    }));

    // Load nerd data if entering nerd mode
    if (newNerdMode && !nerdData[sectionId]) {
      loadNerdData(sectionId);
    }
  };

  const executeAction = async (sectionId: string, actionId: string, payload?: Record<string, unknown>) => {
    try {
      const response = await fetch(`/api/v1/dashboard/sections/${sectionId}/actions/${actionId}`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload || {})
      });

      if (!response.ok) {
        console.error(`Failed to execute action ${actionId}`);
      }
      // No need to manually refresh - WebSocket will send updates
    } catch (err) {
      console.error(`Failed to execute action ${actionId}:`, err);
    }
  };

  const renderSection = (section: DashboardSection) => {
    const mainData = sectionData[section.id];
    const sectionNerdData = nerdData[section.id];
    const isNerdMode = nerdMode[section.id] || false;

    if (section.type === 'transcoder') {
      const commonProps = {
        isNerdMode: isNerdMode,
        onToggleNerdMode: () => toggleNerdMode(section.id)
      };

      return (
        <div key={section.id}>
          {mainData && isTranscoderMainData(mainData) && (
            <TranscoderSectionRenderer
              data={mainData}
              {...commonProps}
              onSessionAction={(sessionId, action) => 
                executeAction(section.id, `session_${action}`, { sessionId })
              }
              onEngineAction={(action) => 
                executeAction(section.id, action)
              }
            />
          )}
        </div>
      );
    }

    // Default fallback for other plugin types
    return (
      <div key={section.id} className="bg-slate-800/50 border border-slate-700 rounded-lg p-6">
        <div className="flex items-center justify-between mb-4">
          <div>
            <h3 className="text-lg font-semibold text-white">{section.title}</h3>
            <p className="text-sm text-slate-400">{section.description}</p>
          </div>
          <div className="flex items-center space-x-3">
            <button
              onClick={() => toggleNerdMode(section.id)}
              className={`px-3 py-1 rounded text-xs border transition-colors ${
                isNerdMode 
                  ? 'bg-purple-600/20 text-purple-400 border-purple-600' 
                  : 'bg-slate-700 text-slate-400 border-slate-600 hover:border-slate-500'
              }`}
            >
              {isNerdMode ? 'Simple View' : 'Advanced'}
            </button>
          </div>
        </div>
        
        {mainData ? (
          <div className="space-y-4">
            <div className="bg-slate-700/50 rounded-lg p-4">
              <pre className="text-sm text-slate-300 overflow-auto">
                {JSON.stringify(mainData, null, 2)}
              </pre>
            </div>
            
            {isNerdMode && sectionNerdData && (
              <div className="bg-slate-700/30 rounded-lg p-4 border-l-4 border-purple-600">
                <h4 className="text-sm font-medium text-purple-400 mb-2">Advanced Metrics</h4>
                <pre className="text-sm text-slate-300 overflow-auto">
                  {JSON.stringify(sectionNerdData, null, 2)}
                </pre>
              </div>
            )}
          </div>
        ) : (
          <div className="text-center py-8 text-slate-400">
            Loading section data...
          </div>
        )}
      </div>
    );
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center py-12">
        <Monitor size={24} className="animate-pulse text-purple-400 mr-3" />
        <span className="text-slate-300">Loading dashboard...</span>
      </div>
    );
  }

  if (sections.length === 0) {
    return (
      <div className="text-center py-12">
        <div className="flex flex-col items-center space-y-4">
          <div className="w-16 h-16 bg-slate-700 rounded-full flex items-center justify-center">
            <Monitor size={24} className="text-slate-400" />
          </div>
          <div>
            <div className="text-slate-400 text-lg font-medium">No Active Services</div>
            <p className="text-slate-500 text-sm mt-1">
              Enable plugins to see their dashboard sections here
            </p>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className={`space-y-6 ${className || ''}`}>
      {/* Error Display */}
      {error && (
        <div className="bg-red-600/10 border border-red-600 rounded-lg p-4 flex items-center space-x-3">
          <AlertTriangle size={20} className="text-red-400" />
          <div>
            <div className="text-red-400 font-medium">Dashboard Error</div>
            <div className="text-red-300 text-sm">{error}</div>
          </div>
          <button
            onClick={() => setError(null)}
            className="ml-auto text-red-400 hover:text-red-300"
          >
            ×
          </button>
        </div>
      )}

      {/* Dashboard Sections */}
      <div className="space-y-6">
        {sections.map(renderSection)}
      </div>

      {/* Status Footer */}
      <div className="text-center pt-6 border-t border-slate-700">
        <div className="flex items-center justify-center space-x-6 text-sm text-slate-400">
          <div className="flex items-center space-x-2">
            <Activity size={16} className="text-green-400" />
            <span>{sections.filter(s => sectionData[s.id]).length} active services</span>
          </div>
          <div className="flex items-center space-x-2">
            <div className={`w-2 h-2 rounded-full ${wsConnected ? 'bg-green-400' : 'bg-red-400'}`}></div>
            <span>{wsConnected ? 'Live updates' : 'Connection lost'}</span>
          </div>
          <div className="flex items-center space-x-2">
            <Clock size={16} className="text-slate-400" />
            <span>Updated {lastUpdate.toLocaleTimeString()}</span>
          </div>
        </div>
      </div>
    </div>
  );
};

export default PluginDashboard; 