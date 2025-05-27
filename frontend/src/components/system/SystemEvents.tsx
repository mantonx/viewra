import React, { useState, useEffect, useCallback } from 'react';

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
  const [statsLoading, setStatsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [filter, setFilter] = useState<string>('all');
  const [sourceFilter, setSourceFilter] = useState<string>('all');
  const [limit] = useState(50);
  const [offset, setOffset] = useState(0);
  const [newEventIds, setNewEventIds] = useState<Set<string>>(new Set());
  const [filteredTotal, setFilteredTotal] = useState<number>(0);
  const [eventTypes, setEventTypes] = useState<string[]>([]);
  const [eventTypesLoading, setEventTypesLoading] = useState<boolean>(true);

  // Format timestamp to a readable date and time
  const formatTimestamp = (timestamp: string): string => {
    const date = new Date(timestamp);
    return date.toLocaleString();
  };

  // Format event type for display
  const formatEventTypeName = (type: string): string => {
    // Convert from format like "media.library.scanned" to "Media Library Scanned"
    const parts = type.split('.');

    // Handle special cases like "error", "warning", etc.
    if (parts.length === 1) {
      const singleWord = parts[0];
      return singleWord.charAt(0).toUpperCase() + singleWord.slice(1);
    }

    // Group by category for better organization
    const category = parts[0].charAt(0).toUpperCase() + parts[0].slice(1);
    const details = parts
      .slice(1)
      .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
      .join(' ');

    return `${category}: ${details}`;
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
  const loadEvents = useCallback(async () => {
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

      const response = await fetch(`/api/events?${params}`);

      if (!response.ok) {
        throw new Error(`HTTP error! status: ${response.status}`);
      }

      const data = (await response.json()) as SystemEventsResponse;
      setEvents(data.events || []);
      setFilteredTotal(data.total || 0);
    } catch (err) {
      setError(
        'Failed to load system events: ' + (err instanceof Error ? err.message : String(err))
      );
      console.error('Error loading system events:', err);
    } finally {
      setLoading(false);
    }
  }, [filter, sourceFilter, limit, offset]);

  // Fetch event statistics
  const loadStats = useCallback(async () => {
    try {
      setStatsLoading(true);
      const response = await fetch('/api/events/stats');
      if (response.ok) {
        const data = await response.json();
        setStats(data);
      }
    } catch (err) {
      console.error('Error loading event stats:', err);
    } finally {
      setStatsLoading(false);
    }
  }, []);

  // Fetch available event types
  const loadEventTypes = useCallback(async () => {
    try {
      setEventTypesLoading(true);
      const response = await fetch('/api/events/types');

      if (response.ok) {
        const data = (await response.json()) as { count: number; event_types: string[] };

        // Group event types by category
        const eventsByCategory: Record<string, string[]> = {};
        (data.event_types || []).forEach((type) => {
          const category = type.split('.')[0];
          if (!eventsByCategory[category]) {
            eventsByCategory[category] = [];
          }
          eventsByCategory[category].push(type);
        });

        // Sort each category internally
        Object.keys(eventsByCategory).forEach((category) => {
          eventsByCategory[category].sort();
        });

        // Create a flat, organized list with categories at the top
        const orderedCategories = ['media', 'user', 'system', 'plugin', 'playback', 'scan'];
        const otherCategories = Object.keys(eventsByCategory)
          .filter((cat) => !orderedCategories.includes(cat))
          .sort();

        const organizedTypes: string[] = [];

        // Add ordered categories first
        orderedCategories.forEach((category) => {
          if (eventsByCategory[category]) {
            // Each type is added individually
            eventsByCategory[category].forEach((type) => organizedTypes.push(type));
          }
        });

        // Add other categories
        otherCategories.forEach((category) => {
          if (eventsByCategory[category]) {
            eventsByCategory[category].forEach((type) => organizedTypes.push(type));
          }
        });

        setEventTypes(organizedTypes);
      }
    } catch (err) {
      console.error('Error loading event types:', err);
      // Use basic event types if API call fails
      setEventTypes([
        'media.library.scanned',
        'media.file.found',
        'user.logged_in',
        'system.started',
        'error',
        'warning',
      ]);
    } finally {
      setEventTypesLoading(false);
    }
  }, []);

  // Clear all events
  const clearEvents = useCallback(async () => {
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
  }, [loadEvents, loadStats]);

  // Delete individual event
  const deleteEvent = useCallback(
    async (eventId: string, eventTitle: string) => {
      if (
        !window.confirm(
          `Are you sure you want to delete the event "${eventTitle}"? This cannot be undone.`
        )
      ) {
        return;
      }

      try {
        const response = await fetch(`/api/events/${eventId}`, {
          method: 'DELETE',
        });

        if (!response.ok) {
          throw new Error(`HTTP error! status: ${response.status}`);
        }

        // Remove event from local state for optimistic UI update
        setEvents((prevEvents) => prevEvents.filter((event) => event.id !== eventId));

        // Reload stats to reflect the change
        loadStats();
      } catch (err) {
        setError('Failed to delete event: ' + (err instanceof Error ? err.message : String(err)));
        console.error('Error deleting event:', err);
        // Reload events to restore state in case of error
        loadEvents();
      }
    },
    [loadEvents, loadStats]
  );

  // Load events and stats when component mounts or filters change
  useEffect(() => {
    loadEvents();
    loadStats();
  }, [loadEvents, loadStats]);

  // Load event types when component mounts
  useEffect(() => {
    loadEventTypes();
  }, [loadEventTypes]);

  // Real-time event streaming using Server-Sent Events
  useEffect(() => {
    let eventSource: EventSource | null = null;

    const connectToEventStream = () => {
      try {
        // Build URL with filter parameters for server-side filtering
        let streamUrl = '/api/events/stream';
        const params = new URLSearchParams();

        // Add type filter if not "all"
        if (filter !== 'all') {
          params.append('types', filter);
        }

        // Add source filter if not "all"
        if (sourceFilter !== 'all') {
          params.append('sources', sourceFilter);
        }

        // Append query parameters if any exist
        if (params.toString()) {
          streamUrl += '?' + params.toString();
        }

        console.log('Connecting to filtered event stream:', streamUrl);
        eventSource = new EventSource(streamUrl);

        eventSource.onopen = () => {
          console.log('Connected to event stream');
          setError(null);
        };

        eventSource.onmessage = (event) => {
          try {
            const data = JSON.parse(event.data);

            if (data.type === 'connected') {
              console.log('Event stream connected:', data.message);
              return;
            }

            if (data.type === 'event' && data.data) {
              const newEvent: SystemEvent = data.data;

              // Add new event to the top of the list
              // (Server-side filtering ensures we only receive events matching our filters)
              setEvents((prevEvents) => {
                // Mark as new for animation
                setNewEventIds((prev) => new Set(prev).add(newEvent.id));

                // Remove animation after 3 seconds
                setTimeout(() => {
                  setNewEventIds((prev) => {
                    const updated = new Set(prev);
                    updated.delete(newEvent.id);
                    return updated;
                  });
                }, 3000);

                // Add to the beginning of the list and keep only limit items
                const updatedEvents = [newEvent, ...prevEvents.slice(0, limit - 1)];
                return updatedEvents;
              });

              // Update stats
              loadStats();
            }
          } catch (err) {
            console.error('Error parsing event data:', err);
          }
        };

        eventSource.onerror = (error) => {
          console.error('EventSource error:', error);
          setError('Lost connection to event stream. Attempting to reconnect...');

          // Attempt to reconnect after a delay
          setTimeout(() => {
            if (eventSource) {
              eventSource.close();
            }
            connectToEventStream();
          }, 5000);
        };
      } catch (err) {
        console.error('Failed to connect to event stream:', err);
        setError('Failed to connect to real-time event stream');
      }
    };

    connectToEventStream();

    return () => {
      if (eventSource) {
        console.log('Closing event stream connection');
        eventSource.close();
      }
    };

    // Include filter, sourceFilter, limit, and loadStats in the dependency array
    // to reconnect with new filters when they change
  }, [filter, sourceFilter, limit, loadStats]);

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
            onChange={(e) => {
              setFilter(e.target.value);
              setOffset(0); // Reset to first page when filter changes
            }}
            className="bg-slate-800 text-white text-sm px-3 py-1 rounded border border-slate-700"
          >
            <option value="all">All Event Types</option>
            {eventTypesLoading ? (
              <option value="loading" disabled>
                Loading event types...
              </option>
            ) : eventTypes.length === 0 ? (
              <option value="none" disabled>
                No event types available
              </option>
            ) : (
              // Render event types
              eventTypes.map((type) => (
                <option key={type} value={type}>
                  {formatEventTypeName(type)}
                </option>
              ))
            )}
          </select>

          <select
            value={sourceFilter}
            onChange={(e) => {
              setSourceFilter(e.target.value);
              setOffset(0); // Reset to first page when source filter changes
            }}
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
            onClick={clearEvents}
            className="bg-red-700 hover:bg-red-600 text-white px-3 py-1 rounded text-sm transition-colors"
          >
            Clear Events
          </button>
        </div>
      </div>

      {/* Event Statistics */}
      {statsLoading ? (
        <div className="grid grid-cols-1 md:grid-cols-4 gap-4 mb-6">
          {[...Array(4)].map((_, i) => (
            <div key={i} className="bg-slate-800 rounded-lg p-4 animate-pulse">
              <div className="h-8 bg-slate-700 rounded mb-2"></div>
              <div className="h-4 bg-slate-700 rounded w-2/3"></div>
            </div>
          ))}
        </div>
      ) : stats ? (
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4 mb-6">
          <div className="bg-slate-800 rounded-lg p-4">
            <div className="flex items-baseline">
              <div className="text-2xl font-bold text-white">{filteredTotal}</div>
            </div>
            <div className="text-sm text-slate-400">Events Found</div>
            <div className="text-xs text-slate-500 mt-1">
              {stats.events_by_type && Object.entries(stats.events_by_type).length > 0 ? (
                <>
                  Most common:{' '}
                  {Object.entries(stats.events_by_type)
                    .sort((a, b) => (b[1] as number) - (a[1] as number))
                    .slice(0, 2)
                    .map(([type]) => type)
                    .join(', ')}
                </>
              ) : (
                'No event type breakdown available'
              )}
            </div>
          </div>
          <div className="bg-slate-800 rounded-lg p-4">
            <div className="text-2xl font-bold text-blue-400">
              {events.length > 0
                ? Object.keys(
                    events.reduce(
                      (acc, event) => {
                        acc[event.type] = true;
                        return acc;
                      },
                      {} as Record<string, boolean>
                    )
                  ).length
                : 0}
            </div>
            <div className="text-sm text-slate-400">Event Types</div>
            <div className="text-xs text-slate-500 mt-1">
              {events.length > 0 ? (
                <>
                  From:{' '}
                  {Array.from(new Set(events.map((e) => e.type)))
                    .slice(0, 3)
                    .join(', ')}
                  {Array.from(new Set(events.map((e) => e.type))).length > 3
                    ? ` + ${Array.from(new Set(events.map((e) => e.type))).length - 3} more`
                    : ''}
                </>
              ) : (
                'No events available'
              )}
            </div>
          </div>
        </div>
      ) : null}

      {error && (
        <div className="bg-red-900/50 border border-red-700 text-red-100 px-4 py-3 rounded mb-4">
          {error}
        </div>
      )}

      {loading ? (
        <div className="text-center py-8 text-slate-400">
          <div className="inline-block animate-spin rounded-full h-8 w-8 border-b-2 border-blue-400 mb-4"></div>
          <div>Loading events...</div>
        </div>
      ) : events.length === 0 ? (
        <div className="text-center py-12 text-slate-400">
          <div className="text-4xl mb-4">📭</div>
          <div className="text-lg mb-2">No system events found</div>
          <div className="text-sm">
            {filter !== 'all' || sourceFilter !== 'all'
              ? 'Try adjusting your filters or refreshing the page.'
              : 'Events will appear here as system activities occur.'}
          </div>
        </div>
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
                <th className="px-4 py-3 text-sm text-slate-300 font-medium">Actions</th>
              </tr>
            </thead>
            <tbody>
              {events.map((event) => {
                const { bg, text, icon } = getEventTypeStyle(event.type);
                const priorityStyle = getPriorityStyle(event.priority);
                const isNewEvent = newEventIds.has(event.id);
                return (
                  <tr
                    key={event.id}
                    className={`border-t border-slate-700 hover:bg-slate-750 transition-all duration-300 ${
                      isNewEvent ? 'new-event-animation bg-green-900/10 border-green-500/30' : ''
                    }`}
                  >
                    <td className="px-4 py-3">
                      <span
                        className={`inline-flex items-center ${bg} ${text} px-2 py-1 rounded text-xs`}
                      >
                        <span className="mr-1">{icon}</span>
                        {formatEventTypeName(event.type)}
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
                    <td className="px-4 py-3">
                      <button
                        onClick={() => deleteEvent(event.id, event.title || event.type)}
                        className="bg-red-700 hover:bg-red-600 text-white px-2 py-1 rounded text-xs transition-colors flex items-center"
                        title="Delete this event"
                      >
                        <span className="mr-1">🗑️</span>
                        Delete
                      </button>
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
          {events.length > 0 ? (
            <>
              Showing rows {offset + 1}-{offset + events.length} of {filteredTotal}
              {events.length === 1 ? ' event' : ' events'}
              {filter !== 'all' || sourceFilter !== 'all' ? ' (filtered)' : ''}
            </>
          ) : (
            <>No events to display</>
          )}
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
            disabled={events.length < limit || offset + events.length >= filteredTotal}
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
