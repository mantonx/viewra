import { useEffect, useState } from 'react';
import { useAtom } from 'jotai';
import { apiStatusAtom } from '../store/atoms';
import ApiTester from '../components/ApiTester';
import SystemInfo from '../components/SystemInfo';
import MediaLibraryManager from '../components/media/MediaLibraryManager';
import MusicLibrary from '../components/MusicLibrary';
import ScanPerformanceManager from '../components/ScanPerformanceManager';
import PluginAdminPageCards from '../components/PluginAdminPageCards';
import type { User } from '../types/system.types';

const Home = () => {
  const [apiStatus, setApiStatus] = useAtom(apiStatusAtom);
  const [dbStatus, setDbStatus] = useState<string>('');
  const [users, setUsers] = useState<User[]>([]);

  useEffect(() => {
    const testConnections = async () => {
      try {
        // Test API connection
        const apiResponse = await fetch('/api/health');
        if (apiResponse.ok) {
          setApiStatus('connected');
        } else {
          setApiStatus('error');
        }

        // Test database connection
        const dbResponse = await fetch('/api/db-status');
        const dbData = await dbResponse.json();
        setDbStatus(dbData.status === 'connected' ? 'Connected' : 'Disconnected');

        // Load users
        const usersResponse = await fetch('/api/users/');
        const usersData = await usersResponse.json();
        setUsers(usersData.users || []);
      } catch (error) {
        setApiStatus('error');
        console.error('Connection test failed:', error);
      }
    };

    testConnections();
  }, [setApiStatus]);

  return (
    <div className="space-y-6">
      <div className="text-center">
        <h1 className="text-3xl font-bold text-white mb-2">Welcome to Viewra</h1>
        <p className="text-slate-400">Your personal media server</p>
      </div>

      {/* System Status */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        <div className="bg-slate-900 rounded-lg p-4">
          <h3 className="text-white font-medium mb-2">API Status</h3>
          <div
            className={`text-sm ${apiStatus === 'connected' ? 'text-green-400' : 'text-red-400'}`}
          >
            {apiStatus === 'connected' ? '✅ Connected' : '❌ Disconnected'}
          </div>
        </div>
        <div className="bg-slate-900 rounded-lg p-4">
          <h3 className="text-white font-medium mb-2">Database</h3>
          <div
            className={`text-sm ${dbStatus === 'Connected' ? 'text-green-400' : 'text-red-400'}`}
          >
            {dbStatus || 'Unknown'}
          </div>
        </div>
        <div className="bg-slate-900 rounded-lg p-4">
          <h3 className="text-white font-medium mb-2">Users</h3>
          <div className="text-sm text-white">{users.length} registered</div>
        </div>
      </div>

      <SystemInfo />
      <ApiTester />
      {/* MediaUpload component removed as app won't support uploads */}
      <MediaLibraryManager />
      <MusicLibrary />
      <ScanPerformanceManager />
      <PluginAdminPageCards />
    </div>
  );
};

export default Home;
