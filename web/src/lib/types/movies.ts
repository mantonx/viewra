/**
 * Movies Type Definitions
 * TypeScript interfaces for movie data
 */

export interface Movie {
  id: number
  library_id: number
  title: string
  file_path: string
  file_size?: number
  duration?: number
  is_extra: boolean

  // Video metadata
  width?: number
  height?: number
  video_codec?: string
  audio_codec?: string
  bitrate?: number
  frame_rate?: number
  container_format?: string

  // Movie-specific metadata
  year?: number
  release_date?: string
  original_title?: string
  sort_title?: string
  runtime_minutes?: number
  imdb_id?: string
  tmdb_id?: number
  director?: string
  cast?: string[]
  genre?: string[]
  plot?: string
  tagline?: string
  content_rating?: string
  maturity_rating?: number
  content_advisories?: string[]
  budget?: number
  revenue?: number
  original_language?: string
  country_of_origin?: string
  awards_summary?: string

  // Timestamps
  created_at: string
  updated_at: string
}

export interface ListMoviesResponse {
  movies: Movie[]
  total: number
}

export interface MovieResponse {
  movie: Movie
}
