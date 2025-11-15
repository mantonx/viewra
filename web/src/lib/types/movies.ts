/**
 * Movies Type Definitions
 * TypeScript interfaces for movie data
 */

export interface Movie {
  id: number
  library_id: number
  title: string
  year?: number
  file_path: string
  file_size: number
  duration: number
  is_extra: boolean

  // Video metadata
  width?: number
  height?: number
  video_codec?: string
  audio_codec?: string
  bitrate?: number
  frame_rate?: number
  container_format?: string

  // Enhanced metadata (Phase 4 - will be null for now)
  director?: string
  cast?: string
  plot?: string
  poster_url?: string
  backdrop_url?: string
  rating?: number
  genres?: string
  tmdb_id?: number
  imdb_id?: string

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
