import type { NetworkStats } from '../network/NetworkMonitor'
import type { VideoQualityPreferences } from '@/lib/preferences'
import { logger } from '@/lib/utils/logger'

export interface QualityLevel {
  index: number
  height: number
  bitrate: number
  name: string
}

export interface AutoQualityConfig {
  minBufferForUpgrade: number
  minBufferForDowngrade: number
  criticalBuffer: number
  upgradeDelayMs: number
  downgradeDelayMs: number
  emergencyDowngradeDelayMs: number
  minStabilityForUpgrade: number
  maxStallsBeforeDowngrade: number
}

export interface QualityDecision {
  action: 'upgrade' | 'downgrade' | 'maintain'
  targetLevel: QualityLevel | null
  reason: string
  confidence: number
}

export const DEFAULT_CONFIG: AutoQualityConfig = {
  minBufferForUpgrade: 15,
  minBufferForDowngrade: 5,
  criticalBuffer: 2,
  upgradeDelayMs: 10000,
  downgradeDelayMs: 3000,
  emergencyDowngradeDelayMs: 500,
  minStabilityForUpgrade: 0.6,
  maxStallsBeforeDowngrade: 2,
}

// Grace period at start of playback before allowing emergency downgrades
// This prevents panic-downgrading while the initial buffer is being filled
const STARTUP_GRACE_PERIOD_MS = 10000 // 10 seconds

const parseQualityToHeight = (quality: string): number | null => {
  const lower = quality.toLowerCase()
  if (lower.includes('4k')) {
    return 2160
  }
  if (lower.includes('8k')) {
    return 4320
  }
  const match = quality.match(/(\d+)p?/i)
  return match ? parseInt(match[1], 10) : null
}

const findLevelForBitrate = (levels: QualityLevel[], targetBitrate: number): QualityLevel | null => {
  for (let i = levels.length - 1; i >= 0; i--) {
    if (levels[i].bitrate <= targetBitrate) {
      return levels[i]
    }
  }
  return levels[0] || null
}

const getMaxAllowedLevel = (
  levels: QualityLevel[],
  preferences: VideoQualityPreferences | null
): QualityLevel | null => {
  if (!preferences?.maxAutoQuality) {
    return null
  }
  const maxHeight = parseQualityToHeight(preferences.maxAutoQuality)
  if (!maxHeight) {
    return null
  }

  for (let i = levels.length - 1; i >= 0; i--) {
    if (levels[i].height <= maxHeight) {
      return levels[i]
    }
  }
  return null
}

/**
 * Evaluate current playback conditions and decide whether to change quality
 */
export const evaluateQuality = (
  levels: QualityLevel[],
  currentLevelIndex: number,
  networkStats: NetworkStats | null,
  bufferLength: number,
  isPlaying: boolean,
  lastChangeTime: number,
  config: AutoQualityConfig = DEFAULT_CONFIG,
  preferences: VideoQualityPreferences | null = null,
  playbackStartTime: number = 0
): QualityDecision => {
  const maintain = (reason: string, confidence = 0.9): QualityDecision => ({
    action: 'maintain',
    targetLevel: levels[currentLevelIndex] || null,
    reason,
    confidence
  })

  if (!isPlaying || levels.length === 0) {
    return maintain('Not playing', 1)
  }

  const currentLevel = levels[currentLevelIndex]
  if (!currentLevel) {
    return maintain('No current level', 1)
  }

  // Check if we're in the startup grace period
  // Use playbackStartTime if provided, otherwise fall back to lastChangeTime for backwards compatibility
  const startTime = playbackStartTime > 0 ? playbackStartTime : lastChangeTime
  const timeSinceStart = Date.now() - startTime
  const isStartupPeriod = timeSinceStart < STARTUP_GRACE_PERIOD_MS

  // Check if network is proven fast (can easily handle current bitrate)
  // If network throughput is 3x+ the current bitrate, low buffer is likely
  // due to transcoding latency, not network issues
  const networkCanHandleQuality = networkStats && currentLevel &&
    (networkStats.averageThroughputMbps * 1_000_000) > (currentLevel.bitrate * 3)


  // Emergency: critical buffer - but be more careful
  // During startup, don't panic-downgrade as the buffer is still filling
  // Instead of jumping to lowest, downgrade one step at a time
  if (bufferLength < config.criticalBuffer) {
    // Skip emergency downgrade during startup - buffer is still filling from transcoder
    // The transcoder needs time to generate initial segments, so low/zero buffer is expected
    if (isStartupPeriod) {
      return maintain(`Startup buffer: ${bufferLength.toFixed(1)}s (grace period)`, 0.7)
    }

    // If network is proven fast, don't downgrade - the issue is transcoding latency
    // This is the key fix: when network throughput is 3x+ the bitrate needs,
    // stalls and low buffer are caused by waiting for transcoding, not network issues.
    // Downgrading quality won't help because transcoding latency is the bottleneck.
    if (networkCanHandleQuality) {
      return maintain(`Buffer low but network fast (${networkStats?.averageThroughputMbps.toFixed(0)} Mbps) - waiting for transcode`, 0.6)
    }

    // Downgrade one step instead of jumping to lowest
    // This gives the buffer a chance to recover without going to minimum quality
    const lowerIndex = currentLevelIndex > 0 ? currentLevelIndex - 1 : 0
    const lowerLevel = levels[lowerIndex]
    if (lowerLevel && lowerLevel.index !== currentLevelIndex) {
      logger.debug('[AutoQuality] DOWNGRADE: Critical buffer', {
        from: currentLevel.name,
        to: lowerLevel.name,
        bufferLength: bufferLength.toFixed(1),
        isStartupPeriod,
        networkCanHandleQuality,
      })
      return {
        action: 'downgrade',
        targetLevel: lowerLevel,
        reason: `Critical buffer: ${bufferLength.toFixed(1)}s`,
        confidence: 1
      }
    }
  }

  if (!networkStats) {
    return maintain('No network stats', 0.5)
  }

  // Too many stalls - but only if network is actually the problem
  // If network is fast (3x+ bitrate needs), stalls are from transcoding latency, not network
  if (networkStats.stallCount > config.maxStallsBeforeDowngrade && !networkCanHandleQuality) {
    const lower = currentLevelIndex > 0 ? levels[currentLevelIndex - 1] : null
    if (lower) {
      logger.debug('[AutoQuality] DOWNGRADE: Too many stalls', {
        from: currentLevel.name,
        to: lower.name,
        stallCount: networkStats.stallCount,
        threshold: config.maxStallsBeforeDowngrade,
      })
      return {
        action: 'downgrade',
        targetLevel: lower,
        reason: `Too many stalls: ${networkStats.stallCount}`,
        confidence: 0.9
      }
    }
  }

  // Low buffer - but consider if network is fast enough
  // If we have proven network capacity, low buffer might be due to transcoding latency
  if (bufferLength < config.minBufferForDowngrade) {
    // During startup, give more time for buffer to fill
    if (isStartupPeriod) {
      return maintain(`Buffer building: ${bufferLength.toFixed(1)}s (startup)`, 0.7)
    }

    // If network is fast (3x+ bitrate needs), don't downgrade - transcoding latency is the issue
    // Stalls during seek are expected while waiting for transcoder, not a network problem
    if (networkCanHandleQuality) {
      return maintain(`Buffer recovering: ${bufferLength.toFixed(1)}s (network fast, waiting for transcode)`, 0.6)
    }

    const lower = currentLevelIndex > 0 ? levels[currentLevelIndex - 1] : null
    if (lower) {
      logger.debug('[AutoQuality] DOWNGRADE: Low buffer', {
        from: currentLevel.name,
        to: lower.name,
        bufferLength: bufferLength.toFixed(1),
        threshold: config.minBufferForDowngrade,
        isStartupPeriod,
        networkCanHandleQuality,
        stallCount: networkStats.stallCount,
      })
      return {
        action: 'downgrade',
        targetLevel: lower,
        reason: `Low buffer: ${bufferLength.toFixed(1)}s`,
        confidence: 0.8
      }
    }
  }

  // Network degrading - but only act if it actually affects playability
  // IMPORTANT: Use averageThroughputMbps, not minThroughputMbps for this check
  // minThroughputMbps gets polluted by stall samples (low/zero throughput during seeks)
  if (networkStats.trend === 'degrading') {
    // If average throughput is still 3x+ what we need, network is fine
    // The "degrading" trend might just be noise from seek stalls
    if (networkCanHandleQuality) {
      return maintain(`Network degrading but still fast (avg ${networkStats.averageThroughputMbps.toFixed(0)} Mbps)`, 0.7)
    }

    const safeBitrate = networkStats.averageThroughputMbps * 1_000_000 * 0.7
    const target = findLevelForBitrate(levels, safeBitrate)
    if (target && target.index < currentLevelIndex) {
      logger.debug('[AutoQuality] DOWNGRADE: Network degrading', {
        from: currentLevel.name,
        to: target.name,
        avgMbps: networkStats.averageThroughputMbps.toFixed(1),
        minMbps: networkStats.minThroughputMbps.toFixed(1),
        safeBitrateMbps: (safeBitrate / 1_000_000).toFixed(1),
        networkCanHandleQuality,
      })
      return {
        action: 'downgrade',
        targetLevel: target,
        reason: 'Network degrading',
        confidence: 0.7
      }
    }
  }

  // Check upgrade opportunity
  const timeSinceChange = Date.now() - lastChangeTime
  const canUpgrade =
    bufferLength >= config.minBufferForUpgrade &&
    networkStats.stability >= config.minStabilityForUpgrade &&
    networkStats.trend !== 'degrading' &&
    timeSinceChange >= config.upgradeDelayMs

  if (canUpgrade) {
    const safeBitrate = networkStats.averageThroughputMbps * 1_000_000 * 0.7
    let target = findLevelForBitrate(levels, safeBitrate)
    const maxLevel = getMaxAllowedLevel(levels, preferences)

    if (target && target.index > currentLevelIndex) {
      // Respect max preference
      if (maxLevel && target.index > maxLevel.index) {
        target = maxLevel.index > currentLevelIndex ? maxLevel : null
      }

      if (target) {
        return {
          action: 'upgrade',
          targetLevel: target,
          reason: maxLevel && target === maxLevel
            ? 'Network supports higher (limited by preference)'
            : `Network supports ${(safeBitrate / 1_000_000).toFixed(1)} Mbps`,
          confidence: networkStats.stability
        }
      }
    }
  }

  return maintain('Conditions stable')
}

/**
 * Check if enough time has passed to apply a quality decision
 */
export const canApplyDecision = (
  decision: QualityDecision,
  lastChangeTime: number,
  config: AutoQualityConfig = DEFAULT_CONFIG
): boolean => {
  if (decision.action === 'maintain' || !decision.targetLevel) {
    return false
  }

  const delay = decision.action === 'downgrade'
    ? (decision.reason.includes('Critical')
      ? config.emergencyDowngradeDelayMs
      : config.downgradeDelayMs)
    : config.upgradeDelayMs

  return Date.now() - lastChangeTime >= delay
}
