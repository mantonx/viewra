/**
 * Core audio playback logic for the audio player.
 * Handles audio element, play/pause, seek, volume, and track loading.
 */
import { useRef, useState, useEffect, useCallback } from 'react'
import type { MusicTrackResponse } from '@/lib/types/music'
import { logger } from '@/lib/utils/logger'
import { authFetch } from '@/lib/utils/authFetch'

interface UseAudioPlaybackOptions {
  onTrackEnded?: () => void
  onError?: (error: Error) => void
}

export const useAudioPlayback = (options: UseAudioPlaybackOptions = {}) => {
  const audioRef = useRef<HTMLAudioElement | null>(null)
  const currentBlobUrlRef = useRef<string | null>(null)

  const [currentTrack, setCurrentTrack] = useState<MusicTrackResponse | null>(null)
  const [isPlaying, setIsPlaying] = useState(false)
  const [isLoading, setIsLoading] = useState(false)
  const [currentTime, setCurrentTime] = useState(0)
  const [duration, setDuration] = useState(0)

  // Volume state with localStorage persistence
  const getInitialVolume = () => {
    try {
      const saved = localStorage.getItem('audioPlayerVolume')
      return saved ? parseFloat(saved) : 0.8
    } catch {
      return 0.8
    }
  }
  const [volume, setVolumeState] = useState(getInitialVolume)
  const [isMuted, setIsMuted] = useState(false)
  const volumeBeforeMuteRef = useRef(0.8)

  // Keep callback ref updated
  const onTrackEndedRef = useRef(options.onTrackEnded)
  useEffect(() => {
    onTrackEndedRef.current = options.onTrackEnded
  }, [options.onTrackEnded])

  // Initialize audio element
  useEffect(() => {
    if (!audioRef.current) {
      audioRef.current = new Audio()
      audioRef.current.volume = volume

      audioRef.current.addEventListener('loadedmetadata', () => {
        setDuration(audioRef.current?.duration || 0)
        setIsLoading(false)
      })

      audioRef.current.addEventListener('timeupdate', () => {
        setCurrentTime(audioRef.current?.currentTime || 0)
      })

      audioRef.current.addEventListener('waiting', () => setIsLoading(true))
      audioRef.current.addEventListener('canplay', () => setIsLoading(false))
      audioRef.current.addEventListener('ended', () => onTrackEndedRef.current?.())

      audioRef.current.addEventListener('error', (e) => {
        if (audioRef.current?.src && audioRef.current.src !== '') {
          logger.error('Audio playback error:', e)
          options.onError?.(new Error('Audio playback failed'))
        }
        setIsPlaying(false)
        setIsLoading(false)
      })
    }

    return () => {
      if (audioRef.current) {
        audioRef.current.pause()
        audioRef.current.removeAttribute('src')
        audioRef.current.load()
      }
      if (currentBlobUrlRef.current) {
        URL.revokeObjectURL(currentBlobUrlRef.current)
        currentBlobUrlRef.current = null
      }
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  // Sync volume to audio element and localStorage
  useEffect(() => {
    if (audioRef.current) {
      audioRef.current.volume = isMuted ? 0 : volume
    }
    if (!isMuted) {
      try {
        localStorage.setItem('audioPlayerVolume', volume.toString())
      } catch (error) {
        logger.error('Failed to save volume:', error)
      }
    }
  }, [volume, isMuted])

  const loadAndPlay = useCallback(async (track: MusicTrackResponse) => {
    if (!audioRef.current) {return}

    setIsLoading(true)
    setCurrentTrack(track)

    // Revoke previous blob URL
    if (currentBlobUrlRef.current) {
      URL.revokeObjectURL(currentBlobUrlRef.current)
      currentBlobUrlRef.current = null
    }

    try {
      const response = await authFetch(`/api/stream/${track.id}`)
      if (!response.ok) {
        throw new Error(`Failed to fetch audio stream: ${response.status}`)
      }

      const blob = await response.blob()
      const blobUrl = URL.createObjectURL(blob)
      currentBlobUrlRef.current = blobUrl

      audioRef.current.src = blobUrl
      await audioRef.current.play()
      setIsPlaying(true)
    } catch (error) {
      logger.error('Failed to play track:', error)
      setIsPlaying(false)
      setIsLoading(false)
    }
  }, [])

  const togglePlayPause = useCallback(() => {
    if (!audioRef.current) {return}

    if (isPlaying) {
      audioRef.current.pause()
      setIsPlaying(false)
    } else {
      audioRef.current.play().catch((error) => logger.error('Failed to play:', error))
      setIsPlaying(true)
    }
  }, [isPlaying])

  const seek = useCallback((time: number) => {
    if (audioRef.current) {
      audioRef.current.currentTime = time
      setCurrentTime(time)
    }
  }, [])

  const setVolume = useCallback((newVolume: number) => {
    const clamped = Math.max(0, Math.min(1, newVolume))
    setVolumeState(clamped)
    if (isMuted && clamped > 0) {
      setIsMuted(false)
    }
  }, [isMuted])

  const toggleMute = useCallback(() => {
    if (isMuted) {
      setIsMuted(false)
    } else {
      volumeBeforeMuteRef.current = volume
      setIsMuted(true)
    }
  }, [isMuted, volume])

  const stop = useCallback(() => {
    if (audioRef.current) {
      audioRef.current.pause()
      audioRef.current.removeAttribute('src')
      audioRef.current.load()
    }
    if (currentBlobUrlRef.current) {
      URL.revokeObjectURL(currentBlobUrlRef.current)
      currentBlobUrlRef.current = null
    }
    setCurrentTrack(null)
    setIsPlaying(false)
    setCurrentTime(0)
    setDuration(0)
  }, [])

  return {
    currentTrack,
    isPlaying,
    isLoading,
    currentTime,
    duration,
    volume,
    isMuted,
    loadAndPlay,
    togglePlayPause,
    seek,
    setVolume,
    toggleMute,
    stop,
    audioRef,
  }
}
