import React, { useState, useEffect } from 'react';
import type { PluginPermission, PermissionsResponse } from '@/types/plugin.types';

const PluginPermissions: React.FC<{ pluginId?: string }> = ({ pluginId }) => {
  const [permissions, setPermissions] = useState<PluginPermission[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const loadPermissions = async () => {
      try {
        setLoading(true);
        setError(null);

        const url = pluginId
          ? `/api/admin/plugins/${pluginId}/permissions`
          : '/api/admin/plugins/permissions';

        const response = await fetch(url);
        const data = (await response.json()) as PermissionsResponse;

        setPermissions(data.permissions || []);
      } catch (err) {
        setError('Failed to load plugin permissions');
        console.error('Error loading plugin permissions:', err);
      } finally {
        setLoading(false);
      }
    };

    loadPermissions();
  }, [pluginId]);

  if (loading) {
    return (
      <div className="bg-slate-900 rounded-lg p-6 shadow-xl">
        <div className="text-center py-8 text-slate-400">Loading permissions...</div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="bg-slate-900 rounded-lg p-6 shadow-xl">
        <div className="bg-red-900/50 border border-red-700 text-red-100 px-4 py-3 rounded">
          {error}
        </div>
      </div>
    );
  }

  return (
    <div className="bg-slate-900 rounded-lg p-6 shadow-xl">
      <div className="flex justify-between items-center mb-6">
        <h2 className="text-xl font-semibold text-white flex items-center">
          <span className="mr-2">🔒</span> Plugin Permissions
        </h2>
      </div>

      {permissions.length === 0 ? (
        <div className="text-center py-8 text-slate-400">No plugin permissions found.</div>
      ) : (
        <div className="space-y-4">
          {permissions.map((permission) => (
            <div
              key={permission.id}
              className="bg-slate-800 rounded-lg p-4 flex items-center justify-between"
            >
              <div>
                <h3 className="text-white font-medium">{permission.permission_name}</h3>
                <p className="text-slate-400 text-sm">{permission.description}</p>
                <p className="text-slate-500 text-xs">Plugin: {permission.plugin.name}</p>
              </div>
              <div className="flex items-center gap-2">
                <span
                  className={`px-2 py-1 rounded text-xs ${
                    permission.granted
                      ? 'bg-green-600/20 text-green-400'
                      : permission.required
                        ? 'bg-red-600/20 text-red-400'
                        : 'bg-yellow-600/20 text-yellow-400'
                  }`}
                >
                  {permission.granted ? 'Granted' : permission.required ? 'Required' : 'Optional'}
                </span>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
};

export default PluginPermissions;
