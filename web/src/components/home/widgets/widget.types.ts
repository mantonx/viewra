/**
 * Widget Types and Interfaces
 *
 * Types for the home screen widget system. Widgets are plugin-provided
 * UI sections that render on the home screen.
 */

/** Widget type constants - must match backend SDK values */
export const WidgetType = {
  SearchHero: 'search-hero',
  FeaturedRow: 'featured-row',
  ContinueRow: 'continue-row',
  MediaRow: 'media-row',
} as const

export type WidgetTypeValue = (typeof WidgetType)[keyof typeof WidgetType]

/** Widget location constants */
export const WidgetLocation = {
  HomepageTop: 'homepage-top',
  HomepageSections: 'homepage-sections',
} as const

export type WidgetLocationValue = (typeof WidgetLocation)[keyof typeof WidgetLocation]

/** Client type for filtering widgets */
export const ClientType = {
  All: 'all',
  Web: 'web',
  iOS: 'ios',
  Android: 'android',
  Roku: 'roku',
  FireTV: 'firetv',
  SmartTV: 'smarttv',
} as const

export type ClientTypeValue = (typeof ClientType)[keyof typeof ClientType]

/**
 * Widget configuration from plugin schema
 */
export interface WidgetConfig {
  id: string
  type: WidgetTypeValue
  location: WidgetLocationValue
  client_types: ClientTypeValue[]
  priority: number
  cache_ttl_seconds?: number
  config?: Record<string, unknown>
  required_capability?: string
  settings_key?: string
}

/**
 * Search suggestion from the backend
 */
export interface Suggestion {
  id: string
  label: string
  icon?: string
  description?: string
  style?: 'primary' | 'secondary' | 'accent'
  action: SuggestionAction
}

export interface SuggestionAction {
  type: 'search' | 'filter' | 'navigate'
  query?: string
  filter?: Record<string, string>
  url?: string
}

/**
 * Trending item from TMDb or other provider
 */
export interface TrendingItem {
  external_id: string
  media_type: string
  title: string
  year: number
  popularity: number
  poster_path?: string
  overview?: string
  local_id?: number
  local_matched: boolean
}

/**
 * Recommendation item from plugins (generic media item with reason)
 */
export interface RecommendationItem {
  entity_type: 'movie' | 'tv_show'
  entity_id: number
  title: string
  year?: number
  reason?: string
}

/**
 * Widget data responses from plugin endpoints
 */
export interface SearchHeroData {
  placeholder: string
  suggestions: Suggestion[]
  search_url: string
}

export interface MediaRowData {
  title: string
  subtitle?: string
  /** Generic items array (from recommendations plugin) */
  items?: RecommendationItem[]
  /** Movie data for rendering */
  movies?: import('@/lib/types/movies').Movie[]
  /** TV show data for rendering */
  shows?: import('@/lib/types/tv').TVShowSummary[]
  see_all_url?: string
}

/**
 * Progress information for continue watching items
 */
export interface MediaProgress {
  /** Progress percentage (0-100) */
  percent: number
  /** Current playback position in seconds */
  position_seconds: number
  /** Total duration in seconds */
  duration_seconds: number
  /** Human-readable remaining time (e.g., "1h 23m left") */
  remaining_text?: string
}

/**
 * Episode context for TV shows in continue watching
 */
export interface EpisodeContext {
  /** Season number */
  season: number
  /** Episode number within the season */
  episode: number
  /** Episode title */
  episode_title?: string
  /** TV show title */
  show_title: string
  /** Media ID of the episode (for direct playback) */
  episode_media_id: number
}

/**
 * Item in the continue watching row with progress and episode info
 */
export interface ContinueWatchingItem {
  /** Media type: "movie" or "tv_show" */
  entity_type: 'movie' | 'tv_show'
  /** Database ID (movie ID or show ID) */
  entity_id: number
  /** Display title */
  title: string
  /** Release/air year */
  year?: number
  /** URL to backdrop/fanart image (16:9) */
  backdrop_url?: string
  /** Playback progress information */
  progress: MediaProgress
  /** Episode details for TV shows (null for movies) */
  episode_context?: EpisodeContext
  /** When this item was last watched */
  last_watched_at: string
}

/**
 * Continue watching row data
 */
export interface ContinueWatchingData {
  title: string
  /** New format: array of continue watching items with progress */
  items?: ContinueWatchingItem[]
  /** Legacy format: movie data */
  movies?: import('@/lib/types/movies').Movie[]
  /** Legacy format: TV show data */
  shows?: import('@/lib/types/tv').TVShowSummary[]
}

export interface TrendingRowData {
  title: string
  items: TrendingItem[]
  window: string
  source: string
}

export interface FeaturedRowData {
  title: string
  suggestions: Suggestion[]
}

/**
 * Home screen section - a widget with its data
 */
export interface HomeSection {
  id: string
  type: WidgetTypeValue
  location: WidgetLocationValue
  priority: number
  plugin_id: string
  data: SearchHeroData | MediaRowData | TrendingRowData | FeaturedRowData | ContinueWatchingData
  cache_ttl_seconds?: number
}

/**
 * Hero data for the home screen backdrop
 */
export interface HeroData {
  /** Media ID to fetch backdrop from */
  backdrop_media_id?: number
  /** Type of media (movie or tv_show) */
  backdrop_media_type?: 'movie' | 'tv_show'
  /** Time-based greeting (Good morning, etc.) */
  greeting?: string
  /** Formatted date (Saturday, January 4) */
  date_text?: string
}

/**
 * Home screen response metadata
 */
export interface HomeMeta {
  generated_at: string
  user_context?: {
    has_watch_history: boolean
    has_ratings: boolean
    time_of_day?: string
    season?: string
  }
  hero?: HeroData
}

/**
 * Home screen API response
 */
export interface HomeResponse {
  sections: HomeSection[]
  user_id: string
  client_type: string
  cached_at?: number
  meta?: HomeMeta
}

/**
 * Search provider info
 */
export interface SearchProviderInfo {
  id: string
  name: string
  description: string
  icon?: string
  priority: number
  capabilities: string[]
}

/**
 * Suggestions response from semantic-search plugin
 * Note: The API returns 'Suggestions' (capital S) from Go struct
 */
export interface SuggestionsResponse {
  Suggestions: Suggestion[]
}
