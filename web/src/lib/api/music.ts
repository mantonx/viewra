/**
 * Music API Client
 * Direct API calls for music endpoints
 */

import { customInstance } from './mutator'
import type {
  ListArtistsResponse,
  ListAlbumsResponse,
  ListTracksResponse,
  MusicTrackResponse,
} from '@/lib/types/music'

export const musicApi = {
  /**
   * List all artists for a library
   */
  listArtists: (libraryId: number) =>
    customInstance<ListArtistsResponse>({
      url: `/api/music/artists?library_id=${libraryId}`,
      method: 'GET',
    }),

  /**
   * List all albums by a specific artist (using artist representative track ID)
   */
  listAlbumsByArtistID: (artistId: number) =>
    customInstance<ListAlbumsResponse>({
      url: `/api/music/artists/${artistId}/albums`,
      method: 'GET',
    }),

  /**
   * List all tracks in a specific album (using album representative track ID)
   */
  listTracksByAlbumID: (albumId: number) =>
    customInstance<ListTracksResponse>({
      url: `/api/music/albums/${albumId}/tracks`,
      method: 'GET',
    }),

  /**
   * Get a specific track by ID
   */
  getTrack: (id: number) =>
    customInstance<MusicTrackResponse>({
      url: `/api/music/tracks/${id}`,
      method: 'GET',
    }),

  /**
   * Search music tracks
   */
  search: (libraryId: number, query: string) =>
    customInstance<ListTracksResponse>({
      url: `/api/music/search?library_id=${libraryId}&q=${encodeURIComponent(query)}`,
      method: 'GET',
    }),
}
