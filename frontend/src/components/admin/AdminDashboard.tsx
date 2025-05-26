import React from 'react';
import PluginManager from '../PluginManager';
import PluginAdminPages from '../PluginAdminPages';
import PluginUIComponents from '../PluginUIComponents';
import PluginEvents from '../PluginEvents';
import PluginPermissions from '../PluginPermissions';
import SystemEvents from '../SystemEvents';

const AdminDashboard: React.FC = () => {
  return (
    <div className="p-6">
      <h1 className="text-2xl font-bold mb-6">Admin Dashboard</h1>
      <div className="space-y-6">
        <PluginManager />
        <PluginAdminPages />
        <PluginUIComponents />
        <PluginEvents />
        <PluginPermissions />
        <SystemEvents />
      </div>
    </div>
  );
};

export default AdminDashboard;
