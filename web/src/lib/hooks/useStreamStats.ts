import { useState, useEffect, useCallback, useRef } from 'react'
import type Hls from 'hls.js'
import { useGetApiMediaIdStreamInfo } from '@/lib/api/generated/media/media'
import { detectCodecSupport, getSupportedCodecsHeader } from '@/lib/capabilities'
import type { CodecSupport } from '@/lib/capabilities'
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
  ToneMappingInfo,
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
    networkStats: _networkStats,
    isPlaying,
    playbackMode,
    selectedAudioIndex: _selectedAudioIndex = 0,
    streamOffset = 0,
    enabled = true,
  } = options

  const [stats, setStats] = useState<StreamStats | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [codecSupport, setCodecSupport] = useState<CodecSupport | null>(null)
  const playbackStartTime = useRef<number>(Date.now())

  // Detect codec support on mount
  useEffect(() => {
    detectCodecSupport().then(setCodecSupport).catch(() => {
      // Ignore errors, will use fallback
    })
  }, [])

  // Fetch source stream info from API with codec support headers
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
    request: {
      headers: {
        'X-Supported-Video-Codecs': getSupportedCodecsHeader(codecSupport),
      },
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
  // Uses strategy from API if available, otherwise falls back to passed-in playbackMode
  const buildOutputInfo = useCallback(() => {
    const hlsStats = getHLSStats()

    // Get actual playback mode from API strategy (more accurate than frontend guess)
    const actualMode: PlaybackMode = (apiData?.strategy?.mode as PlaybackMode) ?? playbackMode
    const strategyReason = apiData?.strategy?.reason

    const stream: OutputStreamInfo = {
      mode: actualMode,
      container: actualMode === 'direct' ? 'Native' : 'HLS',
      bitrate: hlsStats?.currentLevelBitrate,
    }

    // Determine video/audio output based on actual strategy
    // - direct: No processing
    // - remux: Video copied, audio copied
    // - remux_audio: Video copied, audio transcoded to AAC
    // - remux_hevc: HEVC video copied with bitstream filter, audio transcoded
    // - transcode: Full video + audio transcode

    const needsVideoTranscode = actualMode === 'transcode'
    const needsAudioTranscode = actualMode === 'transcode' || actualMode === 'remux_audio' || actualMode === 'remux_hevc'
    const isVideoCopied = actualMode === 'remux' || actualMode === 'remux_audio' || actualMode === 'remux_hevc'

    // Build tone mapping info from API response
    let toneMapping: ToneMappingInfo | undefined
    if (apiData?.tone_mapping) {
      toneMapping = {
        enabled: apiData.tone_mapping.enabled ?? false,
        algorithm: apiData.tone_mapping.algorithm,
        backend: apiData.tone_mapping.backend,
      }
    }

    return {
      stream,
      video: needsVideoTranscode
        ? {
            codec: 'H.264',
            bitrate: hlsStats?.currentLevelBitrate,
            height: hlsStats?.currentLevelHeight,
            reason: strategyReason ?? 'Transcoding for compatibility',
          }
        : isVideoCopied
          ? {
              codec: apiData?.video?.codec ?? 'Copy',
              bitrate: apiData?.video?.bitrate,
              height: apiData?.video?.height,
              reason: strategyReason ?? 'Video stream copied (direct play)',
            }
          : undefined,
      audio: needsAudioTranscode
        ? {
            codec: 'AAC',
            bitrate: 192000,
            reason:
              actualMode === 'remux_hevc'
                ? 'Audio transcoded to AAC (video direct)'
                : actualMode === 'remux_audio'
                  ? 'Audio downmixed to stereo AAC'
                  : 'Audio transcoded to AAC',
          }
        : undefined,
      toneMapping,
    }
  }, [playbackMode, apiData, getHLSStats])

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
