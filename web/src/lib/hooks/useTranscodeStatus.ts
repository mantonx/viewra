import { useQuery } from '@tanstack/react-query';
import {
  getApiMediaIdTranscodeQuality,
  getGetApiMediaIdTranscodeQualityQueryKey,
} from '../api/generated/transcode/transcode';
import type {
  InternalApiHandlersTranscodeJobResponse as TranscodeJobResponse,
  InternalApiHandlersErrorResponse as ErrorResponse,
} from '../api/generated/models';

// Query keys for transcode status
export const transcodeKeys = {
  all: ['transcode'] as const,
  status: (mediaId: number, quality: string) =>
    [...transcodeKeys.all, 'status', mediaId, quality] as const,
};

// Hook to get transcode status with polling support
export const useTranscodeStatus = (
  mediaId: number | undefined,
  quality: string | undefined,
  options?: {
    enabled?: boolean;
    pollingInterval?: number; // milliseconds, default 2000
  }
) => {
  const enabled = options?.enabled ?? true;
  const pollingInterval = options?.pollingInterval ?? 2000;

  return useQuery<TranscodeJobResponse, ErrorResponse>({
    queryKey: mediaId && quality
      ? transcodeKeys.status(mediaId, quality)
      : getGetApiMediaIdTranscodeQualityQueryKey(mediaId, quality),
    queryFn: async () => {
      if (!mediaId || !quality) {
        throw new Error('mediaId and quality are required');
      }
      const response = await getApiMediaIdTranscodeQuality(mediaId, quality);
      if (!response.data) {
        throw new Error('No transcode job found');
      }
      return response.data;
    },
    enabled: enabled && !!mediaId && !!quality,
    refetchInterval: enabled ? pollingInterval : false,
    retry: 3, // Retry a few times in case job is being created
    retryDelay: 1000, // Wait 1 second between retries
    refetchOnWindowFocus: false, // Don't refetch when window regains focus
  });
}

// Helper to check if transcode is complete
export const isTranscodeComplete = (
  status: TranscodeJobResponse | undefined
): boolean => {
  return status?.status === 'completed';
}

// Helper to check if transcode failed
export const isTranscodeFailed = (
  status: TranscodeJobResponse | undefined
): boolean => {
  return status?.status === 'failed';
}

// Helper to check if transcode is in progress
export const isTranscodeInProgress = (
  status: TranscodeJobResponse | undefined
): boolean => {
  const inProgressStatuses = ['queued', 'processing'];
  return status?.status ? inProgressStatuses.includes(status.status) : false;
}

// Helper to get estimated time based on job type
export const getEstimatedTime = (jobType: string | undefined): string => {
  switch (jobType) {
    case 'remux':
      return '2-5 minutes';
    case 'remux_audio':
      return '5-10 minutes';
    case 'transcode':
      return '20-60 minutes';
    default:
      return 'Unknown';
  }
}

// Helper to get user-friendly message based on job type and status
export const getTranscodeMessage = (
  status: TranscodeJobResponse | undefined
): string => {
  if (!status) {
    return 'Checking compatibility...';
  }

  const { status: jobStatus, type: jobType, progress } = status;

  switch (jobStatus) {
    case 'queued':
      return `In queue - ${jobType === 'remux' ? 'Preparing video (very fast)' : jobType === 'remux_audio' ? 'Preparing with audio conversion' : 'Converting video'}...`;
    case 'processing':
      if (jobType === 'remux') {
        return `Preparing video (very fast)... ${progress || 0}%`;
      } else if (jobType === 'remux_audio') {
        return `Preparing with audio conversion... ${progress || 0}%`;
      } else {
        return `Converting video (this may take a while)... ${progress || 0}%`;
      }
    case 'completed':
      return 'Ready to play!';
    case 'failed':
      return status.error || 'Transcoding failed';
    default:
      return 'Processing...';
  }
}
