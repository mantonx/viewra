/**
 * VideoPlayer Component
 * Full-featured video player with HLS streaming, quality selection,
 * keyboard controls, and progress tracking.
 *
 * Quality Control Model (Single-Quality):
 * - Backend picks optimal quality based on client capabilities
 * - User can override via quality picker (triggers stream reload)
 * - Each quality change restarts FFmpeg from current position
 */

import { Button } from '@/components/ui'
import { useProgressUpdater } from '@/lib/hooks/useProgress'
import { useAutoQuality } from '@/lib/hooks/useAutoQuality'
import { usePlaybackAnalytics } from '@/lib/hooks/usePlaybackAnalytics'
import { useStreamStats } from '@/lib/hooks/useStreamStats'
import { useHlsPlayer } from '@/lib/hooks/useHlsPlayer'
import { useVideoEvents } from '@/lib/hooks/useVideoEvents'
import { useVideoKeyboard } from '@/lib/hooks/useVideoKeyboard'
import { useVideoControls } from '@/lib/hooks/useVideoControls'
import { useSubtitles } from '@/lib/hooks/useSubtitles'
import { useCallback, useEffect, useRef, useState } from 'react'
import { VideoControls } from '../VideoControls'
import { StatsPanel } from '../StatsPanel'
import { SubtitleOverlay } from '../SubtitleOverlay'
import type { VideoPlayerProps } from './VideoPlayer.types'
import { useGetApiMediaIdTracks } from '@/lib/api/generated/media/media'

export const VideoPlayer = ({
  mediaId,
  streamUrl,
  initialPosition = 0,
  duration = 0,
  metadata,
  onClose,
  onTimeUpdate,
  availableQualities: backendQualities = [],
  selectedQualityId = null,
  onQualityChange: onQualityChangeCallback,
  savedPreferences = null,
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
  // Key that changes on large seeks to force subtitle components to remount
  // This ensures subtitles re-initialize with the correct streamOffsetRef value
  const [subtitleSeekKey, setSubtitleSeekKey] = useState(0)
  // Backend session ID for analytics correlation (from X-Session-ID header)
  const [backendSessionId, setBackendSessionId] = useState<string | null>(null)

  // Fetch tracks (audio and subtitle) from API
  const { data: tracksData } = useGetApiMediaIdTracks(mediaId)
  const subtitleTracksFromApi =
    tracksData?.status === 200 ? tracksData.data.subtitle_tracks || [] : []
  const audioTracksFromApi =
    tracksData?.status === 200 ? tracksData.data.audio_tracks || [] : []

  // Track current audio stream index for URL construction
  // -1 means default (first audio track)
  const [currentAudioStreamIndex, setCurrentAudioStreamIndex] = useState<number>(-1)

  // Position override for when we switch audio tracks or quality - maintains playback position
  const [trackSwitchPosition, setTrackSwitchPosition] = useState<number | null>(null)

  // Build stream URL with audio track parameter and potentially updated start position
  // When quality changes to a different resolution, we need to reload with the current position
  const buildEffectiveStreamUrl = () => {
    let url = streamUrl
    const params = new URLSearchParams()

    // Parse existing params from URL
    const urlParts = streamUrl.split('?')
    if (urlParts.length > 1) {
      const existingParams = new URLSearchParams(urlParts[1])
      existingParams.forEach((value, key) => params.set(key, value))
      url = urlParts[0]
    }

    // Update audio track if non-default
    if (currentAudioStreamIndex > 0) {
      params.set('audioTrack', String(currentAudioStreamIndex))
    }

    // Update start position if we're doing a track/quality switch
    if (trackSwitchPosition !== null) {
      params.set('start', String(Math.floor(trackSwitchPosition)))
    }

    const paramStr = params.toString()
    return paramStr ? `${url}?${paramStr}` : url
  }

  const effectiveStreamUrl = buildEffectiveStreamUrl()

  // Use track switch position if set (from audio or quality change), otherwise use initial position from props
  const effectiveInitialPosition = trackSwitchPosition ?? initialPosition

  // Subtitle track management
  // Skip auto-selection when we have a saved subtitle preference to restore
  const hasSavedSubtitlePref = savedPreferences?.selectedSubtitleTrack !== undefined
  const { availableSubtitles, currentSubtitle, setCurrentSubtitle, textStreamIndex, bitmapStreamIndex } = useSubtitles({
    subtitleTracks: subtitleTracksFromApi,
    preferredLanguage: 'eng',
    preferSDH: false,
    preferForced: true,
    skipAutoSelect: hasSavedSubtitlePref,
  })

  // Initialize progress updater
  const progressUpdater = useProgressUpdater(mediaId, videoDuration)

  // HLS player hook - handles HLS.js lifecycle, quality
  // Uses Navigator Connection API for instant bandwidth estimate (no slow speed test)
  // Audio tracks are managed via API, not HLS.js (since audio is muxed into video segments)
  const {
    hlsRef,
    availableQualities,
    currentQuality,
    currentBandwidth,
    streamOffsetRef,
    changeQuality,
  } = useHlsPlayer({
    videoRef,
    streamUrl: effectiveStreamUrl,
    initialPosition: effectiveInitialPosition,
    isHlsStream,
    onError: setError,
    onFragLoaded: (bytes, durationMs) => recordSample(bytes, durationMs),
    onSessionIdReceived: setBackendSessionId,
  })

  // Convert API audio tracks to the format expected by VideoControls
  // The id is the stream_index which is what we pass to the backend
  // Filter out tracks without stream_index (shouldn't happen, but type-safe)
  const availableAudioTracks = audioTracksFromApi
    .filter((track): track is typeof track & { stream_index: number } =>
      track.stream_index !== undefined
    )
    .map((track, index) => ({
      id: track.stream_index,
      name: track.title || `Track ${index + 1}`,
      language: track.language || 'Unknown',
    }))

  // Current audio track for UI - this is the stream_index (matches track.id)
  // Default to first track's stream_index if not explicitly set
  const currentAudioTrack = currentAudioStreamIndex > 0
    ? currentAudioStreamIndex
    : (availableAudioTracks[0]?.id ?? 0)

  // Network stats for debug panel
  const { recordSample, recordStall, networkStats } = useAutoQuality({
    enabled: isHlsStream,
    hlsInstance: hlsRef.current,
  })

  // Stream stats for debug panel
  const { stats: streamStats, isLoading: streamStatsLoading, refresh: refreshStreamStats } = useStreamStats({
    mediaId,
    videoRef,
    hlsRef,
    networkStats,
    isPlaying,
    playbackMode: isHlsStream ? 'transcode' : 'direct',
    selectedQualityId,  // Pass selected quality for accurate strategy detection
    streamOffset: streamOffsetRef.current || 0,
    enabled: showDebugOverlay,
  })

  // Playback analytics
  const {
    startSession,
    updateSessionId,
    endSession,
    recordQualitySwitch,
    recordStall: recordAnalyticsStall,
    recordPlayTime,
    recordStartupTime,
  } = usePlaybackAnalytics({ enabled: true })

  // Update analytics session ID when backend session ID arrives
  useEffect(() => {
    if (backendSessionId) {
      updateSessionId(backendSessionId)
    }
  }, [backendSessionId, updateSessionId])

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
    onLargeSeekComplete: () => {
      // Force subtitle components to remount with fresh streamOffsetRef value
      setSubtitleSeekKey((prev) => prev + 1)
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
    backendSessionId,
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
    recordPlayTime,
    recordStartupTime,
    progressUpdater,
  })

  // Keyboard shortcuts hook
  useVideoKeyboard({
    videoRef,
    containerRef: videoContainerRef,
    videoDuration,
    onToggleDebug: () => setShowDebugOverlay((prev) => !prev),
  })

  // Browser close progress save (includes preferences)
  // Note: -1 is used as a sentinel value for "subtitles off" to distinguish from null (don't update)
  useEffect(() => {
    const handleBeforeUnload = () => {
      if (currentTime > 0 && videoDuration > 0) {
        const data = JSON.stringify({
          media_id: mediaId,
          user_id: 1,
          progress_seconds: currentTime,
          duration_seconds: videoDuration,
          selected_quality: selectedQualityId,
          selected_audio_track: currentAudioStreamIndex > 0 ? currentAudioStreamIndex : null,
          // Use -1 to indicate "subtitles off" (null means don't update due to COALESCE in SQL)
          selected_subtitle_track: currentSubtitle?.id ?? -1,
        })
        const apiUrl = `${window.location.origin}/api/progress`
        navigator.sendBeacon(apiUrl, new Blob([data], { type: 'application/json' }))
      }
    }

    window.addEventListener('beforeunload', handleBeforeUnload)
    return () => window.removeEventListener('beforeunload', handleBeforeUnload)
  }, [mediaId, currentTime, videoDuration, selectedQualityId, currentAudioStreamIndex, currentSubtitle])

  // Apply saved audio track preference when tracks load
  // Use a ref to ensure we only apply once per media
  const appliedAudioPrefRef = useRef(false)
  useEffect(() => {
    if (appliedAudioPrefRef.current) return
    if (!savedPreferences?.selectedAudioTrack) return
    if (availableAudioTracks.length === 0) return

    // Check if saved track exists in available tracks
    const trackExists = availableAudioTracks.some(t => t.id === savedPreferences.selectedAudioTrack)
    if (trackExists && savedPreferences.selectedAudioTrack !== currentAudioStreamIndex) {
      setCurrentAudioStreamIndex(savedPreferences.selectedAudioTrack)
    }
    appliedAudioPrefRef.current = true
  }, [savedPreferences, availableAudioTracks, currentAudioStreamIndex])

  // Apply saved subtitle preference when subtitles load
  const appliedSubtitlePrefRef = useRef(false)
  useEffect(() => {
    if (appliedSubtitlePrefRef.current) return
    if (savedPreferences?.selectedSubtitleTrack === undefined) return
    if (availableSubtitles.length === 0 && savedPreferences.selectedSubtitleTrack !== null) return

    // -1 or null means subtitles off, otherwise find the track
    if (savedPreferences.selectedSubtitleTrack === null || savedPreferences.selectedSubtitleTrack === -1) {
      if (currentSubtitle !== null) {
        setCurrentSubtitle(null)
      }
    } else {
      const trackExists = availableSubtitles.some(s => s.id === savedPreferences.selectedSubtitleTrack)
      if (trackExists && currentSubtitle?.id !== savedPreferences.selectedSubtitleTrack) {
        setCurrentSubtitle(savedPreferences.selectedSubtitleTrack)
      }
    }
    appliedSubtitlePrefRef.current = true
  }, [savedPreferences, availableSubtitles, currentSubtitle, setCurrentSubtitle])

  // Reset applied prefs refs when media changes
  useEffect(() => {
    appliedAudioPrefRef.current = false
    appliedSubtitlePrefRef.current = false
  }, [mediaId])

  // Save preferences when they change (via progress updater)
  // Use -1 to indicate "subtitles off" (null means don't update due to COALESCE in SQL)
  useEffect(() => {
    progressUpdater.updatePreferences({
      selectedQuality: selectedQualityId,
      selectedAudioTrack: currentAudioStreamIndex > 0 ? currentAudioStreamIndex : null,
      selectedSubtitleTrack: currentSubtitle?.id ?? -1,
    })
  }, [selectedQualityId, currentAudioStreamIndex, currentSubtitle, progressUpdater])

  // Handle quality change - calls parent callback to rebuild URL and reload stream
  // Single-quality model: each quality change triggers a new FFmpeg session from current position
  const handleQualityChange = useCallback(
    (qualityId: string) => {
      const video = videoRef.current
      if (!video || !onQualityChangeCallback) return

      // Calculate the actual media time (accounting for stream offset in progressive transcoding)
      const currentPosition = video.currentTime + (streamOffsetRef.current || 0)

      // Record quality switch for analytics
      const previousQuality = selectedQualityId
      recordQualitySwitch(
        previousQuality,
        qualityId,
        'user_manual',
        currentPosition,
        null,
        null
      )

      // Call parent callback to rebuild URL with ?quality= and reload
      onQualityChangeCallback(qualityId, currentPosition)

      // Refresh stream stats after a short delay to get updated strategy info
      setTimeout(() => refreshStreamStats(), 1000)
    },
    [onQualityChangeCallback, selectedQualityId, recordQualitySwitch, streamOffsetRef, refreshStreamStats]
  )

  // Handle audio track change
  // streamIndex is the FFmpeg stream index (passed as track.id from AudioSelector)
  const handleAudioTrackChange = useCallback(
    (streamIndex: number) => {
      // Capture current playback position before switching
      const video = videoRef.current
      if (video) {
        // Calculate the actual media time (accounting for stream offset in progressive transcoding)
        const actualTime = video.currentTime + (streamOffsetRef.current || 0)
        setTrackSwitchPosition(actualTime)
      }
      // Setting stream index will update effectiveStreamUrl via state change
      // which triggers HLS.js to reload with the new audio track
      setCurrentAudioStreamIndex(streamIndex)
    },
    [streamOffsetRef]
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
        {/* Key changes on large seeks to force remount with fresh streamOffsetRef */}
        <SubtitleOverlay
          key={subtitleSeekKey}
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
          selectedQuality={backendQualities.find(q => q.id === selectedQualityId) ?? null}
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
          availableQualities={backendQualities}
          selectedQualityId={selectedQualityId}
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
