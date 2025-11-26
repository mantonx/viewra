import { useState, useEffect, useCallback, useRef } from 'react'
import type Hls from 'hls.js'
import { useGetApiMediaIdStreamInfo } from '@/lib/api/generated/media/media'
import type { NetworkStats } from '@/lib/network/NetworkMonitor'
import type {
  StreamStats,
  SourceStreamInfo,
  SourceVideoInfo,
  SourceAudioInfo,
  OutputStreamInfo,
  PlaybackQualityStats,
  HLSStats,
  BufferStats,
  SessionStats,
  PlaybackMode,
} from '@/lib/types/streamStats'

export interface UseStreamStatsOptions {
  mediaId: number | null
  videoRef: React.RefObject<HTMLVideoElement | null>
  hlsRef: React.RefObject<Hls | null>
  networkStats: NetworkStats | null
  isPlaying: boolean
  playbackMode: PlaybackMode
  selectedAudioIndex?: number
  streamOffset?: number
  enabled?: boolean
}

export interface UseStreamStatsReturn {
  stats: StreamStats | null
  isLoading: boolean
  error: string | null
  refresh: () => void
}

/**
 * Hook for collecting and combining stream statistics from multiple sources:
 * - Backend API (source file info via ffprobe)
 * - Browser video element (playback quality, buffer)
 * - HLS.js instance (quality levels, current level)
 * - Network monitor (throughput, stability)
 */
export const useStreamStats = (options: UseStreamStatsOptions): UseStreamStatsReturn => {
  const {
    mediaId,
    videoRef,
    hlsRef,
    networkStats,
    isPlaying,
    playbackMode,
    selectedAudioIndex = 0,
    streamOffset = 0,
    enabled = true,
  } = options

  const [stats, setStats] = useState<StreamStats | null>(null)
  const [error, setError] = useState<string | null>(null)
  const playbackStartTime = useRef<number>(Date.now())

  // Fetch source stream info from API
  const {
    data: apiResponse,
    isLoading,
    refetch,
  } = useGetApiMediaIdStreamInfo(mediaId ?? 0, {
    query: {
      enabled: enabled && mediaId !== null && mediaId > 0,
      staleTime: 5 * 60 * 1000, // Cache for 5 minutes
      refetchOnWindowFocus: false,
    },
  })

  // Extract data from API response
  const apiData = apiResponse?.status === 200 ? apiResponse.data : null

  // Get browser playback quality stats
  const getPlaybackQuality = useCallback((): PlaybackQualityStats => {
    const video = videoRef.current
    if (!video) {
      return {
        droppedFrames: 0,
        totalFrames: 0,
        corruptedFrames: 0,
        droppedFrameRate: 0,
      }
    }

    const quality = video.getVideoPlaybackQuality?.()
    if (!quality) {
      return {
        droppedFrames: 0,
        totalFrames: 0,
        corruptedFrames: 0,
        droppedFrameRate: 0,
      }
    }

    const droppedFrameRate =
      quality.totalVideoFrames > 0
        ? (quality.droppedVideoFrames / quality.totalVideoFrames) * 100
        : 0

    return {
      droppedFrames: quality.droppedVideoFrames,
      totalFrames: quality.totalVideoFrames,
      corruptedFrames: quality.corruptedVideoFrames ?? 0,
      droppedFrameRate,
    }
  }, [videoRef])

  // Get HLS.js stats
  const getHLSStats = useCallback((): HLSStats | undefined => {
    const hls = hlsRef.current
    if (!hls || !hls.levels || hls.levels.length === 0) {
      return undefined
    }

    const currentLevel = hls.currentLevel >= 0 ? hls.currentLevel : 0
    const levelData = hls.levels[currentLevel]

    return {
      currentLevel,
      currentLevelHeight: levelData?.height,
      currentLevelBitrate: levelData?.bitrate,
      availableLevels: hls.levels.map((level) => ({
        height: level.height,
        bitrate: level.bitrate,
      })),
      bandwidthEstimate: hls.bandwidthEstimate,
    }
  }, [hlsRef])

  // Get buffer stats from video element
  const getBufferStats = useCallback((): BufferStats => {
    const video = videoRef.current
    if (!video) {
      return {
        forwardBuffer: 0,
        backBuffer: 0,
        ranges: [],
        hasGaps: false,
      }
    }

    const buffered = video.buffered
    const currentTime = video.currentTime
    const ranges: Array<{ start: number; end: number }> = []

    let forwardBuffer = 0
    let backBuffer = 0
    let hasGaps = false

    for (let i = 0; i < buffered.length; i++) {
      const start = buffered.start(i)
      const end = buffered.end(i)
      ranges.push({ start, end })

      // Check if current time is within this range
      if (currentTime >= start && currentTime <= end) {
        forwardBuffer = end - currentTime
        backBuffer = currentTime - start
      }

      // Check for gaps between ranges
      if (i > 0) {
        const prevEnd = buffered.end(i - 1)
        if (start - prevEnd > 0.1) {
          hasGaps = true
        }
      }
    }

    return {
      forwardBuffer,
      backBuffer,
      ranges,
      hasGaps,
    }
  }, [videoRef])

  // Get session stats
  const getSessionStats = useCallback((): SessionStats => {
    const video = videoRef.current
    return {
      streamOffset,
      playbackRate: video?.playbackRate ?? 1,
      timeSinceStart: Date.now() - playbackStartTime.current,
    }
  }, [videoRef, streamOffset])

  // Build source info from API data
  const buildSourceInfo = useCallback(() => {
    if (!apiData) {
      return null
    }

    const stream: SourceStreamInfo = {
      container: apiData.container ?? 'Unknown',
      fileSize: apiData.file_size ?? 0,
      bitrate: apiData.bitrate ?? 0,
      duration: apiData.duration ?? 0,
    }

    const video: SourceVideoInfo = {
      codec: apiData.video?.codec ?? 'Unknown',
      profile: apiData.video?.profile,
      width: apiData.video?.width ?? 0,
      height: apiData.video?.height ?? 0,
      bitrate: apiData.video?.bitrate,
      frameRate: apiData.video?.frame_rate ?? 0,
      hdrFormat: apiData.video?.hdr_format,
      colorSpace: apiData.video?.color_space,
    }

    const audio: SourceAudioInfo[] = (apiData.audio_tracks ?? []).map((track, index) => ({
      codec: track.codec ?? 'Unknown',
      channels: track.channel_label ?? `${track.channels ?? 2}ch`,
      language: track.language ?? 'Unknown',
      bitrate: track.bitrate,
      isDefault: track.is_default ?? index === 0,
      title: track.title,
    }))

    return { stream, video, audio }
  }, [apiData])

  // Build output info based on playback mode
  const buildOutputInfo = useCallback(() => {
    const hlsStats = getHLSStats()

    const stream: OutputStreamInfo = {
      mode: playbackMode,
      container: playbackMode === 'direct' ? 'Native' : 'HLS',
      bitrate: hlsStats?.currentLevelBitrate,
    }

    // For transcode/remux, we may have transcoded video/audio info
    // This would come from the transcode session info in the future
    // For now, we just indicate the mode

    return {
      stream,
      video:
        playbackMode === 'transcode'
          ? {
              codec: 'H.264', // Usually transcoded to H.264
              bitrate: hlsStats?.currentLevelBitrate,
              height: hlsStats?.currentLevelHeight,
              reason: 'Transcoding for compatibility',
            }
          : undefined,
      audio:
        playbackMode === 'transcode'
          ? {
              codec: 'AAC',
              bitrate: 192000,
              reason: 'Transcoding for compatibility',
            }
          : undefined,
    }
  }, [playbackMode, getHLSStats])

  // Update stats periodically
  useEffect(() => {
    if (!enabled || !apiData) {
      return
    }

    const updateStats = () => {
      const sourceInfo = buildSourceInfo()
      if (!sourceInfo) {
        return
      }

      const newStats: StreamStats = {
        source: sourceInfo,
        output: buildOutputInfo(),
        playback: getPlaybackQuality(),
        hls: getHLSStats(),
        buffer: getBufferStats(),
        session: getSessionStats(),
      }

      setStats(newStats)
    }

    // Initial update
    updateStats()

    // Update every second while playing
    const interval = isPlaying ? setInterval(updateStats, 1000) : null

    return () => {
      if (interval) {
        clearInterval(interval)
      }
    }
  }, [
    enabled,
    apiData,
    isPlaying,
    buildSourceInfo,
    buildOutputInfo,
    getPlaybackQuality,
    getHLSStats,
    getBufferStats,
    getSessionStats,
  ])

  // Handle API errors
  useEffect(() => {
    if (apiResponse && apiResponse.status !== 200) {
      setError('Failed to fetch stream info')
    } else {
      setError(null)
    }
  }, [apiResponse])

  // Reset playback start time when mediaId changes
  useEffect(() => {
    playbackStartTime.current = Date.now()
  }, [mediaId])

  const refresh = useCallback(() => {
    refetch()
  }, [refetch])

  return {
    stats,
    isLoading,
    error,
    refresh,
  }
}

export default useStreamStats
