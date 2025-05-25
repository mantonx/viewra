import type { MusicFile } from './music.types';

export interface ApiResponse {
  music_files: MusicFile[];
  total: number;
  count: number;
  limit: number;
  offset: number;
}
