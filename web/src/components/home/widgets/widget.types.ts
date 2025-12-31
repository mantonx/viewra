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
 * Media item in widget data
 */
export interface MediaItem {
  entity_type: string
  entity_id: number
  title: string
  year?: number
  poster?: string
  backdrop?: string
  reason?: string
  progress?: MediaProgress
  rating?: 'up' | 'down' | 'favorite' | null
}

export interface MediaProgress {
  percent: number
  position_seconds: number
  duration_seconds: number
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
  items: MediaItem[]
  see_all_url?: string
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
  data: SearchHeroData | MediaRowData | TrendingRowData | FeaturedRowData
  cache_ttl_seconds?: number
}

/**
 * Home screen API response
 */
export interface HomeResponse {
  sections: HomeSection[]
  user_id: string
  client_type: string
  cached_at?: number
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
 * Suggestions response
 */
export interface SuggestionsResponse {
  suggestions: Suggestion[]
}
