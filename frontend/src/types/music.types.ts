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
  format: string;
  has_artwork: boolean;
}

export interface MusicFile {
  id: number;
  path: string;
  size: number;
  hash: string;
  library_id: number;
  last_seen: string;
  created_at: string;
  updated_at: string;
  music_metadata: MusicMetadata;
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
