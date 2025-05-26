import React, { useState, useEffect } from 'react';
import PluginInstaller from './PluginInstaller';
import PluginDependencies from './PluginDependencies';
import PluginConfigEditor from './PluginConfigEditor';
import type { Plugin, PluginResponse } from '../types/plugin.types';

const PluginManager: React.FC = () => {
  const [plugins, setPlugins] = useState<Plugin[]>([]);
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [selectedPlugin, setSelectedPlugin] = useState<Plugin | null>(null);
  const [showDetails, setShowDetails] = useState(false);

  // Fetch plugins from the API
  const loadPlugins = async () => {
    try {
      setLoading(true);
      setError(null);

      const response = await fetch('/api/admin/plugins/');
      const data = (await response.json()) as PluginResponse;

      setPlugins(data.plugins || []);
    } catch (err) {
      setError('Failed to load plugins');
      console.error('Error loading plugins:', err);
    } finally {
      setLoading(false);
    }
  };

  // Refresh plugin discovery
  const refreshPlugins = async () => {
    try {
      setRefreshing(true);
      setError(null);

      const response = await fetch('/api/admin/plugins/refresh', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
      });

      if (!response.ok) {
        throw new Error('Failed to refresh plugins');
      }

      await loadPlugins(); // Reload the plugins list
    } catch (err) {
      setError('Failed to refresh plugins');
      console.error('Error refreshing plugins:', err);
    } finally {
      setRefreshing(false);
    }
  };

  // Enable a plugin
  const enablePlugin = async (id: string) => {
    try {
      const response = await fetch(`/api/admin/plugins/${id}/enable`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
      });

      if (!response.ok) {
        throw new Error('Failed to enable plugin');
      }

      // Update local state
      setPlugins(
        plugins.map((plugin) =>
          plugin.id === id ? { ...plugin, enabled: true, status: 'active' } : plugin
        )
      );

      // If this is the selected plugin, update it too
      if (selectedPlugin?.id === id) {
        setSelectedPlugin({ ...selectedPlugin, enabled: true, status: 'active' });
      }
    } catch (err) {
      setError(`Failed to enable plugin: ${id}`);
      console.error(`Error enabling plugin ${id}:`, err);
    }
  };

  // Disable a plugin
  const disablePlugin = async (id: string) => {
    try {
      const response = await fetch(`/api/admin/plugins/${id}/disable`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
      });

      if (!response.ok) {
        throw new Error('Failed to disable plugin');
      }

      // Update local state
      setPlugins(
        plugins.map((plugin) =>
          plugin.id === id ? { ...plugin, enabled: false, status: 'disabled' } : plugin
        )
      );

      // If this is the selected plugin, update it too
      if (selectedPlugin?.id === id) {
        setSelectedPlugin({ ...selectedPlugin, enabled: false, status: 'disabled' });
      }
    } catch (err) {
      setError(`Failed to disable plugin: ${id}`);
      console.error(`Error disabling plugin ${id}:`, err);
    }
  };

  // Uninstall a plugin
  const uninstallPlugin = async (id: string) => {
    if (!confirm(`Are you sure you want to uninstall the plugin "${id}"?`)) {
      return;
    }

    try {
      const response = await fetch(`/api/admin/plugins/${id}`, {
        method: 'DELETE',
      });

      if (!response.ok) {
        throw new Error('Failed to uninstall plugin');
      }

      // Remove from local state
      setPlugins(plugins.filter((plugin) => plugin.id !== id));

      // If this was the selected plugin, close details
      if (selectedPlugin?.id === id) {
        setSelectedPlugin(null);
        setShowDetails(false);
      }
    } catch (err) {
      setError(`Failed to uninstall plugin: ${id}`);
      console.error(`Error uninstalling plugin ${id}:`, err);
    }
  };

  // View plugin details
  const viewPluginDetails = async (id: string) => {
    try {
      const response = await fetch(`/api/admin/plugins/${id}`);
      const data = await response.json();

      if (!response.ok) {
        throw new Error('Failed to get plugin details');
      }

      // Also fetch the manifest
      const manifestResponse = await fetch(`/api/admin/plugins/${id}/manifest`);
      const manifestData = await manifestResponse.json();

      if (manifestResponse.ok && manifestData.manifest) {
        data.plugin.manifest = manifestData.manifest;
      }

      // Fetch current config if there's a config schema
      if (data.plugin.manifest?.config_schema) {
        try {
          const configResponse = await fetch(`/api/admin/plugins/${id}/config`);
          const configData = await configResponse.json();

          if (configResponse.ok && configData.config) {
            data.plugin.config = configData.config;
          }
        } catch (configErr) {
          console.error(`Error loading plugin config ${id}:`, configErr);
        }
      }

      setSelectedPlugin(data.plugin);
      setShowDetails(true);
    } catch (err) {
      setError(`Failed to load plugin details: ${id}`);
      console.error(`Error loading plugin details ${id}:`, err);
    }
  };

  // Load plugins when component mounts
  useEffect(() => {
    loadPlugins();
  }, []);

  // Render the plugin type badge
  const renderTypeBadge = (type: string) => {
    const typeColors: Record<string, string> = {
      metadata_scraper: 'bg-blue-600',
      admin_page: 'bg-purple-600',
      ui_component: 'bg-green-600',
      scanner: 'bg-yellow-600',
      analyzer: 'bg-orange-600',
      notification: 'bg-red-600',
      transcoder: 'bg-indigo-600',
      default: 'bg-slate-600',
    };

    const color = typeColors[type] || typeColors.default;
    const formattedType = type.replace('_', ' ').replace(/\b\w/g, (c) => c.toUpperCase());

    return <span className={`${color} text-white text-xs px-2 py-1 rounded`}>{formattedType}</span>;
  };

  // Calculate plugin stats
  const pluginStats = {
    total: plugins.length,
    enabled: plugins.filter((p) => p.enabled).length,
    disabled: plugins.filter((p) => !p.enabled).length,
    types: {} as Record<string, number>,
  };

  // Count plugins by type
  plugins.forEach((plugin) => {
    const type = plugin.type || 'unknown';
    if (!pluginStats.types[type]) {
      pluginStats.types[type] = 0;
    }
    pluginStats.types[type]++;
  });

  return (
    <div className="bg-slate-900 rounded-lg p-6 shadow-xl">
      <div className="flex justify-between items-center mb-6">
        <h2 className="text-xl font-semibold text-white flex items-center">
          <span className="mr-2">🔌</span> Plugin Manager
        </h2>

        <div className="flex gap-2">
          <button
            onClick={refreshPlugins}
            disabled={refreshing}
            className="bg-blue-600 hover:bg-blue-700 text-white px-4 py-2 rounded-lg transition-colors flex items-center gap-1"
          >
            {refreshing ? '🔄 Refreshing...' : '🔄 Refresh Plugins'}
          </button>
        </div>
      </div>

      {/* Plugin Stats */}
      {!loading && plugins.length > 0 && (
        <div className="grid grid-cols-1 md:grid-cols-4 gap-4 mb-6">
          <div className="bg-slate-800 rounded-lg p-4 border border-slate-700">
            <div className="text-3xl font-bold text-white mb-1">{pluginStats.total}</div>
            <div className="text-slate-400 text-sm">Total Plugins</div>
          </div>
          <div className="bg-slate-800 rounded-lg p-4 border border-slate-700">
            <div className="text-3xl font-bold text-green-400 mb-1">{pluginStats.enabled}</div>
            <div className="text-slate-400 text-sm">Enabled Plugins</div>
          </div>
          <div className="bg-slate-800 rounded-lg p-4 border border-slate-700">
            <div className="text-3xl font-bold text-slate-400 mb-1">{pluginStats.disabled}</div>
            <div className="text-slate-400 text-sm">Disabled Plugins</div>
          </div>
          <div className="bg-slate-800 rounded-lg p-4 border border-slate-700">
            <div className="text-3xl font-bold text-blue-400 mb-1">
              {Object.keys(pluginStats.types).length}
            </div>
            <div className="text-slate-400 text-sm">Plugin Types</div>
          </div>
        </div>
      )}

      {error && (
        <div className="bg-red-900/50 border border-red-700 text-red-100 px-4 py-3 rounded mb-4">
          {error}
        </div>
      )}

      {/* Plugin Installer */}
      <div className="mb-6">
        <PluginInstaller />
      </div>

      {loading ? (
        <div className="text-center py-8 text-slate-400">Loading plugins...</div>
      ) : plugins.length === 0 ? (
        <div className="text-center py-8 text-slate-400">
          No plugins found. Place plugin directories in the plugins folder and click "Refresh
          Plugins".
        </div>
      ) : (
        <div className="space-y-4">
          {/* Plugin list */}
          <div className="grid grid-cols-1 gap-4">
            {plugins.map((plugin) => (
              <div
                key={plugin.id}
                className="bg-slate-800 rounded-lg p-4 hover:bg-slate-750 transition-colors"
              >
                <div className="flex justify-between items-start">
                  <div>
                    <div className="flex items-center gap-2 mb-1">
                      <h3 className="text-white font-medium">{plugin.name}</h3>
                      <span className="text-slate-400 text-xs">v{plugin.version}</span>
                      {renderTypeBadge(plugin.type || 'unknown')}
                      {plugin.enabled ? (
                        <span className="bg-green-600/20 text-green-400 text-xs px-2 py-1 rounded">
                          Enabled
                        </span>
                      ) : (
                        <span className="bg-slate-600/20 text-slate-400 text-xs px-2 py-1 rounded">
                          Disabled
                        </span>
                      )}
                    </div>
                    <p className="text-slate-400 text-sm mb-2">{plugin.description}</p>
                    <div className="text-slate-500 text-xs">Author: {plugin.author}</div>
                  </div>

                  <div className="flex items-center gap-2">
                    <button
                      onClick={() => viewPluginDetails(plugin.id)}
                      className="bg-slate-700 hover:bg-slate-600 text-white px-3 py-1 rounded text-sm transition-colors"
                    >
                      Details
                    </button>

                    {plugin.enabled ? (
                      <button
                        onClick={() => disablePlugin(plugin.id)}
                        className="bg-slate-700 hover:bg-slate-600 text-white px-3 py-1 rounded text-sm transition-colors"
                      >
                        Disable
                      </button>
                    ) : (
                      <button
                        onClick={() => enablePlugin(plugin.id)}
                        className="bg-blue-600 hover:bg-blue-700 text-white px-3 py-1 rounded text-sm transition-colors"
                      >
                        Enable
                      </button>
                    )}

                    <button
                      onClick={() => uninstallPlugin(plugin.id)}
                      className="bg-red-600 hover:bg-red-700 text-white px-3 py-1 rounded text-sm transition-colors"
                    >
                      Uninstall
                    </button>
                  </div>
                </div>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Plugin details dialog */}
      {showDetails && selectedPlugin && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4">
          <div className="bg-slate-900 rounded-lg max-w-4xl w-full max-h-[90vh] overflow-y-auto">
            <div className="sticky top-0 bg-slate-800 px-6 py-4 border-b border-slate-700 flex justify-between items-center">
              <h3 className="text-lg font-medium text-white">
                {selectedPlugin.name}{' '}
                <span className="text-slate-400 text-sm">v{selectedPlugin.version}</span>
              </h3>
              <button
                onClick={() => setShowDetails(false)}
                className="text-slate-400 hover:text-white"
              >
                ✕
              </button>
            </div>

            <div className="p-6">
              <div className="flex flex-wrap gap-2 mb-4">
                {renderTypeBadge(selectedPlugin.type || 'unknown')}
                {selectedPlugin.enabled ? (
                  <span className="bg-green-600/20 text-green-400 text-xs px-2 py-1 rounded">
                    Enabled
                  </span>
                ) : (
                  <span className="bg-slate-600/20 text-slate-400 text-xs px-2 py-1 rounded">
                    Disabled
                  </span>
                )}
              </div>

              <div className="mb-6">
                <p className="text-slate-300 mb-4">{selectedPlugin.description}</p>
                <div className="text-sm text-slate-400 mb-1">Author: {selectedPlugin.author}</div>
                {selectedPlugin.manifest?.website && (
                  <div className="text-sm text-slate-400 mb-1">
                    Website:
                    <a
                      href={selectedPlugin.manifest.website}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="text-blue-400 hover:underline ml-1"
                    >
                      {selectedPlugin.manifest.website}
                    </a>
                  </div>
                )}
                {selectedPlugin.manifest?.repository && (
                  <div className="text-sm text-slate-400 mb-1">
                    Repository:
                    <a
                      href={selectedPlugin.manifest.repository}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="text-blue-400 hover:underline ml-1"
                    >
                      {selectedPlugin.manifest.repository}
                    </a>
                  </div>
                )}
                {selectedPlugin.manifest?.license && (
                  <div className="text-sm text-slate-400">
                    License: {selectedPlugin.manifest.license}
                  </div>
                )}
              </div>

              {/* Plugin capabilities */}
              {selectedPlugin.manifest?.capabilities && (
                <div className="mb-6">
                  <h4 className="text-white font-medium mb-2">Capabilities</h4>
                  <div className="grid grid-cols-2 sm:grid-cols-3 gap-2">
                    {Object.entries(selectedPlugin.manifest.capabilities).map(([key, value]) => (
                      <div key={key} className="flex items-center">
                        <span className={value ? 'text-green-400' : 'text-slate-600'}>
                          {value ? '✓' : '✗'}
                        </span>
                        <span className="ml-2 text-sm text-slate-300">
                          {key.replace(/_/g, ' ').replace(/\b\w/g, (c) => c.toUpperCase())}
                        </span>
                      </div>
                    ))}
                  </div>
                </div>
              )}

              {/* Plugin dependencies */}
              {selectedPlugin.manifest?.dependencies && (
                <div className="mb-6">
                  <h4 className="text-white font-medium mb-2">Dependencies</h4>
                  <PluginDependencies
                    viewraDependency={selectedPlugin.manifest.dependencies.viewra_version}
                    pluginDependencies={selectedPlugin.manifest.dependencies.plugins}
                  />
                </div>
              )}

              {/* Plugin configuration */}
              {selectedPlugin.manifest?.config_schema && (
                <div className="mb-6">
                  <h4 className="text-white font-medium mb-2">Configuration</h4>
                  <div className="bg-slate-800 rounded-lg p-4">
                    <PluginConfigEditor
                      pluginId={selectedPlugin.id}
                      schema={selectedPlugin.manifest.config_schema}
                      onSave={async (values) => {
                        try {
                          const response = await fetch(
                            `/api/admin/plugins/${selectedPlugin.id}/config`,
                            {
                              method: 'PUT',
                              headers: {
                                'Content-Type': 'application/json',
                              },
                              body: JSON.stringify({ config: values }),
                            }
                          );

                          if (!response.ok) {
                            throw new Error('Failed to update config');
                          }

                          return Promise.resolve();
                        } catch (err) {
                          console.error('Error updating config:', err);
                          return Promise.reject(err);
                        }
                      }}
                    />
                  </div>
                </div>
              )}

              {/* Plugin admin pages */}
              {selectedPlugin.manifest?.ui?.admin_pages &&
                selectedPlugin.manifest.ui.admin_pages.length > 0 && (
                  <div className="mb-6">
                    <h4 className="text-white font-medium mb-2">Admin Pages</h4>
                    <div className="bg-slate-800 rounded-lg overflow-hidden">
                      {selectedPlugin.manifest.ui.admin_pages.map((page) => (
                        <div key={page.id} className="border-b border-slate-700 last:border-b-0">
                          <div className="p-3 flex justify-between items-center">
                            <div>
                              <div className="text-white font-medium">{page.title}</div>
                              <div className="text-slate-400 text-sm">
                                {page.category || 'General'}
                              </div>
                            </div>
                            <a
                              href={page.url}
                              target="_blank"
                              rel="noopener noreferrer"
                              className="bg-blue-600 hover:bg-blue-700 text-white px-3 py-1 rounded text-sm transition-colors"
                            >
                              Open
                            </a>
                          </div>
                        </div>
                      ))}
                    </div>
                  </div>
                )}

              {/* Plugin UI components */}
              {selectedPlugin.manifest?.ui?.components &&
                selectedPlugin.manifest.ui.components.length > 0 && (
                  <div className="mb-6">
                    <h4 className="text-white font-medium mb-2">UI Components</h4>
                    <div className="bg-slate-800 rounded-lg overflow-hidden">
                      {selectedPlugin.manifest.ui.components.map((component) => (
                        <div
                          key={component.id}
                          className="border-b border-slate-700 last:border-b-0 p-3"
                        >
                          <div className="text-white font-medium">{component.name}</div>
                          <div className="text-slate-400 text-sm">Type: {component.type}</div>
                        </div>
                      ))}
                    </div>
                  </div>
                )}

              {/* Plugin permissions */}
              {selectedPlugin.manifest?.permissions &&
                selectedPlugin.manifest.permissions.length > 0 && (
                  <div className="mb-6">
                    <h4 className="text-white font-medium mb-2">Permissions</h4>
                    <div className="flex flex-wrap gap-2">
                      {selectedPlugin.manifest.permissions.map((permission) => (
                        <span
                          key={permission}
                          className="bg-yellow-600/20 text-yellow-400 text-xs px-2 py-1 rounded"
                        >
                          {permission}
                        </span>
                      ))}
                    </div>
                  </div>
                )}

              <div className="flex justify-between items-center mt-6 pt-6 border-t border-slate-700">
                {/* Left side actions */}
                <div>
                  <button
                    onClick={() => uninstallPlugin(selectedPlugin.id)}
                    className="bg-red-600 hover:bg-red-700 text-white px-4 py-2 rounded text-sm transition-colors"
                  >
                    Uninstall Plugin
                  </button>
                </div>

                {/* Right side actions */}
                <div>
                  {selectedPlugin.enabled ? (
                    <button
                      onClick={() => disablePlugin(selectedPlugin.id)}
                      className="bg-slate-700 hover:bg-slate-600 text-white px-4 py-2 rounded text-sm transition-colors"
                    >
                      Disable Plugin
                    </button>
                  ) : (
                    <button
                      onClick={() => enablePlugin(selectedPlugin.id)}
                      className="bg-blue-600 hover:bg-blue-700 text-white px-4 py-2 rounded text-sm transition-colors"
                    >
                      Enable Plugin
                    </button>
                  )}
                </div>
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  );
};

export default PluginManager;
