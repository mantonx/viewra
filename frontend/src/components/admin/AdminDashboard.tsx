import React, { useState } from 'react';
import {
  PluginManager,
  PluginAdminPages,
  PluginUIComponents,
  PluginEvents,
  PluginPermissions,
} from '../plugins';
import { SystemEvents } from '../system';

type AdminTab =
  | 'plugins'
  | 'admin-pages'
  | 'ui-components'
  | 'events'
  | 'system-events'
  | 'permissions';

const AdminDashboard: React.FC = () => {
  const [activeTab, setActiveTab] = useState<AdminTab>('plugins');

  return (
    <div className="space-y-6">
      <div className="bg-slate-800 rounded-lg overflow-hidden flex flex-wrap">
        <button
          onClick={() => setActiveTab('plugins')}
          className={`px-5 py-3 text-sm font-medium ${
            activeTab === 'plugins' ? 'bg-blue-600 text-white' : 'bg-slate-800 text-slate-300'
          }`}
        >
          🔌 Plugins
        </button>
        <button
          onClick={() => setActiveTab('admin-pages')}
          className={`px-5 py-3 text-sm font-medium ${
            activeTab === 'admin-pages' ? 'bg-blue-600 text-white' : 'bg-slate-800 text-slate-300'
          }`}
        >
          📄 Admin Pages
        </button>
        <button
          onClick={() => setActiveTab('ui-components')}
          className={`px-5 py-3 text-sm font-medium ${
            activeTab === 'ui-components' ? 'bg-blue-600 text-white' : 'bg-slate-800 text-slate-300'
          }`}
        >
          🧩 UI Components
        </button>
        <button
          onClick={() => setActiveTab('permissions')}
          className={`px-5 py-3 text-sm font-medium ${
            activeTab === 'permissions' ? 'bg-blue-600 text-white' : 'bg-slate-800 text-slate-300'
          }`}
        >
          🔒 Permissions
        </button>
        <button
          onClick={() => setActiveTab('events')}
          className={`px-5 py-3 text-sm font-medium ${
            activeTab === 'events' ? 'bg-blue-600 text-white' : 'bg-slate-800 text-slate-300'
          }`}
        >
          📝 Plugin Events
        </button>
        <button
          onClick={() => setActiveTab('system-events')}
          className={`px-5 py-3 text-sm font-medium ${
            activeTab === 'system-events' ? 'bg-blue-600 text-white' : 'bg-slate-800 text-slate-300'
          }`}
        >
          🌐 System Events
        </button>
      </div>

      {activeTab === 'plugins' && <PluginManager />}
      {activeTab === 'admin-pages' && <PluginAdminPages />}
      {activeTab === 'ui-components' && <PluginUIComponents />}
      {activeTab === 'permissions' && <PluginPermissions />}
      {activeTab === 'events' && <PluginEvents />}
      {activeTab === 'system-events' && <SystemEvents />}
    </div>
  );
};

export default AdminDashboard;
