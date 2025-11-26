import Hls from 'hls.js'
import { useEffect, useRef, useState } from 'react'

// HLS configuration constants
// CRITICAL: Ultra-conservative buffer limits to prevent bufferFullError
const HLS_CONFIG = {
  MAX_BUFFER_LENGTH: 30, // Buffer 30 seconds ahead (5 segments)
  MAX_MAX_BUFFER_LENGTH: 60, // Maximum buffer of 1 minute
  MAX_BUFFER_SIZE: 100 * 1000 * 1000, // 100MB max buffer size
  MAX_BUFFER_HOLE: 2.0, // Maximum gap tolerance (2s for keyframe alignment)
  ENABLE_WORKER: false, // Disable worker - can cause audio issues
  LOW_LATENCY_MODE: false,
  DEBUG: true, // Enable debug logging to diagnose quality drops
} as const

export interface QualityLevel {
  height: number
  bandwidth: number
}

export interface AudioTrack {
  id: number
  name: string
  language: string
}

export interface UseHlsPlayerReturn {
  hlsRef: React.MutableRefObject<Hls | null>
  availableQualities: QualityLevel[]
  currentQuality: number | null
  availableAudioTracks: AudioTrack[]
  currentAudioTrack: number
  handleQualityChange: (level: number) => void
  handleAudioTrackChange: (trackId: number) => void
}

/**
 * Hook for managing HLS.js player lifecycle and quality/audio track selection.
 * Handles initialization, quality detection, track management, and cleanup.
 */
export const useHlsPlayer = (
  videoRef: React.RefObject<HTMLVideoElement>,
  streamUrl: string,
  initialPosition: number,
  isHlsStream: boolean,
  onBuffering: (buffering: boolean) => void,
  onError: (error: string) => void
): UseHlsPlayerReturn => {
  const hlsRef = useRef<Hls | null>(null)
  const [availableQualities, setAvailableQualities] = useState<QualityLevel[]>([])
  const [currentQuality, setCurrentQuality] = useState<number | null>(null)
  const [availableAudioTracks, setAvailableAudioTracks] = useState<AudioTrack[]>([])
  const [currentAudioTrack, setCurrentAudioTrack] = useState<number>(-1)

  // Initialize HLS player for HLS streams
  useEffect(() => {
    const video = videoRef.current
    if (!video) {return}

    // If streamUrl is empty, show buffering indicator and wait
    if (!streamUrl) {
      onBuffering(true)
      return
    }

    // Direct stream (non-HLS) - use native video element
    if (!isHlsStream) {
      video.src = streamUrl
      if (initialPosition > 0) {
        video.currentTime = initialPosition
      }
      video.load()
      return
    }

    // HLS stream - use HLS.js
    if (!Hls.isSupported()) {
      onError('HLS is not supported in this browser')
      return
    }

    // Create HLS instance with ABR enabled
    // HLS.js will automatically switch between qualities based on network conditions
    const hls = new Hls({
      debug: HLS_CONFIG.DEBUG,
      enableWorker: HLS_CONFIG.ENABLE_WORKER,
      lowLatencyMode: HLS_CONFIG.LOW_LATENCY_MODE,
      maxBufferLength: HLS_CONFIG.MAX_BUFFER_LENGTH,
      maxMaxBufferLength: HLS_CONFIG.MAX_MAX_BUFFER_LENGTH,
      maxBufferSize: HLS_CONFIG.MAX_BUFFER_SIZE,
      maxBufferHole: HLS_CONFIG.MAX_BUFFER_HOLE,
    })
    hlsRef.current = hls

    // Load stream
    hls.loadSource(streamUrl)
    hls.attachMedia(video)

    // Wait for manifest to be parsed
    hls.on(Hls.Events.MANIFEST_PARSED, (_event, data) => {
      // Extract quality levels from master playlist
      const qualities = data.levels.map((level, _index) => ({
        height: level.height,
        bandwidth: level.bitrate,
      }))
      setAvailableQualities(qualities)

      // Start with auto ABR - HLS.js will select based on network conditions
      // currentQuality of -1 indicates auto mode
      setCurrentQuality(-1)

      // Extract audio tracks
      if (data.audioTracks && data.audioTracks.length > 0) {
        const tracks = data.audioTracks.map((track, idx) => ({
          id: idx,
          name: track.name || `Track ${idx + 1}`,
          language: track.lang || 'unknown',
        }))
        setAvailableAudioTracks(tracks)
      }

      // Seek to initial position if specified
      if (initialPosition > 0 && video.readyState >= 2) {
        video.currentTime = initialPosition
      }

      // Start playback
      video.play().catch(() => {
        // Autoplay blocked - user will need to click play
      })
    })

    // Handle fragment loaded (for buffering indicator)
    hls.on(Hls.Events.FRAG_LOADED, () => {
      onBuffering(false)
    })

    // Handle errors
    hls.on(Hls.Events.ERROR, (_event, data) => {
      if (data.fatal) {
        switch (data.type) {
          case Hls.ErrorTypes.NETWORK_ERROR:
            hls.startLoad()
            break
          case Hls.ErrorTypes.MEDIA_ERROR:
            hls.recoverMediaError()
            break
          default:
            onError('Fatal HLS error - cannot recover')
            hls.destroy()
            break
        }
      }
    })

    // Track current audio track
    hls.on(Hls.Events.AUDIO_TRACK_SWITCHED, (_event, data) => {
      setCurrentAudioTrack(data.id)
    })

    // Track level switching for debugging quality drops
    hls.on(Hls.Events.LEVEL_SWITCHING, (_event, data) => {
      console.log('[HLS] LEVEL_SWITCHING:', {
        level: data.level,
        height: hls.levels[data.level]?.height,
        currentLevel: hls.currentLevel,
        loadLevel: hls.loadLevel,
        nextLevel: hls.nextLevel,
      })
    })

    hls.on(Hls.Events.LEVEL_SWITCHED, (_event, data) => {
      console.log('[HLS] LEVEL_SWITCHED:', {
        level: data.level,
        height: hls.levels[data.level]?.height,
      })
      // Update our tracked current quality
      setCurrentQuality(data.level)
    })

    // Cleanup on unmount
    return () => {
      if (hlsRef.current) {
        hlsRef.current.destroy()
        hlsRef.current = null
      }
    }
  }, [streamUrl, initialPosition, isHlsStream, videoRef, onBuffering, onError])

  // Handle quality change - supports both auto (-1) and manual (0+) modes
  const handleQualityChange = (level: number) => {
    if (hlsRef.current) {
      // level -1 = auto ABR, level >= 0 = locked to specific quality
      hlsRef.current.currentLevel = level
      setCurrentQuality(level)
    }
  }

  // Handle audio track change
  const handleAudioTrackChange = (trackId: number) => {
    if (hlsRef.current) {
      hlsRef.current.audioTrack = trackId
    }
  }

  return {
    hlsRef,
    availableQualities,
    currentQuality,
    availableAudioTracks,
    currentAudioTrack,
    handleQualityChange,
    handleAudioTrackChange,
  }
}
