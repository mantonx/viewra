import React, { useState, useEffect } from 'react';
import type { UIComponent, UIComponentsResponse } from '../types/plugin.types';

const PluginUIComponents: React.FC = () => {
  const [components, setComponents] = useState<UIComponent[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // Fetch UI components from the API
  const loadUIComponents = async () => {
    try {
      setLoading(true);
      setError(null);

      const response = await fetch('/api/admin/plugins/ui-components');
      const data = (await response.json()) as UIComponentsResponse;

      setComponents(data.ui_components || []);
    } catch (err) {
      setError('Failed to load UI components');
      console.error('Error loading UI components:', err);
    } finally {
      setLoading(false);
    }
  };

  // Load UI components when component mounts
  useEffect(() => {
    loadUIComponents();
  }, []);

  // Group components by type
  const groupedComponents: Record<string, UIComponent[]> = {};
  components.forEach((component) => {
    const type = component.type || 'widget';
    if (!groupedComponents[type]) {
      groupedComponents[type] = [];
    }
    groupedComponents[type].push(component);
  });

  // Function to get a color for component type
  const getTypeColor = (type: string): string => {
    const colorMap: Record<string, string> = {
      widget: 'bg-blue-600',
      modal: 'bg-purple-600',
      page: 'bg-green-600',
      card: 'bg-orange-600',
      section: 'bg-yellow-600',
      footer: 'bg-slate-600',
      header: 'bg-slate-600',
    };

    return colorMap[type] || 'bg-slate-600';
  };

  return (
    <div className="bg-slate-900 rounded-lg p-6 shadow-xl">
      <div className="flex justify-between items-center mb-6">
        <h2 className="text-xl font-semibold text-white flex items-center">
          <span className="mr-2">🧩</span> Plugin UI Components
        </h2>
      </div>

      {error && (
        <div className="bg-red-900/50 border border-red-700 text-red-100 px-4 py-3 rounded mb-4">
          {error}
        </div>
      )}

      {loading ? (
        <div className="text-center py-8 text-slate-400">Loading UI components...</div>
      ) : components.length === 0 ? (
        <div className="text-center py-8 text-slate-400">
          No plugin UI components available. Enable plugins with UI component capabilities to see
          them here.
        </div>
      ) : (
        <div className="space-y-6">
          {Object.entries(groupedComponents).map(([type, typeComponents]) => (
            <div key={type} className="space-y-4">
              <h3 className="text-lg font-medium text-white flex items-center">
                <span className={`${getTypeColor(type)} w-3 h-3 rounded-full mr-2`}></span>
                {type.charAt(0).toUpperCase() + type.slice(1)} Components
              </h3>

              <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
                {typeComponents.map((component) => (
                  <div
                    key={component.id}
                    className="bg-slate-800 rounded-lg p-4 hover:bg-slate-700 transition-colors"
                  >
                    <div className="flex justify-between items-start">
                      <div>
                        <div className="flex items-center gap-2 mb-1">
                          <h4 className="text-white font-medium">{component.name}</h4>
                          <span
                            className={`${getTypeColor(component.type)} text-white text-xs px-2 py-1 rounded`}
                          >
                            {component.type}
                          </span>
                        </div>
                        <div className="text-slate-500 text-xs">
                          From: <span className="text-slate-400">{component.plugin.name}</span>
                        </div>
                      </div>

                      <div className="flex items-center gap-2">
                        {component.url && (
                          <a
                            href={component.url}
                            target="_blank"
                            rel="noopener noreferrer"
                            className="bg-blue-600 hover:bg-blue-700 text-white px-3 py-1 rounded text-sm transition-colors"
                          >
                            View
                          </a>
                        )}
                      </div>
                    </div>
                  </div>
                ))}
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
};

export default PluginUIComponents;
