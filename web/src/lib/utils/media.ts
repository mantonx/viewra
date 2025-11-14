/**
 * Media-related utility functions
 */

import type { GithubComViewraViewraInternalApplicationMediaMediaResponse as Media } from '@/lib/api/generated/models'

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
export function findMediaById(
  mediaList: Media[],
  id: number | undefined | null
): Media | undefined {
  if (!id) {
    return undefined
  }
  return mediaList.find((m) => m.id === id)
}
