import type {
  GithubComMantonxViewraInternalApplicationProgressWatchProgressResponse as WatchProgressResponse,
  GithubComMantonxViewraInternalApplicationProgressListProgressResponse as ListProgressResponse,
} from '../api/generated/models'

// Type guards for API responses
export const isProgressResponse = (response: unknown): response is { data: WatchProgressResponse; status: number } => {
  if (typeof response !== 'object' || response === null || !('data' in response)) {
    return false
  }
  const r = response as { data: unknown }
  return (
    typeof r.data === 'object' &&
    r.data !== null &&
    'media_id' in r.data
  )
}

export const isListProgressResponse = (response: unknown): response is { data: ListProgressResponse; status: number } => {
  if (typeof response !== 'object' || response === null || !('data' in response)) {
    return false
  }
  const r = response as { data: unknown }
  return (
    typeof r.data === 'object' &&
    r.data !== null &&
    'progress' in r.data
  )
}

// Helper to extract progress response data
export const extractProgressData = (response: unknown): WatchProgressResponse | null => {
  if (isProgressResponse(response)) {
    return response.data
  }
  return null
}

// Helper to extract list progress response data
export const extractListProgressData = (response: unknown): ListProgressResponse | null => {
  if (isListProgressResponse(response)) {
    return response.data
  }
  return null
}

// Constants
export const DEFAULT_USER_ID = 1

// Helpers for handling optional progress values
export const getProgressPercentage = (progress: WatchProgressResponse | null | undefined): number => {
  return progress?.progress_percentage ?? 0
}

export const getProgressSeconds = (progress: WatchProgressResponse | null | undefined): number => {
  return progress?.progress_seconds ?? 0
}

export const getDurationSeconds = (progress: WatchProgressResponse | null | undefined): number => {
  return progress?.duration_seconds ?? 0
}

export const hasProgress = (progress: WatchProgressResponse | null | undefined): boolean => {
  return getProgressPercentage(progress) > 0
}
