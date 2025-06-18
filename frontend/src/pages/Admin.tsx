import React, { useState } from 'react';
import { AdminDashboard } from '@/components/admin';

const Admin: React.FC = () => {
  const [activeTab, setActiveTab] = useState<'overview' | 'plugins' | 'pages'>('overview');

  return (
    <div className="min-h-screen bg-slate-900 p-6">
      <div className="max-w-7xl mx-auto">
        <AdminDashboard
          activeTab={activeTab}
          onTabChange={(tab) => setActiveTab(tab as 'overview' | 'plugins' | 'pages')}
        />
      </div>
    </div>
  );
};

export default Admin;
