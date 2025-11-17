/**
 * TypeScript types for TV shows
 * Matches backend DTOs from internal/application/tv/dto.go
 */

export interface TVShowSummary {
  id: number
  library_id: number
  title: string
  season_count: number
  episode_count: number
}

export interface TVShowDetailResponse {
  id: number
  library_id: number
  title: string
  season_count: number
  episode_count: number
}

export interface TVEpisodeResponse {
  // Base Media fields
  id: number
  library_id: number
  title: string // filename
  file_path: string
  file_size: number
  duration: number // in seconds
  is_extra: boolean

  // Technical metadata
  width?: number
  height?: number
  video_codec?: string
  audio_codec?: string
  bitrate?: number
  frame_rate?: number
  container_format?: string

  // TV Episode-specific fields
  show_title: string
  season_id: number
  season: number
  episode: number
  episode_title?: string // actual episode name
  tvdb_id?: number
  imdb_id?: string
  air_date?: string
  description?: string

  created_at: string
  updated_at: string
}

export interface ListTVShowsResponse {
  shows: TVShowSummary[]
  total: number
}

export interface ListTVEpisodesResponse {
  episodes: TVEpisodeResponse[]
  total: number
}

export interface SeasonGroup {
  season: number
  season_id?: number
  episode_count: number
  episodes: TVEpisodeResponse[]
}
