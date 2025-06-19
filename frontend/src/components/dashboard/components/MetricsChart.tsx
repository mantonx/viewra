import React from 'react';

interface MetricsChartProps {
  title?: string;
  height?: number;
}

export const MetricsChart: React.FC<MetricsChartProps> = ({
  title = 'Metrics Chart',
  height = 200,
}) => {
  return (
    <div className="bg-slate-800 border border-slate-700 rounded-lg p-4">
      <h3 className="text-sm font-medium text-white mb-3">{title}</h3>
      <div 
        className="flex items-center justify-center bg-slate-900 rounded border border-slate-600"
        style={{ height }}
      >
        <div className="text-center">
          <div className="text-slate-400 text-sm">📊</div>
          <div className="text-slate-500 text-xs mt-1">Metrics visualization coming soon</div>
        </div>
      </div>
    </div>
  );
}; 