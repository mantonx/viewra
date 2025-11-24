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
    (caps: ClientCapabilities): Parameters<typeof adaptiveApi.recommendQuality>[0] => ({
      // Network (required)
      networkSpeedMbps: caps.networkSpeedMbps,
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
      supportedCodecs: caps.supportedCodecs.length > 0 ? caps.supportedCodecs : undefined,
      hardwareAcceleration: caps.hardwareAcceleration,
      maxDecodingProfile: caps.maxDecodingProfile || undefined,
    }),
    []
  )

  /**
   * Detect capabilities and get quality recommendation
   */
  const detectAndRecommend = useCallback(
    async (forceCapabilityRefresh = false) => {
      setState((prev) => ({ ...prev, loading: true, error: null }))

      try {
        console.log('[QualityRecommendation] Starting capability detection...')

        // Step 1: Detect client capabilities
        const capabilities = await capabilityDetector.detectCapabilities(
          forceCapabilityRefresh || forceRefresh
        )
        console.log('[QualityRecommendation] Capabilities detected:', capabilities)

        // Step 2: Map to API request format
        const request = mapCapabilitiesToRequest(capabilities)
        console.log('[QualityRecommendation] API request:', request)

        // Step 3: Get recommendation or ladder from backend
        if (useLadder) {
          console.log('[QualityRecommendation] Requesting ladder...')
          const ladder = await adaptiveApi.getAdaptiveLadder(request)
          console.log('[QualityRecommendation] Ladder received:', ladder)
          setState({
            capabilities,
            recommendation: null,
            ladder,
            loading: false,
            error: null,
            detectedAt: new Date(),
          })
        } else {
          console.log('[QualityRecommendation] Requesting recommendation...')
          const recommendation = await adaptiveApi.recommendQuality(request)
          console.log('[QualityRecommendation] Recommendation received:', recommendation)
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
