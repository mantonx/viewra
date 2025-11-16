import { createContext, useContext, useState, useRef, useEffect, type ReactNode } from 'react'
import type { MusicTrackResponse } from '@/lib/types/music'
import { API_BASE_URL } from '@/lib/config'
import { logger } from '@/lib/utils/logger'

interface AudioPlayerContextType {
  // State
  currentTrack: MusicTrackResponse | null
  queue: MusicTrackResponse[]
  currentIndex: number
  isPlaying: boolean
  volume: number
  currentTime: number
  duration: number
  isShuffle: boolean
  repeatMode: 'off' | 'one' | 'all'

  // Actions
  playTrack: (track: MusicTrackResponse) => void
  playQueue: (tracks: MusicTrackResponse[], startIndex?: number) => void
  togglePlayPause: () => void
  playNext: () => void
  playPrevious: () => void
  seek: (time: number) => void
  setVolume: (volume: number) => void
  toggleShuffle: () => void
  toggleRepeat: () => void
  clearQueue: () => void
  removeFromQueue: (index: number) => void
}

const AudioPlayerContext = createContext<AudioPlayerContextType | undefined>(undefined)

export const useAudioPlayer = () => {
  const context = useContext(AudioPlayerContext)
  if (!context) {
    throw new Error('useAudioPlayer must be used within AudioPlayerProvider')
  }
  return context
}

interface AudioPlayerProviderProps {
  children: ReactNode
}

export const AudioPlayerProvider = ({ children }: AudioPlayerProviderProps) => {
  const audioRef = useRef<HTMLAudioElement | null>(null)
  const progressIntervalRef = useRef<ReturnType<typeof setInterval> | null>(null)

  const [currentTrack, setCurrentTrack] = useState<MusicTrackResponse | null>(null)
  const [queue, setQueue] = useState<MusicTrackResponse[]>([])
  const [currentIndex, setCurrentIndex] = useState(0)
  const [isPlaying, setIsPlaying] = useState(false)
  const [volume, setVolumeState] = useState(0.8)
  const [currentTime, setCurrentTime] = useState(0)
  const [duration, setDuration] = useState(0)
  const [isShuffle, setIsShuffle] = useState(false)
  const [repeatMode, setRepeatMode] = useState<'off' | 'one' | 'all'>('off')

  // Initialize audio element
  useEffect(() => {
    if (!audioRef.current) {
      audioRef.current = new Audio()
      audioRef.current.volume = volume

      // Event listeners
      audioRef.current.addEventListener('loadedmetadata', () => {
        setDuration(audioRef.current?.duration || 0)
      })

      audioRef.current.addEventListener('timeupdate', () => {
        setCurrentTime(audioRef.current?.currentTime || 0)
      })

      audioRef.current.addEventListener('ended', handleTrackEnded)

      audioRef.current.addEventListener('error', (e) => {
        logger.error('Audio playback error:', e)
        setIsPlaying(false)
      })
    }

    return () => {
      if (audioRef.current) {
        audioRef.current.pause()
        audioRef.current.src = ''
      }
      if (progressIntervalRef.current) {
        clearInterval(progressIntervalRef.current)
      }
    }
  }, [])

  // Update volume when changed
  useEffect(() => {
    if (audioRef.current) {
      audioRef.current.volume = volume
    }
  }, [volume])

  // Report progress to backend every 5 seconds
  useEffect(() => {
    if (isPlaying && currentTrack) {
      progressIntervalRef.current = setInterval(() => {
        reportProgress()
      }, 5000)
    } else {
      if (progressIntervalRef.current) {
        clearInterval(progressIntervalRef.current)
        progressIntervalRef.current = null
      }
    }

    return () => {
      if (progressIntervalRef.current) {
        clearInterval(progressIntervalRef.current)
      }
    }
  }, [isPlaying, currentTrack, currentTime])

  const reportProgress = async () => {
    if (!currentTrack || !audioRef.current) return

    try {
      const position = Math.floor(audioRef.current.currentTime)
      await fetch(`${API_BASE_URL}/api/progress/${currentTrack.id}`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          position_seconds: position,
          duration_seconds: currentTrack.duration,
        }),
      })
    } catch (error) {
      logger.error('Failed to report progress:', error)
    }
  }

  const handleTrackEnded = () => {
    // Report final progress
    reportProgress()

    // Handle repeat modes
    if (repeatMode === 'one') {
      // Replay current track
      if (audioRef.current) {
        audioRef.current.currentTime = 0
        audioRef.current.play()
      }
    } else if (repeatMode === 'all' && currentIndex === queue.length - 1) {
      // Loop back to first track
      playQueueAtIndex(0)
    } else {
      // Play next track
      playNext()
    }
  }

  const playTrack = (track: MusicTrackResponse) => {
    setCurrentTrack(track)
    setQueue([track])
    setCurrentIndex(0)
    loadAndPlay(track)
  }

  const playQueue = (tracks: MusicTrackResponse[], startIndex: number = 0) => {
    if (tracks.length === 0) return
    setQueue(tracks)
    setCurrentIndex(startIndex)
    setCurrentTrack(tracks[startIndex])
    loadAndPlay(tracks[startIndex])
  }

  const playQueueAtIndex = (index: number) => {
    if (index < 0 || index >= queue.length) return
    setCurrentIndex(index)
    setCurrentTrack(queue[index])
    loadAndPlay(queue[index])
  }

  const loadAndPlay = async (track: MusicTrackResponse) => {
    if (!audioRef.current) return

    const streamUrl = `${API_BASE_URL}/api/stream/${track.id}`
    audioRef.current.src = streamUrl

    try {
      await audioRef.current.play()
      setIsPlaying(true)
    } catch (error) {
      logger.error('Failed to play track:', error)
      setIsPlaying(false)
    }
  }

  const togglePlayPause = () => {
    if (!audioRef.current) return

    if (isPlaying) {
      audioRef.current.pause()
      setIsPlaying(false)
      // Report progress on pause
      reportProgress()
    } else {
      audioRef.current.play().catch((error) => {
        logger.error('Failed to play:', error)
      })
      setIsPlaying(true)
    }
  }

  const playNext = () => {
    if (queue.length === 0) return

    let nextIndex = currentIndex + 1
    if (nextIndex >= queue.length) {
      if (repeatMode === 'all') {
        nextIndex = 0
      } else {
        setIsPlaying(false)
        return
      }
    }

    playQueueAtIndex(nextIndex)
  }

  const playPrevious = () => {
    if (queue.length === 0) return

    // If more than 3 seconds into track, restart it
    if (audioRef.current && audioRef.current.currentTime > 3) {
      audioRef.current.currentTime = 0
      return
    }

    let prevIndex = currentIndex - 1
    if (prevIndex < 0) {
      if (repeatMode === 'all') {
        prevIndex = queue.length - 1
      } else {
        return
      }
    }

    playQueueAtIndex(prevIndex)
  }

  const seek = (time: number) => {
    if (audioRef.current) {
      audioRef.current.currentTime = time
      setCurrentTime(time)
    }
  }

  const setVolume = (newVolume: number) => {
    const clampedVolume = Math.max(0, Math.min(1, newVolume))
    setVolumeState(clampedVolume)
  }

  const toggleShuffle = () => {
    setIsShuffle(!isShuffle)
    // TODO: Implement shuffle queue reordering
  }

  const toggleRepeat = () => {
    const modes: Array<'off' | 'one' | 'all'> = ['off', 'all', 'one']
    const currentModeIndex = modes.indexOf(repeatMode)
    const nextMode = modes[(currentModeIndex + 1) % modes.length]
    setRepeatMode(nextMode)
  }

  const clearQueue = () => {
    if (audioRef.current) {
      audioRef.current.pause()
      audioRef.current.src = ''
    }
    setQueue([])
    setCurrentTrack(null)
    setCurrentIndex(0)
    setIsPlaying(false)
    setCurrentTime(0)
    setDuration(0)
  }

  const removeFromQueue = (index: number) => {
    const newQueue = queue.filter((_, i) => i !== index)
    setQueue(newQueue)

    if (index === currentIndex) {
      // Removed currently playing track
      if (newQueue.length === 0) {
        clearQueue()
      } else {
        const newIndex = Math.min(currentIndex, newQueue.length - 1)
        playQueueAtIndex(newIndex)
      }
    } else if (index < currentIndex) {
      // Adjust current index if track before current was removed
      setCurrentIndex(currentIndex - 1)
    }
  }

  const value: AudioPlayerContextType = {
    currentTrack,
    queue,
    currentIndex,
    isPlaying,
    volume,
    currentTime,
    duration,
    isShuffle,
    repeatMode,
    playTrack,
    playQueue,
    togglePlayPause,
    playNext,
    playPrevious,
    seek,
    setVolume,
    toggleShuffle,
    toggleRepeat,
    clearQueue,
    removeFromQueue,
  }

  return <AudioPlayerContext.Provider value={value}>{children}</AudioPlayerContext.Provider>
}
