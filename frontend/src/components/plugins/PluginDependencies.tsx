import React from 'react';
import type { PluginDependenciesProps } from '@/types/plugin.types';

const PluginDependencies: React.FC<PluginDependenciesProps> = ({
  viewraDependency,
  pluginDependencies,
  satisfied = true,
}) => {
  const hasViewraDependency = viewraDependency && viewraDependency !== '';
  const hasPluginDependencies = pluginDependencies && Object.keys(pluginDependencies).length > 0;

  if (!hasViewraDependency && !hasPluginDependencies) {
    return <div className="text-slate-400 text-sm italic">No dependencies required</div>;
  }

  return (
    <div className="space-y-3">
      {hasViewraDependency && (
        <div className="flex items-start">
          <div
            className={`rounded-full w-4 h-4 mt-0.5 mr-2 flex-shrink-0 ${
              satisfied ? 'bg-green-500' : 'bg-red-500'
            }`}
          ></div>
          <div>
            <div className="text-white text-sm font-medium">Viewra Core</div>
            <div className="text-slate-400 text-xs">
              Version {viewraDependency}
              {!satisfied && (
                <span className="text-red-400 ml-2">(incompatible with current version)</span>
              )}
            </div>
          </div>
        </div>
      )}

      {hasPluginDependencies && (
        <div className="space-y-2">
          {Object.entries(pluginDependencies).map(([name, version]) => (
            <div key={name} className="flex items-start">
              <div className="rounded-full w-4 h-4 mt-0.5 mr-2 flex-shrink-0 bg-blue-500"></div>
              <div>
                <div className="text-white text-sm font-medium">{name}</div>
                <div className="text-slate-400 text-xs">Version {version}</div>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
};

export default PluginDependencies;
