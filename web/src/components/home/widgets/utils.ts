/**
 * Normalize media type from various backend formats to MediaCard format
 */
export const normalizeMediaType = (
  type: string
): 'movie' | 'tv-show' | 'music-album' | 'music-artist' => {
  switch (type.toLowerCase()) {
    case 'movie':
    case 'movies':
      return 'movie'
    case 'tv':
    case 'tv_show':
    case 'tv-show':
    case 'tvshow':
    case 'tv_episode':
      return 'tv-show'
    case 'music_album':
    case 'music-album':
    case 'album':
      return 'music-album'
    case 'music_artist':
    case 'music-artist':
    case 'artist':
      return 'music-artist'
    default:
      return 'movie'
  }
}
