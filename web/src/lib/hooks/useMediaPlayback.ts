import { useState, useEffect, useRef } from 'react'
import { selectBestQuality } from '../utils/quality'
import { getProgressSeconds } from '../utils'
import { API_BASE_URL } from '@/lib/config'
import { logger } from '@/lib/utils/logger'

type TranscodeState = 'idle' | 'checking' | 'transcoding' | 'ready' | 'direct'

interface PlaybackState {
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
  const timeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  // Poll manifest URL when transcoding to check if it's ready
  useEffect(() => {
    if (transcodeState !== 'transcoding' || !mediaId || !selectedQuality) {
      return
    }

    logger.debug('⏰ Starting manifest polling every 2 seconds')
    const pollInterval = setInterval(async () => {
      try {
        const manifestUrl = `${API_BASE_URL}/api/media/${mediaId}/hls/${selectedQuality}/playlist.m3u8`
        logger.debug('🔄 Polling manifest:', manifestUrl)

        // Use GET instead of HEAD to properly handle 202 responses
        const response = await fetch(manifestUrl, {
          redirect: 'manual', // Don't follow redirects
        })

        logger.debug('📊 Poll response:', response.status)

        // If we get 200, manifest is ready
        if (response.status === 200) {
          logger.debug('✅ Manifest ready, starting playback')
          setStreamUrl(manifestUrl)
          setTranscodeState('ready')
          if (timeoutRef.current) {
            clearTimeout(timeoutRef.current)
            timeoutRef.current = null
          }
          clearInterval(pollInterval)
        }
        // 202 means still transcoding, keep polling
        else if (response.status === 202) {
          const body = await response.json()
          logger.debug('⏳ Still transcoding:', body)

          // Check if job failed - immediately fall back to direct stream
          if (body.status === 'failed') {
            logger.error('❌ Transcode failed, falling back to direct stream')
            const directUrl = `${API_BASE_URL}/api/stream/${mediaId}`
            setStreamUrl(directUrl)
            setTranscodeState('direct')
            if (timeoutRef.current) {
              clearTimeout(timeoutRef.current)
              timeoutRef.current = null
            }
            clearInterval(pollInterval)
            return
          }
        }
        // 302 would mean direct play (shouldn't happen in polling, but handle it)
        else if (response.status === 302 || response.type === 'opaqueredirect') {
          logger.debug('🔀 Got redirect during polling, switching to direct stream')
          const directUrl = `${API_BASE_URL}/api/stream/${mediaId}`
          setStreamUrl(directUrl)
          setTranscodeState('direct')
          if (timeoutRef.current) {
            clearTimeout(timeoutRef.current)
            timeoutRef.current = null
          }
          clearInterval(pollInterval)
        }
        // Any other status, log it but continue polling
        else {
          logger.warn('⚠️ Unexpected poll status:', response.status)
        }
      } catch (error) {
        logger.error('❌ Error polling manifest:', error)
        // Continue polling on error
      }
    }, 2000)

    return () => clearInterval(pollInterval)
  }, [transcodeState, mediaId, selectedQuality])

  // Cleanup timeout on unmount
  useEffect(() => {
    return () => {
      if (timeoutRef.current) {
        clearTimeout(timeoutRef.current)
      }
    }
  }, [])

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
      logger.debug('📡 Fetching manifest:', manifestUrl)

      // Fetch manifest with manual redirect handling
      const response = await fetch(manifestUrl, {
        redirect: 'manual',
      })
      logger.debug('📨 Manifest response status:', response.status, 'type:', response.type)

      // Handle Direct Play (302 redirect)
      if (response.status === 302 || response.type === 'opaqueredirect') {
        const directUrl = `${API_BASE_URL}/api/stream/${id}`
        setStreamUrl(directUrl)
        setTranscodeState('direct')
        setIsPlaying(true)
        return
      }

      // Handle manifest ready (200 OK) - cached transcode
      if (response.status === 200) {
        setStreamUrl(manifestUrl)
        setTranscodeState('ready')
        setIsPlaying(true)
        return
      }

      // Handle progressive transcoding (202 Accepted)
      if (response.status === 202) {
        // Parse the response body to get job info
        const body = await response.json()
        logger.debug('Got 202 response with job info:', body)

        // If job already failed, immediately use direct stream
        if (body.status === 'failed') {
          logger.error('❌ Transcode job already failed, using direct stream:', body.error)
          fallbackToDirectStream(id)
          return
        }

        // If job is already complete (shouldn't happen with 202, but check anyway)
        if (body.status === 'completed') {
          setStreamUrl(manifestUrl)
          setTranscodeState('ready')
          setIsPlaying(true)
          return
        }

        // Job is in progress - start polling
        setTranscodeState('transcoding')
        setIsPlaying(true)

        // Safety timeout to fallback if transcode doesn't complete (3 seconds)
        // If segments aren't ready within 3 seconds, just use direct stream
        timeoutRef.current = setTimeout(() => {
          logger.warn('⏱️ Transcoding taking too long (>3s), falling back to direct stream')
          const directUrl = `${API_BASE_URL}/api/stream/${id}`
          setStreamUrl(directUrl)
          setTranscodeState('direct')
        }, 3000)
        return
      }

      // Handle errors - fall back to direct stream
      fallbackToDirectStream(id)
    } catch (error) {
      logger.error('Error checking manifest:', error)
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
    if (timeoutRef.current) {
      clearTimeout(timeoutRef.current)
      timeoutRef.current = null
    }
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
