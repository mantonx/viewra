/**
 * Media-related utility functions
 */

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
