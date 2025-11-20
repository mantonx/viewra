/**
 * Music API Client
 * Wrapper around generated API functions
 */

import { getApiMusicArtists, getApiMusicArtistsIdAlbums, getApiMusicAlbumsIdTracks, getApiMusicTracksId, getApiMusicSearch } from './generated/music/music'
import type { GetApiMusicArtistsParams, GetApiMusicSearchParams } from './generated/models'

export const musicApi = {
  /**
   * List all artists for a library with optional pagination
   */
  listArtists: (params: GetApiMusicArtistsParams) => getApiMusicArtists(params),

  /**
   * List all albums by a specific artist (using artist representative track ID)
   */
  listAlbumsByArtistID: (artistId: number) => getApiMusicArtistsIdAlbums(artistId),

  /**
   * List all tracks in a specific album (using album representative track ID)
   */
  listTracksByAlbumID: (albumId: number) => getApiMusicAlbumsIdTracks(albumId),

  /**
   * Get a specific track by ID
   */
  getTrack: (id: number) => getApiMusicTracksId(id),

  /**
   * Search music tracks with optional pagination
   */
  search: (params: GetApiMusicSearchParams) => getApiMusicSearch(params),
}
