/**
 * Shared subtitle types used across the application.
 * SubtitleTrack is a camelCase mapping of the API response for internal use.
 */

import type {
  GithubComMantonxViewraInternalApplicationMediaSubtitleTrackResponse as ApiSubtitleTrack,
  GithubComMantonxViewraInternalInfrastructureSubtitlesPGSFrame as ApiPGSFrame,
} from '@/lib/api/generated/models'
import {
  getGetApiMediaIdSubtitlesPgsIndexStreamUrl,
  getGetApiMediaIdSubtitlesTextIndexStreamUrl,
  getGetApiMediaIdSubtitlesTrackIdUrl,
} from '@/lib/api/generated/subtitles/subtitles'

// Re-export the API type for consumers that need it
export type { ApiSubtitleTrack }

/**
 * PGS (bitmap) subtitle frame with all required fields.
 * The backend always sends all fields (Go JSON marshaling includes zero values),
 * so we use Required<> to reflect the actual runtime behavior.
 */
export type PGSFrame = Required<ApiPGSFrame>

/**
 * Text subtitle cue (parsed from WebVTT).
 */
export interface TextCue {
  start: number // seconds
  end: number // seconds
  text: string
}

/**
 * Time window parameters for subtitle streaming endpoints.
 */
export interface SubtitleTimeWindow {
  start?: number // milliseconds
  end?: number // milliseconds
}

/**
 * URL builders for subtitle streaming endpoints.
 * These wrap the generated URL builders for cleaner usage.
 */
export const subtitleUrls = {
  /** Get PGS subtitle stream URL with time window */
  pgsStream: (mediaId: number, bitmapIndex: number, window?: SubtitleTimeWindow): string =>
    getGetApiMediaIdSubtitlesPgsIndexStreamUrl(mediaId, bitmapIndex, window),

  /** Get text subtitle stream URL with time window */
  textStream: (mediaId: number, textIndex: number, window?: SubtitleTimeWindow): string =>
    getGetApiMediaIdSubtitlesTextIndexStreamUrl(mediaId, textIndex, window),

  /** Get subtitle by track ID (full WebVTT conversion) */
  byTrackId: (mediaId: number, trackId: number): string =>
    getGetApiMediaIdSubtitlesTrackIdUrl(mediaId, trackId),
}

/**
 * Internal subtitle track type with camelCase properties.
 * Mapped from API response via mapApiToSubtitleTrack in useSubtitles hook.
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
  streamIndex?: number
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
