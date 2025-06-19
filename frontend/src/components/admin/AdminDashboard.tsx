import React, { useState, useEffect, useCallback } from 'react';
import {
  Server,
  Settings,
  AlertTriangle,
  TrendingUp,
} from 'lucide-react';
import { ConfigEditor } from '@/components/plugins';
import PluginDashboard from '../dashboard/PluginDashboard';
import type {
  Plugin,
  APIResponse,
} from '@/types/plugin.types';
import pluginApi from '@/lib/api/plugins';

interface AdminDashboardProps {
  activeTab?: string;
  onTabChange?: (tab: string) => void;
}

const AdminDashboard: React.FC<AdminDashboardProps> = ({
  activeTab = 'dashboard',
  onTabChange,
}) => {
  const [plugins, setPlugins] = useState<Plugin[]>([]);
  
  // UI States
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [configEditorPlugin, setConfigEditorPlugin] = useState<Plugin | null>(null);

  const loadData = useCallback(async () => {
    try {
      setLoading(true);
      setError(null);

      const response = await pluginApi.listAllPlugins(1, 100, {});

      if (response.success && response.data) {
        setPlugins(response.data);
      }

    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load data');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    loadData();
    
    // Set up periodic refresh for real-time updates
    const interval = setInterval(loadData, 30000); // Refresh every 30 seconds
    return () => clearInterval(interval);
  }, [loadData]);

  const handlePluginAction = async (plugin: Plugin, action: 'enable' | 'disable' | 'restart' | 'reload') => {
    try {
      let response: APIResponse<void>;
      
      switch (action) {
        case 'enable':
          response = await pluginApi.enablePlugin(plugin.id || plugin.name);
          break;
        case 'disable':
          response = await pluginApi.disablePlugin(plugin.id || plugin.name);
          break;
        case 'restart':
          response = await pluginApi.restartPlugin(plugin.id || plugin.name);
          break;
        case 'reload':
          response = await pluginApi.reloadPlugin(plugin.id || plugin.name);
          break;
      }

      if (response.success) {
        await loadData(); // Refresh data
      } else {
        setError(response.error || `Failed to ${action} plugin`);
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : `Failed to ${action} plugin`);
    }
  };

  const renderPluginGrid = () => (
    <div className="space-y-6">
      {/* Plugin Grid */}
      <div className="grid gap-6 md:grid-cols-2 lg:grid-cols-3">
        {plugins.map((plugin) => (
          <div
            key={plugin.id || plugin.name}
            className="bg-slate-800/50 border border-slate-700 rounded-lg p-6 hover:border-slate-600 transition-colors"
          >
            <div className="flex items-start justify-between mb-4">
              <div className="flex-1">
                <div className="flex items-center space-x-3 mb-2">
                  <h3 className="text-lg font-semibold text-white">{plugin.name}</h3>
                  <span
                    className={`px-2 py-1 text-xs rounded-full ${
                      plugin.enabled
                        ? 'bg-green-600/20 text-green-400 border border-green-600'
                        : 'bg-red-600/20 text-red-400 border border-red-600'
                    }`}
                  >
                    {plugin.enabled ? 'Enabled' : 'Disabled'}
                  </span>
                </div>
                <p className="text-sm text-slate-400 mb-3">{plugin.description}</p>
                <div className="text-xs text-slate-500">
                  Version: {plugin.version} • Type: {plugin.type}
                </div>
              </div>
            </div>

            {/* Prominent Configuration Button */}
            <div className="space-y-3">
              <button
                onClick={() => setConfigEditorPlugin(plugin)}
                className="w-full px-4 py-3 bg-purple-600 text-white rounded-lg hover:bg-purple-700 transition-colors font-medium"
              >
                <Settings size={16} className="inline mr-2" />
                Configure Plugin
              </button>
              
              {/* Action Buttons */}
              <div className="flex space-x-2">
                {plugin.enabled ? (
                  <>
                    <button
                      onClick={() => handlePluginAction(plugin, 'restart')}
                      className="flex-1 px-3 py-2 bg-blue-600/20 text-blue-400 border border-blue-600 rounded hover:bg-blue-600/30 transition-colors text-sm"
                    >
                      Restart
                    </button>
                    <button
                      onClick={() => handlePluginAction(plugin, 'disable')}
                      className="flex-1 px-3 py-2 bg-red-600/20 text-red-400 border border-red-600 rounded hover:bg-red-600/30 transition-colors text-sm"
                    >
                      Disable
                    </button>
                  </>
                ) : (
                  <button
                    onClick={() => handlePluginAction(plugin, 'enable')}
                    className="flex-1 px-3 py-2 bg-green-600/20 text-green-400 border border-green-600 rounded hover:bg-green-600/30 transition-colors text-sm"
                  >
                    Enable
                  </button>
                )}
              </div>
            </div>
          </div>
        ))}
      </div>

      {plugins.length === 0 && (
        <div className="text-center py-12">
          <Server size={48} className="text-slate-600 mx-auto mb-4" />
          <div className="text-slate-400 mb-2">No plugins found</div>
          <p className="text-sm text-slate-500">
            Install plugins to see them here
          </p>
        </div>
      )}
    </div>
  );

  const tabs = [
    { id: 'dashboard', label: 'Dashboard', icon: TrendingUp },
    { id: 'plugins', label: 'Plugins', icon: Server },
  ];

  if (loading) {
    return (
      <div className="flex items-center justify-center py-12">
        <span className="text-slate-300">Loading admin dashboard...</span>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-white">Plugin Administration</h1>
          <p className="text-slate-400 mt-1">Manage plugins and view real-time dashboard</p>
        </div>
      </div>

      {/* Error Display */}
      {error && (
        <div className="bg-red-600/20 border border-red-600 rounded-lg p-4">
          <div className="flex items-center">
            <AlertTriangle size={20} className="text-red-400 mr-2" />
            <span className="text-red-400">{error}</span>
            <button
              onClick={() => setError(null)}
              className="ml-auto text-red-400 hover:text-red-300"
            >
              ×
            </button>
          </div>
        </div>
      )}

      {/* Tabs */}
      <div className="border-b border-slate-700">
        <div className="flex space-x-8">
          {tabs.map((tab) => {
            const Icon = tab.icon;
            return (
              <button
                key={tab.id}
                onClick={() => onTabChange?.(tab.id)}
                className={`flex items-center space-x-2 pb-4 border-b-2 transition-colors ${
                  activeTab === tab.id
                    ? 'border-purple-500 text-purple-400'
                    : 'border-transparent text-slate-400 hover:text-slate-300'
                }`}
              >
                <Icon size={16} />
                <span>{tab.label}</span>
              </button>
            );
          })}
        </div>
      </div>

      {/* Tab Content */}
      <div>
        {activeTab === 'dashboard' && <PluginDashboard />}
        {activeTab === 'plugins' && renderPluginGrid()}
      </div>

      {/* Configuration Editor Modal */}
      {configEditorPlugin && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4">
          <div className="bg-slate-800 border border-slate-700 rounded-lg max-w-4xl w-full max-h-[90vh] overflow-y-auto">
            <div className="flex items-center justify-between p-6 border-b border-slate-700">
              <h2 className="text-xl font-bold text-white">Configure {configEditorPlugin.name}</h2>
              <button
                onClick={() => setConfigEditorPlugin(null)}
                className="text-slate-400 hover:text-white"
              >
                ×
              </button>
            </div>
            <div className="p-6">
              <ConfigEditor
                plugin={configEditorPlugin}
                onConfigChange={() => {
                  // Refresh plugin data after config change
                  loadData();
                }}
              />
            </div>
          </div>
        </div>
      )}
    </div>
  );
};

export default AdminDashboard;
