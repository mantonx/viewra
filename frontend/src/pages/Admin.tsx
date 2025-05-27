import React from 'react';
import { AdminDashboard } from '../components';

const Admin: React.FC = () => {
  return (
    <div>
      <div className="mb-8">
        <h1 className="text-3xl font-bold text-white mb-4">Admin Dashboard</h1>
        <p className="text-slate-300">
          Manage your Viewra system settings, plugins, and configuration.
        </p>
      </div>

      <AdminDashboard />
    </div>
  );
};

export default Admin;
