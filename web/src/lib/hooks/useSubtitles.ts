/**
 * useSubtitles Hook
 * Manages subtitle track selection. Rendering is handled by SubtitleOverlay component.
 *
 * All subtitle types (text and bitmap/PGS) are rendered as client-side overlays.
 * Text subtitles use WebVTT, bitmap subtitles use WebP images.
 */

import { useEffect, useState, useCallback, useMemo } from 'react'
import type { GithubComMantonxViewraInternalApplicationMediaSubtitleTrackResponse } from '@/lib/api/generated/models'
import { type SubtitleTrack, calculateBitmapIndex, calculateTextIndex } from '@/lib/types/subtitles'

export type { SubtitleTrack }

export interface UseSubtitlesOptions {
  subtitleTracks: GithubComMantonxViewraInternalApplicationMediaSubtitleTrackResponse[]
  preferredLanguage?: string
  preferSDH?: boolean
  preferForced?: boolean
}

export interface UseSubtitlesReturn {
  availableSubtitles: SubtitleTrack[]
  currentSubtitle: SubtitleTrack | null
  setCurrentSubtitle: (trackId: number | null) => void
  /** Relative index among text subtitle tracks (for text streaming API) */
  textStreamIndex: number | undefined
  /** Relative index among bitmap subtitle tracks (for PGS API) */
  bitmapStreamIndex: number | undefined
}

const mapApiToSubtitleTrack = (
  apiTrack: GithubComMantonxViewraInternalApplicationMediaSubtitleTrackResponse
): SubtitleTrack => ({
  id: apiTrack.id ?? 0,
  language: apiTrack.language ?? 'und',
  title: apiTrack.title,
  isDefault: apiTrack.is_default,
  isForced: apiTrack.is_forced,
  isSDH: apiTrack.is_sdh,
  isCommentary: apiTrack.is_commentary,
  isBitmap: apiTrack.is_bitmap,
  sourceType: apiTrack.source_type as 'embedded' | 'external' | undefined,
  streamIndex: apiTrack.stream_index,
})

export const useSubtitles = ({
  subtitleTracks,
  preferredLanguage = 'eng',
  preferSDH = false,
  preferForced = true,
}: UseSubtitlesOptions): UseSubtitlesReturn => {
  const [currentSubtitleId, setCurrentSubtitleId] = useState<number | null>(null)
  const [hasAutoSelected, setHasAutoSelected] = useState(false)

  const availableSubtitles: SubtitleTrack[] = useMemo(
    () => subtitleTracks.map(mapApiToSubtitleTrack),
    [subtitleTracks]
  )

  const currentSubtitle = useMemo(
    () => availableSubtitles.find((t) => t.id === currentSubtitleId) ?? null,
    [availableSubtitles, currentSubtitleId]
  )

  // Calculate relative stream indices for API calls (sorted by streamIndex for consistency)
  const textStreamIndex = useMemo(() => {
    if (!currentSubtitle || currentSubtitle.isBitmap) {return undefined}
    const index = calculateTextIndex(currentSubtitle, availableSubtitles)
    return index >= 0 ? index : undefined
  }, [currentSubtitle, availableSubtitles])

  const bitmapStreamIndex = useMemo(() => {
    if (!currentSubtitle || !currentSubtitle.isBitmap) {return undefined}
    const index = calculateBitmapIndex(currentSubtitle, availableSubtitles)
    return index >= 0 ? index : undefined
  }, [currentSubtitle, availableSubtitles])

  /**
   * Find the best default track from a list of subtitle tracks.
   */
  const findBestTrackFromList = useCallback(
    (tracks: SubtitleTrack[]): SubtitleTrack | null => {
      if (tracks.length === 0) {
        return null
      }

      // 1. Look for forced subtitles in preferred language
      if (preferForced) {
        const forcedTrack = tracks.find((t) => t.language === preferredLanguage && t.isForced)
        if (forcedTrack) {
          return forcedTrack
        }
      }

      // 2. If user prefers subtitles off, return null
      if (preferredLanguage === 'off') {
        return null
      }

      // 3. Look for preferred language (excluding commentary)
      const matchingLang = tracks.filter(
        (t) => t.language === preferredLanguage && !t.isCommentary
      )

      if (matchingLang.length > 0) {
        // Prefer SDH if user wants it
        if (preferSDH) {
          const sdhTrack = matchingLang.find((t) => t.isSDH)
          if (sdhTrack) {
            return sdhTrack
          }
        }
        // Otherwise prefer non-SDH
        const regularTrack = matchingLang.find((t) => !t.isSDH)
        if (regularTrack) {
          return regularTrack
        }
        return matchingLang[0]
      }

      // 4. Fall back to track marked as default
      const defaultTrack = tracks.find((t) => t.isDefault && !t.isCommentary)
      if (defaultTrack) {
        return defaultTrack
      }

      // 5. Fall back to first non-commentary track
      const firstTrack = tracks.find((t) => !t.isCommentary)
      if (firstTrack) {
        return firstTrack
      }

      // 6. Last resort: any track
      return tracks[0] ?? null
    },
    [preferredLanguage, preferSDH, preferForced]
  )

  /**
   * Find the default subtitle track to auto-select.
   * Prioritizes text subtitles, but falls back to bitmap subtitles if no text available.
   */
  const findDefaultTrack = useCallback((): SubtitleTrack | null => {
    const textSubtitles = availableSubtitles.filter((track) => !track.isBitmap)
    const bitmapSubtitles = availableSubtitles.filter((track) => track.isBitmap)

    // First, try to find a text subtitle (preferred - faster loading)
    const textTrack = findBestTrackFromList(textSubtitles)
    if (textTrack) {
      return textTrack
    }

    // No text subtitles available - try bitmap subtitles
    const bitmapTrack = findBestTrackFromList(bitmapSubtitles)
    if (bitmapTrack) {
      return bitmapTrack
    }

    return null
  }, [availableSubtitles, findBestTrackFromList])

  // Auto-select default subtitle when tracks become available
  useEffect(() => {
    if (hasAutoSelected || availableSubtitles.length === 0 || currentSubtitleId !== null) {
      return
    }

    const track = findDefaultTrack()

    if (track) {
      setCurrentSubtitleId(track.id)
    }
    setHasAutoSelected(true)
  }, [availableSubtitles, currentSubtitleId, hasAutoSelected, findDefaultTrack])

  // Reset auto-selection state when tracks change completely (e.g., new media)
  useEffect(() => {
    if (subtitleTracks.length === 0) {
      setHasAutoSelected(false)
      setCurrentSubtitleId(null)
    }
  }, [subtitleTracks.length])

  const setCurrentSubtitle = useCallback((trackId: number | null) => {
    setCurrentSubtitleId(trackId)
  }, [])

  return {
    availableSubtitles,
    currentSubtitle,
    setCurrentSubtitle,
    textStreamIndex: textStreamIndex !== undefined && textStreamIndex >= 0 ? textStreamIndex : undefined,
    bitmapStreamIndex: bitmapStreamIndex !== undefined && bitmapStreamIndex >= 0 ? bitmapStreamIndex : undefined,
  }
}
