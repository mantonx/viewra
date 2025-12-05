/**
 * Shared subtitle types used across the application.
 */

export interface SubtitleTrack {
  id: number
  language: string
  title?: string
  isDefault?: boolean
  isForced?: boolean
  isSDH?: boolean
  isCommentary?: boolean
  isBitmap?: boolean
  sourceType?: 'embedded' | 'external'
  streamIndex?: number // Absolute stream index in the file (for FFmpeg)
}

/**
 * Calculate the relative stream index for a subtitle track.
 * This is used for API endpoints that expect a 0-based index among subtitle streams only.
 * Returns -1 for external subtitles (they don't have stream indices).
 */
export const calculateRelativeStreamIndex = (
  track: SubtitleTrack,
  allTracks: SubtitleTrack[]
): number => {
  // External subtitles don't have stream indices
  if (track.sourceType === 'external' || track.streamIndex === undefined) {
    return -1
  }

  // Get all embedded tracks with valid stream indices, sorted by absolute stream index
  const embeddedTracks = [...allTracks]
    .filter((t) => t.streamIndex !== undefined && t.sourceType !== 'external')
    .sort((a, b) => (a.streamIndex ?? 0) - (b.streamIndex ?? 0))

  // Find this track's position (0-based relative index)
  return embeddedTracks.findIndex((t) => t.id === track.id)
}

/**
 * Calculate the relative index among bitmap (PGS) subtitle tracks.
 * Returns -1 if the track is not a bitmap track.
 */
export const calculateBitmapIndex = (
  track: SubtitleTrack,
  allTracks: SubtitleTrack[]
): number => {
  if (!track.isBitmap || track.streamIndex === undefined) {
    return -1
  }

  const bitmapTracks = allTracks
    .filter((t) => t.isBitmap && t.streamIndex !== undefined)
    .sort((a, b) => (a.streamIndex ?? 0) - (b.streamIndex ?? 0))

  return bitmapTracks.findIndex((t) => t.id === track.id)
}

/**
 * Calculate the relative index among text (non-bitmap) subtitle tracks.
 * Returns -1 if the track is a bitmap track or external.
 */
export const calculateTextIndex = (
  track: SubtitleTrack,
  allTracks: SubtitleTrack[]
): number => {
  if (track.isBitmap || track.sourceType === 'external' || track.streamIndex === undefined) {
    return -1
  }

  const textTracks = allTracks
    .filter((t) => !t.isBitmap && t.streamIndex !== undefined && t.sourceType !== 'external')
    .sort((a, b) => (a.streamIndex ?? 0) - (b.streamIndex ?? 0))

  return textTracks.findIndex((t) => t.id === track.id)
}
