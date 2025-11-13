import type {
  GithubComViewraViewraInternalApplicationProgressWatchProgressResponse as WatchProgressResponse,
  GithubComViewraViewraInternalApplicationProgressListProgressResponse as ListProgressResponse,
} from '../api/generated/models'

// Type guards for API responses
export function isProgressResponse(response: unknown): response is { data: WatchProgressResponse; status: number } {
  return (
    typeof response === 'object' &&
    response !== null &&
    'data' in response &&
    typeof (response as any).data === 'object' &&
    (response as any).data !== null &&
    'media_id' in (response as any).data
  )
}

export function isListProgressResponse(response: unknown): response is { data: ListProgressResponse; status: number } {
  return (
    typeof response === 'object' &&
    response !== null &&
    'data' in response &&
    typeof (response as any).data === 'object' &&
    (response as any).data !== null &&
    'progress' in (response as any).data
  )
}

// Helper to extract progress response data
export function extractProgressData(response: unknown): WatchProgressResponse | null {
  if (isProgressResponse(response)) {
    return response.data
  }
  return null
}

// Helper to extract list progress response data
export function extractListProgressData(response: unknown): ListProgressResponse | null {
  if (isListProgressResponse(response)) {
    return response.data
  }
  return null
}

// Constants
export const DEFAULT_USER_ID = 1

// Helpers for handling optional progress values
export function getProgressPercentage(progress: WatchProgressResponse | null | undefined): number {
  return progress?.progress_percentage ?? 0
}

export function getProgressSeconds(progress: WatchProgressResponse | null | undefined): number {
  return progress?.progress_seconds ?? 0
}

export function getDurationSeconds(progress: WatchProgressResponse | null | undefined): number {
  return progress?.duration_seconds ?? 0
}

export function hasProgress(progress: WatchProgressResponse | null | undefined): boolean {
  return getProgressPercentage(progress) > 0
}
