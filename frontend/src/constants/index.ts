export const API_ENDPOINTS = {
  HEALTH: '/api/health',
  USERS: '/api/users',
  MEDIA: '/api/media',
  MEDIA_LIBRARIES: '/api/admin/media-libraries',
  SCANNER: {
    START: '/api/admin/scanner/start',
    PAUSE: '/api/admin/scanner/pause',
    RESUME: '/api/scanner/resume',
    STATUS: '/api/admin/scanner/status',
    STATS: '/api/admin/scanner/stats',
    CONFIG: '/api/admin/scanner/config',
  },
  PLUGINS: {
    LIST: '/api/admin/plugins',
    ADMIN_PAGES: '/api/admin/plugins/admin-pages',
    UI_COMPONENTS: '/api/admin/plugins/ui-components',
    EVENTS: '/api/admin/plugins/events',
  },
  EVENTS: {
    LIST: '/api/events',
    TYPES: '/api/events/types',
    CLEAR: '/api/events/clear',
  },
} as const;

export const MEDIA_TYPES = {
  MOVIE: 'movie',
  TV: 'tv',
  MUSIC: 'music',
} as const;

export const SCAN_STATUS = {
  IDLE: 'idle',
  RUNNING: 'running',
  PAUSED: 'paused',
  COMPLETED: 'completed',
  ERROR: 'error',
} as const;

export const QUALITY_TIERS = {
  LOSSLESS: 'lossless',
  HIGH: 'high',
  MEDIUM: 'medium',
  STANDARD: 'standard',
} as const;

export const POLLING_INTERVALS = {
  SCAN_STATUS: 3000,
  SYSTEM_EVENTS: 5000,
  PLUGIN_EVENTS: 10000,
} as const;
