/**
 * API endpoint definitions with HTTP methods
 * All endpoints are relative to the API base URL
 */

export const API_BASE_URL = '/api';

export const API_ENDPOINTS = {
  // Media endpoints
  MEDIA: {
    FILES: {
      path: '/media/files',
      method: 'GET' as const,
      description: 'Get media files with pagination',
    },
    FILE_BY_ID: {
      path: (id: string) => `/media/files/${id}`,
      method: 'GET' as const,
      description: 'Get specific media file by ID',
    },
    FILE_METADATA: {
      path: (id: string) => `/media/files/${id}/metadata`,
      method: 'GET' as const,
      description: 'Get metadata for a media file',
    },
    FILE_STREAM: {
      path: (id: string) => `/playback/stream/file/${id}`,
      method: 'GET' as const,
      description: 'Stream media file',
    },
    FILE_ALBUM_ARTWORK: {
      path: (id: string | number) => `/media/files/${id}/album-artwork`,
      method: 'GET' as const,
      description: 'Get album artwork for media file',
    },
    FILE_ALBUM_ID: {
      path: (id: string | number) => `/media/files/${id}/album-id`,
      method: 'GET' as const,
      description: 'Get album ID for media file',
    },
  },

  // Playback endpoints - match actual backend implementation
  PLAYBACK: {
    DECIDE: {
      path: '/playback/decide',
      method: 'POST' as const,
      description: 'Get playback decision for media',
    },
    START_SESSION: {
      path: '/playback/session/start',
      method: 'POST' as const,
      description: 'Start transcoding session',
    },
    STOP_SESSION: {
      path: (sessionId: string) => `/playback/session/${sessionId}`,
      method: 'DELETE' as const,
      description: 'Stop transcoding session',
    },
    STOP_ALL_SESSIONS: {
      path: '/playback/sessions/all',
      method: 'DELETE' as const,
      description: 'Stop all transcoding sessions',
    },
    ANALYTICS: {
      path: '/analytics/session',
      method: 'POST' as const,
      description: 'Track playback analytics',
    },
    COMPATIBILITY: {
      path: '/playback/compatibility',
      method: 'POST' as const,
      description: 'Get playback compatibility data',
    },
  },


  // TV Show endpoints
  TV: {
    SHOWS: {
      path: '/tv/shows',
      method: 'GET' as const,
      description: 'Get all TV shows',
    },
    SHOW_BY_ID: {
      path: (id: string) => `/tv/shows/${id}`,
      method: 'GET' as const,
      description: 'Get TV show by ID',
    },
    SHOW_SEASONS: {
      path: (id: string) => `/tv/shows/${id}/seasons`,
      method: 'GET' as const,
      description: 'Get seasons for TV show',
    },
    SEASON_EPISODES: {
      path: (showId: string, seasonId: string) => `/tv/shows/${showId}/seasons/${seasonId}/episodes`,
      method: 'GET' as const,
      description: 'Get episodes for season',
    },
    EPISODE_BY_ID: {
      path: (episodeId: string) => `/tv/episodes/${episodeId}`,
      method: 'GET' as const,
      description: 'Get episode by ID',
    },
  },

  // Movie endpoints
  MOVIES: {
    LIST: {
      path: '/movies',
      method: 'GET' as const,
      description: 'Get all movies',
    },
    BY_ID: {
      path: (id: string) => `/movies/${id}`,
      method: 'GET' as const,
      description: 'Get movie by ID',
    },
  },

  // Search endpoints
  SEARCH: {
    MULTI: {
      path: '/search',
      method: 'GET' as const,
      description: 'Search across all media types',
    },
    TV_SHOWS: {
      path: '/search/tv',
      method: 'GET' as const,
      description: 'Search TV shows',
    },
    MOVIES: {
      path: '/search/movies',
      method: 'GET' as const,
      description: 'Search movies',
    },
  },

  // User/Settings endpoints
  USER: {
    PROFILE: {
      path: '/user/profile',
      method: 'GET' as const,
      description: 'Get user profile',
    },
    SETTINGS: {
      path: '/user/settings',
      method: 'GET' as const,
      description: 'Get user settings',
    },
    UPDATE_SETTINGS: {
      path: '/user/settings',
      method: 'PUT' as const,
      description: 'Update user settings',
    },
  },

  // System endpoints
  SYSTEM: {
    HEALTH: {
      path: '/health',
      method: 'GET' as const,
      description: 'Health check endpoint',
    },
    STATUS: {
      path: '/status',
      method: 'GET' as const,
      description: 'System status',
    },
  },
} as const;

// Type definitions for API methods
export type ApiMethod = 'GET' | 'POST' | 'PUT' | 'DELETE' | 'PATCH' | 'HEAD' | 'OPTIONS';

// Helper type to extract all endpoint definitions
export type ApiEndpoint = typeof API_ENDPOINTS[keyof typeof API_ENDPOINTS][keyof typeof API_ENDPOINTS[keyof typeof API_ENDPOINTS]];

// Helper function to build full URL
export function buildApiUrl(endpoint: string | ((param: string) => string), param?: string): string {
  const path = typeof endpoint === 'function' && param ? endpoint(param) : endpoint;
  return `${API_BASE_URL}${path}`;
}

// Helper function to build URL with query params
export function buildApiUrlWithParams(
  endpoint: string | ((param: string) => string), 
  params?: Record<string, string | number | boolean | undefined>,
  pathParam?: string
): string {
  const path = typeof endpoint === 'function' && pathParam ? endpoint(pathParam) : endpoint;
  const url = `${API_BASE_URL}${path}`;
  
  if (!params) return url;
  
  const searchParams = new URLSearchParams();
  Object.entries(params).forEach(([key, value]) => {
    if (value !== undefined && value !== null) {
      searchParams.append(key, String(value));
    }
  });
  
  const queryString = searchParams.toString();
  return queryString ? `${url}?${queryString}` : url;
}

// Common query parameter types
export interface PaginationParams {
  page?: number;
  limit?: number;
  offset?: number;
}

export interface SortParams {
  sort?: string;
  order?: 'asc' | 'desc';
}

export interface FilterParams {
  filter?: Record<string, string | number | boolean>;
}

// Combined common params
export type CommonQueryParams = PaginationParams & SortParams & FilterParams;