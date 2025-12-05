/**
 * VideoPlayer Component
 * Full-featured video player with HLS streaming, quality selection,
 * keyboard controls, and progress tracking.
 */

import { Button } from '@/components/ui'
import { useProgressUpdater } from '@/lib/hooks/useProgress'
import { useQualityRecommendation } from '@/lib/hooks/useQualityRecommendation'
import { useAutoQuality } from '@/lib/hooks/useAutoQuality'
import { usePlaybackAnalytics } from '@/lib/hooks/usePlaybackAnalytics'
import { useStreamStats } from '@/lib/hooks/useStreamStats'
import { useHlsPlayer } from '@/lib/hooks/useHlsPlayer'
import { useVideoEvents } from '@/lib/hooks/useVideoEvents'
import { useVideoKeyboard } from '@/lib/hooks/useVideoKeyboard'
import { useVideoControls } from '@/lib/hooks/useVideoControls'
import { useSubtitles } from '@/lib/hooks/useSubtitles'
import { useCallback, useEffect, useRef, useState } from 'react'
import { VideoControls } from './VideoControls'
import { StatsPanel } from './StatsPanel'
import { SubtitleOverlay } from './SubtitleOverlay'
import type { VideoPlayerProps } from './VideoPlayer.types'
import type { QualityRecommendationResponse } from '@/lib/api/adaptive'
import { useGetApiMediaIdTracks } from '@/lib/api/generated/media/media'

export const VideoPlayer = ({
  mediaId,
  streamUrl,
  initialPosition = 0,
  duration = 0,
  metadata,
  onClose,
  onTimeUpdate,
}: VideoPlayerProps) => {
  const videoRef = useRef<HTMLVideoElement>(null)
  const videoContainerRef = useRef<HTMLDivElement>(null)
  const isSeekingRef = useRef<boolean>(false)

  // Detect if this is an HLS stream
  const isHlsStream = streamUrl.includes('.m3u8')

  // Core playback state
  const [isPlaying, setIsPlaying] = useState(false)
  const [videoDuration, setVideoDuration] = useState(duration)
  const [error, setError] = useState<string | null>(null)
  const [isBuffering, setIsBuffering] = useState<boolean>(false)
  const [currentTime, setCurrentTime] = useState(0)
  const [volume, setVolume] = useState(1)
  const [isMuted, setIsMuted] = useState(false)
  const [isFullscreen, setIsFullscreen] = useState(false)
  const [isPiP, setIsPiP] = useState(false)
  const [playbackSpeed, setPlaybackSpeed] = useState<number>(1)
  const [showDebugOverlay, setShowDebugOverlay] = useState(false)
  const [recommendedQuality, setRecommendedQuality] =
    useState<QualityRecommendationResponse | null>(null)

  // Fetch subtitle tracks from API
  const { data: tracksData } = useGetApiMediaIdTracks(mediaId)
  const subtitleTracksFromApi =
    tracksData?.status === 200 ? tracksData.data.subtitle_tracks || [] : []

  // Subtitle track management
  // All subtitles (text and bitmap/PGS) are rendered as client-side overlays
  const { availableSubtitles, currentSubtitle, setCurrentSubtitle, textStreamIndex, bitmapStreamIndex } = useSubtitles({
    subtitleTracks: subtitleTracksFromApi,
    preferredLanguage: 'eng', // TODO: Get from user settings
    preferSDH: false,
    preferForced: true,
  })

  // Initialize progress updater
  const progressUpdater = useProgressUpdater(mediaId, videoDuration)

  // Get quality recommendation
  const qualityRecommendation = useQualityRecommendation()

  // HLS player hook - handles HLS.js lifecycle, quality, audio
  const {
    hlsRef,
    availableQualities,
    currentQuality,
    currentBandwidth,
    availableAudioTracks,
    currentAudioTrack,
    streamOffsetRef,
    setCurrentQuality,
    changeQuality,
    changeAudioTrack,
  } = useHlsPlayer({
    videoRef,
    streamUrl,
    initialPosition,
    isHlsStream,
    onError: setError,
    onFragLoaded: (bytes, durationMs) => recordSample(bytes, durationMs),
    qualityRecommendation: qualityRecommendation.recommendation
      ? {
          height: qualityRecommendation.recommendation.height,
          isReady: qualityRecommendation.isReady,
        }
      : null,
  })

  // Set recommended quality when available
  useEffect(() => {
    if (qualityRecommendation.recommendation && qualityRecommendation.isReady) {
      setRecommendedQuality(qualityRecommendation.recommendation)
    }
  }, [qualityRecommendation.recommendation, qualityRecommendation.isReady])

  // Auto quality control
  const handleAutoQualityChange = useCallback(
    (level: { name: string; height: number }, _reason: string) => {
      setCurrentQuality(level.height)
    },
    [setCurrentQuality]
  )

  const { isAutoMode, setAutoMode, recordSample, recordStall, networkStats } = useAutoQuality({
    enabled: isHlsStream,
    hlsInstance: hlsRef.current,
    onQualityChange: handleAutoQualityChange,
  })

  // Stream stats for debug panel
  const { stats: streamStats, isLoading: streamStatsLoading } = useStreamStats({
    mediaId,
    videoRef,
    hlsRef,
    networkStats,
    isPlaying,
    playbackMode: isHlsStream ? 'transcode' : 'direct',
    streamOffset: streamOffsetRef.current || 0,
    enabled: showDebugOverlay,
  })

  // Playback analytics
  const {
    startSession,
    endSession,
    recordQualitySwitch,
    recordStall: recordAnalyticsStall,
  } = usePlaybackAnalytics({ enabled: true })

  // Video controls hook
  const {
    handlePlayPause,
    handleSeek,
    handleVolumeChange: handleVolumeChangeControl,
    handleMuteToggle,
    handleFullscreenToggle,
    handlePiPToggle,
    handleSkip,
    handlePlaybackSpeedChange,
  } = useVideoControls({
    videoRef,
    containerRef: videoContainerRef,
    hlsRef,
    streamOffsetRef,
    isSeekingRef,
    isHlsStream,
    videoDuration,
    progressUpdater,
    onTimeUpdate: (time) => {
      setCurrentTime(time)
      if (onTimeUpdate) {
        onTimeUpdate(time)
      }
    },
  })

  // Video events hook
  useVideoEvents({
    videoRef,
    mediaId,
    duration,
    videoDuration,
    isPlaying,
    streamOffsetRef,
    isSeekingRef,
    onPlay: () => setIsPlaying(true),
    onPause: () => setIsPlaying(false),
    onTimeUpdate: (time) => {
      setCurrentTime(time)
      if (onTimeUpdate) {
        onTimeUpdate(time)
      }
    },
    onEnded: () => {
      setIsPlaying(false)
      endSession()
    },
    onBufferingStart: () => setIsBuffering(true),
    onBufferingEnd: (stallDuration) => {
      setIsBuffering(false)
      if (stallDuration > 100) {
        recordAnalyticsStall(stallDuration)
      }
    },
    onVolumeChange: (vol, muted) => {
      setVolume(vol)
      setIsMuted(muted)
    },
    onFullscreenChange: setIsFullscreen,
    onPiPEnter: () => setIsPiP(true),
    onPiPExit: () => setIsPiP(false),
    onDurationChange: setVideoDuration,
    startAnalyticsSession: startSession,
    recordStall,
    progressUpdater,
  })

  // Keyboard shortcuts hook
  useVideoKeyboard({
    videoRef,
    containerRef: videoContainerRef,
    videoDuration,
    onToggleDebug: () => setShowDebugOverlay((prev) => !prev),
  })

  // Browser close progress save
  useEffect(() => {
    const handleBeforeUnload = () => {
      if (currentTime > 0 && videoDuration > 0) {
        const data = JSON.stringify({
          media_id: mediaId,
          user_id: 1,
          progress_seconds: currentTime,
          duration_seconds: videoDuration,
        })
        const apiUrl = `${window.location.origin}/api/progress`
        navigator.sendBeacon(apiUrl, new Blob([data], { type: 'application/json' }))
      }
    }

    window.addEventListener('beforeunload', handleBeforeUnload)
    return () => window.removeEventListener('beforeunload', handleBeforeUnload)
  }, [mediaId, currentTime, videoDuration])

  // Handle auto mode toggle
  const handleAutoToggle = useCallback(() => {
    const hls = hlsRef.current
    if (!hls) {
      return
    }

    if (isAutoMode) {
      setAutoMode(false)
    } else {
      hls.currentLevel = -1
      setAutoMode(true)
      setCurrentQuality(null)
    }
  }, [isAutoMode, setAutoMode, setCurrentQuality, hlsRef])

  // Handle quality change with analytics
  const handleQualityChange = useCallback(
    (height: number, bandwidth?: number) => {
      const video = videoRef.current
      const previousQuality = currentQuality ? `${currentQuality}p` : null

      if (height === 0) {
        changeQuality(0)
        setAutoMode(true)
        recordQualitySwitch(
          previousQuality,
          'auto',
          'user_manual',
          video?.currentTime || 0,
          null,
          null
        )
      } else {
        const levelIndex = changeQuality(height, bandwidth)
        if (levelIndex !== null) {
          setAutoMode(false)
          recordQualitySwitch(
            previousQuality,
            `${height}p`,
            'user_manual',
            video?.currentTime || 0,
            null,
            null
          )
        }
      }
    },
    [changeQuality, currentQuality, recordQualitySwitch, setAutoMode]
  )

  // Handle audio track change
  const handleAudioTrackChange = useCallback(
    (trackId: number) => {
      changeAudioTrack(trackId)
    },
    [changeAudioTrack]
  )

  // Handle playback speed with state update
  const handleSpeedChange = useCallback(
    (speed: number) => {
      handlePlaybackSpeedChange(speed)
      setPlaybackSpeed(speed)
    },
    [handlePlaybackSpeedChange]
  )

  // Handle subtitle selection
  const handleSubtitleChange = useCallback(
    (trackId: number | null) => {
      setCurrentSubtitle(trackId)
    },
    [setCurrentSubtitle]
  )

  return (
    <div className="fixed inset-0 z-50 bg-black flex flex-col">
      {/* Close button */}
      <div className="absolute top-4 right-4 z-30">
        {onClose && (
          <Button
            variant="ghost"
            size="sm"
            onClick={onClose}
            className="text-white hover:bg-white/20 cursor-pointer"
          >
            Close
          </Button>
        )}
      </div>

      {/* Error display */}
      {error && (
        <div className="absolute top-20 left-1/2 transform -translate-x-1/2 z-20 bg-red-600/90 text-white px-6 py-3 rounded-lg shadow-lg max-w-md">
          <p className="text-sm font-medium">{error}</p>
        </div>
      )}

      {/* Buffering indicator */}
      {isBuffering && (
        <div className="absolute inset-0 z-20 flex items-center justify-center pointer-events-none">
          <div
            className="w-16 h-16 rounded-full animate-spin"
            style={{
              border: '4px solid transparent',
              borderTopColor: 'white',
              borderRightColor: 'rgba(255, 255, 255, 0.3)',
              borderBottomColor: 'rgba(255, 255, 255, 0.1)',
            }}
          />
        </div>
      )}

      {/* Video player */}
      <div
        ref={videoContainerRef}
        className="flex-1 flex items-center justify-center bg-black relative"
      >
        <video
          ref={videoRef}
          className="w-full h-full max-h-screen cursor-pointer"
          style={{ objectFit: 'contain' }}
          autoPlay
          playsInline
          onClick={handlePlayPause}
        >
          Your browser does not support the video tag.
        </video>

        {/* Subtitle overlay - renders all subtitle types (text and bitmap/PGS) */}
        <SubtitleOverlay
          videoRef={videoRef}
          mediaId={mediaId}
          trackId={currentSubtitle?.id ?? null}
          isBitmap={currentSubtitle?.isBitmap}
          streamIndex={textStreamIndex}
          bitmapIndex={bitmapStreamIndex}
          streamOffsetRef={streamOffsetRef}
        />

        {/* Stats panel */}
        <StatsPanel
          stats={streamStats}
          networkStats={networkStats}
          isVisible={showDebugOverlay}
          onClose={() => setShowDebugOverlay(false)}
          isLoading={streamStatsLoading}
        />

        {/* Video controls */}
        <VideoControls
          videoRef={videoRef}
          isPlaying={isPlaying}
          currentTime={currentTime}
          duration={videoDuration}
          volume={volume}
          isMuted={isMuted}
          isFullscreen={isFullscreen}
          isPiP={isPiP}
          availableQualities={availableQualities}
          currentQuality={currentQuality}
          currentBandwidth={currentBandwidth}
          recommendedQuality={recommendedQuality}
          autoMode={isAutoMode}
          availableAudioTracks={availableAudioTracks}
          currentAudioTrack={currentAudioTrack}
          availableSubtitles={availableSubtitles}
          currentSubtitle={currentSubtitle?.id ?? null}
          playbackSpeed={playbackSpeed}
          metadata={metadata}
          showStats={showDebugOverlay}
          onPlayPause={handlePlayPause}
          onSeek={handleSeek}
          onVolumeChange={handleVolumeChangeControl}
          onMuteToggle={handleMuteToggle}
          onFullscreenToggle={handleFullscreenToggle}
          onPiPToggle={handlePiPToggle}
          onQualityChange={handleQualityChange}
          onAutoToggle={handleAutoToggle}
          onAudioTrackChange={handleAudioTrackChange}
          onSubtitleChange={handleSubtitleChange}
          onSpeedChange={handleSpeedChange}
          onSkip={handleSkip}
          onToggleStats={() => setShowDebugOverlay((prev) => !prev)}
        />
      </div>
    </div>
  )
}

export type { VideoPlayerProps, MediaMetadata } from './VideoPlayer.types'
