import { useState } from 'react'
import { selectBestQuality } from '../utils/quality'
import { getProgressSeconds } from '../utils'
import { API_BASE_URL } from '@/lib/config'
import { logger } from '@/lib/utils/logger'

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
  playMedia: (mediaId: number, media: any) => Promise<void>
  stopPlayback: () => void
}

export function useMediaPlayback(): UseMediaPlaybackReturn {
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

  const playMedia = async (id: number, media: any) => {
    setMediaId(id)
    setTranscodeState('checking')

    // Fetch progress to determine resume position
    let resumePosition = 0
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

    try {
      // Select best quality based on media and screen
      const quality = selectBestQuality(media)

      // Build HLS manifest URL with start position for resume
      // If resuming, tell the backend to start transcoding from that position
      let manifestUrl = `${API_BASE_URL}/api/media/${id}/hls/${quality}/playlist.m3u8`
      if (resumePosition > 0) {
        manifestUrl += `?start=${resumePosition}`
      }

      // Fetch manifest with manual redirect handling
      const response = await fetch(manifestUrl, {
        redirect: 'manual',
      })

      // Handle Direct Play (302 redirect) - compatible video, no transcoding needed
      if (response.status === 302 || response.type === 'opaqueredirect') {
        const directUrl = `${API_BASE_URL}/api/stream/${id}`
        setStreamUrl(directUrl)
        setTranscodeState('direct')
        setIsPlaying(true) // Show player now that we have the stream URL
        return
      }

      // Handle manifest ready (200 OK) - instant manifest generation
      // Segments will be generated on-demand as player requests them
      if (response.status === 200) {
        setStreamUrl(manifestUrl)
        setTranscodeState('ready')
        setIsPlaying(true) // Show player now that we have the stream URL
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
