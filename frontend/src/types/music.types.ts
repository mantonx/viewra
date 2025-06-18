export interface TrackMetadata {
  id: string;
  title: string;
  album: string;
  artist: string;
  album_artist: string;
  track_number: number;
  duration: number; // in nanoseconds
  lyrics?: string;
}

export interface MusicFile {
  id: string;
  path: string;
  size_bytes: number;
  hash: string;
  library_id: number;
  last_seen: string;
  created_at: string;
  updated_at: string;
  track: TrackMetadata | null; // Updated structure

  // Technical metadata fields from FFmpeg plugin
  container?: string;
  video_codec?: string;
  audio_codec?: string;
  channels?: string;
  sample_rate?: number;
  resolution?: string;
  duration?: number;
  bitrate_kbps?: number;
  language?: string;
  version_name?: string;
  media_id?: string;
  media_type?: string;
  scan_job_id?: number;
}

export interface Album {
  id: string;
  title: string;
  release_date: string;
  total_tracks: number;
  total_discs: number;
  created_at: string;
  updated_at: string;
  artist_id: string;
  tracks: Track[];
  artwork?: string;
  year?: number;
}

export interface SimpleAlbum {
  title: string;
  artwork?: string;
  year?: number;
  tracks: MusicFile[];
}

export interface GroupedMusicFile {
  artist: string;
  albums: SimpleAlbum[];
}

export type SortField = 'title' | 'artist' | 'album' | 'year' | 'genre';
export type SortDirection = 'asc' | 'desc';

// Modern music types for the new entity-based system
export interface Track {
  id: string;
  title: string;
  duration: number;
  track_number: number;
  disc_number: number;
  created_at: string;
  updated_at: string;
  album_id: string;
  artist_id: string;
  media_files: MediaFile[];
}

export interface Artist {
  id: string;
  name: string;
  biography: string;
  formed_date: string;
  disbanded_date: string;
  country: string;
  created_at: string;
  updated_at: string;
  albums: Album[];
  tracks: Track[];
}

export interface MediaFile {
  id: string;
  file_path: string;
  file_name: string;
  file_size: number;
  mime_type: string;
  created_at: string;
  updated_at: string;
  library_id: number;
  // Technical metadata
  duration: number;
  bitrate: number;
  sample_rate: number;
  channels: number;
  format: string;
  has_artwork: boolean;
}
