import { useState, useEffect, useRef } from 'react'
import { getProgressSeconds } from '../utils'
import { API_BASE_URL } from '@/lib/config'
import { logger } from '@/lib/utils/logger'
import { detectCodecSupport } from '@/lib/capabilities'
import type { CodecSupport } from '@/lib/capabilities'
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

// Helper to convert CodecSupport to header values
const getSupportedCodecsHeader = (codecSupport: CodecSupport | null): string => {
  if (!codecSupport) {
    return 'h264' // Safe fallback
  }
  const supported: string[] = []
  if (codecSupport.h264.supported) {
    supported.push('h264')
  }
  if (codecSupport.h265.supported) {
    supported.push('h265')
  }
  if (codecSupport.vp9.supported) {
    supported.push('vp9')
  }
  if (codecSupport.av1.supported) {
    supported.push('av1')
  }
  return supported.join(',')
}

// Supported containers - detected from browser capabilities
const getSupportedContainersHeader = (): string => {
  // All modern browsers support mp4 and webm
  return 'mp4,webm'
}

export const useMediaPlayback = (): UseMediaPlaybackReturn => {
  const [isPlaying, setIsPlaying] = useState(false)
  const [mediaId, setMediaId] = useState<number | null>(null)
  const [streamUrl, setStreamUrl] = useState<string | null>(null)
  const [initialPosition, setInitialPosition] = useState(0)
  const [transcodeState, setTranscodeState] = useState<TranscodeState>('idle')

  // Cache codec support detection (runs once on mount)
  const codecSupportRef = useRef<CodecSupport | null>(null)
  const codecDetectionPromiseRef = useRef<Promise<CodecSupport> | null>(null)

  useEffect(() => {
    // Start codec detection on mount (non-blocking)
    if (!codecDetectionPromiseRef.current) {
      codecDetectionPromiseRef.current = detectCodecSupport().then(support => {
        codecSupportRef.current = support
        logger.debug('Detected codec support:', {
          h264: support.h264.supported,
          h265: support.h265.supported,
          vp9: support.vp9.supported,
          av1: support.av1.supported,
        })
        return support
      }).catch(err => {
        logger.warn('Failed to detect codec support:', err)
        // Return minimal fallback
        return {
          h264: { codec: 'h264' as const, supported: true, hardwareAccelerated: false, powerEfficient: false, smooth: true, maxWidth: 1920, maxHeight: 1080, maxFps: 30 },
          h265: { codec: 'h265' as const, supported: false, hardwareAccelerated: false, powerEfficient: false, smooth: false, maxWidth: 0, maxHeight: 0, maxFps: 0 },
          vp9: { codec: 'vp9' as const, supported: false, hardwareAccelerated: false, powerEfficient: false, smooth: false, maxWidth: 0, maxHeight: 0, maxFps: 0 },
          av1: { codec: 'av1' as const, supported: false, hardwareAccelerated: false, powerEfficient: false, smooth: false, maxWidth: 0, maxHeight: 0, maxFps: 0 },
          preferredOrder: ['h264' as const],
        }
      })
    }
  }, [])

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
        const response = await fetch(`${API_BASE_URL}/api/progress/${id}`)
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

    // Build master manifest URL with resume position
    // Master playlist provides all available quality levels for adaptive bitrate streaming
    // HLS.js will parse all variants and enable quality switching in the player
    let manifestUrl = `${API_BASE_URL}/api/media/${id}/hls/master.m3u8`
    if (resumePosition > 0) {
      manifestUrl += `?start=${resumePosition}`
    }

    // Wait for codec detection if still in progress (should be fast, <100ms typically)
    let codecSupport = codecSupportRef.current
    if (!codecSupport && codecDetectionPromiseRef.current) {
      try {
        codecSupport = await codecDetectionPromiseRef.current
      } catch {
        // Use fallback - will be handled by getSupportedCodecsHeader
      }
    }

    // Fetch manifest with codec capability headers (enables direct play for H.265/VP9/AV1)
    try {
      const response = await fetch(manifestUrl, {
        redirect: 'manual',
        headers: {
          'X-Supported-Video-Codecs': getSupportedCodecsHeader(codecSupport),
          'X-Supported-Containers': getSupportedContainersHeader(),
        },
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
