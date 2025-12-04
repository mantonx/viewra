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

export interface SubtitleSelection {
  trackId: number | null
  requiresBurnIn: boolean
  streamIndex?: number
}

/**
 * Calculate the relative stream index for a subtitle track.
 * FFmpeg's [0:s:N] notation uses 0-based indexing among subtitle streams only.
 * Returns -1 for external subtitles (they don't have stream indices).
 */
export const calculateRelativeStreamIndex = (
  track: SubtitleTrack,
  allTracks: SubtitleTrack[]
): number => {
  // External subtitles don't have stream indices - can't be burned in via FFmpeg stream selector
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
