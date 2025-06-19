import { useEffect, useRef, useState } from 'react';

interface WebSocketMessage {
  type: 'section_data' | 'section_update' | 'error';
  section_id?: string;
  data?: Record<string, unknown>;
  data_type?: 'main' | 'nerd' | 'metrics';
  timestamp?: number;
  error?: string;
}

interface UseDashboardWebSocketOptions {
  enabled?: boolean;
  reconnectDelay?: number;
  maxReconnectAttempts?: number;
}

export const useDashboardWebSocket = (
  onMessage: (message: WebSocketMessage) => void,
  options: UseDashboardWebSocketOptions = {}
) => {
  const {
    enabled = true,
    reconnectDelay = 3000,
    maxReconnectAttempts = 5
  } = options;

  const [connected, setConnected] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [reconnectAttempts, setReconnectAttempts] = useState(0);

  const wsRef = useRef<WebSocket | null>(null);
  const reconnectTimeoutRef = useRef<number | null>(null);

  const connect = () => {
    if (!enabled || wsRef.current?.readyState === WebSocket.OPEN) {
      return;
    }

    try {
      // Use the current host with ws:// or wss:// protocol
      const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
      const wsUrl = `${protocol}//${window.location.host}/api/v1/dashboard/ws`;
      
      console.log('Connecting to WebSocket:', wsUrl);
      wsRef.current = new WebSocket(wsUrl);

      wsRef.current.onopen = () => {
        setConnected(true);
        setError(null);
        setReconnectAttempts(0);
        console.log('Dashboard WebSocket connected');
      };

      wsRef.current.onmessage = (event) => {
        try {
          const message: WebSocketMessage = JSON.parse(event.data);
          console.log('WebSocket message received:', message);
          onMessage(message);
        } catch (err) {
          console.error('Failed to parse WebSocket message:', err);
        }
      };

      wsRef.current.onclose = (event) => {
        setConnected(false);
        console.log('Dashboard WebSocket disconnected:', event.code, event.reason);

        // Attempt to reconnect if enabled and within retry limits
        if (enabled && reconnectAttempts < maxReconnectAttempts) {
          console.log(`Attempting to reconnect (${reconnectAttempts + 1}/${maxReconnectAttempts})...`);
          reconnectTimeoutRef.current = window.setTimeout(() => {
            setReconnectAttempts(prev => prev + 1);
            connect();
          }, reconnectDelay);
        } else if (reconnectAttempts >= maxReconnectAttempts) {
          setError('Max reconnection attempts reached');
        }
      };

      wsRef.current.onerror = (event) => {
        setError('WebSocket connection error');
        console.error('Dashboard WebSocket error:', event);
      };

    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create WebSocket connection');
    }
  };

  const disconnect = () => {
    if (reconnectTimeoutRef.current) {
      clearTimeout(reconnectTimeoutRef.current);
      reconnectTimeoutRef.current = null;
    }

    if (wsRef.current) {
      wsRef.current.close();
      wsRef.current = null;
    }

    setConnected(false);
  };

  const sendMessage = (message: Record<string, unknown>) => {
    if (wsRef.current?.readyState === WebSocket.OPEN) {
      wsRef.current.send(JSON.stringify(message));
    }
  };

  useEffect(() => {
    if (enabled) {
      connect();
    } else {
      disconnect();
    }

    return () => {
      disconnect();
    };
  }, [enabled]);

  // Reset reconnect attempts when connection is restored
  useEffect(() => {
    if (connected) {
      setReconnectAttempts(0);
    }
  }, [connected]);

  return {
    connected,
    error,
    reconnectAttempts,
    connect,
    disconnect,
    sendMessage
  };
}; 