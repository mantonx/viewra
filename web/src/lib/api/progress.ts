/**
 * Progress API Client
 * Direct API calls for watch progress endpoints
 */

import { customInstance } from './mutator'
import type {
  GithubComViewraViewraInternalApplicationProgressWatchProgressResponse as WatchProgressResponse,
  GithubComViewraViewraInternalApplicationProgressBatchProgressResponse as BatchProgressResponse,
} from './generated/models'

export const progressApi = {
  /**
   * Get watch progress for a specific media item
   */
  getProgress: (mediaId: number) =>
    customInstance<WatchProgressResponse>({
      url: `/api/progress/${mediaId}`,
      method: 'GET',
    }),

  /**
   * Get watch progress for multiple media items in a single batch request
   * Reduces N+1 query problem by fetching all progress at once
   */
  getBatchProgress: (mediaIds: number[]) =>
    customInstance<BatchProgressResponse>({
      url: `/api/progress/batch?media_ids=${mediaIds.join(',')}`,
      method: 'GET',
    }),

  /**
   * Update watch progress for a media item
   */
  updateProgress: (data: {
    media_id: number
    user_id?: number
    progress_seconds: number
    duration_seconds: number
  }) =>
    customInstance<WatchProgressResponse>({
      url: '/api/progress',
      method: 'PUT',
      data,
    }),

  /**
   * Mark a media item as watched
   */
  markWatched: (data: { media_id: number; user_id?: number }) =>
    customInstance<WatchProgressResponse>({
      url: '/api/progress/mark-watched',
      method: 'POST',
      data,
    }),

  /**
   * Mark a media item as unwatched
   */
  markUnwatched: (data: { media_id: number; user_id?: number }) =>
    customInstance<WatchProgressResponse>({
      url: '/api/progress/mark-unwatched',
      method: 'POST',
      data,
    }),

  /**
   * Delete watch progress for a media item
   */
  deleteProgress: (mediaId: number) =>
    customInstance<void>({
      url: `/api/progress/${mediaId}`,
      method: 'DELETE',
    }),
}
