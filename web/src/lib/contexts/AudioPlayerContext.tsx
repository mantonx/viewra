import { createContext, useContext, useState, useRef, useEffect, useCallback, type ReactNode } from 'react'
import type { MusicTrackResponse } from '@/lib/types/music'
import { logger } from '@/lib/utils/logger'
import { authFetch } from '@/lib/utils/authFetch'
import { useAudioPlayback } from '@/lib/hooks/useAudioPlayback'
import { useAudioQueue, type RepeatMode } from '@/lib/hooks/useAudioQueue'

// Visibility states for the audio player
type PlayerVisibility = 'expanded' | 'minimized' | 'hidden'

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
  repeatMode: RepeatMode
  isMinimized: boolean
  visibility: PlayerVisibility
  isOnMusicPage: boolean

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
  setVisibility: (visibility: PlayerVisibility) => void
  notifyRouteChange: (pathname: string) => void
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
  // Use extracted hooks for playback and queue management
  const queueHook = useAudioQueue()
  
  const handleTrackEnded = useCallback(() => {
    reportProgress()
    
    if (queueHook.repeatMode === 'one') {
      playback.seek(0)
      playback.audioRef.current?.play()
    } else {
      const nextIdx = queueHook.getNextIndex()
      if (nextIdx !== null) {
        const track = queueHook.goToIndex(nextIdx)
        if (track) {playback.loadAndPlay(track)}
      } else {
        // End of queue
      }
    }
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [queueHook.repeatMode, queueHook.getNextIndex, queueHook.goToIndex])

  const playback = useAudioPlayback({ onTrackEnded: handleTrackEnded })

  // Visibility state
  const [isMinimized, setIsMinimized] = useState(false)
  const [visibility, setVisibilityState] = useState<PlayerVisibility>('expanded')
  const [isOnMusicPage, setIsOnMusicPage] = useState(false)
  
  // Auto-minimize refs
  const autoMinimizeTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const lastUserInteractionRef = useRef<number>(Date.now())
  const userExpandedManuallyRef = useRef<boolean>(false)
  const consecutiveAutoMinimizesRef = useRef<number>(0)
  const wasPlayingBeforeNavRef = useRef<boolean>(false)
  const progressIntervalRef = useRef<ReturnType<typeof setInterval> | null>(null)

  // Report progress to backend
  const reportProgress = useCallback(async () => {
    if (!playback.currentTrack || !playback.audioRef.current) {return}

    try {
      const position = Math.floor(playback.audioRef.current.currentTime)
      await authFetch(`/api/progress/${playback.currentTrack.id}`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          position_seconds: position,
          duration_seconds: playback.currentTrack.duration,
        }),
      })
    } catch (error) {
      logger.error('Failed to report progress:', error)
    }
  }, [playback.currentTrack, playback.audioRef])

  // Report progress every 5 seconds while playing
  useEffect(() => {
    if (playback.isPlaying && playback.currentTrack) {
      progressIntervalRef.current = setInterval(reportProgress, 5000)
    } else {
      if (progressIntervalRef.current) {
        clearInterval(progressIntervalRef.current)
        progressIntervalRef.current = null
      }
    }
    return () => {
      if (progressIntervalRef.current) {clearInterval(progressIntervalRef.current)}
    }
  }, [playback.isPlaying, playback.currentTrack, reportProgress])

  // High-level player actions
  const playTrack = useCallback((track: MusicTrackResponse) => {
    queueHook.setQueueWithTrack([track], 0)
    playback.loadAndPlay(track)
  }, [queueHook, playback])

  const playQueue = useCallback((tracks: MusicTrackResponse[], startIndex = 0) => {
    if (tracks.length === 0) {return}
    queueHook.setQueueWithTrack(tracks, startIndex)
    playback.loadAndPlay(tracks[startIndex])
  }, [queueHook, playback])

  const playNext = useCallback(() => {
    const nextIdx = queueHook.getNextIndex()
    if (nextIdx !== null) {
      const track = queueHook.goToIndex(nextIdx)
      if (track) {playback.loadAndPlay(track)}
    }
  }, [queueHook, playback])

  const playPrevious = useCallback(() => {
    // If more than 3 seconds in, restart current track
    if (playback.audioRef.current && playback.audioRef.current.currentTime > 3) {
      playback.seek(0)
      return
    }
    const prevIdx = queueHook.getPreviousIndex()
    if (prevIdx !== null) {
      const track = queueHook.goToIndex(prevIdx)
      if (track) {playback.loadAndPlay(track)}
    }
  }, [queueHook, playback])

  const clearQueue = useCallback(() => {
    playback.stop()
    queueHook.clearQueue()
  }, [playback, queueHook])

  const removeFromQueue = useCallback((index: number) => {
    if (index === queueHook.currentIndex) {
      if (queueHook.queue.length === 1) {
        clearQueue()
      } else {
        const nextTrack = queueHook.queue[index + 1] || queueHook.queue[index - 1]
        queueHook.removeFromQueue(index)
        if (nextTrack) {playback.loadAndPlay(nextTrack)}
      }
    } else {
      queueHook.removeFromQueue(index)
    }
  }, [queueHook, playback, clearQueue])

  const togglePlayPause = useCallback(() => {
    if (playback.isPlaying) {reportProgress()}
    playback.togglePlayPause()
  }, [playback, reportProgress])

  // Visibility management
  const toggleMinimized = useCallback(() => {
    setIsMinimized((prev) => {
      if (prev) {
        userExpandedManuallyRef.current = true
        consecutiveAutoMinimizesRef.current = 0
      }
      return !prev
    })
    lastUserInteractionRef.current = Date.now()
  }, [])

  const setVisibility = useCallback((newVisibility: PlayerVisibility) => {
    setVisibilityState(newVisibility)
    if (newVisibility === 'minimized') {setIsMinimized(true)}
    else if (newVisibility === 'expanded') {setIsMinimized(false)}
  }, [])

  // Auto-minimize logic
  useEffect(() => {
    const getDelay = () => {
      if (userExpandedManuallyRef.current) {return 5 * 60 * 1000}
      if (consecutiveAutoMinimizesRef.current >= 3) {return 1 * 60 * 1000}
      return 2 * 60 * 1000
    }

    if (autoMinimizeTimerRef.current) {
      clearTimeout(autoMinimizeTimerRef.current)
      autoMinimizeTimerRef.current = null
    }

    if (playback.isPlaying && !isMinimized) {
      const delay = getDelay()
      autoMinimizeTimerRef.current = setTimeout(() => {
        const timeSince = Date.now() - lastUserInteractionRef.current
        if (playback.isPlaying && !isMinimized && timeSince >= delay) {
          setIsMinimized(true)
          consecutiveAutoMinimizesRef.current += 1
          userExpandedManuallyRef.current = false
        }
      }, delay)
    }

    return () => {
      if (autoMinimizeTimerRef.current) {clearTimeout(autoMinimizeTimerRef.current)}
    }
  }, [playback.isPlaying, isMinimized])

  // Expand on new track
  const currentTrackId = queueHook.currentTrack?.id
  useEffect(() => {
    if (currentTrackId && consecutiveAutoMinimizesRef.current < 3) {
      setIsMinimized(false)
      setVisibilityState('expanded')
      lastUserInteractionRef.current = Date.now()
    }
  }, [currentTrackId])

  // Route change handling
  const isMusicPath = (pathname: string) => pathname.startsWith('/music')

  const notifyRouteChange = useCallback((pathname: string) => {
    const onMusicPage = isMusicPath(pathname)
    const wasOnMusicPage = isOnMusicPage
    setIsOnMusicPage(onMusicPage)

    if (!queueHook.currentTrack) {return}

    if (onMusicPage && !wasOnMusicPage) {
      if (playback.isPlaying) {
        setVisibilityState('expanded')
        setIsMinimized(false)
      } else if (visibility === 'hidden') {
        setVisibilityState('minimized')
        setIsMinimized(true)
      }
      return
    }

    if (!onMusicPage && wasOnMusicPage) {
      wasPlayingBeforeNavRef.current = playback.isPlaying
      if (playback.isPlaying) {
        setVisibilityState('minimized')
        setIsMinimized(true)
      } else {
        setVisibilityState('hidden')
      }
      return
    }

    if (!onMusicPage && !playback.isPlaying && visibility !== 'hidden') {
      setTimeout(() => {
        if (!playback.isPlaying && !isMusicPath(pathname)) {
          setVisibilityState('hidden')
        }
      }, 3000)
    }
  }, [isOnMusicPage, queueHook.currentTrack, playback.isPlaying, visibility])

  // Hide when playback stops on non-music page
  useEffect(() => {
    if (!playback.isPlaying && !isOnMusicPage && queueHook.currentTrack && visibility !== 'hidden') {
      const timer = setTimeout(() => {
        if (!playback.isPlaying && !isOnMusicPage) {
          setVisibilityState('hidden')
        }
      }, 5000)
      return () => clearTimeout(timer)
    }
  }, [playback.isPlaying, isOnMusicPage, queueHook.currentTrack, visibility])

  const value: AudioPlayerContextType = {
    currentTrack: queueHook.currentTrack,
    queue: queueHook.queue,
    currentIndex: queueHook.currentIndex,
    isPlaying: playback.isPlaying,
    isLoading: playback.isLoading,
    volume: playback.volume,
    isMuted: playback.isMuted,
    currentTime: playback.currentTime,
    duration: playback.duration,
    isShuffle: queueHook.isShuffle,
    repeatMode: queueHook.repeatMode,
    isMinimized,
    visibility,
    isOnMusicPage,
    playTrack,
    playQueue,
    togglePlayPause,
    playNext,
    playPrevious,
    seek: playback.seek,
    setVolume: playback.setVolume,
    toggleMute: playback.toggleMute,
    toggleShuffle: queueHook.toggleShuffle,
    toggleRepeat: queueHook.toggleRepeat,
    clearQueue,
    removeFromQueue,
    setMinimized: setIsMinimized,
    toggleMinimized,
    setVisibility,
    notifyRouteChange,
  }

  return <AudioPlayerContext.Provider value={value}>{children}</AudioPlayerContext.Provider>
}
