import React, { useState, useEffect } from 'react';
import type { Plugin, PluginEvent, PluginEventsResponse } from '../types/plugin.types';

const PluginEvents: React.FC = () => {
  const [events, setEvents] = useState<PluginEvent[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [filter, setFilter] = useState<string>('all'); // all, error, info, warning, success

  // Format timestamp to a readable date and time
  const formatTimestamp = (timestamp: string): string => {
    const date = new Date(timestamp);
    return date.toLocaleString();
  };

  // Fetch plugin events from the API
  const loadEvents = async () => {
    try {
      setLoading(true);
      setError(null);

      const response = await fetch('/api/admin/plugins/events');
      const data = (await response.json()) as PluginEventsResponse;

      setEvents(data.events || []);
    } catch (err) {
      setError('Failed to load plugin events');
      console.error('Error loading plugin events:', err);
    } finally {
      setLoading(false);
    }
  };

  // Load events when component mounts
  useEffect(() => {
    loadEvents();
  }, []);

  // Get event-type specific styles
  const getEventTypeStyle = (eventType: string): { bg: string; text: string; icon: string } => {
    const typeMap: Record<string, { bg: string; text: string; icon: string }> = {
      initialize: { bg: 'bg-blue-600/20', text: 'text-blue-400', icon: '🚀' },
      start: { bg: 'bg-green-600/20', text: 'text-green-400', icon: '▶️' },
      stop: { bg: 'bg-orange-600/20', text: 'text-orange-400', icon: '⏹️' },
      error: { bg: 'bg-red-600/20', text: 'text-red-400', icon: '❌' },
      warning: { bg: 'bg-yellow-600/20', text: 'text-yellow-400', icon: '⚠️' },
      info: { bg: 'bg-blue-600/20', text: 'text-blue-400', icon: 'ℹ️' },
      default: { bg: 'bg-slate-600/20', text: 'text-slate-400', icon: '📋' },
    };

    return typeMap[eventType.toLowerCase()] || typeMap.default;
  };

  // Filter events
  const filteredEvents =
    filter === 'all' ? events : events.filter((event) => event.event_type.toLowerCase() === filter);

  return (
    <div className="bg-slate-900 rounded-lg p-6 shadow-xl">
      <div className="flex justify-between items-center mb-6">
        <h2 className="text-xl font-semibold text-white flex items-center">
          <span className="mr-2">📝</span> Plugin Activity Log
        </h2>

        <div className="flex gap-2">
          <select
            value={filter}
            onChange={(e) => setFilter(e.target.value)}
            className="bg-slate-800 text-white text-sm px-3 py-1 rounded border border-slate-700"
          >
            <option value="all">All Events</option>
            <option value="initialize">Initialize</option>
            <option value="start">Start</option>
            <option value="stop">Stop</option>
            <option value="error">Errors</option>
            <option value="warning">Warnings</option>
            <option value="info">Info</option>
          </select>

          <button
            onClick={loadEvents}
            className="bg-slate-700 hover:bg-slate-600 text-white px-3 py-1 rounded text-sm transition-colors"
          >
            Refresh
          </button>
        </div>
      </div>

      {error && (
        <div className="bg-red-900/50 border border-red-700 text-red-100 px-4 py-3 rounded mb-4">
          {error}
        </div>
      )}

      {loading ? (
        <div className="text-center py-8 text-slate-400">Loading events...</div>
      ) : filteredEvents.length === 0 ? (
        <div className="text-center py-8 text-slate-400">No plugin events found.</div>
      ) : (
        <div className="bg-slate-800 rounded-lg overflow-hidden">
          <table className="w-full text-left">
            <thead className="bg-slate-700">
              <tr>
                <th className="px-4 py-3 text-sm text-slate-300 font-medium">Type</th>
                <th className="px-4 py-3 text-sm text-slate-300 font-medium">Plugin</th>
                <th className="px-4 py-3 text-sm text-slate-300 font-medium">Message</th>
                <th className="px-4 py-3 text-sm text-slate-300 font-medium">Time</th>
                <th className="px-4 py-3 text-sm text-slate-300 font-medium">Status</th>
              </tr>
            </thead>
            <tbody>
              {filteredEvents.map((event) => {
                const { bg, text, icon } = getEventTypeStyle(event.event_type);
                return (
                  <tr key={event.id} className="border-t border-slate-700 hover:bg-slate-750">
                    <td className="px-4 py-3">
                      <span
                        className={`inline-flex items-center ${bg} ${text} px-2 py-1 rounded text-xs`}
                      >
                        <span className="mr-1">{icon}</span>
                        {event.event_type}
                      </span>
                    </td>
                    <td className="px-4 py-3 text-white text-sm">{event.plugin.name}</td>
                    <td className="px-4 py-3 text-slate-300 text-sm">{event.message}</td>
                    <td className="px-4 py-3 text-slate-400 text-xs">
                      {formatTimestamp(event.timestamp)}
                    </td>
                    <td className="px-4 py-3">
                      <span
                        className={`inline-flex items-center ${
                          event.status === 'success'
                            ? 'bg-green-600/20 text-green-400'
                            : event.status === 'error'
                              ? 'bg-red-600/20 text-red-400'
                              : event.status === 'warning'
                                ? 'bg-yellow-600/20 text-yellow-400'
                                : 'bg-blue-600/20 text-blue-400'
                        } px-2 py-1 rounded text-xs`}
                      >
                        {event.status}
                      </span>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
};

export default PluginEvents;
