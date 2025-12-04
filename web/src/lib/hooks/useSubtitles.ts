/**
 * useSubtitles Hook
 * Manages subtitle track selection. Rendering is handled by SubtitleOverlay component.
 * Text subtitles are rendered as overlay, bitmap subtitles use burn-in during transcode.
 */

import { useEffect, useState, useCallback, useMemo } from 'react'
import type { GithubComMantonxViewraInternalApplicationMediaSubtitleTrackResponse } from '@/lib/api/generated/models'
import {
  type SubtitleTrack,
  type SubtitleSelection,
  calculateRelativeStreamIndex,
} from '@/lib/types/subtitles'

export type { SubtitleTrack, SubtitleSelection }

export interface UseSubtitlesOptions {
  subtitleTracks: GithubComMantonxViewraInternalApplicationMediaSubtitleTrackResponse[]
  preferredLanguage?: string
  preferSDH?: boolean
  preferForced?: boolean
  onBurnInSubtitleChange?: (selection: SubtitleSelection | null) => void
}

export interface UseSubtitlesReturn {
  availableSubtitles: SubtitleTrack[]
  currentSubtitle: number | null
  setCurrentSubtitle: (trackId: number | null, selection?: SubtitleSelection) => void
  currentBurnIn: SubtitleSelection | null
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
  onBurnInSubtitleChange,
}: UseSubtitlesOptions): UseSubtitlesReturn => {
  const [currentSubtitle, setCurrentSubtitleState] = useState<number | null>(null)
  const [currentBurnIn, setCurrentBurnIn] = useState<SubtitleSelection | null>(null)
  // Track if we've done initial auto-selection to avoid re-triggering
  const [hasAutoSelected, setHasAutoSelected] = useState(false)

  const availableSubtitles: SubtitleTrack[] = useMemo(
    () => subtitleTracks.map(mapApiToSubtitleTrack),
    [subtitleTracks]
  )

  const textSubtitles = useMemo(
    () => availableSubtitles.filter((track) => !track.isBitmap),
    [availableSubtitles]
  )

  const bitmapSubtitles = useMemo(
    () => availableSubtitles.filter((track) => track.isBitmap),
    [availableSubtitles]
  )

  /**
   * Find the best default track from a list of subtitle tracks.
   * Used for both text and bitmap subtitle selection.
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
      const matchingLang = tracks.filter((t) => t.language === preferredLanguage && !t.isCommentary)

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

      return null
    },
    [preferredLanguage, preferSDH, preferForced]
  )

  /**
   * Find the default subtitle track to auto-select.
   * Prioritizes text subtitles, but falls back to bitmap subtitles if no text available.
   * Returns both the track and whether it requires burn-in.
   */
  const findDefaultTrack = useCallback((): {
    track: SubtitleTrack | null
    requiresBurnIn: boolean
  } => {
    // First, try to find a text subtitle (preferred - no transcoding needed)
    const textTrack = findBestTrackFromList(textSubtitles)
    if (textTrack) {
      return { track: textTrack, requiresBurnIn: false }
    }

    // No text subtitles available - try bitmap subtitles (requires burn-in)
    const bitmapTrack = findBestTrackFromList(bitmapSubtitles)
    if (bitmapTrack) {
      return { track: bitmapTrack, requiresBurnIn: true }
    }

    return { track: null, requiresBurnIn: false }
  }, [textSubtitles, bitmapSubtitles, findBestTrackFromList])

  // Auto-select default subtitle when tracks become available
  useEffect(() => {
    // Only auto-select once, and only if we have tracks and no current selection
    if (hasAutoSelected || availableSubtitles.length === 0 || currentSubtitle !== null) {
      return
    }

    const { track, requiresBurnIn } = findDefaultTrack()

    if (track) {
      setCurrentSubtitleState(track.id)
      setHasAutoSelected(true)

      // If it's a bitmap subtitle, set up burn-in
      if (requiresBurnIn) {
        const streamIndex = calculateRelativeStreamIndex(track, availableSubtitles)
        if (streamIndex >= 0) {
          const selection: SubtitleSelection = {
            trackId: track.id,
            requiresBurnIn: true,
            streamIndex,
          }
          setCurrentBurnIn(selection)
          onBurnInSubtitleChange?.(selection)
        }
      }
    } else {
      // No suitable track found, but mark as attempted
      setHasAutoSelected(true)
    }
  }, [
    availableSubtitles,
    currentSubtitle,
    hasAutoSelected,
    findDefaultTrack,
    onBurnInSubtitleChange,
  ])

  // Reset auto-selection state when tracks change completely (e.g., new media)
  useEffect(() => {
    if (subtitleTracks.length === 0) {
      setHasAutoSelected(false)
      setCurrentSubtitleState(null)
      setCurrentBurnIn(null)
    }
  }, [subtitleTracks.length])

  const setCurrentSubtitle = useCallback(
    (trackId: number | null, selection?: SubtitleSelection) => {
      setCurrentSubtitleState(trackId)

      if (selection?.requiresBurnIn) {
        setCurrentBurnIn(selection)
        onBurnInSubtitleChange?.(selection)
      } else {
        // Text subtitle or off - clear burn-in
        if (currentBurnIn !== null) {
          setCurrentBurnIn(null)
          onBurnInSubtitleChange?.(null)
        }
      }
    },
    [currentBurnIn, onBurnInSubtitleChange]
  )

  return {
    availableSubtitles,
    currentSubtitle,
    setCurrentSubtitle,
    currentBurnIn,
  }
}
