import React, { useState, useEffect } from 'react';
import type { SystemInfo as SystemInfoType } from '../types/system.types';

const SystemInfo = () => {
  const [systemInfo, setSystemInfo] = useState<SystemInfoType>({
    frontend: {
      url: window.location.origin,
      port: window.location.port || '5175',
      framework: 'React 19 + Vite + TypeScript',
    },
    backend: {
      url: 'http://localhost:8083',
      port: '8083',
      framework: 'Go + Gin + GORM',
    },
    database: {
      type: 'SQLite',
      status: 'Unknown',
    },
    environment: 'development',
  });

  const [expanded, setExpanded] = useState(false);

  useEffect(() => {
    const checkDatabaseStatus = async () => {
      try {
        const response = await fetch('/api/db-status');
        const data = await response.json();
        setSystemInfo((prev) => ({
          ...prev,
          database: {
            type: 'SQLite',
            status: data.status === 'connected' ? 'Connected' : 'Disconnected',
          },
        }));
      } catch {
        setSystemInfo((prev) => ({
          ...prev,
          database: {
            ...prev.database,
            status: 'Error',
          },
        }));
      }
    };

    checkDatabaseStatus();
  }, []);

  const getStatusColor = (status: string) => {
    switch (status.toLowerCase()) {
      case 'connected':
        return 'text-green-400';
      case 'error':
      case 'disconnected':
        return 'text-red-400';
      default:
        return 'text-yellow-400';
    }
  };

  return (
    <div className="bg-slate-900 rounded-lg p-6 shadow-xl">
      <div
        className="flex justify-between items-center cursor-pointer"
        onClick={() => setExpanded(!expanded)}
      >
        <h2 className="text-xl font-semibold text-white flex items-center gap-2">
          ⚙️ System Information
        </h2>
        <button className="text-slate-400 hover:text-white transition-colors">
          {expanded ? '▼' : '▶'}
        </button>
      </div>

      {expanded && (
        <div className="mt-4 grid grid-cols-1 md:grid-cols-3 gap-6">
          {/* Frontend Info */}
          <div className="bg-slate-800 rounded-lg p-4">
            <h3 className="text-lg font-medium text-blue-400 mb-3">Frontend</h3>
            <div className="space-y-2 text-sm">
              <div>
                <span className="text-slate-400">URL:</span>
                <span className="text-white ml-2">{systemInfo.frontend.url}</span>
              </div>
              <div>
                <span className="text-slate-400">Port:</span>
                <span className="text-white ml-2">{systemInfo.frontend.port}</span>
              </div>
              <div>
                <span className="text-slate-400">Stack:</span>
                <span className="text-white ml-2">{systemInfo.frontend.framework}</span>
              </div>
              <div>
                <span className="text-slate-400">Hot Reload:</span>
                <span className="text-green-400 ml-2">✅ Enabled</span>
              </div>
            </div>
          </div>

          {/* Backend Info */}
          <div className="bg-slate-800 rounded-lg p-4">
            <h3 className="text-lg font-medium text-green-400 mb-3">Backend</h3>
            <div className="space-y-2 text-sm">
              <div>
                <span className="text-slate-400">URL:</span>
                <span className="text-white ml-2">{systemInfo.backend.url}</span>
              </div>
              <div>
                <span className="text-slate-400">Port:</span>
                <span className="text-white ml-2">{systemInfo.backend.port}</span>
              </div>
              <div>
                <span className="text-slate-400">Stack:</span>
                <span className="text-white ml-2">{systemInfo.backend.framework}</span>
              </div>
              <div>
                <span className="text-slate-400">Hot Reload:</span>
                <span className="text-green-400 ml-2">✅ Available</span>
              </div>
            </div>
          </div>

          {/* Database Info */}
          <div className="bg-slate-800 rounded-lg p-4">
            <h3 className="text-lg font-medium text-purple-400 mb-3">Database</h3>
            <div className="space-y-2 text-sm">
              <div>
                <span className="text-slate-400">Type:</span>
                <span className="text-white ml-2">{systemInfo.database.type}</span>
              </div>
              <div>
                <span className="text-slate-400">Status:</span>
                <span className={`ml-2 ${getStatusColor(systemInfo.database.status)}`}>
                  {systemInfo.database.status}
                </span>
              </div>
              <div>
                <span className="text-slate-400">File:</span>
                <span className="text-white ml-2 text-xs">./data/viewra.db</span>
              </div>
              <div>
                <span className="text-slate-400">Tables:</span>
                <span className="text-white ml-2">users, media</span>
              </div>
            </div>
          </div>
        </div>
      )}

      {expanded && (
        <div className="mt-4 bg-slate-800 rounded-lg p-4">
          <h3 className="text-lg font-medium text-yellow-400 mb-3">Development Tools</h3>
          <div className="grid grid-cols-2 md:grid-cols-4 gap-4 text-sm">
            <div>
              <span className="text-slate-400">Environment:</span>
              <span className="text-white ml-2 capitalize">{systemInfo.environment}</span>
            </div>
            <div>
              <span className="text-slate-400">Styling:</span>
              <span className="text-green-400 ml-2">✅ Tailwind</span>
            </div>
          </div>
        </div>
      )}
    </div>
  );
};

export default SystemInfo;
