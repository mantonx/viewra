import type { MusicFile } from './music.types';

export interface ApiResponse {
  status?: number;
  message?: string;
  error?: string;
  music_files?: MusicFile[];
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
