/**
 * useSubtitles Hook
 * Manages subtitle track selection. Rendering is handled by SubtitleOverlay component.
 */

import { useEffect, useState, useCallback } from 'react'
import type { GithubComMantonxViewraInternalApplicationMediaSubtitleTrackResponse } from '@/lib/api/generated/models'

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

export interface SubtitleSelection {
  trackId: number | null
  requiresBurnIn: boolean
  streamIndex?: number
}

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

  const availableSubtitles: SubtitleTrack[] = subtitleTracks.map(mapApiToSubtitleTrack)
  const textSubtitles = availableSubtitles.filter((track) => !track.isBitmap)

  const findDefaultTrack = useCallback((): number | null => {
    if (textSubtitles.length === 0) {
      return null
    }

    if (preferForced) {
      const forcedTrack = textSubtitles.find(
        (t) => t.language === preferredLanguage && t.isForced
      )
      if (forcedTrack) {
        return forcedTrack.id
      }
    }

    if (preferredLanguage === 'off') {
      return null
    }

    const matchingLang = textSubtitles.filter(
      (t) => t.language === preferredLanguage && !t.isCommentary
    )

    if (matchingLang.length > 0) {
      if (preferSDH) {
        const sdhTrack = matchingLang.find((t) => t.isSDH)
        if (sdhTrack) {
          return sdhTrack.id
        }
      }
      const regularTrack = matchingLang.find((t) => !t.isSDH)
      if (regularTrack) {
        return regularTrack.id
      }
      return matchingLang[0].id
    }

    const defaultTrack = textSubtitles.find((t) => t.isDefault && !t.isCommentary)
    if (defaultTrack) {
      return defaultTrack.id
    }

    return null
  }, [textSubtitles, preferredLanguage, preferSDH, preferForced])

  useEffect(() => {
    if (availableSubtitles.length > 0 && currentSubtitle === null) {
      const defaultTrack = findDefaultTrack()
      if (defaultTrack !== null) {
        setCurrentSubtitleState(defaultTrack)
      }
    }
  }, [availableSubtitles.length, findDefaultTrack, currentSubtitle])

  const setCurrentSubtitle = useCallback(
    (trackId: number | null, selection?: SubtitleSelection) => {
      setCurrentSubtitleState(trackId)

      if (selection?.requiresBurnIn) {
        setCurrentBurnIn(selection)
        onBurnInSubtitleChange?.(selection)
      } else if (currentBurnIn !== null) {
        setCurrentBurnIn(null)
        onBurnInSubtitleChange?.(null)
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
