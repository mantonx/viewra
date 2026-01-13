/**
 * Semantic Search Plugin API Client
 *
 * Provides typed access to the semantic-search plugin endpoints.
 */

import { pluginApi } from '@/lib/api/pluginApi'

const PLUGIN_ID = 'semantic-search'

// Types matching backend response
export interface IntentChip {
  id: string
  type: string
  value: string
  display: string
  removable: boolean
  refinements?: string[]
  role?: string
}

export interface ResultReason {
  type: string
  display: string
}

export interface RecoveryAction {
  type: string
  description: string
  original?: string
  relaxed?: string
}

export interface SemanticSearchResult {
  entity_type: string
  entity_id: number
  similarity: number
  text: string
}

export interface SemanticSearchResponse {
  results: SemanticSearchResult[]
  total: number
  recovery_applied?: RecoveryAction[]
  original_query?: string
  intent_chips?: IntentChip[]
  result_reasons?: ResultReason[]
}

export interface SearchParams {
  query: string
  entity_types?: string[]
  limit?: number
  playback_constraints?: {
    min_resolution?: string
    max_resolution?: string
    hdr_formats?: string[]
    audio_formats?: string[]
    min_channels?: number
    has_subtitles?: boolean
    subtitle_language?: string
    max_bitrate?: number
  }
}

export interface PluginStatus {
  indexed: Record<string, { indexed: number; total: number }>
  provider: string
  embedding_model: string
}

/**
 * Check if semantic search plugin is available
 */
export const getStatus = async (): Promise<PluginStatus> => {
  return pluginApi.get<PluginStatus>(PLUGIN_ID, '/status')
}

/**
 * Perform semantic search
 */
export const search = async (params: SearchParams): Promise<SemanticSearchResponse> => {
  return pluginApi.post<SemanticSearchResponse>(PLUGIN_ID, '/search', params)
}

/**
 * Find similar items to a given entity
 */
export const findSimilar = async (
  entityType: string,
  entityId: number,
  limit?: number
): Promise<SemanticSearchResponse> => {
  return pluginApi.post<SemanticSearchResponse>(PLUGIN_ID, '/similar', {
    entity_type: entityType,
    entity_id: entityId,
    limit,
  })
}

/**
 * Get query explanation (debug endpoint)
 */
export const explain = async (query: string): Promise<unknown> => {
  return pluginApi.get(PLUGIN_ID, '/explain', { q: query })
}

export const semanticSearchApi = {
  getStatus,
  search,
  findSimilar,
  explain,
}
