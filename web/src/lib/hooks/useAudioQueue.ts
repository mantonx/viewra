/**
 * Queue management for the audio player.
 * Handles queue operations, shuffle, repeat modes.
 */
import { useState, useCallback } from 'react'
import type { MusicTrackResponse } from '@/lib/types/music'

export type RepeatMode = 'off' | 'one' | 'all'

export const useAudioQueue = () => {
  const [queue, setQueue] = useState<MusicTrackResponse[]>([])
  const [currentIndex, setCurrentIndex] = useState(0)
  const [isShuffle, setIsShuffle] = useState(false)
  const [repeatMode, setRepeatMode] = useState<RepeatMode>('off')

  const currentTrack = queue[currentIndex] ?? null

  const setQueueWithTrack = useCallback((tracks: MusicTrackResponse[], startIndex = 0) => {
    setQueue(tracks)
    setCurrentIndex(Math.min(startIndex, tracks.length - 1))
  }, [])

  const goToIndex = useCallback((index: number) => {
    if (index >= 0 && index < queue.length) {
      setCurrentIndex(index)
      return queue[index]
    }
    return null
  }, [queue])

  const getNextIndex = useCallback(() => {
    if (queue.length === 0) return null
    
    let nextIndex = currentIndex + 1
    if (nextIndex >= queue.length) {
      if (repeatMode === 'all') {
        nextIndex = 0
      } else {
        return null
      }
    }
    return nextIndex
  }, [currentIndex, queue.length, repeatMode])

  const getPreviousIndex = useCallback(() => {
    if (queue.length === 0) return null
    
    let prevIndex = currentIndex - 1
    if (prevIndex < 0) {
      if (repeatMode === 'all') {
        prevIndex = queue.length - 1
      } else {
        return null
      }
    }
    return prevIndex
  }, [currentIndex, queue.length, repeatMode])

  const toggleShuffle = useCallback(() => {
    setIsShuffle((prev) => {
      const newShuffleState = !prev
      if (newShuffleState && queue.length > 1) {
        const current = queue[currentIndex]
        const remaining = queue.filter((_, idx) => idx !== currentIndex)
        
        // Fisher-Yates shuffle
        const shuffled = [...remaining]
        for (let i = shuffled.length - 1; i > 0; i--) {
          const j = Math.floor(Math.random() * (i + 1))
          ;[shuffled[i], shuffled[j]] = [shuffled[j], shuffled[i]]
        }
        
        setQueue([current, ...shuffled])
        setCurrentIndex(0)
      }
      return newShuffleState
    })
  }, [queue, currentIndex])

  const toggleRepeat = useCallback(() => {
    const modes: RepeatMode[] = ['off', 'all', 'one']
    setRepeatMode((prev) => {
      const idx = modes.indexOf(prev)
      return modes[(idx + 1) % modes.length]
    })
  }, [])

  const removeFromQueue = useCallback((index: number) => {
    setQueue((prev) => {
      const newQueue = prev.filter((_, i) => i !== index)
      if (index < currentIndex) {
        setCurrentIndex((i) => i - 1)
      } else if (index === currentIndex && newQueue.length > 0) {
        setCurrentIndex(Math.min(currentIndex, newQueue.length - 1))
      }
      return newQueue
    })
  }, [currentIndex])

  const clearQueue = useCallback(() => {
    setQueue([])
    setCurrentIndex(0)
  }, [])

  return {
    queue,
    currentIndex,
    currentTrack,
    isShuffle,
    repeatMode,
    setQueueWithTrack,
    goToIndex,
    getNextIndex,
    getPreviousIndex,
    toggleShuffle,
    toggleRepeat,
    removeFromQueue,
    clearQueue,
  }
}
