/**
 * Adaptive Quality API Client
 * Wrapper around generated API functions for adaptive video quality recommendations
 */

import {
  postApiAdaptiveRecommend,
  postApiAdaptiveLadder,
  getApiSpeedtestChunk,
} from './generated/adaptive/adaptive'
import type {
  InternalApiHandlersRecommendQualityRequestBody,
  InternalApiHandlersQualityRecommendationResponse,
  InternalApiHandlersAdaptiveLadderResponse,
} from './generated/models'

export const adaptiveApi = {
  /**
   * Get quality recommendation based on client capabilities
   */
  recommendQuality: async (
    request: InternalApiHandlersRecommendQualityRequestBody
  ): Promise<InternalApiHandlersQualityRecommendationResponse> => {
    const response = await postApiAdaptiveRecommend(request)
    if (response.status !== 200) {
      throw new Error('error' in response.data ? String(response.data.error) : 'Failed to get recommendation')
    }
    return response.data
  },

  /**
   * Get adaptive bitrate ladder for ABR streaming
   */
  getAdaptiveLadder: async (
    request: InternalApiHandlersRecommendQualityRequestBody
  ): Promise<InternalApiHandlersAdaptiveLadderResponse> => {
    const response = await postApiAdaptiveLadder(request)
    if (response.status !== 200) {
      throw new Error('error' in response.data ? String(response.data.error) : 'Failed to get ladder')
    }
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
