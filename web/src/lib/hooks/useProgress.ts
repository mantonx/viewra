import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { useCallback, useRef, useEffect } from 'react';
import {
  getApiProgress,
  getApiProgressId,
  putApiProgress,
  postApiProgressMarkWatched,
  postApiProgressMarkUnwatched,
  deleteApiProgressId,
  getApiProgressWatched,
  getApiProgressInProgress,
} from '../api/generated/progress/progress';
import type {
  GithubComMantonxViewraInternalApplicationProgressUpdateProgressRequest as UpdateProgressRequest,
  GithubComMantonxViewraInternalApplicationProgressMarkWatchedRequest as MarkWatchedRequest,
  GithubComMantonxViewraInternalApplicationProgressWatchProgressResponse as WatchProgressResponse,
} from '../api/generated/models';
import { extractProgressData, extractListProgressData } from '../utils/progress';
import { getDeviceProfileHash } from '../capabilities';

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
export const useMediaProgress = (mediaId: number | undefined, enabled = true) => {
  return useQuery({
    queryKey: progressKeys.detail(mediaId || 0),
    queryFn: async () => {
      if (!mediaId) {
        return null
      }
      const response = await getApiProgressId(mediaId)
      return extractProgressData(response)
    },
    enabled: enabled && !!mediaId,
    retry: false, // Don't retry if progress doesn't exist
  });
}

// Hook to list all progress
export const useProgressList = (params?: { limit?: number; offset?: number }) => {
  return useQuery({
    queryKey: progressKeys.list(params || {}),
    queryFn: async () => {
      const response = await getApiProgress(params);
      return extractListProgressData(response) || { progress: [], total: 0 };
    },
  });
}

// Hook to list watched items
export const useWatchedList = (params?: { limit?: number; offset?: number }) => {
  return useQuery({
    queryKey: progressKeys.watched(params || {}),
    queryFn: async () => {
      const response = await getApiProgressWatched(params);
      return extractListProgressData(response) || { progress: [], total: 0 };
    },
  });
}

// Hook to list in-progress items
export const useInProgressList = (params?: { limit?: number; offset?: number }) => {
  return useQuery({
    queryKey: progressKeys.inProgress(params || {}),
    queryFn: async () => {
      const response = await getApiProgressInProgress(params);
      return extractListProgressData(response) || { progress: [], total: 0 };
    },
  });
}

// Hook to update progress
export const useUpdateProgress = () => {
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
export const useMarkWatched = () => {
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
export const useMarkUnwatched = () => {
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
export const useDeleteProgress = () => {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (mediaId: number) => deleteApiProgressId(mediaId),
    onSuccess: (_, mediaId) => {
      queryClient.invalidateQueries({ queryKey: progressKeys.all });
      queryClient.removeQueries({ queryKey: progressKeys.detail(mediaId) });
    },
  });
}

// Playback preferences to be saved with progress
export interface PlaybackPreferences {
  selectedQuality?: string | null
  selectedAudioTrack?: number | null
  selectedSubtitleTrack?: number | null
}

// Utility hook for continuous progress updates during playback
export const useProgressUpdater = (
  mediaId: number,
  durationSeconds: number,
  updateIntervalMs = 10000, // Update every 10 seconds by default
) => {
  const updateProgress = useUpdateProgress();
  const intervalRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const currentTimeRef = useRef<number>(0);
  const preferencesRef = useRef<PlaybackPreferences>({});

  // Build the progress update payload including preferences and device profile
  const buildPayload = useCallback(() => {
    const prefs = preferencesRef.current;
    return {
      media_id: mediaId,
      user_id: 1, // Default user
      progress_seconds: currentTimeRef.current,
      duration_seconds: durationSeconds,
      // Device profile for device-specific playback preferences
      device_profile: getDeviceProfileHash(),
      // Only include preferences if they have values (undefined means don't update)
      ...(prefs.selectedQuality !== undefined && { selected_quality: prefs.selectedQuality ?? undefined }),
      ...(prefs.selectedAudioTrack !== undefined && { selected_audio_track: prefs.selectedAudioTrack ?? undefined }),
      ...(prefs.selectedSubtitleTrack !== undefined && { selected_subtitle_track: prefs.selectedSubtitleTrack ?? undefined }),
    };
  }, [mediaId, durationSeconds]);

  // Use useCallback for stable function references
  const startTracking = useCallback((currentTimeSeconds: number) => {
    currentTimeRef.current = currentTimeSeconds;

    // Clear any existing interval
    if (intervalRef.current) {
      clearInterval(intervalRef.current);
    }

    // Set up periodic updates
    intervalRef.current = setInterval(() => {
      if (currentTimeRef.current > 0) {
        updateProgress.mutate(buildPayload());
      }
    }, updateIntervalMs);
  }, [updateIntervalMs, updateProgress, buildPayload]);

  const updateCurrentTime = useCallback((currentTimeSeconds: number) => {
    currentTimeRef.current = currentTimeSeconds;
  }, []);

  // Update preferences (stored in ref to avoid triggering re-renders)
  const updatePreferences = useCallback((prefs: Partial<PlaybackPreferences>) => {
    preferencesRef.current = { ...preferencesRef.current, ...prefs };
  }, []);

  const immediateUpdate = useCallback(() => {
    if (currentTimeRef.current > 0) {
      updateProgress.mutate(buildPayload());
    }
  }, [updateProgress, buildPayload]);

  // Immediate update with new preferences (for quality/audio/subtitle changes)
  const immediateUpdateWithPreferences = useCallback((prefs: Partial<PlaybackPreferences>) => {
    preferencesRef.current = { ...preferencesRef.current, ...prefs };
    if (currentTimeRef.current > 0) {
      updateProgress.mutate(buildPayload());
    }
  }, [updateProgress, buildPayload]);

  const stopTracking = useCallback(() => {
    if (intervalRef.current) {
      clearInterval(intervalRef.current);
      intervalRef.current = null;
    }

    // Send final update
    if (currentTimeRef.current > 0) {
      updateProgress.mutate(buildPayload());
    }
  }, [updateProgress, buildPayload]);

  // Automatic cleanup on unmount
  useEffect(() => {
    return () => {
      if (intervalRef.current) {
        clearInterval(intervalRef.current);
      }
    };
  }, []);

  return {
    startTracking,
    updateCurrentTime,
    updatePreferences,
    immediateUpdate,
    immediateUpdateWithPreferences,
    stopTracking,
  };
}
