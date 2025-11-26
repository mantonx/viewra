import { useState, useEffect, useRef, useCallback } from 'react'
import { evaluateQuality, canApplyDecision, DEFAULT_CONFIG } from '../streaming/AutoQualityController'
import type { QualityLevel, QualityDecision, AutoQualityConfig } from '../streaming/AutoQualityController'
import { useNetworkMonitor } from './useNetworkMonitor'
import { loadVideoPreferences } from '@/lib/preferences'
import type { VideoQualityPreferences } from '@/lib/preferences'
import type Hls from 'hls.js'

export interface UseAutoQualityOptions {
  enabled?: boolean
  hlsInstance: Hls | null
  config?: Partial<AutoQualityConfig>
  onQualityChange?: (level: QualityLevel, reason: string) => void
}

export interface UseAutoQualityReturn {
  currentDecision: QualityDecision | null
  isAutoMode: boolean
  setAutoMode: (auto: boolean) => void
  qualityLevels: QualityLevel[]
  currentLevelIndex: number
  setManualQuality: (index: number) => void
  recordSample: (bytes: number, durationMs: number, wasStall?: boolean) => void
  recordStall: () => void
  networkStats: import('../network/NetworkMonitor').NetworkStats | null
  bufferLength: number
  sampleCount: number
}

/**
 * React hook for automatic quality control during video playback
 *
 * Combines network monitoring with quality switching logic to provide
 * smooth adaptive bitrate streaming.
 *
 * Usage:
 * ```tsx
 * const {
 *   isAutoMode,
 *   setAutoMode,
 *   currentDecision,
 *   recordSample,
 *   recordStall
 * } = useAutoQuality({
 *   hlsInstance: hls,
 *   onQualityChange: (level, reason) => console.log(`Switched to ${level.name}: ${reason}`)
 * })
 * ```
 */
export const useAutoQuality = (options: UseAutoQualityOptions): UseAutoQualityReturn => {
  const { enabled = true, hlsInstance, config, onQualityChange } = options

  const [isAutoMode, setIsAutoMode] = useState(true)
  const [qualityLevels, setQualityLevels] = useState<QualityLevel[]>([])
  const [currentLevelIndex, setCurrentLevelIndex] = useState(-1)
  const [currentDecision, setCurrentDecision] = useState<QualityDecision | null>(null)
  const [bufferLength, setBufferLength] = useState(0)

  const lastChangeTimeRef = useRef(0)
  const playbackStartTimeRef = useRef(0) // Track when playback started for startup grace period
  const preferencesRef = useRef<VideoQualityPreferences | null>(null)
  const configRef = useRef<AutoQualityConfig>({ ...DEFAULT_CONFIG, ...config })
  const evaluationIntervalRef = useRef<ReturnType<typeof setInterval> | null>(null)

  // Network monitoring - always enabled when hook is enabled so we can show stats in overlay
  // even when auto quality switching is disabled
  const { stats: networkStats, recordSample, recordStall, sampleCount } = useNetworkMonitor({
    enabled,
  })

  // Load preferences on mount
  useEffect(() => {
    const prefs = loadVideoPreferences()
    preferencesRef.current = prefs
    setIsAutoMode(prefs.autoQualityEnabled)
  }, [])

  // Sync quality levels from HLS.js
  useEffect(() => {
    if (!hlsInstance) {
      return
    }

    const updateLevels = () => {
      const levels: QualityLevel[] = hlsInstance.levels.map((level, index) => ({
        index,
        height: level.height,
        bitrate: level.bitrate,
        name: `${level.height}p`,
      }))
      // Sort by bitrate ascending for consistent indexing
      levels.sort((a, b) => a.bitrate - b.bitrate)
      setQualityLevels(levels)
    }

    const updateCurrentLevel = () => {
      setCurrentLevelIndex(hlsInstance.currentLevel)
    }

    // Initial sync
    if (hlsInstance.levels.length > 0) {
      updateLevels()
    }
    updateCurrentLevel()

    // Listen for level changes
    hlsInstance.on('hlsManifestParsed' as Parameters<typeof hlsInstance.on>[0], updateLevels)
    hlsInstance.on('hlsLevelSwitched' as Parameters<typeof hlsInstance.on>[0], updateCurrentLevel)

    return () => {
      hlsInstance.off('hlsManifestParsed' as Parameters<typeof hlsInstance.off>[0], updateLevels)
      hlsInstance.off('hlsLevelSwitched' as Parameters<typeof hlsInstance.off>[0], updateCurrentLevel)
    }
  }, [hlsInstance])

  // Auto quality evaluation loop
  useEffect(() => {
    if (!enabled || !isAutoMode || !hlsInstance || qualityLevels.length === 0) {
      if (evaluationIntervalRef.current) {
        clearInterval(evaluationIntervalRef.current)
        evaluationIntervalRef.current = null
      }
      return
    }

    // Set playback start time when auto quality evaluation begins
    // This gives us a proper grace period for initial buffering
    if (playbackStartTimeRef.current === 0) {
      playbackStartTimeRef.current = Date.now()
    }

    const evaluate = () => {
      const video = hlsInstance.media
      if (!video) {
        return
      }

      const currentBufferLength = getBufferLength(video)
      setBufferLength(currentBufferLength)
      const isPlaying = !video.paused && !video.ended

      const decision = evaluateQuality(
        qualityLevels,
        currentLevelIndex,
        networkStats,
        currentBufferLength,
        isPlaying,
        lastChangeTimeRef.current,
        configRef.current,
        preferencesRef.current,
        playbackStartTimeRef.current
      )

      setCurrentDecision(decision)

      // Apply decision if conditions are met
      if (decision.action !== 'maintain' && decision.targetLevel) {
        if (canApplyDecision(decision, lastChangeTimeRef.current, configRef.current)) {
          hlsInstance.currentLevel = decision.targetLevel.index
          lastChangeTimeRef.current = Date.now()
          onQualityChange?.(decision.targetLevel, decision.reason)
        }
      }
    }

    // Evaluate every 2 seconds
    evaluationIntervalRef.current = setInterval(evaluate, 2000)

    // Initial evaluation
    evaluate()

    return () => {
      if (evaluationIntervalRef.current) {
        clearInterval(evaluationIntervalRef.current)
        evaluationIntervalRef.current = null
      }
    }
  }, [enabled, isAutoMode, hlsInstance, qualityLevels, currentLevelIndex, networkStats, onQualityChange])

  const setAutoMode = useCallback((auto: boolean) => {
    setIsAutoMode(auto)
    if (auto) {
      // Reset last change time to allow immediate evaluation
      lastChangeTimeRef.current = 0
    }
  }, [])

  const setManualQuality = useCallback(
    (index: number) => {
      if (hlsInstance && index >= 0 && index < qualityLevels.length) {
        setIsAutoMode(false)
        hlsInstance.currentLevel = index
        lastChangeTimeRef.current = Date.now()
      }
    },
    [hlsInstance, qualityLevels.length]
  )

  return {
    currentDecision,
    isAutoMode,
    setAutoMode,
    qualityLevels,
    currentLevelIndex,
    setManualQuality,
    recordSample,
    recordStall,
    networkStats,
    bufferLength,
    sampleCount,
  }
}

/**
 * Calculate buffered time ahead of current playback position
 */
const getBufferLength = (video: HTMLMediaElement): number => {
  if (video.buffered.length === 0) {
    return 0
  }

  const currentTime = video.currentTime
  for (let i = 0; i < video.buffered.length; i++) {
    const start = video.buffered.start(i)
    const end = video.buffered.end(i)
    if (currentTime >= start && currentTime <= end) {
      return end - currentTime
    }
  }
  return 0
}

export default useAutoQuality
