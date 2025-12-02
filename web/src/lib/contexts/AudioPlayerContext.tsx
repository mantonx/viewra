import { createContext, useContext, useState, useRef, useEffect, type ReactNode } from 'react'
import type { MusicTrackResponse } from '@/lib/types/music'
import { API_BASE_URL } from '@/lib/config'
import { logger } from '@/lib/utils/logger'
import { authFetch } from '@/lib/utils/authFetch'

interface AudioPlayerContextType {
  // State
  currentTrack: MusicTrackResponse | null
  queue: MusicTrackResponse[]
  currentIndex: number
  isPlaying: boolean
  isLoading: boolean
  volume: number
  isMuted: boolean
  currentTime: number
  duration: number
  isShuffle: boolean
  repeatMode: 'off' | 'one' | 'all'
  isMinimized: boolean

  // Actions
  playTrack: (track: MusicTrackResponse) => void
  playQueue: (tracks: MusicTrackResponse[], startIndex?: number) => void
  togglePlayPause: () => void
  playNext: () => void
  playPrevious: () => void
  seek: (time: number) => void
  setVolume: (volume: number) => void
  toggleMute: () => void
  toggleShuffle: () => void
  toggleRepeat: () => void
  clearQueue: () => void
  removeFromQueue: (index: number) => void
  setMinimized: (minimized: boolean) => void
  toggleMinimized: () => void
}

const AudioPlayerContext = createContext<AudioPlayerContextType | undefined>(undefined)

// eslint-disable-next-line react-refresh/only-export-components
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
  const volumeBeforeMuteRef = useRef<number>(0.8)
  const handleTrackEndedRef = useRef<() => void>(() => {})

  // Load volume from localStorage or use default
  const getInitialVolume = () => {
    try {
      const savedVolume = localStorage.getItem('audioPlayerVolume')
      return savedVolume ? parseFloat(savedVolume) : 0.8
    } catch {
      return 0.8
    }
  }

  const [currentTrack, setCurrentTrack] = useState<MusicTrackResponse | null>(null)
  const [queue, setQueue] = useState<MusicTrackResponse[]>([])
  const [currentIndex, setCurrentIndex] = useState(0)
  const [isPlaying, setIsPlaying] = useState(false)
  const [isLoading, setIsLoading] = useState(false)
  const [volume, setVolumeState] = useState(getInitialVolume)
  const [isMuted, setIsMuted] = useState(false)
  const [currentTime, setCurrentTime] = useState(0)
  const [duration, setDuration] = useState(0)
  const [isShuffle, setIsShuffle] = useState(false)
  const [repeatMode, setRepeatMode] = useState<'off' | 'one' | 'all'>('off')
  const [isMinimized, setIsMinimized] = useState(false)
  const autoMinimizeTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const lastUserInteractionRef = useRef<number>(Date.now())
  const userExpandedManuallyRef = useRef<boolean>(false) // Track if user manually expanded
  const consecutiveAutoMinimizesRef = useRef<number>(0) // Track auto-minimize patterns

  // Initialize audio element
  useEffect(() => {
    if (!audioRef.current) {
      audioRef.current = new Audio()
      audioRef.current.volume = volume

      // Event listeners
      audioRef.current.addEventListener('loadedmetadata', () => {
        setDuration(audioRef.current?.duration || 0)
        setIsLoading(false)
      })

      audioRef.current.addEventListener('timeupdate', () => {
        setCurrentTime(audioRef.current?.currentTime || 0)
      })

      audioRef.current.addEventListener('waiting', () => {
        setIsLoading(true)
      })

      audioRef.current.addEventListener('canplay', () => {
        setIsLoading(false)
      })

      audioRef.current.addEventListener('ended', () => handleTrackEndedRef.current())

      audioRef.current.addEventListener('error', (e) => {
        // Only log errors if there's an actual source (ignore errors from empty src)
        if (audioRef.current?.src && audioRef.current.src !== '') {
          logger.error('Audio playback error:', e)
        }
        setIsPlaying(false)
        setIsLoading(false)
      })
    }

    return () => {
      if (audioRef.current) {
        audioRef.current.pause()
        // Don't set src to empty string - it causes error events
        audioRef.current.removeAttribute('src')
        audioRef.current.load() // Reset the media element
      }
      if (progressIntervalRef.current) {
        clearInterval(progressIntervalRef.current)
      }
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []) // Only run once on mount - volume changes handled by separate effect

  // Update volume when changed and save to localStorage
  useEffect(() => {
    if (audioRef.current) {
      audioRef.current.volume = isMuted ? 0 : volume
    }
    // Save to localStorage (only save non-muted volume)
    if (!isMuted) {
      try {
        localStorage.setItem('audioPlayerVolume', volume.toString())
      } catch (error) {
        logger.error('Failed to save volume to localStorage:', error)
      }
    }
  }, [volume, isMuted])

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
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [isPlaying, currentTrack, currentTime])

  const reportProgress = async () => {
    if (!currentTrack || !audioRef.current) {return}

    try {
      const position = Math.floor(audioRef.current.currentTime)
      await authFetch(`/api/progress/${currentTrack.id}`, {
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

  // Keep the track ended handler ref updated to avoid stale closures
  useEffect(() => {
    handleTrackEndedRef.current = () => {
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
  })

  const playTrack = (track: MusicTrackResponse) => {
    setCurrentTrack(track)
    setQueue([track])
    setCurrentIndex(0)
    loadAndPlay(track)
  }

  const playQueue = (tracks: MusicTrackResponse[], startIndex: number = 0) => {
    if (tracks.length === 0) {return}
    setQueue(tracks)
    setCurrentIndex(startIndex)
    setCurrentTrack(tracks[startIndex])
    loadAndPlay(tracks[startIndex])
  }

  const playQueueAtIndex = (index: number) => {
    if (index < 0 || index >= queue.length) {return}
    setCurrentIndex(index)
    setCurrentTrack(queue[index])
    loadAndPlay(queue[index])
  }

  const loadAndPlay = async (track: MusicTrackResponse) => {
    if (!audioRef.current) {return}

    setIsLoading(true)
    const streamUrl = `${API_BASE_URL}/api/stream/${track.id}`
    audioRef.current.src = streamUrl

    try {
      await audioRef.current.play()
      setIsPlaying(true)
    } catch (error) {
      logger.error('Failed to play track:', error)
      setIsPlaying(false)
      setIsLoading(false)
    }
  }

  const togglePlayPause = () => {
    if (!audioRef.current) {return}

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
    if (queue.length === 0) {return}

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
    if (queue.length === 0) {return}

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
    // Unmute if volume is changed from 0
    if (isMuted && clampedVolume > 0) {
      setIsMuted(false)
    }
  }

  const toggleMute = () => {
    if (isMuted) {
      // Unmute: restore previous volume
      setIsMuted(false)
    } else {
      // Mute: save current volume and mute
      volumeBeforeMuteRef.current = volume
      setIsMuted(true)
    }
  }

  const toggleShuffle = () => {
    const newShuffleState = !isShuffle
    setIsShuffle(newShuffleState)

    if (newShuffleState && queue.length > 1) {
      // Shuffle the queue while keeping the current track at the front
      const currentTrackInQueue = queue[currentIndex]
      const remainingTracks = queue.filter((_, idx) => idx !== currentIndex)

      // Fisher-Yates shuffle algorithm
      const shuffled = [...remainingTracks]
      for (let i = shuffled.length - 1; i > 0; i--) {
        const j = Math.floor(Math.random() * (i + 1))
        ;[shuffled[i], shuffled[j]] = [shuffled[j], shuffled[i]]
      }

      // Put current track at the beginning
      const newQueue = [currentTrackInQueue, ...shuffled]
      setQueue(newQueue)
      setCurrentIndex(0)
    }
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
      // Don't set src to empty string - it causes error events
      audioRef.current.removeAttribute('src')
      audioRef.current.load() // Reset the media element
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

  const toggleMinimized = () => {
    setIsMinimized((prev) => {
      const newValue = !prev
      // Track if user manually expanded (was minimized, now expanded)
      if (prev && !newValue) {
        userExpandedManuallyRef.current = true
        // Reset consecutive auto-minimizes since user is actively engaging
        consecutiveAutoMinimizesRef.current = 0
      }
      return newValue
    })
    // Reset auto-minimize timer when user interacts
    lastUserInteractionRef.current = Date.now()
  }

  // Smart auto-minimize logic
  useEffect(() => {
    // Base delay: 2 minutes
    const BASE_AUTO_MINIMIZE_DELAY = 2 * 60 * 1000

    // If user manually expanded recently, use longer delay (5 minutes)
    // This respects user intent - they expanded it for a reason
    const getAutoMinimizeDelay = () => {
      if (userExpandedManuallyRef.current) {
        return 5 * 60 * 1000 // 5 minutes if manually expanded
      }
      // After 3 consecutive auto-minimizes without user expanding,
      // the user seems to prefer minimized - use shorter delay (1 minute)
      if (consecutiveAutoMinimizesRef.current >= 3) {
        return 1 * 60 * 1000 // 1 minute
      }
      return BASE_AUTO_MINIMIZE_DELAY
    }

    const checkAndMinimize = () => {
      const timeSinceInteraction = Date.now() - lastUserInteractionRef.current
      const delay = getAutoMinimizeDelay()

      if (isPlaying && !isMinimized && timeSinceInteraction >= delay) {
        setIsMinimized(true)
        consecutiveAutoMinimizesRef.current += 1
        // Reset manual expand flag after auto-minimize
        userExpandedManuallyRef.current = false
      }
    }

    // Clear existing timer
    if (autoMinimizeTimerRef.current) {
      clearTimeout(autoMinimizeTimerRef.current)
      autoMinimizeTimerRef.current = null
    }

    // Only set timer if playing and not minimized
    if (isPlaying && !isMinimized) {
      const delay = getAutoMinimizeDelay()
      autoMinimizeTimerRef.current = setTimeout(checkAndMinimize, delay)
    }

    return () => {
      if (autoMinimizeTimerRef.current) {
        clearTimeout(autoMinimizeTimerRef.current)
      }
    }
  }, [isPlaying, isMinimized])

  // Expand player when a new track starts (but be smart about it)
  useEffect(() => {
    if (currentTrack) {
      // Only auto-expand on new track if:
      // 1. User hasn't established a pattern of preferring minimized (< 3 consecutive auto-minimizes)
      // 2. OR it's a brand new listening session (queue was empty before)
      const shouldExpand = consecutiveAutoMinimizesRef.current < 3
      if (shouldExpand) {
        setIsMinimized(false)
      }
      lastUserInteractionRef.current = Date.now()
    }
  }, [currentTrack?.id])

  const value: AudioPlayerContextType = {
    currentTrack,
    queue,
    currentIndex,
    isPlaying,
    isLoading,
    volume,
    isMuted,
    currentTime,
    duration,
    isShuffle,
    repeatMode,
    isMinimized,
    playTrack,
    playQueue,
    togglePlayPause,
    playNext,
    playPrevious,
    seek,
    setVolume,
    toggleMute,
    toggleShuffle,
    toggleRepeat,
    clearQueue,
    removeFromQueue,
    setMinimized: setIsMinimized,
    toggleMinimized,
  }

  return <AudioPlayerContext.Provider value={value}>{children}</AudioPlayerContext.Provider>
}
