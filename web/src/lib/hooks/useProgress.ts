import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import {
  getApiProgress,
  getApiProgressMediaId,
  putApiProgress,
  postApiProgressMarkWatched,
  postApiProgressMarkUnwatched,
  deleteApiProgressMediaId,
  getApiProgressWatched,
  getApiProgressInProgress,
} from '../api/generated/progress/progress';
import type {
  GithubComViewraViewraInternalApplicationProgressUpdateProgressRequest as UpdateProgressRequest,
  GithubComViewraViewraInternalApplicationProgressMarkWatchedRequest as MarkWatchedRequest,
  GithubComViewraViewraInternalApplicationProgressWatchProgressResponse as WatchProgressResponse,
} from '../api/generated/models';
import { extractProgressData, extractListProgressData } from '../utils/progress';

// Query keys for progress
export const progressKeys = {
  all: ['progress'] as const,
  lists: () => [...progressKeys.all, 'list'] as const,
  list: (filters: { limit?: number; offset?: number }) =>
    [...progressKeys.lists(), { filters }] as const,
  watched: (filters: { limit?: number; offset?: number }) =>
    [...progressKeys.all, 'watched', { filters }] as const,
  inProgress: (filters: { limit?: number; offset?: number }) =>
    [...progressKeys.all, 'in-progress', { filters }] as const,
  detail: (mediaId: number) => [...progressKeys.all, 'detail', mediaId] as const,
};

// Hook to get progress for a specific media item
export function useMediaProgress(mediaId: number | undefined, enabled = true) {
  return useQuery({
    queryKey: progressKeys.detail(mediaId || 0),
    queryFn: async () => {
      if (!mediaId) return null;
      const response = await getApiProgressMediaId(mediaId);
      return extractProgressData(response);
    },
    enabled: enabled && !!mediaId,
    retry: false, // Don't retry if progress doesn't exist
  });
}

// Hook to list all progress
export function useProgressList(params?: { limit?: number; offset?: number }) {
  return useQuery({
    queryKey: progressKeys.list(params || {}),
    queryFn: async () => {
      const response = await getApiProgress(params);
      return extractListProgressData(response) || { progress: [], total: 0 };
    },
  });
}

// Hook to list watched items
export function useWatchedList(params?: { limit?: number; offset?: number }) {
  return useQuery({
    queryKey: progressKeys.watched(params || {}),
    queryFn: async () => {
      const response = await getApiProgressWatched(params);
      return extractListProgressData(response) || { progress: [], total: 0 };
    },
  });
}

// Hook to list in-progress items
export function useInProgressList(params?: { limit?: number; offset?: number }) {
  return useQuery({
    queryKey: progressKeys.inProgress(params || {}),
    queryFn: async () => {
      const response = await getApiProgressInProgress(params);
      return extractListProgressData(response) || { progress: [], total: 0 };
    },
  });
}

// Hook to update progress
export function useUpdateProgress() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (data: UpdateProgressRequest): Promise<WatchProgressResponse> => {
      const response = await putApiProgress(data);
      const progressData = extractProgressData(response);
      if (!progressData) {
        throw new Error('Failed to update progress');
      }
      return progressData;
    },
    onSuccess: (response: WatchProgressResponse) => {
      // Invalidate all progress queries
      queryClient.invalidateQueries({ queryKey: progressKeys.all });

      // Update the specific media progress in cache
      if (response.media_id) {
        queryClient.setQueryData(
          progressKeys.detail(response.media_id),
          response
        );
      }
    },
  });
}

// Hook to mark as watched
export function useMarkWatched() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (data: MarkWatchedRequest): Promise<WatchProgressResponse> => {
      const response = await postApiProgressMarkWatched(data);
      const progressData = extractProgressData(response);
      if (!progressData) {
        throw new Error('Failed to mark as watched');
      }
      return progressData;
    },
    onSuccess: (response: WatchProgressResponse) => {
      queryClient.invalidateQueries({ queryKey: progressKeys.all });
      if (response.media_id) {
        queryClient.setQueryData(
          progressKeys.detail(response.media_id),
          response
        );
      }
    },
  });
}

// Hook to mark as unwatched
export function useMarkUnwatched() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (data: MarkWatchedRequest): Promise<WatchProgressResponse> => {
      const response = await postApiProgressMarkUnwatched(data);
      const progressData = extractProgressData(response);
      if (!progressData) {
        throw new Error('Failed to mark as unwatched');
      }
      return progressData;
    },
    onSuccess: (response: WatchProgressResponse) => {
      queryClient.invalidateQueries({ queryKey: progressKeys.all });
      if (response.media_id) {
        queryClient.setQueryData(
          progressKeys.detail(response.media_id),
          response
        );
      }
    },
  });
}

// Hook to delete progress
export function useDeleteProgress() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (mediaId: number) => deleteApiProgressMediaId(mediaId),
    onSuccess: (_, mediaId) => {
      queryClient.invalidateQueries({ queryKey: progressKeys.all });
      queryClient.removeQueries({ queryKey: progressKeys.detail(mediaId) });
    },
  });
}

// Utility hook for continuous progress updates during playback
export function useProgressUpdater(
  mediaId: number,
  durationSeconds: number,
  updateIntervalMs = 10000 // Update every 10 seconds by default
) {
  const updateProgress = useUpdateProgress();
  let intervalId: ReturnType<typeof setInterval> | null = null;
  let lastProgressSeconds = 0;

  const startTracking = (currentTimeSeconds: number) => {
    lastProgressSeconds = Math.floor(currentTimeSeconds);

    // Clear any existing interval
    if (intervalId) {
      clearInterval(intervalId);
    }

    // Set up periodic updates
    intervalId = setInterval(() => {
      if (lastProgressSeconds > 0) {
        updateProgress.mutate({
          media_id: mediaId,
          user_id: 1, // Default user
          progress_seconds: lastProgressSeconds,
          duration_seconds: Math.floor(durationSeconds),
        });
      }
    }, updateIntervalMs);
  };

  const updateCurrentTime = (currentTimeSeconds: number) => {
    lastProgressSeconds = Math.floor(currentTimeSeconds);
  };

  const stopTracking = () => {
    if (intervalId) {
      clearInterval(intervalId);
      intervalId = null;
    }

    // Send final update
    if (lastProgressSeconds > 0) {
      updateProgress.mutate({
        media_id: mediaId,
        user_id: 1,
        progress_seconds: lastProgressSeconds,
        duration_seconds: Math.floor(durationSeconds),
      });
    }
  };

  return {
    startTracking,
    updateCurrentTime,
    stopTracking,
  };
}
