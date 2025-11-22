/**
 * Media-related utility functions
 */

import type { GithubComMantonxViewraInternalApplicationMediaMediaResponse as Media } from '@/lib/api/generated/models'

/**
 * Finds a media item by ID from a list of media items.
 * Type-safe alternative to inline `.find()` with safer null handling.
 *
 * @param mediaList - Array of media items to search
 * @param id - Media ID to find (can be null/undefined)
 * @returns The media item if found, undefined otherwise
 *
 * @example
 * const media = findMediaById(allMedia, mediaId)
 * if (media) {
 *   console.log(`Found: ${media.title}`)
 * }
 */
export const findMediaById = (
  mediaList: Media[],
  id: number | undefined | null
): Media | undefined => {
  if (!id) {
    return undefined
  }
  return mediaList.find((m) => m.id === id)
}

/**
 * Gets the appropriate Tailwind badge color class for a video codec.
 * Used for consistent codec badge styling across the application.
 *
 * @param codec - Video codec string (e.g., "hevc", "h264", "av1")
 * @returns Tailwind background color class
 *
 * @example
 * getCodecBadgeColor('hevc') // 'bg-green-600'
 * getCodecBadgeColor('h264') // 'bg-blue-600'
 * getCodecBadgeColor('av1')  // 'bg-purple-600'
 */
export const getCodecBadgeColor = (codec?: string): string => {
  if (!codec) {
    return 'bg-gray-500'
  }
  const c = codec.toLowerCase()
  if (c.includes('hevc') || c.includes('h265')) {
    return 'bg-green-600'
  }
  if (c.includes('h264') || c.includes('avc')) {
    return 'bg-blue-600'
  }
  return 'bg-purple-600'
}
