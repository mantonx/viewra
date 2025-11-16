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
   * List all albums by a specific artist
   */
  listAlbumsByArtist: (libraryId: number, artist: string) =>
    customInstance<ListAlbumsResponse>({
      url: `/api/music/artists/${encodeURIComponent(artist)}/albums?library_id=${libraryId}`,
      method: 'GET',
    }),

  /**
   * List all tracks in a specific album
   */
  listTracksByAlbum: (libraryId: number, album: string) =>
    customInstance<ListTracksResponse>({
      url: `/api/music/albums/${encodeURIComponent(album)}/tracks?library_id=${libraryId}`,
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
