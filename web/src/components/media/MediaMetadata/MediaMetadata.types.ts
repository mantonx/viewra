import type { GithubComMantonxViewraInternalApplicationProgressWatchProgressResponse } from '@/lib/api/generated/models'

export interface MediaMetadataProps {
  title: string
  year?: number
  duration?: number // in seconds
  genres?: string[]
  plot?: string
  director?: string
  fileSize?: number
  progress?: GithubComMantonxViewraInternalApplicationProgressWatchProgressResponse | null
  links?: {
    imdb?: string
    tmdb?: string
  }
  // For TV shows
  seasonCount?: number
  episodeCount?: number
  // For rating controls
  rating?: {
    entityType: 'movie' | 'tv_show' | 'tv_episode'
    entityId: number
  }
}
