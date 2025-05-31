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
  title: string;
  year?: number;
  artwork?: string;
  tracks: MusicFile[];
}

export interface GroupedMusicFile {
  artist: string;
  albums: Album[];
}

export type SortField = 'title' | 'artist' | 'album' | 'year' | 'genre';
export type SortDirection = 'asc' | 'desc';

// Legacy interface for backward compatibility
export interface MusicMetadata {
  id: number;
  media_file_id: number;
  title: string;
  album: string;
  artist: string;
  album_artist: string;
  genre: string;
  year: number;
  track: number;
  track_total: number;
  disc: number;
  disc_total: number;
  duration: number;
  bitrate: number;
  sample_rate: number;
  channels: number;
  format: string;
  has_artwork: boolean;
}
