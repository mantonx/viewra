// TV Show related types

export interface TVShow {
  id: string;
  title: string;
  description?: string;
  first_air_date?: string;
  status?: string; // Running, Ended, etc.
  poster?: string;
  backdrop?: string;
  tmdb_id?: string;
  created_at: string;
  updated_at: string;
}

export interface Season {
  id: string;
  tv_show_id: string;
  tv_show?: TVShow;
  season_number: number;
  description?: string;
  poster?: string;
  air_date?: string;
  created_at: string;
  updated_at: string;
}

export interface Episode {
  id: string;
  season_id: string;
  season?: Season;
  title: string;
  episode_number: number;
  air_date?: string;
  description?: string;
  duration?: number; // In seconds
  still_image?: string;
  created_at: string;
  updated_at: string;
}

export interface TVShowFile {
  id: string;
  media_id: string; // Episode ID
  media_type: 'episode';
  library_id: number;
  path: string;
  container?: string;
  video_codec?: string;
  audio_codec?: string;
  resolution?: string;
  duration?: number;
  size_bytes: number;
  episode?: Episode;
}

// Grouped TV Show data
export interface GroupedTVShow {
  show: TVShow;
  seasons: GroupedSeason[];
}

export interface GroupedSeason {
  season: Season;
  episodes: Episode[];
}

// Sorting and filtering types
export type SortField = 'title' | 'first_air_date' | 'status' | 'created_at';
export type SortDirection = 'asc' | 'desc';

// Utility functions
export const buildTVShowPosterUrl = (showId: string) =>
  `/api/v1/assets/entity/tv_show/${showId}/preferred/poster`;

export const buildSeasonPosterUrl = (seasonId: string) =>
  `/api/v1/assets/entity/season/${seasonId}/preferred/poster`;

export const buildEpisodeStillUrl = (episodeId: string) =>
  `/api/v1/assets/entity/episode/${episodeId}/preferred/still`;
