import { useState } from 'react'
import { getProgressSeconds } from '../utils'
import { API_BASE_URL } from '@/lib/config'
import { logger } from '@/lib/utils/logger'
import { authFetch } from '@/lib/utils/authFetch'
import { detectCodecSupportSync, getSupportedCodecsHeader } from '@/lib/capabilities'
import type { GithubComMantonxViewraInternalApplicationMediaMediaResponse as Media } from '@/lib/api/generated/models'

type TranscodeState = 'idle' | 'checking' | 'ready' | 'direct'

export interface PlaybackState {
  isPlaying: boolean
  mediaId: number | null
  streamUrl: string | null
  initialPosition: number
  transcodeState: TranscodeState
}

interface UseMediaPlaybackReturn {
  playbackState: PlaybackState
  playMedia: (mediaId: number, media: Media, urlTime?: number) => Promise<void>
  stopPlayback: () => void
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

export const useMediaPlayback = (): UseMediaPlaybackReturn => {
  const [isPlaying, setIsPlaying] = useState(false)
  const [mediaId, setMediaId] = useState<number | null>(null)
  const [streamUrl, setStreamUrl] = useState<string | null>(null)
  const [initialPosition, setInitialPosition] = useState(0)
  const [transcodeState, setTranscodeState] = useState<TranscodeState>('idle')

  const fallbackToDirectStream = (id: number) => {
    const directUrl = `${API_BASE_URL}/api/stream/${id}`
    setStreamUrl(directUrl)
    setTranscodeState('direct')
    setIsPlaying(true)
  }

  const playMedia = async (id: number, _media: Media, urlTime?: number) => {
    setMediaId(id)
    setIsPlaying(true) // Show player immediately with loading state
    setTranscodeState('checking')

    // Determine resume position: URL time takes precedence, then saved progress
    let resumePosition = 0

    if (urlTime !== undefined && urlTime > 0) {
      // URL time specified - use it directly (from bookmarked link)
      resumePosition = urlTime
      setInitialPosition(resumePosition)
    } else {
      // Fetch progress to determine resume position (usually fast, <50ms from cache)
      try {
        const response = await authFetch(`/api/progress/${id}`)
        const progressData = response.ok ? await response.json() : null
        const progressSecs = progressData ? getProgressSeconds(progressData) : 0
        const durationSecs = progressData?.duration_seconds ?? 0

        // Resume unless user finished watching (within 1 second of end)
        const isNearEnd = durationSecs > 0 && progressSecs >= durationSecs - 1
        resumePosition = (progressSecs > 0 && !isNearEnd) ? progressSecs : 0
        setInitialPosition(resumePosition)
      } catch (error) {
        logger.error('Error fetching progress:', error)
        setInitialPosition(0)
      }
    }

    // Build master manifest URL with query parameters
    // We pass codec support via URL params because custom headers may not survive
    // CORS preflight in all browsers, especially with redirect: 'manual'
    const params = new URLSearchParams()
    if (resumePosition > 0) {
      params.set('start', String(resumePosition))
    }
    // Use module-level codec detection (instant, no async wait)
    params.set('codecs', getSupportedCodecsHeader(codecSupport))
    params.set('containers', getSupportedContainersHeader())

    const manifestUrl = `${API_BASE_URL}/api/media/${id}/hls/master.m3u8?${params.toString()}`

    // Fetch manifest (enables direct play for H.265/VP9/AV1)
    try {
      const response = await authFetch(manifestUrl, {
        redirect: 'manual',
      })

      // Handle Direct Play (302 redirect) - compatible video, no transcoding needed
      if (response.status === 302 || response.type === 'opaqueredirect') {
        const directUrl = `${API_BASE_URL}/api/stream/${id}`
        setStreamUrl(directUrl)
        setTranscodeState('direct')
        return
      }

      // Handle manifest ready (200 OK) - instant manifest generation
      if (response.status === 200) {
        setStreamUrl(manifestUrl)
        setTranscodeState('ready')
        return
      }

      // Any other status - fall back to direct stream
      logger.warn('Unexpected manifest response status:', response.status)
      fallbackToDirectStream(id)
    } catch (error) {
      logger.error('Error requesting manifest:', error)
      fallbackToDirectStream(id)
    }
  }

  const stopPlayback = () => {
    setIsPlaying(false)
    setMediaId(null)
    setStreamUrl(null)
    setTranscodeState('idle')
    setInitialPosition(0)
  }

  return {
    playbackState: {
      isPlaying,
      mediaId,
      streamUrl,
      initialPosition,
      transcodeState,
    },
    playMedia,
    stopPlayback,
  }
}
