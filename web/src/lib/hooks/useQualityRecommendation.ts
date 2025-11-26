/**
 * Hook for getting quality recommendations based on client capabilities
 * Detects device/network capabilities and requests optimal quality profile from backend
 */
import type {
  AdaptiveLadderResponse,
  QualityProfile,
  QualityRecommendationResponse,
} from '@/lib/api/adaptive'
import { adaptiveApi } from '@/lib/api/adaptive'
import { capabilityDetector } from '@/lib/capabilities/CapabilityDetector'
import type { ClientCapabilities } from '@/lib/capabilities/types'
import { useCallback, useEffect, useState } from 'react'

export interface UseQualityRecommendationOptions {
  /** Skip initial automatic detection on mount */
  skipAutoDetect?: boolean
  /** Force refresh capabilities cache */
  forceRefresh?: boolean
  /** Get full ABR ladder instead of single recommendation */
  useLadder?: boolean
}

export interface QualityRecommendationState {
  /** Detected client capabilities */
  capabilities: ClientCapabilities | null
  /** Recommended quality profile */
  recommendation: QualityRecommendationResponse | null
  /** ABR ladder with multiple quality options */
  ladder: AdaptiveLadderResponse | null
  /** Loading state */
  loading: boolean
  /** Error if detection or API call failed */
  error: Error | null
  /** Timestamp of last detection */
  detectedAt: Date | null
}

export const useQualityRecommendation = (options: UseQualityRecommendationOptions = {}) => {
  const { skipAutoDetect = false, forceRefresh = false, useLadder = false } = options

  const [state, setState] = useState<QualityRecommendationState>({
    capabilities: null,
    recommendation: null,
    ladder: null,
    loading: !skipAutoDetect,
    error: null,
    detectedAt: null,
  })

  /**
   * Convert ClientCapabilities to API request format
   */
  const mapCapabilitiesToRequest = useCallback(
    (caps: ClientCapabilities): Parameters<typeof adaptiveApi.recommendQuality>[0] => {
      // Build supported codecs list from the new CodecSupport structure
      const supportedCodecs: string[] = []
      if (caps.codecSupport) {
        // Add codecs in preferred order (already sorted by compression efficiency)
        for (const codec of caps.codecSupport.preferredOrder) {
          if (caps.codecSupport[codec]?.supported) {
            supportedCodecs.push(codec)
          }
        }
      }

      // If network speed measurement failed (-1), assume a decent connection
      // This prevents defaulting to lowest quality when speedtest fails
      // The actual quality will be adjusted during playback based on real throughput
      const networkSpeed = caps.networkSpeedMbps > 0 ? caps.networkSpeedMbps : 50

      const request = {
        // Network (required)
        networkSpeedMbps: networkSpeed,
        connectionType: caps.connectionType,
        isMetered: caps.isMetered,

        // Device (required)
        deviceType: caps.deviceType,
        screenWidth: caps.screenWidth,
        screenHeight: caps.screenHeight,
        pixelRatio: caps.pixelRatio,

        // Performance (optional)
        cpuCores: caps.cpuCores,
        memoryGB: caps.memoryGB > 0 ? caps.memoryGB : undefined,
        batteryLevel: caps.batteryLevel > 0 ? caps.batteryLevel : undefined,
        lowPowerMode: caps.lowPowerMode,
        isCharging: caps.isCharging,

        // Media capabilities (optional)
        supportedCodecs: supportedCodecs.length > 0 ? supportedCodecs : undefined,
        hardwareAcceleration: caps.hardwareAcceleration,
        maxDecodingProfile: caps.maxDecodingProfile || undefined,
      }

      console.log('[QualityRecommendation] Sending request:', {
        networkSpeedMbps: request.networkSpeedMbps,
        deviceType: request.deviceType,
        screenWidth: request.screenWidth,
        screenHeight: request.screenHeight,
        supportedCodecs: request.supportedCodecs,
      })

      return request
    },
    []
  )

  /**
   * Detect capabilities and get quality recommendation
   */
  const detectAndRecommend = useCallback(
    async (forceCapabilityRefresh = false) => {
      setState((prev) => ({ ...prev, loading: true, error: null }))

      try {
        // Step 1: Detect client capabilities
        const capabilities = await capabilityDetector.detectCapabilities(
          forceCapabilityRefresh || forceRefresh
        )

        // Step 2: Map to API request format
        const request = mapCapabilitiesToRequest(capabilities)

        // Step 3: Get recommendation or ladder from backend
        if (useLadder) {
          const ladder = await adaptiveApi.getAdaptiveLadder(request)
          setState({
            capabilities,
            recommendation: null,
            ladder,
            loading: false,
            error: null,
            detectedAt: new Date(),
          })
        } else {
          const recommendation = await adaptiveApi.recommendQuality(request)
          console.log('[QualityRecommendation] Got recommendation:', {
            profileId: recommendation.profileId,
            height: recommendation.height,
            displayName: recommendation.displayName,
            score: recommendation.score,
            reason: recommendation.reason,
          })
          setState({
            capabilities,
            recommendation,
            ladder: null,
            loading: false,
            error: null,
            detectedAt: new Date(),
          })
        }
      } catch (error) {
        console.error('[QualityRecommendation] Error:', error)
        setState((prev) => ({
          ...prev,
          loading: false,
          error: error instanceof Error ? error : new Error('Unknown error occurred'),
        }))
      }
    },
    [forceRefresh, useLadder, mapCapabilitiesToRequest]
  )

  /**
   * Manually trigger a new detection and recommendation
   */
  const refresh = useCallback(() => {
    return detectAndRecommend(true)
  }, [detectAndRecommend])

  /**
   * Get quality options from ladder (if using ladder mode)
   */
  const getQualityOptions = useCallback((): QualityProfile[] => {
    if (!state.ladder?.profiles) {
      return []
    }
    return state.ladder.profiles
  }, [state.ladder])

  // Auto-detect on mount unless disabled
  useEffect(() => {
    if (!skipAutoDetect) {
      detectAndRecommend()
    }
  }, [skipAutoDetect, detectAndRecommend])

  return {
    // State
    capabilities: state.capabilities,
    recommendation: state.recommendation,
    ladder: state.ladder,
    loading: state.loading,
    error: state.error,
    detectedAt: state.detectedAt,

    // Actions
    refresh,
    detectAndRecommend,
    getQualityOptions,

    // Helpers
    isReady: !state.loading && !state.error && state.capabilities !== null,
    hasRecommendation: state.recommendation !== null || state.ladder !== null,
  }
}
