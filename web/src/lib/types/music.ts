/**
 * Music Types
 * TypeScript types matching backend DTOs for music endpoints
 */

export interface ArtistSummary {
  id?: number // Representative media_id (first track)
  name?: string
  album_count?: number
  track_count?: number
}

export interface AlbumSummary {
  id?: number // Album entity ID
  artist_id?: number // Artist entity ID for navigation
  album?: string
  artist?: string
  year?: number
  track_count?: number
}

export interface MusicTrackResponse {
  // Base Media fields
  id: number
  library_id: number
  title: string
  file_path: string
  file_size: number
  duration: number // in seconds
  is_extra: boolean

  // Technical metadata
  width?: number
  height?: number
  video_codec?: string
  audio_codec?: string
  media_bitrate?: number
  frame_rate?: number
  container_format?: string

  // Music Track-specific fields
  album_id?: number // Album entity ID for navigation
  artist_id?: number // Artist entity ID for navigation
  artist?: string
  album?: string
  album_artist?: string
  track_number?: number
  disc_number?: number
  year?: number
  genre?: string
  composer?: string
  publisher?: string
  bitrate?: number // Music-specific bitrate in kbps

  created_at: string
  updated_at: string
}

export interface ListArtistsResponse {
  artists: ArtistSummary[]
  total: number
}

export interface ListAlbumsResponse {
  albums: AlbumSummary[]
  total: number
}

export interface ListTracksResponse {
  tracks: MusicTrackResponse[]
  total: number
}

// UI-specific types for the audio player
export interface AudioQueueItem {
  track: MusicTrackResponse
  index: number
}

export interface AudioPlayerState {
  currentTrack: MusicTrackResponse | null
  queue: MusicTrackResponse[]
  currentIndex: number
  isPlaying: boolean
  volume: number
  currentTime: number
  duration: number
  isShuffle: boolean
  repeatMode: 'off' | 'one' | 'all'
}
