import React, { useState, useEffect } from 'react';

interface SystemEvent {
  id: string;
  type: string;
  source: string;
  target?: string;
  title: string;
  message: string;
  data: Record<string, unknown>;
  priority: number;
  tags: string[];
  timestamp: string;
  ttl?: number;
}

interface SystemEventsResponse {
  events: SystemEvent[];
  total: number;
  limit: number;
  offset: number;
}

interface EventStats {
  total_events: number;
  events_by_type: Record<string, number>;
  events_by_source: Record<string, number>;
  events_by_priority: Record<string, number>;
  recent_events: SystemEvent[];
  active_subscriptions: number;
}

const SystemEvents: React.FC = () => {
  const [events, setEvents] = useState<SystemEvent[]>([]);
  const [stats, setStats] = useState<EventStats | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [filter, setFilter] = useState<string>('all');
  const [sourceFilter, setSourceFilter] = useState<string>('all');
  const [limit] = useState(50);
  const [offset, setOffset] = useState(0);

  // Format timestamp to a readable date and time
  const formatTimestamp = (timestamp: string): string => {
    const date = new Date(timestamp);
    return date.toLocaleString();
  };

  // Get priority label and style
  const getPriorityStyle = (priority: number): { label: string; bg: string; text: string } => {
    if (priority >= 20) return { label: 'Critical', bg: 'bg-red-600/20', text: 'text-red-400' };
    if (priority >= 10) return { label: 'High', bg: 'bg-orange-600/20', text: 'text-orange-400' };
    if (priority >= 5) return { label: 'Normal', bg: 'bg-blue-600/20', text: 'text-blue-400' };
    return { label: 'Low', bg: 'bg-gray-600/20', text: 'text-gray-400' };
  };

  // Get event type style
  const getEventTypeStyle = (eventType: string): { bg: string; text: string; icon: string } => {
    if (eventType.startsWith('media.')) {
      return { bg: 'bg-purple-600/20', text: 'text-purple-400', icon: '🎵' };
    }
    if (eventType.startsWith('user.')) {
      return { bg: 'bg-green-600/20', text: 'text-green-400', icon: '👤' };
    }
    if (eventType.startsWith('plugin.')) {
      return { bg: 'bg-blue-600/20', text: 'text-blue-400', icon: '🔌' };
    }
    if (eventType.startsWith('system.')) {
      return { bg: 'bg-yellow-600/20', text: 'text-yellow-400', icon: '⚙️' };
    }
    if (eventType.startsWith('scan.')) {
      return { bg: 'bg-indigo-600/20', text: 'text-indigo-400', icon: '🔍' };
    }
    if (eventType.startsWith('playback.')) {
      return { bg: 'bg-pink-600/20', text: 'text-pink-400', icon: '▶️' };
    }
    if (eventType === 'error') {
      return { bg: 'bg-red-600/20', text: 'text-red-400', icon: '❌' };
    }
    if (eventType === 'warning') {
      return { bg: 'bg-orange-600/20', text: 'text-orange-400', icon: '⚠️' };
    }
    return { bg: 'bg-slate-600/20', text: 'text-slate-400', icon: '📋' };
  };

  // Fetch system events from the API
  const loadEvents = async () => {
    try {
      setLoading(true);
      setError(null);

      const params = new URLSearchParams({
        limit: limit.toString(),
        offset: offset.toString(),
      });

      if (filter !== 'all') {
        params.append('type', filter);
      }

      if (sourceFilter !== 'all') {
        params.append('source', sourceFilter);
      }

      console.log('Fetching events with params:', Object.fromEntries(params));
      const response = await fetch(`/api/events?${params}`);

      if (!response.ok) {
        throw new Error(`HTTP error! status: ${response.status}`);
      }

      const data = (await response.json()) as SystemEventsResponse;
      console.log('Received events:', data.events.length, 'total:', data.total);

      setEvents(data.events || []);
    } catch (err) {
      setError(
        'Failed to load system events: ' + (err instanceof Error ? err.message : String(err))
      );
      console.error('Error loading system events:', err);
    } finally {
      setLoading(false);
    }
  };

  // Fetch event statistics
  const loadStats = async () => {
    try {
      const response = await fetch('/api/events/stats');
      if (response.ok) {
        const data = await response.json();
        setStats(data);
      }
    } catch (err) {
      console.error('Error loading event stats:', err);
    }
  };

  // Clear all events
  const clearEvents = async () => {
    if (
      !window.confirm('Are you sure you want to clear all system events? This cannot be undone.')
    ) {
      return;
    }

    try {
      setLoading(true);
      const response = await fetch('/api/events/', {
        method: 'DELETE',
      });

      if (!response.ok) {
        throw new Error(`HTTP error! status: ${response.status}`);
      }

      await response.json(); // Read response but we don't need to use it

      // Reload data after clearing
      loadEvents();
      loadStats();

      // Show success message
      alert('All events have been cleared successfully.');
    } catch (err) {
      setError('Failed to clear events: ' + (err instanceof Error ? err.message : String(err)));
      console.error('Error clearing events:', err);
    } finally {
      setLoading(false);
    }
  };

  // Load events and stats when component mounts or filters change
  useEffect(() => {
    loadEvents();
    loadStats();
  }, [filter, sourceFilter, limit, offset, loadEvents, loadStats]);

  // Get unique sources for filter dropdown
  const uniqueSources = Array.from(new Set(events.map((event) => event.source))).sort();

  return (
    <div className="bg-slate-900 rounded-lg p-6 shadow-xl">
      <div className="flex justify-between items-center mb-6">
        <h2 className="text-xl font-semibold text-white flex items-center">
          <span className="mr-2">🌐</span> System Events
        </h2>

        <div className="flex gap-2">
          <select
            value={filter}
            onChange={(e) => setFilter(e.target.value)}
            className="bg-slate-800 text-white text-sm px-3 py-1 rounded border border-slate-700"
          >
            <option value="all">All Event Types</option>
            <option value="media.library.scanned">Media Library Scanned</option>
            <option value="media.file.found">Media File Found</option>
            <option value="user.logged_in">User Logged In</option>
            <option value="plugin.loaded">Plugin Loaded</option>
            <option value="system.started">System Started</option>
            <option value="scan.completed">Scan Completed</option>
            <option value="error">Errors</option>
            <option value="warning">Warnings</option>
          </select>

          <select
            value={sourceFilter}
            onChange={(e) => setSourceFilter(e.target.value)}
            className="bg-slate-800 text-white text-sm px-3 py-1 rounded border border-slate-700"
          >
            <option value="all">All Sources</option>
            <option value="system">System</option>
            {uniqueSources
              .filter((source) => source !== 'system')
              .map((source) => (
                <option key={source} value={source}>
                  {source}
                </option>
              ))}
          </select>

          <button
            onClick={loadEvents}
            className="bg-slate-700 hover:bg-slate-600 text-white px-3 py-1 rounded text-sm transition-colors"
          >
            Refresh
          </button>

          <button
            onClick={clearEvents}
            className="bg-red-700 hover:bg-red-600 text-white px-3 py-1 rounded text-sm transition-colors"
          >
            Clear Events
          </button>
        </div>
      </div>

      {/* Event Statistics */}
      {stats && (
        <div className="grid grid-cols-1 md:grid-cols-4 gap-4 mb-6">
          <div className="bg-slate-800 rounded-lg p-4">
            <div className="text-2xl font-bold text-white">{stats.total_events}</div>
            <div className="text-sm text-slate-400">Total Events</div>
          </div>
          <div className="bg-slate-800 rounded-lg p-4">
            <div className="text-2xl font-bold text-blue-400">{stats.active_subscriptions}</div>
            <div className="text-sm text-slate-400">Active Subscriptions</div>
          </div>
          <div className="bg-slate-800 rounded-lg p-4">
            <div className="text-2xl font-bold text-green-400">
              {Object.keys(stats.events_by_type).length}
            </div>
            <div className="text-sm text-slate-400">Event Types</div>
          </div>
          <div className="bg-slate-800 rounded-lg p-4">
            <div className="text-2xl font-bold text-purple-400">
              {Object.keys(stats.events_by_source).length}
            </div>
            <div className="text-sm text-slate-400">Event Sources</div>
          </div>
        </div>
      )}

      {error && (
        <div className="bg-red-900/50 border border-red-700 text-red-100 px-4 py-3 rounded mb-4">
          {error}
        </div>
      )}

      {/* Debug information */}
      <div className="bg-gray-800/50 border border-gray-700 text-gray-300 px-4 py-3 rounded mb-4 text-xs">
        <div>API URL: /api/events</div>
        <div>Filter: {filter}</div>
        <div>Source: {sourceFilter}</div>
        <div>Offset: {offset}</div>
        <div>Limit: {limit}</div>
        <div>Events loaded: {events.length}</div>
        <div className="mt-2 p-1 bg-gray-900 overflow-auto max-h-32">
          <pre>
            {JSON.stringify(
              events.map((e) => ({ id: e.id, type: e.type, title: e.title })),
              null,
              2
            )}
          </pre>
        </div>
      </div>

      {loading ? (
        <div className="text-center py-8 text-slate-400">Loading events...</div>
      ) : events.length === 0 ? (
        <div className="text-center py-8 text-slate-400">No system events found.</div>
      ) : (
        <div className="bg-slate-800 rounded-lg overflow-hidden">
          <table className="w-full text-left">
            <thead className="bg-slate-700">
              <tr>
                <th className="px-4 py-3 text-sm text-slate-300 font-medium">Type</th>
                <th className="px-4 py-3 text-sm text-slate-300 font-medium">Source</th>
                <th className="px-4 py-3 text-sm text-slate-300 font-medium">Title</th>
                <th className="px-4 py-3 text-sm text-slate-300 font-medium">Message</th>
                <th className="px-4 py-3 text-sm text-slate-300 font-medium">Priority</th>
                <th className="px-4 py-3 text-sm text-slate-300 font-medium">Time</th>
              </tr>
            </thead>
            <tbody>
              {events.map((event) => {
                const { bg, text, icon } = getEventTypeStyle(event.type);
                const priorityStyle = getPriorityStyle(event.priority);
                return (
                  <tr key={event.id} className="border-t border-slate-700 hover:bg-slate-750">
                    <td className="px-4 py-3">
                      <span
                        className={`inline-flex items-center ${bg} ${text} px-2 py-1 rounded text-xs`}
                      >
                        <span className="mr-1">{icon}</span>
                        {event.type}
                      </span>
                    </td>
                    <td className="px-4 py-3 text-slate-300 text-sm">
                      <code className="bg-slate-700 px-2 py-1 rounded text-xs">{event.source}</code>
                    </td>
                    <td className="px-4 py-3 text-white text-sm font-medium">
                      {event.title || '-'}
                    </td>
                    <td className="px-4 py-3 text-slate-300 text-sm">{event.message || '-'}</td>
                    <td className="px-4 py-3">
                      <span
                        className={`inline-flex items-center ${priorityStyle.bg} ${priorityStyle.text} px-2 py-1 rounded text-xs`}
                      >
                        {priorityStyle.label}
                      </span>
                    </td>
                    <td className="px-4 py-3 text-slate-400 text-xs">
                      {formatTimestamp(event.timestamp)}
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}

      {/* Pagination */}
      <div className="mt-4 flex justify-between items-center">
        <div className="text-sm text-slate-400">
          Showing {offset + 1} to {offset + events.length} events
        </div>
        <div className="flex gap-2">
          <button
            onClick={() => setOffset(Math.max(0, offset - limit))}
            disabled={offset === 0}
            className="bg-slate-700 hover:bg-slate-600 disabled:bg-slate-800 disabled:text-slate-500 text-white px-3 py-1 rounded text-sm transition-colors"
          >
            Previous
          </button>
          <button
            onClick={() => setOffset(offset + limit)}
            disabled={events.length < limit}
            className="bg-slate-700 hover:bg-slate-600 disabled:bg-slate-800 disabled:text-slate-500 text-white px-3 py-1 rounded text-sm transition-colors"
          >
            Next
          </button>
        </div>
      </div>
    </div>
  );
};

export default SystemEvents;
