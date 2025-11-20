import { useAudioPlayer } from '@/lib/contexts/AudioPlayerContext'
import { useEffect, useRef, useState } from 'react'
import type { AudioPlayerProps } from './AudioPlayer.types'

const formatTime = (seconds: number): string => {
  if (!isFinite(seconds)) {
    return '0:00'
  }
  const mins = Math.floor(seconds / 60)
  const secs = Math.floor(seconds % 60)
  return `${mins}:${secs.toString().padStart(2, '0')}`
}

const AudioPlayer = ({ className = '' }: AudioPlayerProps) => {
  const {
    currentTrack,
    isPlaying,
    volume,
    currentTime,
    duration,
    repeatMode,
    isShuffle,
    togglePlayPause,
    playNext,
    playPrevious,
    seek,
    setVolume,
    toggleRepeat,
    toggleShuffle,
  } = useAudioPlayer()

  const [isSeeking, setIsSeeking] = useState(false)
  const [seekPosition, setSeekPosition] = useState(0)
  const [showVolumeSlider, setShowVolumeSlider] = useState(false)
  const volumeRef = useRef<HTMLDivElement>(null)

  // Close volume slider when clicking outside
  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      if (volumeRef.current && !volumeRef.current.contains(event.target as Node)) {
        setShowVolumeSlider(false)
      }
    }

    if (showVolumeSlider) {
      document.addEventListener('mousedown', handleClickOutside)
      return () => document.removeEventListener('mousedown', handleClickOutside)
    }
  }, [showVolumeSlider])

  // Handle keyboard shortcuts
  useEffect(() => {
    const handleKeyPress = (e: KeyboardEvent) => {
      // Only handle if not typing in an input
      if (e.target instanceof HTMLInputElement || e.target instanceof HTMLTextAreaElement) {
        return
      }

      switch (e.key) {
        case ' ':
          e.preventDefault()
          togglePlayPause()
          break
        case 'ArrowLeft':
          e.preventDefault()
          seek(Math.max(0, currentTime - 5))
          break
        case 'ArrowRight':
          e.preventDefault()
          seek(Math.min(duration, currentTime + 5))
          break
      }
    }

    document.addEventListener('keydown', handleKeyPress)
    return () => document.removeEventListener('keydown', handleKeyPress)
  }, [currentTime, duration, togglePlayPause, seek])

  if (!currentTrack) {
    return null
  }

  const progress = duration > 0 ? (currentTime / duration) * 100 : 0
  const displayTime = isSeeking ? seekPosition : currentTime

  const handleSeekStart = () => {
    setIsSeeking(true)
    setSeekPosition(currentTime)
  }

  const handleSeekChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const newTime = parseFloat(e.target.value)
    setSeekPosition(newTime)
  }

  const handleSeekEnd = () => {
    seek(seekPosition)
    setIsSeeking(false)
  }

  const handleVolumeChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    setVolume(parseFloat(e.target.value))
  }

  const getRepeatIcon = () => {
    switch (repeatMode) {
      case 'off':
        return '↻'
      case 'all':
        return '🔁'
      case 'one':
        return '🔂'
    }
  }

  const getVolumeIcon = () => {
    if (volume === 0) {
      return '🔇'
    }
    if (volume < 0.5) {
      return '🔉'
    }
    return '🔊'
  }

  return (
    <div
      className={`fixed bottom-0 left-0 right-0 bg-gradient-to-r from-gray-900 to-gray-800 text-white shadow-lg z-50 ${className}`}
    >
      <div className="max-w-screen-2xl mx-auto px-6 py-3">
        {/* Progress bar */}
        <div className="mb-3">
          <input
            type="range"
            min="0"
            max={duration || 0}
            value={displayTime}
            onChange={handleSeekChange}
            onMouseDown={handleSeekStart}
            onMouseUp={handleSeekEnd}
            onTouchStart={handleSeekStart}
            onTouchEnd={handleSeekEnd}
            className="w-full h-1 bg-gray-700 rounded-lg appearance-none cursor-pointer slider"
            style={{
              background: `linear-gradient(to right, rgb(244, 63, 94) 0%, rgb(244, 63, 94) ${progress}%, rgb(55, 65, 81) ${progress}%, rgb(55, 65, 81) 100%)`,
            }}
          />
          <div className="flex justify-between text-xs text-gray-400 mt-1">
            <span>{formatTime(displayTime)}</span>
            <span>{formatTime(duration)}</span>
          </div>
        </div>

        <div className="flex items-center justify-between gap-4">
          {/* Track info */}
          <div className="flex-1 min-w-0">
            <h4 className="font-semibold text-sm truncate">{currentTrack.title}</h4>
            <p className="text-xs text-gray-400 truncate">
              {currentTrack.artist || 'Unknown Artist'}
              {currentTrack.album && ` • ${currentTrack.album}`}
            </p>
          </div>

          {/* Playback controls */}
          <div className="flex items-center gap-3">
            {/* Shuffle */}
            <button
              onClick={toggleShuffle}
              className={`p-2 min-h-11 min-w-11 flex items-center justify-center rounded hover:bg-gray-700 transition-colors ${
                isShuffle ? 'text-rose-500' : 'text-gray-400'
              }`}
              title="Shuffle"
              aria-label="Toggle shuffle"
            >
              <span className="text-lg">🔀</span>
            </button>

            {/* Previous */}
            <button
              onClick={playPrevious}
              className="p-2 min-h-11 min-w-11 flex items-center justify-center rounded hover:bg-gray-700 transition-colors"
              title="Previous track"
              aria-label="Previous track"
            >
              <span className="text-xl">⏮</span>
            </button>

            {/* Play/Pause */}
            <button
              onClick={togglePlayPause}
              className="p-3 min-h-[52px] min-w-[52px] flex items-center justify-center bg-rose-600 rounded-full hover:bg-rose-700 transition-colors"
              title={isPlaying ? 'Pause' : 'Play'}
              aria-label={isPlaying ? 'Pause' : 'Play'}
            >
              <span className="text-2xl">{isPlaying ? '⏸' : '▶'}</span>
            </button>

            {/* Next */}
            <button
              onClick={playNext}
              className="p-2 min-h-11 min-w-11 flex items-center justify-center rounded hover:bg-gray-700 transition-colors"
              title="Next track"
              aria-label="Next track"
            >
              <span className="text-xl">⏭</span>
            </button>

            {/* Repeat */}
            <button
              onClick={toggleRepeat}
              className={`p-2 min-h-11 min-w-11 flex items-center justify-center rounded hover:bg-gray-700 transition-colors ${
                repeatMode !== 'off' ? 'text-rose-500' : 'text-gray-400'
              }`}
              title={`Repeat: ${repeatMode}`}
              aria-label="Toggle repeat"
            >
              <span className="text-lg">{getRepeatIcon()}</span>
            </button>
          </div>

          {/* Volume control */}
          <div className="flex items-center gap-2 relative" ref={volumeRef}>
            <button
              onClick={() => setShowVolumeSlider(!showVolumeSlider)}
              className="p-2 min-h-11 min-w-11 flex items-center justify-center rounded hover:bg-gray-700 transition-colors text-gray-400"
              title="Volume"
              aria-label="Toggle volume control"
            >
              <span className="text-lg">{getVolumeIcon()}</span>
            </button>

            {showVolumeSlider && (
              <div className="absolute bottom-full right-0 mb-2 bg-gray-900 rounded-lg shadow-xl p-4">
                <input
                  type="range"
                  min="0"
                  max="1"
                  step="0.01"
                  value={volume}
                  onChange={handleVolumeChange}
                  className="w-24 h-1 bg-gray-700 rounded-lg appearance-none cursor-pointer slider"
                  style={{
                    background: `linear-gradient(to right, rgb(244, 63, 94) 0%, rgb(244, 63, 94) ${
                      volume * 100
                    }%, rgb(55, 65, 81) ${volume * 100}%, rgb(55, 65, 81) 100%)`,
                  }}
                  aria-label="Volume slider"
                />
                <div className="text-xs text-center text-gray-400 mt-1">
                  {Math.round(volume * 100)}%
                </div>
              </div>
            )}
          </div>
        </div>
      </div>

      <style>{`
        .slider::-webkit-slider-thumb {
          appearance: none;
          width: 12px;
          height: 12px;
          border-radius: 50%;
          background: white;
          cursor: pointer;
          box-shadow: 0 0 2px rgba(0, 0, 0, 0.5);
        }

        .slider::-moz-range-thumb {
          width: 12px;
          height: 12px;
          border-radius: 50%;
          background: white;
          cursor: pointer;
          border: none;
          box-shadow: 0 0 2px rgba(0, 0, 0, 0.5);
        }
      `}</style>
    </div>
  )
}

export type { AudioPlayerProps } from './AudioPlayer.types'
export { AudioPlayer }
