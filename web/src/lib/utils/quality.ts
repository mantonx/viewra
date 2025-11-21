import type { GithubComViewraViewraInternalApplicationMediaMediaResponse as Media } from '../api/generated/models';

/**
 * Quality levels supported by the transcoding system
 */
export type QualityLevel = '360p' | '720p' | '1080p' | '4k';

/**
 * Selects the best quality level based on media resolution and screen size
 * @param media - The media item to select quality for
 * @returns The recommended quality level
 */
export const selectBestQuality = (media: Media | undefined): QualityLevel => {
  if (!media) {
    return '720p'; // Default fallback
  }

  const mediaHeight = media.height || 720;
  const screenHeight = typeof window !== 'undefined' ? window.screen.height : 1080;

  // Use the smaller of media height and screen height to avoid upscaling
  const targetHeight = Math.min(mediaHeight, screenHeight);

  // Select quality based on target height
  if (targetHeight >= 2160) {
    return '4k';
  }
  if (targetHeight >= 1080) {
    return '1080p';
  }
  if (targetHeight >= 720) {
    return '720p';
  }
  return '360p';
}

/**
 * Gets the pixel height for a quality level
 * @param quality - The quality level
 * @returns The pixel height
 */
export const getQualityHeight = (quality: QualityLevel): number => {
  switch (quality) {
    case '4k':
      return 2160;
    case '1080p':
      return 1080;
    case '720p':
      return 720;
    case '360p':
      return 360;
    default:
      return 720;
  }
}

/**
 * Gets a user-friendly label for a quality level
 * @param quality - The quality level
 * @returns Human-readable quality label
 */
export const getQualityLabel = (quality: QualityLevel): string => {
  switch (quality) {
    case '4k':
      return '4K Ultra HD (2160p)';
    case '1080p':
      return 'Full HD (1080p)';
    case '720p':
      return 'HD (720p)';
    case '360p':
      return 'SD (360p)';
    default:
      return quality;
  }
}

/**
 * Validates if a quality string is a valid quality level
 * @param quality - The quality string to validate
 * @returns True if valid, false otherwise
 */
export const isValidQuality = (quality: string): quality is QualityLevel => {
  return ['360p', '720p', '1080p', '4k'].includes(quality);
}

/**
 * Formats a video height into a resolution label (e.g., "4K", "1080p", "720p").
 * Consolidated function used across MediaCard, VideoPlayer, and other components.
 *
 * @param height - Video height in pixels (can be undefined)
 * @param includeAuto - If true and height is 0, returns "Auto" instead of null
 * @returns Resolution label string or null if height is undefined/invalid
 *
 * @example
 * formatResolutionLabel(2160) // "4K"
 * formatResolutionLabel(1080) // "1080p"
 * formatResolutionLabel(0, true) // "Auto"
 * formatResolutionLabel(undefined) // null
 */
export const formatResolutionLabel = (
  height: number | undefined,
  includeAuto = false
): string | null  => {
  if (height === undefined) {
    return null
  }

  if (includeAuto && height === 0) {
    return 'Auto'
  }

  if (height >= 2160) {
    return '4K'
  }

  if (height >= 1080) {
    return '1080p'
  }

  if (height >= 720) {
    return '720p'
  }

  if (height >= 480) {
    return '480p'
  }

  if (height === 0) {
    return null
  }

  return `${height}p`
}
