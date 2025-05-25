import React, { useState, useEffect } from 'react';

interface Plugin {
  id: string;
  name: string;
}

interface AdminPage {
  id: number;
  plugin_id: number;
  plugin: Plugin;
  title: string;
  path: string;
  icon: string;
  category: string;
  url: string;
  type: string;
  enabled: boolean;
  sort_order: number;
}

interface AdminPagesResponse {
  admin_pages: AdminPage[];
  count: number;
}

const PluginAdminPageCards: React.FC = () => {
  const [adminPages, setAdminPages] = useState<AdminPage[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // Fetch admin pages from the API
  useEffect(() => {
    const loadAdminPages = async () => {
      try {
        setLoading(true);
        setError(null);

        const response = await fetch('/api/admin/plugins/admin-pages');
        const data = (await response.json()) as AdminPagesResponse;

        setAdminPages(data.admin_pages || []);
      } catch (err) {
        setError('Failed to load admin pages');
        console.error('Error loading admin pages:', err);
      } finally {
        setLoading(false);
      }
    };

    loadAdminPages();
  }, []);

  // Function to get icon class based on icon name
  const getIconClass = (icon: string): string => {
    const iconMap: Record<string, string> = {
      'chart-bar': '📊',
      cog: '⚙️',
      user: '👤',
      plugin: '🔌',
      database: '💾',
      folder: '📁',
      file: '📄',
      bell: '🔔',
      search: '🔍',
      gear: '⚙️',
      settings: '⚙️',
      dashboard: '📊',
      home: '🏠',
    };

    return iconMap[icon] || '📋';
  };

  if (loading) {
    return (
      <div className="text-center py-6 text-slate-400 text-sm">
        <div className="animate-pulse flex justify-center items-center">
          <div className="h-5 w-5 bg-blue-500 rounded-full mr-2"></div>
          <div className="h-5 bg-slate-700 rounded w-32"></div>
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="text-center py-4 text-red-400 text-sm bg-red-900/20 rounded-lg p-3 border border-red-800/30">
        Error loading plugin admin pages. Please check your network connection and try again.
      </div>
    );
  }

  if (adminPages.length === 0) {
    return (
      <div className="text-center py-6 text-slate-400 text-sm flex flex-col items-center justify-center">
        <svg
          xmlns="http://www.w3.org/2000/svg"
          className="h-10 w-10 text-slate-500 mb-2"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            strokeWidth={1.5}
            d="M19.5 14.25v-2.625a3.375 3.375 0 00-3.375-3.375h-1.5A1.125 1.125 0 0113.5 7.125v-1.5a3.375 3.375 0 00-3.375-3.375H8.25m0 12.75h7.5m-7.5 3H12M10.5 2.25H5.625c-.621 0-1.125.504-1.125 1.125v17.25c0 .621.504 1.125 1.125 1.125h12.75c.621 0 1.125-.504 1.125-1.125V11.25a9 9 0 00-9-9z"
          />
        </svg>
        <div>No plugin admin pages available</div>
        <div className="text-xs text-slate-500 mt-1">
          Enable plugins with admin pages to see them here
        </div>
      </div>
    );
  }

  return (
    <div className="overflow-x-auto pb-4">
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4">
        {adminPages.map((page) => (
          <a
            key={page.id}
            href={page.url}
            target="_blank"
            rel="noopener noreferrer"
            className="bg-slate-800 rounded-lg p-4 hover:bg-slate-750 transition-colors border border-slate-700 hover:border-blue-600 flex flex-col h-full group"
          >
            <div className="text-3xl mb-3 group-hover:scale-110 transition-transform transform-gpu">
              {getIconClass(page.icon)}
            </div>
            <h3 className="text-white font-medium mb-1 group-hover:text-blue-400 transition-colors">
              {page.title}
            </h3>
            <div className="text-slate-400 text-xs mb-3">{page.category}</div>
            <div className="text-slate-500 text-xs mt-auto flex items-center">
              <span className="w-2 h-2 bg-green-500 rounded-full mr-2"></span>
              {page.plugin.name}
            </div>
          </a>
        ))}
      </div>
    </div>
  );
};

export default PluginAdminPageCards;
