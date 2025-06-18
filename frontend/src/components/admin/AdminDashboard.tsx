import React, { useState, useEffect, useCallback } from 'react';
import {
  Activity,
  Server,
  Zap,
  Settings,
  RefreshCw,
  Search,
  AlertTriangle,
  CheckCircle,
  XCircle,
  Clock,
  TrendingUp,
  ChevronDown,
} from 'lucide-react';
import { ConfigEditor, PluginAdminPageRenderer } from '@/components/plugins';
import type {
  Plugin,
  AdminPage,
  APIResponse,
  PluginFilters,
} from '@/types/plugin.types';
import pluginApi from '@/lib/api/plugins';

interface AdminDashboardProps {
  activeTab?: string;
  onTabChange?: (tab: string) => void;
}

interface SystemStats {
  total_plugins: number;
  enabled_plugins: number;
  disabled_plugins: number;
  healthy_plugins: number;
  unhealthy_plugins: number;
  total_admin_pages: number;
  hot_reload_enabled: boolean;
  memory_usage: number;
  cpu_usage: number;
  uptime: number;
}

interface HotReloadStatus {
  enabled: boolean;
  pending_reloads?: number;
  pending_plugins?: string[];
  debounce_delay?: string;
  error?: string;
}

const AdminDashboard: React.FC<AdminDashboardProps> = ({
  activeTab = 'overview',
  onTabChange,
}) => {
  const [plugins, setPlugins] = useState<Plugin[]>([]);
  const [adminPages, setAdminPages] = useState<AdminPage[]>([]);
  const [systemStats, setSystemStats] = useState<SystemStats | null>(null);
  const [hotReloadStatus, setHotReloadStatus] = useState<HotReloadStatus | null>(null);
  const [selectedAdminPage, setSelectedAdminPage] = useState<AdminPage | null>(null);
  
  // UI States
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [searchTerm, setSearchTerm] = useState('');
  const [filters, setFilters] = useState<PluginFilters>({});
  const [refreshing, setRefreshing] = useState(false);
  const [expandedCards, setExpandedCards] = useState<Record<string, boolean>>({});

  const [configEditorPlugin, setConfigEditorPlugin] = useState<Plugin | null>(null);

  const loadData = useCallback(async () => {
    try {
      setLoading(true);
      setError(null);

      // Load all data in parallel
      const [
        pluginsResponse,
        adminPagesResponse,
        systemStatsResponse,
        hotReloadResponse,
      ] = await Promise.all([
        pluginApi.listAllPlugins(1, 100, filters),
        pluginApi.getAllAdminPages(),
        pluginApi.getSystemStats(),
        pluginApi.getHotReloadStatus(),
      ]);

      if (pluginsResponse.success && pluginsResponse.data) {
        setPlugins(pluginsResponse.data);
      }

      if (adminPagesResponse.success && adminPagesResponse.data) {
        setAdminPages(adminPagesResponse.data);
        
        // Auto-expand plugins that have admin pages
        const pluginsWithAdminPages = adminPagesResponse.data.reduce((acc, page) => {
          const pathParts = page.path.split('/');
          const pluginId = pathParts[3]; // /admin/plugins/{plugin_id}/...
          acc[`plugin-${pluginId}`] = true;
          return acc;
        }, {} as Record<string, boolean>);
        
        setExpandedCards(prev => ({
          ...prev,
          ...pluginsWithAdminPages
        }));
      }

      if (systemStatsResponse.success && systemStatsResponse.data) {
        setSystemStats(systemStatsResponse.data as SystemStats);
      }

      if (hotReloadResponse.success && hotReloadResponse.data) {
        setHotReloadStatus(hotReloadResponse.data);
      }

    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load data');
    } finally {
      setLoading(false);
    }
  }, [filters]);

  useEffect(() => {
    loadData();
    
    // Set up periodic refresh for real-time updates
    const interval = setInterval(loadData, 30000); // Refresh every 30 seconds
    return () => clearInterval(interval);
  }, [loadData]);

  const handleRefresh = async () => {
    setRefreshing(true);
    await loadData();
    setRefreshing(false);
  };

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

  const handleHotReloadToggle = async () => {
    try {
      if (hotReloadStatus?.enabled) {
        await pluginApi.disableHotReload();
      } else {
        await pluginApi.enableHotReload();
      }
      
      // Refresh hot reload status
      const response = await pluginApi.getHotReloadStatus();
      if (response.success && response.data) {
        setHotReloadStatus(response.data);
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to toggle hot reload');
    }
  };

  const filteredPlugins = plugins.filter(plugin => {
    if (searchTerm) {
      return plugin.name.toLowerCase().includes(searchTerm.toLowerCase()) ||
             plugin.description?.toLowerCase().includes(searchTerm.toLowerCase());
    }
    return true;
  });

  const toggleCardExpansion = (cardId: string) => {
    setExpandedCards(prev => ({
      ...prev,
      [cardId]: !prev[cardId]
    }));
  };

  const renderSystemOverview = () => (
    <div className="space-y-6">
      {/* System Stats Cards */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6">
        <div className="bg-slate-800 border border-slate-700 rounded-lg p-6">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm text-slate-400">Total Plugins</p>
              <p className="text-2xl font-bold text-white">{systemStats?.total_plugins || 0}</p>
            </div>
            <Server className="text-blue-400" size={24} />
          </div>
          <div className="mt-4 flex text-xs text-slate-400">
            <span className="flex items-center">
              <CheckCircle size={12} className="text-green-400 mr-1" />
              {systemStats?.enabled_plugins || 0} enabled
            </span>
            <span className="flex items-center ml-4">
              <XCircle size={12} className="text-gray-400 mr-1" />
              {systemStats?.disabled_plugins || 0} disabled
            </span>
          </div>
        </div>

        <div className="bg-slate-800 border border-slate-700 rounded-lg p-6">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm text-slate-400">Plugin Health</p>
              <p className="text-2xl font-bold text-white">{systemStats?.healthy_plugins || 0}</p>
            </div>
            <Activity className="text-green-400" size={24} />
          </div>
          <div className="mt-4 flex text-xs text-slate-400">
            <span className="flex items-center">
              <CheckCircle size={12} className="text-green-400 mr-1" />
              {systemStats?.healthy_plugins || 0} healthy
            </span>
            <span className="flex items-center ml-4">
              <AlertTriangle size={12} className="text-red-400 mr-1" />
              {systemStats?.unhealthy_plugins || 0} issues
            </span>
          </div>
        </div>

        <div className="bg-slate-800 border border-slate-700 rounded-lg p-6">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm text-slate-400">Admin Pages</p>
              <p className="text-2xl font-bold text-white">{systemStats?.total_admin_pages || adminPages.length}</p>
            </div>
            <Settings className="text-purple-400" size={24} />
          </div>
          <div className="mt-4 flex text-xs text-slate-400">
            <span>Available interfaces</span>
          </div>
        </div>

        <div className="bg-slate-800 border border-slate-700 rounded-lg p-6">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm text-slate-400">Hot Reload</p>
              <p className="text-2xl font-bold text-white">
                {hotReloadStatus?.enabled ? 'ON' : 'OFF'}
              </p>
            </div>
            <Zap className={`${hotReloadStatus?.enabled ? 'text-yellow-400' : 'text-gray-400'}`} size={24} />
          </div>
          <div className="mt-4">
            <button
              onClick={handleHotReloadToggle}
              className={`text-xs px-2 py-1 rounded ${
                hotReloadStatus?.enabled 
                  ? 'bg-yellow-600/20 text-yellow-400 border border-yellow-600' 
                  : 'bg-gray-600/20 text-gray-400 border border-gray-600'
              }`}
            >
              {hotReloadStatus?.enabled ? 'Disable' : 'Enable'}
            </button>
          </div>
        </div>
      </div>

      {/* System Performance */}
      {systemStats && (
        <div className="bg-slate-800 border border-slate-700 rounded-lg p-6">
          <div className="flex items-center justify-between mb-4">
            <h3 className="text-lg font-medium text-white">System Performance</h3>
            <TrendingUp className="text-blue-400" size={20} />
          </div>
          
          <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
            <div className="space-y-2">
              <div className="flex items-center justify-between">
                <span className="text-sm text-slate-400">Memory Usage</span>
                <span className="text-sm text-white">{systemStats.memory_usage?.toFixed(1) || 0}%</span>
              </div>
              <div className="w-full bg-slate-700 rounded-full h-2">
                <div 
                  className="bg-blue-500 h-2 rounded-full transition-all duration-300" 
                  style={{ width: `${Math.min(systemStats.memory_usage || 0, 100)}%` }}
                />
              </div>
            </div>
            
            <div className="space-y-2">
              <div className="flex items-center justify-between">
                <span className="text-sm text-slate-400">CPU Usage</span>
                <span className="text-sm text-white">{systemStats.cpu_usage?.toFixed(1) || 0}%</span>
              </div>
              <div className="w-full bg-slate-700 rounded-full h-2">
                <div 
                  className="bg-green-500 h-2 rounded-full transition-all duration-300" 
                  style={{ width: `${Math.min(systemStats.cpu_usage || 0, 100)}%` }}
                />
              </div>
            </div>
            
            <div className="space-y-2">
              <div className="flex items-center justify-between">
                <span className="text-sm text-slate-400">Uptime</span>
                <span className="text-sm text-white">
                  {systemStats.uptime ? `${Math.floor(systemStats.uptime / 3600)}h ${Math.floor((systemStats.uptime % 3600) / 60)}m` : 'N/A'}
                </span>
              </div>
              <div className="flex items-center text-xs text-slate-400">
                <Clock size={12} className="mr-1" />
                System running normally
              </div>
            </div>
          </div>
        </div>
      )}

      {/* Hot Reload Status */}
      {hotReloadStatus && hotReloadStatus.enabled && (
        <div className="bg-yellow-600/10 border border-yellow-600/30 rounded-lg p-4">
          <div className="flex items-center justify-between">
            <div className="flex items-center">
              <Zap className="text-yellow-400 mr-2" size={20} />
              <div>
                <h4 className="text-sm font-medium text-yellow-300">Hot Reload Active</h4>
                <p className="text-xs text-yellow-400/80">
                  Monitoring plugin changes for automatic reloading
                </p>
              </div>
            </div>
            <div className="text-right">
              {hotReloadStatus.pending_reloads && hotReloadStatus.pending_reloads > 0 && (
                <div className="text-xs text-yellow-300">
                  {hotReloadStatus.pending_reloads} pending reload{hotReloadStatus.pending_reloads !== 1 ? 's' : ''}
                </div>
              )}
              <div className="text-xs text-yellow-400/60">
                Debounce: {hotReloadStatus.debounce_delay || '500ms'}
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  );

  const renderPluginGrid = () => (
    <div className="space-y-6">
      {/* Search and Filters */}
      <div className="flex flex-col sm:flex-row gap-4">
        <div className="flex-1 relative">
          <Search className="absolute left-3 top-1/2 transform -translate-y-1/2 text-slate-400" size={16} />
          <input
            type="text"
            placeholder="Search plugins..."
            value={searchTerm}
            onChange={(e) => setSearchTerm(e.target.value)}
            className="w-full pl-10 pr-4 py-2 bg-slate-800 border border-slate-700 rounded-lg text-white placeholder-slate-400 focus:outline-none focus:ring-2 focus:ring-purple-500"
          />
        </div>
        
        <select
          value={filters.category || ''}
          onChange={(e) => setFilters(prev => ({ ...prev, category: e.target.value || undefined }))}
          className="px-4 py-2 bg-slate-800 border border-slate-700 rounded-lg text-white focus:outline-none focus:ring-2 focus:ring-purple-500"
        >
          <option value="">All Categories</option>
          <option value="core">Core</option>
          <option value="external">External</option>
          <option value="enrichment">Enrichment</option>
          <option value="transcoder">Transcoder</option>
        </select>

        <select
          value={filters.status || ''}
          onChange={(e) => setFilters(prev => ({ ...prev, status: e.target.value || undefined }))}
          className="px-4 py-2 bg-slate-800 border border-slate-700 rounded-lg text-white focus:outline-none focus:ring-2 focus:ring-purple-500"
        >
          <option value="">All Status</option>
          <option value="enabled">Enabled</option>
          <option value="disabled">Disabled</option>
        </select>
      </div>

      {/* Plugin Grid */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
        {filteredPlugins.map((plugin) => (
          <div
            key={plugin.id || plugin.name}
            className="bg-slate-800 border border-slate-700 rounded-lg p-6 hover:border-slate-600 transition-colors"
          >
            <div className="flex items-start justify-between mb-4">
              <div className="flex-1">
                <h3 className="font-medium text-white truncate">{plugin.name}</h3>
                <p className="text-sm text-slate-400 mt-1">v{plugin.version}</p>
              </div>
              <div className="flex items-center space-x-2">
                <span
                  className={`px-2 py-1 text-xs rounded-full ${
                    plugin.enabled
                      ? 'bg-green-600/20 text-green-400 border border-green-600'
                      : 'bg-gray-600/20 text-gray-400 border border-gray-600'
                  }`}
                >
                  {plugin.enabled ? 'Enabled' : 'Disabled'}
                </span>
                <button
                  onClick={() => toggleCardExpansion(plugin.id || plugin.name)}
                  className="text-slate-400 hover:text-white"
                >
                  <ChevronDown 
                    size={16} 
                    className={`transform transition-transform ${expandedCards[plugin.id || plugin.name] ? 'rotate-180' : ''}`} 
                  />
                </button>
              </div>
            </div>

            <div className="space-y-3">
              <p className="text-sm text-slate-300 line-clamp-2">{plugin.description}</p>
              
              <div className="flex items-center space-x-2 text-xs text-slate-400">
                <span className={`px-2 py-1 rounded ${plugin.is_core ? 'bg-blue-600/20 text-blue-400' : 'bg-purple-600/20 text-purple-400'}`}>
                  {plugin.is_core ? 'Core' : 'External'}
                </span>
                <span className="px-2 py-1 bg-slate-700 rounded">{plugin.type}</span>
              </div>

              {expandedCards[plugin.id || plugin.name] && (
                <div className="space-y-3 pt-3 border-t border-slate-700">
                  <div className="grid grid-cols-2 gap-3">
                    <button
                      onClick={() => handlePluginAction(plugin, plugin.enabled ? 'disable' : 'enable')}
                      className={`px-3 py-2 rounded text-sm font-medium transition-colors ${
                        plugin.enabled
                          ? 'bg-red-600 hover:bg-red-700 text-white'
                          : 'bg-green-600 hover:bg-green-700 text-white'
                      }`}
                    >
                      {plugin.enabled ? 'Disable' : 'Enable'}
                    </button>
                    
                    <button
                      onClick={() => handlePluginAction(plugin, 'restart')}
                      disabled={!plugin.enabled}
                      className="px-3 py-2 rounded text-sm font-medium bg-blue-600 hover:bg-blue-700 text-white disabled:bg-gray-600 disabled:cursor-not-allowed transition-colors"
                    >
                      Restart
                    </button>
                  </div>

                  <div className="grid grid-cols-2 gap-3">
                    <button
                      onClick={() => setConfigEditorPlugin(plugin)}
                      className="px-3 py-2 rounded text-sm font-medium bg-purple-600 hover:bg-purple-700 text-white transition-colors"
                    >
                      Configure
                    </button>
                    
                    <button
                      onClick={() => handlePluginAction(plugin, 'reload')}
                      disabled={!plugin.enabled || plugin.is_core}
                      className="px-3 py-2 rounded text-sm font-medium bg-yellow-600 hover:bg-yellow-700 text-white disabled:bg-gray-600 disabled:cursor-not-allowed transition-colors"
                    >
                      Hot Reload
                    </button>
                  </div>

                  {plugin.health && (
                    <div className="text-xs text-slate-400 space-y-1">
                      <div className="flex items-center justify-between">
                        <span>Health:</span>
                        <span className={`${
                          plugin.health.status === 'healthy' ? 'text-green-400' : 
                          plugin.health.status === 'unhealthy' ? 'text-red-400' : 'text-yellow-400'
                        }`}>
                          {plugin.health.status}
                        </span>
                      </div>
                      {plugin.health.average_response_time && (
                        <div className="flex items-center justify-between">
                          <span>Response:</span>
                          <span>{plugin.health.average_response_time}</span>
                        </div>
                      )}
                    </div>
                  )}
                </div>
              )}
            </div>
          </div>
        ))}
      </div>

      {filteredPlugins.length === 0 && (
        <div className="text-center py-12">
          <div className="text-slate-400 mb-2">No plugins found</div>
          <p className="text-sm text-slate-500">
            {searchTerm ? 'Try adjusting your search or filters' : 'No plugins are currently available'}
          </p>
        </div>
      )}
    </div>
  );

  const renderAdminPages = () => {
    // Group admin pages by plugin
    const pagesByPlugin = adminPages.reduce((acc, page) => {
      // Extract plugin name from the path
      const pathParts = page.path.split('/');
      const pluginId = pathParts[3]; // /admin/plugins/{plugin_id}/...
      
      if (!acc[pluginId]) {
        // Find the plugin info
        const plugin = plugins.find(p => p.id === pluginId);
        const defaultPlugin: Plugin = {
          id: pluginId,
          name: pluginId,
          type: 'unknown',
          version: '1.0.0',
          description: '',
          enabled: false,
          status: 'disabled',
          is_core: false
        };
        acc[pluginId] = {
          plugin: plugin || defaultPlugin,
          pages: []
        };
      }
      
      acc[pluginId].pages.push(page);
      return acc;
    }, {} as Record<string, { plugin: Plugin; pages: AdminPage[] }>);

    return (
      <div className="space-y-6">
        {/* Header with stats */}
        <div className="bg-slate-800 border border-slate-700 rounded-lg p-6">
          <div className="flex items-center justify-between">
            <div>
              <h2 className="text-lg font-medium text-white">Plugin Admin Interfaces</h2>
              <p className="text-sm text-slate-400 mt-1">
                Configure and monitor plugin-specific settings and features
              </p>
            </div>
            <div className="text-right">
              <div className="text-2xl font-bold text-white">{adminPages.length}</div>
              <div className="text-xs text-slate-400">admin pages</div>
            </div>
          </div>
        </div>

        {/* Plugin groups */}
        {Object.entries(pagesByPlugin).map(([pluginId, { plugin, pages }]) => (
          <div key={pluginId} className="bg-slate-800 border border-slate-700 rounded-lg">
            {/* Plugin header */}
            <div className="flex items-center justify-between p-6 border-b border-slate-700">
              <div className="flex items-center space-x-4">
                <div className="flex-1">
                  <h3 className="text-lg font-medium text-white flex items-center">
                    {plugin.name}
                    <span
                      className={`ml-3 px-2 py-1 text-xs rounded-full ${
                        plugin.enabled
                          ? 'bg-green-600/20 text-green-400 border border-green-600'
                          : 'bg-gray-600/20 text-gray-400 border border-gray-600'
                      }`}
                    >
                      {plugin.enabled ? 'Enabled' : 'Disabled'}
                    </span>
                  </h3>
                  <p className="text-sm text-slate-400 mt-1">
                    {plugin.type} • {pages.length} admin interface{pages.length !== 1 ? 's' : ''}
                  </p>
                </div>
              </div>
              <div className="flex items-center space-x-2">
                {plugin.enabled && (
                  <span className="text-xs text-green-400 bg-green-600/10 px-2 py-1 rounded">
                    Ready
                  </span>
                )}
                <button
                  onClick={() => toggleCardExpansion(`plugin-${pluginId}`)}
                  className="text-slate-400 hover:text-white transition-colors"
                >
                  <ChevronDown 
                    size={16} 
                    className={`transform transition-transform ${expandedCards[`plugin-${pluginId}`] ? 'rotate-180' : ''}`} 
                  />
                </button>
              </div>
            </div>

            {/* Plugin admin pages */}
            {expandedCards[`plugin-${pluginId}`] && (
              <div className="p-6">
                <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
                  {pages.map((page) => (
                    <div
                      key={page.id}
                      className="bg-slate-700/50 border border-slate-600 rounded-lg p-4 hover:border-slate-500 transition-colors cursor-pointer group"
                      onClick={() => setSelectedAdminPage(page)}
                    >
                      <div className="flex items-start justify-between mb-3">
                        <div className="flex-1">
                          <h4 className="font-medium text-white group-hover:text-purple-300 transition-colors">
                            {page.title}
                          </h4>
                          <p className="text-xs text-slate-400 mt-1 truncate">{page.path}</p>
                        </div>
                        {page.icon && (
                          <span className="text-slate-400 text-sm">{page.icon}</span>
                        )}
                      </div>

                      <div className="flex items-center justify-between">
                        {page.category && (
                          <span className="px-2 py-1 text-xs rounded bg-slate-600/50 text-slate-300 border border-slate-600">
                            {page.category}
                          </span>
                        )}
                        <span className="text-xs text-slate-400 group-hover:text-purple-400 transition-colors">
                          Open →
                        </span>
                      </div>
                    </div>
                  ))}
                </div>

                {/* Quick actions for the plugin */}
                <div className="mt-4 pt-4 border-t border-slate-700 flex items-center justify-between">
                  <div className="text-xs text-slate-400">
                    Plugin ID: <code className="text-slate-300">{pluginId}</code>
                  </div>
                  <div className="flex items-center space-x-2">
                    <button
                      onClick={() => setConfigEditorPlugin(plugin)}
                      className="text-xs px-3 py-1 bg-purple-600/20 text-purple-400 border border-purple-600 rounded hover:bg-purple-600/30 transition-colors"
                    >
                      Configure
                    </button>
                    {plugin.enabled && (
                      <button
                        onClick={() => handlePluginAction(plugin, 'restart')}
                        className="text-xs px-3 py-1 bg-blue-600/20 text-blue-400 border border-blue-600 rounded hover:bg-blue-600/30 transition-colors"
                      >
                        Restart
                      </button>
                    )}
                  </div>
                </div>
              </div>
            )}
          </div>
        ))}

        {adminPages.length === 0 && (
          <div className="text-center py-12">
            <Settings size={48} className="text-slate-600 mx-auto mb-4" />
            <div className="text-slate-400 mb-2">No admin pages available</div>
            <p className="text-sm text-slate-500">
              Enable plugins to see their configuration interfaces here
            </p>
          </div>
        )}
      </div>
    );
  };

  const tabs = [
    { id: 'overview', label: 'Overview', icon: Activity },
    { id: 'plugins', label: 'Plugins', icon: Server },
    { id: 'pages', label: 'Admin Pages', icon: Settings },
  ];

  if (loading) {
    return (
      <div className="flex items-center justify-center py-12">
        <RefreshCw size={24} className="animate-spin text-purple-400 mr-3" />
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
          <p className="text-slate-400 mt-1">Manage plugins, configurations, and system settings</p>
        </div>
        <button
          onClick={handleRefresh}
          disabled={refreshing}
          className="flex items-center space-x-2 bg-purple-600 hover:bg-purple-700 disabled:bg-purple-600/50 px-4 py-2 rounded-lg text-white transition-colors"
        >
          <RefreshCw size={16} className={refreshing ? 'animate-spin' : ''} />
          <span>Refresh</span>
        </button>
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
        {activeTab === 'overview' && renderSystemOverview()}
        {activeTab === 'plugins' && renderPluginGrid()}
        {activeTab === 'pages' && renderAdminPages()}
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

      {/* Admin Page Renderer Modal */}
      {selectedAdminPage && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4">
          <div className="bg-slate-800 border border-slate-700 rounded-lg max-w-6xl w-full max-h-[90vh] overflow-y-auto">
            <div className="flex items-center justify-between p-6 border-b border-slate-700">
              <h2 className="text-xl font-bold text-white">{selectedAdminPage.title}</h2>
              <button
                onClick={() => setSelectedAdminPage(null)}
                className="text-slate-400 hover:text-white"
              >
                ×
              </button>
            </div>
            <div className="p-6">
              <PluginAdminPageRenderer
                page={selectedAdminPage}
                plugins={plugins}
                onBack={() => setSelectedAdminPage(null)}
              />
            </div>
          </div>
        </div>
      )}
    </div>
  );
};

export default AdminDashboard;
