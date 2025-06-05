import type { MusicFile } from './music.types';
import type { TVShow } from './tv.types';

export interface ApiResponse {
  status?: number;
  message?: string;
  error?: string;
  music_files?: MusicFile[];
  tv_shows?: TVShow[];
  total?: number;
  data?: unknown;
}

export interface DatabaseStatus {
  status: string;
  connected?: boolean;
}

export interface UsersResponse {
  users: Array<{
    id: number;
    username: string;
    email: string;
    created_at: string;
  }>;
}
