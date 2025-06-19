import React from 'react';
import { ArrowLeft, Settings, ExternalLink, BarChart3, Activity } from 'lucide-react';
import type { AdminPage, Plugin } from '@/types/plugin.types';
import ConfigEditor from './ConfigEditor';

interface PluginAdminPageRendererProps {
  page: AdminPage;
  plugins: Plugin[];
  onBack: () => void;
}

const PluginAdminPageRenderer: React.FC<PluginAdminPageRendererProps> = ({
  page,
  plugins,
  onBack,
}) => {
  // Find the plugin that owns this admin page
  const ownerPlugin = plugins.find(plugin => 
    plugin.admin_pages?.some(ap => ap.id === page.id)
  );

  const renderPageContent = () => {
    switch (page.type) {
      case 'configuration':
        if (!ownerPlugin) {
          return (
            <div className="text-center py-8">
              <Settings size={48} className="mx-auto text-slate-500 mb-4" />
              <p className="text-slate-400">Plugin not found</p>
            </div>
          );
        }
        return <ConfigEditor plugin={ownerPlugin} />;

      case 'dashboard':
        return (
          <div className="space-y-6">
            <div className="bg-slate-700 rounded-lg p-6">
              <div className="flex items-center mb-4">
                <BarChart3 size={24} className="text-purple-400 mr-3" />
                <h3 className="text-lg font-medium text-white">Plugin Dashboard</h3>
              </div>
              <p className="text-slate-400 mb-4">
                Custom dashboard content for {ownerPlugin?.name || 'this plugin'} would be loaded here.
              </p>
              <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
                <div className="bg-slate-600 rounded-lg p-4">
                  <h4 className="text-white font-medium mb-2">Status</h4>
                  <div className="text-2xl font-bold text-green-400">Active</div>
                </div>
                <div className="bg-slate-600 rounded-lg p-4">
                  <h4 className="text-white font-medium mb-2">Operations</h4>
                  <div className="text-2xl font-bold text-blue-400">
                    {ownerPlugin?.health?.total_requests || 0}
                  </div>
                </div>
                <div className="bg-slate-600 rounded-lg p-4">
                  <h4 className="text-white font-medium mb-2">Success Rate</h4>
                  <div className="text-2xl font-bold text-green-400">
                    {ownerPlugin?.health?.total_requests 
                      ? Math.round((ownerPlugin.health.successful_requests / ownerPlugin.health.total_requests) * 100)
                      : 100}%
                  </div>
                </div>
              </div>
            </div>
          </div>
        );

      case 'status':
        return (
          <div className="space-y-6">
            <div className="bg-slate-700 rounded-lg p-6">
              <div className="flex items-center mb-4">
                <Activity size={24} className="text-green-400 mr-3" />
                <h3 className="text-lg font-medium text-white">Plugin Health Status</h3>
              </div>
              
              {ownerPlugin?.health ? (
                <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
                  <div className="space-y-4">
                    <div className="flex justify-between items-center">
                      <span className="text-slate-300">Status:</span>
                      <span className={`font-medium ${ownerPlugin.health.healthy ? 'text-green-400' : 'text-red-400'}`}>
                        {ownerPlugin.health.healthy ? 'Healthy' : 'Unhealthy'}
                      </span>
                    </div>
                    <div className="flex justify-between items-center">
                      <span className="text-slate-300">Running:</span>
                      <span className={`font-medium ${ownerPlugin.health.running ? 'text-green-400' : 'text-red-400'}`}>
                        {ownerPlugin.health.running ? 'Yes' : 'No'}
                      </span>
                    </div>
                    <div className="flex justify-between items-center">
                      <span className="text-slate-300">Uptime:</span>
                      <span className="text-white">{ownerPlugin.health.uptime}</span>
                    </div>
                    <div className="flex justify-between items-center">
                      <span className="text-slate-300">Error Rate:</span>
                      <span className={`font-medium ${ownerPlugin.health.error_rate > 5 ? 'text-red-400' : 'text-green-400'}`}>
                        {ownerPlugin.health.error_rate}%
                      </span>
                    </div>
                  </div>
                  
                  <div className="space-y-4">
                    <div className="flex justify-between items-center">
                      <span className="text-slate-300">Total Requests:</span>
                      <span className="text-white">{ownerPlugin.health.total_requests}</span>
                    </div>
                    <div className="flex justify-between items-center">
                      <span className="text-slate-300">Successful:</span>
                      <span className="text-green-400">{ownerPlugin.health.successful_requests}</span>
                    </div>
                    <div className="flex justify-between items-center">
                      <span className="text-slate-300">Failed:</span>
                      <span className="text-red-400">{ownerPlugin.health.failed_requests}</span>
                    </div>
                    <div className="flex justify-between items-center">
                      <span className="text-slate-300">Avg Response Time:</span>
                      <span className="text-white">{ownerPlugin.health.average_response_time}</span>
                    </div>
                  </div>
                </div>
              ) : (
                <p className="text-slate-400">No health information available</p>
              )}
              
              {ownerPlugin?.health?.last_error && (
                <div className="mt-6 bg-red-600/20 border border-red-600 rounded-lg p-4">
                  <h4 className="text-red-400 font-medium mb-2">Last Error</h4>
                  <p className="text-red-300 text-sm">{ownerPlugin.health.last_error}</p>
                  <p className="text-red-400 text-xs mt-2">
                    {new Date(ownerPlugin.health.last_check_time).toLocaleString()}
                  </p>
                </div>
              )}
            </div>
          </div>
        );

      case 'external':
        return (
          <div className="space-y-6">
            <div className="bg-slate-700 rounded-lg p-6">
              <div className="flex items-center mb-4">
                <ExternalLink size={24} className="text-blue-400 mr-3" />
                <h3 className="text-lg font-medium text-white">External Plugin Interface</h3>
              </div>
              <p className="text-slate-400 mb-4">
                This page would typically load content from the plugin's own web interface.
              </p>
              <div className="bg-slate-600 rounded-lg p-4 border-2 border-dashed border-slate-500">
                <div className="text-center">
                  <ExternalLink size={48} className="mx-auto text-slate-400 mb-4" />
                  <p className="text-slate-400 mb-2">External content would be loaded here</p>
                  <p className="text-sm text-slate-500">URL: {page.url}</p>
                  <a
                    href={page.url}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="inline-flex items-center mt-4 px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white rounded-md text-sm"
                  >
                    <ExternalLink size={16} className="mr-2" />
                    Open in New Tab
                  </a>
                </div>
              </div>
            </div>
          </div>
        );

      default:
        return (
          <div className="text-center py-8">
            <div className="text-slate-400 mb-4">Unknown page type: {page.type}</div>
            <p className="text-sm text-slate-500">This admin page type is not supported</p>
          </div>
        );
    }
  };

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center space-x-4">
        <button
          onClick={onBack}
          className="flex items-center px-3 py-2 bg-slate-700 hover:bg-slate-600 text-slate-300 rounded-md"
        >
          <ArrowLeft size={16} className="mr-2" />
          Back to Dashboard
        </button>
        
        <div>
          <h1 className="text-2xl font-bold text-white">{page.title}</h1>
          <div className="flex items-center space-x-2 mt-1">
            {page.icon && <span className="text-lg">{page.icon}</span>}
            <span className="text-slate-400">
              {ownerPlugin ? `from ${ownerPlugin.name}` : page.path}
            </span>
            {page.category && (
              <span className="px-2 py-1 bg-purple-600 rounded text-xs text-white">
                {page.category}
              </span>
            )}
            <span className="px-2 py-1 bg-slate-600 rounded text-xs text-slate-300">
              {page.type}
            </span>
          </div>
        </div>
      </div>

      {/* Page Content */}
      {renderPageContent()}
    </div>
  );
};

export default PluginAdminPageRenderer; 