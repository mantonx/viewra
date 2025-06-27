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
      path: (id: string) => `/media/files/${id}/stream`,
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

  // Playback endpoints
  PLAYBACK: {
    DECIDE: {
      path: '/playback/decide',
      method: 'POST' as const,
      description: 'Get playback decision for media',
    },
    START: {
      path: '/playback/start',
      method: 'POST' as const,
      description: 'Start transcoding session',
    },
    SESSION: {
      path: (sessionId: string) => `/playback/session/${sessionId}`,
      method: 'DELETE' as const,
      description: 'Stop transcoding session',
    },
    SEEK_AHEAD: {
      path: '/playback/seek-ahead',
      method: 'POST' as const,
      description: 'Request seek-ahead transcoding',
    },
  },

  // Content-addressable storage endpoints (v1)
  CONTENT: {
    MANIFEST: {
      path: (contentHash: string) => `/v1/content/${contentHash}/manifest.mpd`,
      method: 'GET' as const,
      description: 'Get DASH manifest from content store',
    },
    HLS_MANIFEST: {
      path: (contentHash: string) => `/v1/content/${contentHash}/playlist.m3u8`,
      method: 'GET' as const,
      description: 'Get HLS playlist from content store',
    },
    FILE: {
      path: (contentHash: string, filename: string) => `/v1/content/${contentHash}/${filename}`,
      method: 'GET' as const,
      description: 'Get content file (segments, init files, etc.)',
    },
    INFO: {
      path: (contentHash: string) => `/v1/content/${contentHash}/info`,
      method: 'GET' as const,
      description: 'Get content metadata and status',
    },
  },

  // Asset endpoints (v1)
  ASSETS_V1: {
    ASSET_DATA: {
      path: (assetId: string) => `/v1/assets/${assetId}/data`,
      method: 'GET' as const,
      description: 'Get asset data by ID',
    },
    ENTITY_PREFERRED: {
      path: (entityType: string, entityId: string, assetType: string) => 
        `/v1/assets/entity/${entityType}/${entityId}/preferred/${assetType}`,
      method: 'GET' as const,
      description: 'Get preferred asset for entity',
    },
    ENTITY_PREFERRED_DATA: {
      path: (entityType: string, entityId: string, assetType: string) => 
        `/v1/assets/entity/${entityType}/${entityId}/preferred/${assetType}/data`,
      method: 'GET' as const,
      description: 'Get preferred asset data for entity',
    },
    ENTITY_ASSETS: {
      path: (entityType: string, entityId: string) => 
        `/v1/assets/entity/${entityType}/${entityId}`,
      method: 'GET' as const,
      description: 'Get all assets for entity',
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