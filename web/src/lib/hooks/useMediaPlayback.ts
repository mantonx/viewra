import { useCallback, useState } from 'react'
import { getProgressSeconds } from '../utils'
import { API_BASE_URL } from '@/lib/config'
import { logger } from '@/lib/utils/logger'
import { authFetch } from '@/lib/utils/authFetch'
import { detectCodecSupportSync, getSupportedCodecsHeader, detectHDRDisplaySync } from '@/lib/capabilities'
import type { GithubComMantonxViewraInternalApplicationMediaMediaResponse as Media } from '@/lib/api/generated/models'

type TranscodeState = 'idle' | 'checking' | 'ready' | 'direct'

/** Quality option returned from backend, filtered by source resolution */
export interface QualityOption {
  id: string              // Quality ID (e.g., "1080p-10m", "4k-25m", "original")
  displayName: string     // Human-readable name (e.g., "1080p (10 Mbps)")
  width: number
  height: number
  bandwidth: number       // Video bitrate in bps
  isSelected: boolean     // True if this is the currently selected quality
  isOriginalQuality: boolean // True if this represents the source quality (remux or same resolution transcode)
}

export interface PlaybackState {
  isPlaying: boolean
  mediaId: number | null
  streamUrl: string | null
  initialPosition: number
  transcodeState: TranscodeState
  /** Available quality options for this media (filtered by source resolution) */
  availableQualities: QualityOption[]
  /** Currently selected quality ID */
  selectedQualityId: string | null
}

interface UseMediaPlaybackReturn {
  playbackState: PlaybackState
  playMedia: (mediaId: number, media: Media, urlTime?: number) => Promise<void>
  stopPlayback: () => void
  /** Change quality - rebuilds URL with ?quality= and reloads stream from current position */
  changeQuality: (qualityId: string, currentPosition: number) => Promise<void>
}

// Supported containers - detected from browser capabilities
const getSupportedContainersHeader = (): string => {
  // All modern browsers support mp4 and webm
  return 'mp4,webm'
}

// Detect codec support once at module load (instant, synchronous)
// Uses MediaSource.isTypeSupported() which is what HLS.js uses
const codecSupport = detectCodecSupportSync()
logger.debug('Detected codec support (sync):', {
  h264: codecSupport.h264.supported,
  h265: codecSupport.h265.supported,
  vp9: codecSupport.vp9.supported,
  av1: codecSupport.av1.supported,
})

// Note: HDR detection is called inside buildManifestUrl() to pick up localStorage overrides

/**
 * Build master manifest URL with all necessary query parameters
 */
const buildManifestUrl = (
  mediaId: number,
  startPosition: number,
  qualityOverride?: string
): string => {
  const params = new URLSearchParams()

  if (startPosition > 0) {
    params.set('start', String(startPosition))
  }

  // Codec support
  params.set('codecs', getSupportedCodecsHeader(codecSupport))
  params.set('containers', getSupportedContainersHeader())

  // Client capabilities for quality recommendation
  const screenWidth = Math.round(window.screen.width * window.devicePixelRatio)
  const screenHeight = Math.round(window.screen.height * window.devicePixelRatio)
  params.set('screenWidth', String(screenWidth))
  params.set('screenHeight', String(screenHeight))

  // Bandwidth estimate from Navigator Connection API
  const connection = (navigator as { connection?: { downlink?: number } }).connection
  if (connection?.downlink && connection.downlink > 0) {
    const bandwidthBps = Math.round(connection.downlink * 1_000_000)
    params.set('bandwidth', String(bandwidthBps))
  }

  // Quality override (user manually selected)
  if (qualityOverride) {
    params.set('quality', qualityOverride)
  }

  // HDR display capability - called here (not at module load) to pick up localStorage overrides
  const hdrDisplay = detectHDRDisplaySync()
  if (hdrDisplay.displaySupportsHDR) {
    params.set('hdrDisplay', 'true')
  }

  const url = `${API_BASE_URL}/api/media/${mediaId}/hls/master.m3u8?${params.toString()}`
  logger.info('[Playback] Built manifest URL:', { url, hdrDisplay: hdrDisplay.displaySupportsHDR })
  return url
}

/**
 * Fetch manifest and parse available qualities from response header
 */
const fetchManifestWithQualities = async (
  manifestUrl: string
): Promise<{ qualities: QualityOption[]; selectedId: string | null } | null> => {
  const response = await authFetch(manifestUrl, { redirect: 'manual' })

  // Direct play redirect - no qualities needed
  if (response.status === 302 || response.type === 'opaqueredirect') {
    return null
  }

  if (response.status !== 200) {
    return null
  }

  // Parse qualities from header
  const qualitiesHeader = response.headers.get('X-Available-Qualities')
  if (!qualitiesHeader) {
    return { qualities: [], selectedId: null }
  }

  try {
    const qualities = JSON.parse(qualitiesHeader) as QualityOption[]
    const selected = qualities.find(q => q.isSelected)
    return { qualities, selectedId: selected?.id ?? null }
  } catch (e) {
    logger.warn('Failed to parse available qualities header:', e)
    return { qualities: [], selectedId: null }
  }
}

export const useMediaPlayback = (): UseMediaPlaybackReturn => {
  const [isPlaying, setIsPlaying] = useState(false)
  const [mediaId, setMediaId] = useState<number | null>(null)
  const [streamUrl, setStreamUrl] = useState<string | null>(null)
  const [initialPosition, setInitialPosition] = useState(0)
  const [transcodeState, setTranscodeState] = useState<TranscodeState>('idle')
  const [availableQualities, setAvailableQualities] = useState<QualityOption[]>([])
  const [selectedQualityId, setSelectedQualityId] = useState<string | null>(null)

  const fallbackToDirectStream = (id: number) => {
    const directUrl = `${API_BASE_URL}/api/stream/${id}`
    setStreamUrl(directUrl)
    setTranscodeState('direct')
    setIsPlaying(true)
  }

  const playMedia = async (id: number, _media: Media, urlTime?: number) => {
    setMediaId(id)
    setIsPlaying(true)
    setTranscodeState('checking')

    // Determine resume position
    let resumePosition = 0

    if (urlTime !== undefined && urlTime > 0) {
      resumePosition = urlTime
    } else {
      const progressPromise = authFetch(`/api/progress/${id}`)
        .then(async (response) => {
          const progressData = response.ok ? await response.json() : null
          const progressSecs = progressData ? getProgressSeconds(progressData) : 0
          const durationSecs = progressData?.duration_seconds ?? 0
          const isNearEnd = durationSecs > 0 && progressSecs >= durationSecs - 1
          return (progressSecs > 0 && !isNearEnd) ? progressSecs : 0
        })
        .catch((error) => {
          logger.error('Error fetching progress:', error)
          return 0
        })

      resumePosition = await progressPromise
    }

    setInitialPosition(resumePosition)

    const manifestUrl = buildManifestUrl(id, resumePosition)

    try {
      const response = await authFetch(manifestUrl, { redirect: 'manual' })

      // Direct Play (302 redirect)
      if (response.status === 302 || response.type === 'opaqueredirect') {
        const directUrl = `${API_BASE_URL}/api/stream/${id}`
        setStreamUrl(directUrl)
        setTranscodeState('direct')
        return
      }

      // Manifest ready (200 OK)
      if (response.status === 200) {
        const qualitiesHeader = response.headers.get('X-Available-Qualities')
        if (qualitiesHeader) {
          try {
            const qualities = JSON.parse(qualitiesHeader) as QualityOption[]
            setAvailableQualities(qualities)
            const selected = qualities.find(q => q.isSelected)
            setSelectedQualityId(selected?.id ?? null)
          } catch (e) {
            logger.warn('Failed to parse available qualities header:', e)
          }
        }

        setStreamUrl(manifestUrl)
        setTranscodeState('ready')
        return
      }

      logger.warn('Unexpected manifest response status:', response.status)
      fallbackToDirectStream(id)
    } catch (error) {
      logger.error('Error requesting manifest:', error)
      fallbackToDirectStream(id)
    }
  }

  /**
   * Change quality - rebuilds URL with ?quality= override and reloads from current position.
   * This triggers a new FFmpeg session starting from the current playback position.
   */
  const changeQuality = useCallback(async (qualityId: string, currentPosition: number) => {
    if (!mediaId) {
      logger.warn('Cannot change quality: no media playing')
      return
    }

    logger.info('[Playback] Changing quality', { qualityId, currentPosition })

    // Build new URL with quality override and current position
    const newManifestUrl = buildManifestUrl(mediaId, currentPosition, qualityId)

    try {
      // Fetch to verify and get updated qualities
      const result = await fetchManifestWithQualities(newManifestUrl)

      if (result) {
        // Update qualities with new selection
        const updatedQualities = result.qualities.map(q => ({
          ...q,
          isSelected: q.id === qualityId,
        }))
        setAvailableQualities(updatedQualities)
        setSelectedQualityId(qualityId)
      }

      // Update stream URL - this will trigger HLS.js to reload
      setInitialPosition(currentPosition)
      setStreamUrl(newManifestUrl)
    } catch (error) {
      logger.error('Error changing quality:', error)
    }
  }, [mediaId])

  const stopPlayback = () => {
    setIsPlaying(false)
    setMediaId(null)
    setStreamUrl(null)
    setTranscodeState('idle')
    setInitialPosition(0)
    setAvailableQualities([])
    setSelectedQualityId(null)
  }

  return {
    playbackState: {
      isPlaying,
      mediaId,
      streamUrl,
      initialPosition,
      transcodeState,
      availableQualities,
      selectedQualityId,
    },
    playMedia,
    stopPlayback,
    changeQuality,
  }
}
