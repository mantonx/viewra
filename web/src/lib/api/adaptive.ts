/**
 * Adaptive Quality API Client
 * Wrapper around generated API functions for adaptive video quality recommendations
 */

import {
  postApiAdaptiveRecommend,
  postApiAdaptiveLadder,
  getApiSpeedtestChunk,
} from './generated/adaptive/adaptive'
import type { InternalApiHandlersRecommendQualityRequestBody } from './generated/models'

export const adaptiveApi = {
  /**
   * Get quality recommendation based on client capabilities
   */
  recommendQuality: async (request: InternalApiHandlersRecommendQualityRequestBody) => {
    const response = await postApiAdaptiveRecommend(request)
    return response.data
  },

  /**
   * Get adaptive bitrate ladder for ABR streaming
   */
  getAdaptiveLadder: async (request: InternalApiHandlersRecommendQualityRequestBody) => {
    const response = await postApiAdaptiveLadder(request)
    return response.data
  },

  /**
   * Download speed test chunk for network speed measurement
   */
  getSpeedTestChunk: async (params?: { size?: number }) => {
    const response = await getApiSpeedtestChunk(params)
    return response.data
  },
}

// Re-export types for convenience
export type {
  InternalApiHandlersRecommendQualityRequestBody as RecommendQualityRequest,
  InternalApiHandlersQualityRecommendationResponse as QualityRecommendationResponse,
  InternalApiHandlersAdaptiveLadderResponse as AdaptiveLadderResponse,
  InternalApiHandlersQualityProfile as QualityProfile,
} from './generated/models'
