export interface MediaFile {
  id: string;
  media_id: string;
  media_type: string;
  path: string;
  container?: string;
  video_codec?: string;
  audio_codec?: string;
  resolution?: string;
  bitrate_kbps?: number;
  duration?: number;
  size_bytes: number;
  video_width?: number;
  video_height?: number;
  video_framerate?: string;
  // Legacy fields for backward compatibility
  bitrate?: number;
  width?: number;
  height?: number;
  framerate?: number;
}

export interface Episode {
  id: string;
  title: string;
  episode_number: number;
  air_date?: string;
  description?: string;
  duration?: number;
  still_image?: string;
  season: {
    id: string;
    season_number: number;
    tv_show: {
      id: string;
      title: string;
      description?: string;
      poster?: string;
      backdrop?: string;
      tmdb_id?: string;
    };
  };
  mediaFile?: MediaFile;
}

export interface Movie {
  id: string;
  title: string;
  release_date?: string;
  description?: string;
  duration?: number;
  poster?: string;
  backdrop?: string;
  tmdb_id?: string;
  mediaFile?: MediaFile;
}

export type MediaItem = {
  type: 'episode';
  data: Episode;
} | {
  type: 'movie';
  data: Movie;
};