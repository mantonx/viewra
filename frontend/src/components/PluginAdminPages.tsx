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

const PluginAdminPages: React.FC = () => {
  const [adminPages, setAdminPages] = useState<AdminPage[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [activeFrame, setActiveFrame] = useState<string | null>(null);

  // Fetch admin pages from the API
  const loadAdminPages = async () => {
    try {
      setLoading(true);
      setError(null);

      const response = await fetch('/api/admin/plugins/admin-pages');
      const data = (await response.json()) as AdminPagesResponse;

      // Group by category
      setAdminPages(data.admin_pages || []);

      // If we have pages, set the first one as active by default
      if (data.admin_pages && data.admin_pages.length > 0) {
        setActiveFrame(data.admin_pages[0].url);
      }
    } catch (err) {
      setError('Failed to load admin pages');
      console.error('Error loading admin pages:', err);
    } finally {
      setLoading(false);
    }
  };

  // Load admin pages when component mounts
  useEffect(() => {
    loadAdminPages();
  }, []);

  // Group admin pages by category
  const groupedPages: Record<string, AdminPage[]> = {};
  adminPages.forEach((page) => {
    const category = page.category || 'General';
    if (!groupedPages[category]) {
      groupedPages[category] = [];
    }
    groupedPages[category].push(page);
  });

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

  return (
    <div className="bg-slate-900 rounded-lg shadow-xl overflow-hidden">
      <div className="text-xl font-semibold text-white p-6 border-b border-slate-700 flex items-center">
        <span className="mr-2">📄</span> Plugin Admin Pages
      </div>

      {error && (
        <div className="bg-red-900/50 border border-red-700 text-red-100 px-4 py-3 m-4 rounded">
          {error}
        </div>
      )}

      {loading ? (
        <div className="text-center py-8 text-slate-400">Loading admin pages...</div>
      ) : adminPages.length === 0 ? (
        <div className="text-center py-8 text-slate-400">
          No plugin admin pages available. Enable plugins with admin page capabilities to see them
          here.
        </div>
      ) : (
        <div className="flex">
          {/* Sidebar navigation */}
          <div className="w-64 bg-slate-800 border-r border-slate-700 h-[calc(100vh-12rem)] overflow-y-auto">
            <nav>
              {Object.entries(groupedPages).map(([category, pages]) => (
                <div key={category}>
                  <div className="px-4 py-2 text-xs uppercase tracking-wider text-slate-400 font-semibold bg-slate-750">
                    {category}
                  </div>
                  <ul>
                    {pages.map((page) => (
                      <li key={page.id}>
                        <button
                          onClick={() => setActiveFrame(page.url)}
                          className={`w-full px-4 py-3 flex items-center text-left ${
                            activeFrame === page.url
                              ? 'bg-blue-700 text-white'
                              : 'text-slate-300 hover:bg-slate-700'
                          }`}
                        >
                          <span className="mr-3">{getIconClass(page.icon)}</span>
                          <span>{page.title}</span>
                        </button>
                      </li>
                    ))}
                  </ul>
                </div>
              ))}
            </nav>
          </div>

          {/* Content area with iframe */}
          <div className="flex-1">
            {activeFrame ? (
              <iframe
                src={activeFrame}
                className="w-full h-[calc(100vh-12rem)]"
                frameBorder="0"
                title="Plugin Admin Page"
              />
            ) : (
              <div className="flex items-center justify-center h-full text-slate-400">
                Select a page from the sidebar
              </div>
            )}
          </div>
        </div>
      )}
    </div>
  );
};

export default PluginAdminPages;
