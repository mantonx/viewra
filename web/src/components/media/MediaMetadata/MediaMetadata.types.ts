export interface MediaMetadataProps {
  title: string
  year?: number
  duration?: number // in seconds
  genres?: string[]
  plot?: string
  director?: string
  fileSize?: number
  progress?: any // ProgressData type from existing hooks
  links?: {
    imdb?: string
    tmdb?: string
  }
  // For TV shows
  seasonCount?: number
  episodeCount?: number
}
