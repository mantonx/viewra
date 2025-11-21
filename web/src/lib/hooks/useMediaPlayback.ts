import { useState, useEffect, useRef } from 'react'
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
  const [selectedQuality, setSelectedQuality] = useState<string | null>(null)

  const fallbackToDirectStream = (id: number) => {
    const directUrl = `${API_BASE_URL}/api/stream/${id}`
    setStreamUrl(directUrl)
    setTranscodeState('direct')
    setIsPlaying(true)
  }

  const playMedia = async (id: number, media: any) => {
    logger.debug('🎬 playMedia called with ID:', id, 'media:', media)
    setMediaId(id)
    setTranscodeState('checking')

    // Fetch progress to determine initial position
    try {
      const response = await fetch(`${API_BASE_URL}/api/progress/${id}`)
      const progressData = response.ok ? await response.json() : null
      const progressSecs = progressData ? getProgressSeconds(progressData) : 0
      const shouldResume = progressSecs > 0 && !progressData?.is_watched
      setInitialPosition(shouldResume ? progressSecs : 0)
      logger.debug('📍 Initial position:', shouldResume ? progressSecs : 0)
    } catch (error) {
      logger.error('Error fetching progress:', error)
      setInitialPosition(0)
    }

    try {
      // Select best quality based on media and screen
      const quality = selectBestQuality(media)
      setSelectedQuality(quality)
      logger.debug('🎞️  Selected quality:', quality)

      // Build HLS manifest URL
      const manifestUrl = `${API_BASE_URL}/api/media/${id}/hls/${quality}/playlist.m3u8`
      logger.debug('📡 Requesting manifest:', manifestUrl)

      // Fetch manifest with manual redirect handling
      const response = await fetch(manifestUrl, {
        redirect: 'manual',
      })
      logger.debug('📨 Manifest response status:', response.status, 'type:', response.type)

      // Handle Direct Play (302 redirect) - compatible video, no transcoding needed
      if (response.status === 302 || response.type === 'opaqueredirect') {
        logger.debug('✅ Direct play available, using direct stream')
        const directUrl = `${API_BASE_URL}/api/stream/${id}`
        setStreamUrl(directUrl)
        setTranscodeState('direct')
        setIsPlaying(true)
        return
      }

      // Handle manifest ready (200 OK) - instant manifest generation
      // Segments will be generated on-demand as player requests them
      if (response.status === 200) {
        logger.debug('✅ Manifest generated, starting playback (segments on-demand)')
        setStreamUrl(manifestUrl)
        setTranscodeState('ready')
        setIsPlaying(true)
        return
      }

      // Any other status - fall back to direct stream
      logger.warn('⚠️ Unexpected response status:', response.status)
      fallbackToDirectStream(id)
    } catch (error) {
      logger.error('❌ Error requesting manifest:', error)
      fallbackToDirectStream(id)
    }
  }

  const stopPlayback = () => {
    setIsPlaying(false)
    setMediaId(null)
    setStreamUrl(null)
    setTranscodeState('idle')
    setSelectedQuality(null)
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
