/**
 * Images Type Definitions
 * TypeScript interfaces for media image data
 */

export type MediaType = 'movie' | 'tv_show' | 'tv_season' | 'tv_episode' | 'music_album' | 'music_artist'

export type ImageType =
  | 'poster'
  | 'fanart'
  | 'banner'
  | 'clearlogo'
  | 'clearart'
  | 'landscape'
  | 'thumb'
  | 'discart'
  | 'actor'
  | 'extrafanart'
  | 'cover'
  | 'folder'
  | 'logo'

export type SourceType = 'local' | 'tmdb' | 'tvdb' | 'musicbrainz' | 'fanart_tv' | 'other'

export interface Image {
  id: number
  media_id?: number
  media_type: MediaType
  entity_id: number
  image_type: ImageType
  source_type: SourceType
  file_path?: string
  external_url?: string
  local_cache_path?: string
  width?: number
  height?: number
  file_size_bytes?: number
  mime_type?: string
  file_hash?: string
  language?: string
  priority: number
  created_at: string
  updated_at: string
}

export interface ImageResponse {
  image: Image
}

export interface ListImagesResponse {
  images: Image[]
}

export interface BatchImagesResponse {
  media_images: Record<number, Image[]> // map[mediaID][]images
}

/**
 * Image preset sizes available from the backend
 */
export type ImagePreset = 'thumb' | 'medium' | 'large' | 'xlarge'

/**
 * Helper to get the URL to serve an image file
 * @param imageId - The image ID
 * @param preset - Optional preset size (thumb, medium, large, xlarge). Defaults to medium.
 */
export const getImageUrl = (imageId: number, preset?: ImagePreset): string => {
  if (preset) {
    return `/api/images/${imageId}/file?preset=${preset}`
  }
  return `/api/images/${imageId}/file`
}

/**
 * @deprecated Use getImageUrl(imageId, preset) instead. Backend only supports preset sizes, not arbitrary dimensions.
 * Helper to get the URL to serve an image file with resize parameters
 */
export const getImageUrlWithSize = (imageId: number, width?: number, height?: number): string => {
  const params = new URLSearchParams()
  if (width) {
    params.append('width', width.toString())
  }
  if (height) {
    params.append('height', height.toString())
  }

  const queryString = params.toString()
  return `/api/images/${imageId}/file${queryString ? `?${queryString}` : ''}`
}

/**
 * Helper to find the first image of a specific type from a list
 */
export const findImageByType = (images: Image[], type: ImageType): Image | undefined => {
  return images.find((img) => img.image_type === type)
}

/**
 * Helper to get all images of a specific type from a list
 */
export const filterImagesByType = (images: Image[], type: ImageType): Image[] => {
  return images.filter((img) => img.image_type === type)
}

/**
 * Helper to get the primary poster from a list of images
 */
export const getPosterImage = (images: Image[]): Image | undefined => {
  return findImageByType(images, 'poster')
}

/**
 * Helper to get the primary fanart from a list of images
 */
export const getFanartImage = (images: Image[]): Image | undefined => {
  return findImageByType(images, 'fanart')
}

/**
 * Helper to get the logo image from a list of images
 */
export const getLogoImage = (images: Image[]): Image | undefined => {
  return findImageByType(images, 'clearlogo') || findImageByType(images, 'logo')
}

/**
 * Helper to get the episode thumbnail from a list of images
 */
export const getEpisodeThumbnail = (images: Image[]): Image | undefined => {
  return findImageByType(images, 'thumb')
}

/**
 * Helper to get the album cover from a list of images
 */
export const getAlbumCover = (images: Image[]): Image | undefined => {
  return findImageByType(images, 'cover')
}

/**
 * Helper to get the artist image from a list of images
 * Prefers folder image, falls back to fanart
 */
export const getArtistImage = (images: Image[]): Image | undefined => {
  return findImageByType(images, 'folder') || findImageByType(images, 'fanart')
}
