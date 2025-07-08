/**
 * Unified MediaPlaybackService - Single source of truth for all media operations
 * Replaces the legacy MediaService and PlaybackService with a clean, simple interface
 */

import { API_ENDPOINTS, buildApiUrl, buildApiUrlWithParams } from '../constants/api';
import { ViewraError, ErrorCode, fetchWithErrorHandling, logError } from '../utils/errors/errorHandler';

// Core types
export interface MediaFile {
  id: string;
  media_id: string;
  media_type: 'episode' | 'movie' | 'music';
  library_id: number;
  path: string;
  container: string;
  video_codec?: string;
  audio_codec?: string;
  channels?: string;
  sample_rate?: number;
  resolution?: string;
  duration?: number;
  size_bytes: number;
  bitrate_kbps?: number;
  video_width?: number;
  video_height?: number;
  video_framerate?: string;
  audio_channels?: number;
  audio_layout?: string;
  last_seen: string;
  created_at: string;
  updated_at: string;
}

export interface MediaItem {
  type: 'episode' | 'movie';
  episode_id?: string;
  movie_id?: string;
  title: string;
  episode_number?: number;
  season?: {
    id: string;
    season_number: number;
    tv_show: {
      id: string;
      title: string;
    };
  };
}

export interface PlaybackDecision {
  method: 'direct' | 'remux' | 'transcode';  // Primary field from backend
  reason: string;
  direct_play_url?: string;
  stream_url?: string;
  session_id?: string;
  transcode_params?: {
    target_container: string;
    target_codec: string;
    quality: number;
    remux_only?: boolean;
  };
}

export interface TranscodingSession {
  id: string;
  status: string;
  manifest_url?: string;
  stream_url?: string;
  content_hash?: string;
  content_url?: string;
}

/**
 * Unified service for all media playback operations
 */
export class MediaPlaybackService {
  /**
   * Get a media file by ID
   */
  static async getMediaFile(id: string): Promise<MediaFile | null> {
    try {
      console.log('🔍 MediaPlaybackService: Getting media file:', id);
      
      const url = buildApiUrl(API_ENDPOINTS.MEDIA.FILE_BY_ID.path(id));
      const response = await fetchWithErrorHandling(url, {
        method: API_ENDPOINTS.MEDIA.FILE_BY_ID.method,
      });
      
      const mediaFile = await response.json();
      console.log('✅ MediaPlaybackService: Found media file:', mediaFile.path);
      return mediaFile;
    } catch (error) {
      if (error instanceof ViewraError && error.code === ErrorCode.NOT_FOUND) {
        console.log('❌ MediaPlaybackService: Media file not found:', id);
        return null;
      }
      
      console.error('❌ MediaPlaybackService: Error getting media file:', error);
      logError(error as Error, { context: 'getMediaFile', id });
      return null;
    }
  }

  /**
   * Get media metadata by file ID
   */
  static async getMediaMetadata(fileId: string): Promise<MediaItem | null> {
    try {
      console.log('🔍 MediaPlaybackService: Getting metadata for file:', fileId);
      
      const url = buildApiUrl(API_ENDPOINTS.MEDIA.FILE_METADATA.path(fileId));
      const response = await fetchWithErrorHandling(url, {
        method: API_ENDPOINTS.MEDIA.FILE_METADATA.method,
      });
      
      const data = await response.json();
      
      // Handle metadata response structure
      if (data.metadata) {
        if (data.metadata.type === 'episode') {
          return {
            ...data.metadata,
            season: data.metadata.season || {},
          };
        } else if (data.metadata.type === 'movie') {
          return data.metadata;
        }
      }
      
      // Fallback to direct structure
      return data.episode || data.movie || null;
    } catch (error) {
      if (error instanceof ViewraError && error.code === ErrorCode.NOT_FOUND) {
        console.log('❌ MediaPlaybackService: Metadata not found for file:', fileId);
        return null;
      }
      
      console.error('❌ MediaPlaybackService: Error getting metadata:', error);
      logError(error as Error, { context: 'getMediaMetadata', fileId });
      return null;
    }
  }

  /**
   * Get playback decision for a media file
   */
  static async getPlaybackDecision(mediaPath: string, fileId: string): Promise<PlaybackDecision> {
    try {
      console.log('🎯 MediaPlaybackService: Getting playback decision for:', { mediaPath, fileId });

      const url = buildApiUrl(API_ENDPOINTS.PLAYBACK.DECIDE.path);
      const response = await fetchWithErrorHandling(url, {
        method: API_ENDPOINTS.PLAYBACK.DECIDE.method,
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          media_path: mediaPath,
          device_profile: {
            name: 'Web Browser',
            type: 'web',
            supported_video_codecs: ['h264'],
            supported_audio_codecs: ['aac', 'mp3', 'eac3', 'ac3'],
            supported_containers: ['mp4', 'webm', 'mov'], // Modern browsers support these
          },
        }),
      });

      const decision = await response.json();
      console.log('✅ MediaPlaybackService: Playback decision:', decision.reason);
      return decision;
    } catch (error) {
      console.error('❌ MediaPlaybackService: Error getting playback decision:', error);
      logError(error as Error, { context: 'getPlaybackDecision', mediaPath, fileId });
      throw error;
    }
  }

  /**
   * Start a transcoding session
   */
  static async startTranscodingSession(mediaFileId: string, container: string = 'mp4', inputPath?: string): Promise<TranscodingSession> {
    try {
      console.log('🎬 MediaPlaybackService: Starting transcoding session for:', mediaFileId);

      // For transcoding, we need to use the transcoding module's API
      const url = '/api/v1/transcoding/transcode';
      const response = await fetchWithErrorHandling(url, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          mediaId: mediaFileId,
          container: container,
          inputPath: inputPath || '', // Backend should be able to look this up
          encodingOptions: {
            videoCodec: 'h264',
            audioCodec: 'aac',
            quality: 23,
          },
        }),
      });

      const sessionData = await response.json();
      console.log('✅ MediaPlaybackService: Transcoding session started:', sessionData.sessionId);
      
      // Transform response to match our interface
      // For now, we'll need to poll for the transcoded content
      // The backend uses content-addressable storage
      return {
        id: sessionData.sessionId,
        status: sessionData.status,
        stream_url: '', // Will be set when transcoding completes
        manifest_url: sessionData.manifestUrl,
        content_hash: sessionData.contentHash,
        content_url: sessionData.contentUrl,
      };
    } catch (error) {
      console.error('❌ MediaPlaybackService: Error starting transcoding session:', error);
      logError(error as Error, { context: 'startTranscodingSession', mediaFileId });
      throw error;
    }
  }

  /**
   * Get direct stream URL for a media file
   */
  static getStreamUrl(mediaFileId: string): string {
    return buildApiUrl(API_ENDPOINTS.MEDIA.FILE_STREAM.path(mediaFileId));
  }

  /**
   * Stop a transcoding session
   */
  static async stopSession(sessionId: string): Promise<void> {
    try {
      console.log('🛑 MediaPlaybackService: Stopping session:', sessionId);

      // Use transcoding module's API to stop session
      const url = `/api/v1/transcoding/transcode/${sessionId}`;
      await fetchWithErrorHandling(url, {
        method: 'DELETE',
      });

      console.log('✅ MediaPlaybackService: Session stopped:', sessionId);
    } catch (error) {
      if (error instanceof ViewraError && error.code === ErrorCode.NOT_FOUND) {
        console.log('ℹ️ MediaPlaybackService: Session already stopped:', sessionId);
        return;
      }
      
      console.error('❌ MediaPlaybackService: Error stopping session:', error);
      logError(error as Error, { context: 'stopSession', sessionId });
    }
  }

  /**
   * Stop all transcoding sessions
   */
  static async stopAllSessions(): Promise<void> {
    try {
      console.log('🛑 MediaPlaybackService: Stopping all sessions');

      const url = buildApiUrl(API_ENDPOINTS.PLAYBACK.STOP_ALL_SESSIONS.path);
      await fetchWithErrorHandling(url, {
        method: API_ENDPOINTS.PLAYBACK.STOP_ALL_SESSIONS.method,
      });

      console.log('✅ MediaPlaybackService: All sessions stopped');
    } catch (error) {
      console.warn('⚠️ MediaPlaybackService: Error stopping all sessions:', error);
      // Don't throw - this is often called during cleanup
    }
  }

  /**
   * Track playback analytics
   */
  static async trackAnalytics(sessionId: string, event: string, data?: any): Promise<void> {
    try {
      const url = buildApiUrl(API_ENDPOINTS.PLAYBACK.ANALYTICS.path);
      await fetchWithErrorHandling(url, {
        method: API_ENDPOINTS.PLAYBACK.ANALYTICS.method,
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          session_id: sessionId,
          event,
          data,
          timestamp: new Date().toISOString(),
        }),
      });
    } catch (error) {
      console.warn('⚠️ MediaPlaybackService: Error tracking analytics:', error);
      // Don't throw - analytics shouldn't break playback
    }
  }

  /**
   * Get playback compatibility for multiple media files
   * Returns a map of file IDs to their playback methods (direct, remux, transcode)
   */
  static async getPlaybackCompatibility(
    mediaFileIds: string[],
    deviceProfile?: Partial<any>
  ): Promise<Record<string, { method: 'direct' | 'remux' | 'transcode'; reason: string; can_direct_play: boolean }>> {
    try {
      console.log('🔍 MediaPlaybackService: Getting playback compatibility for', mediaFileIds.length, 'files');

      const url = buildApiUrl(API_ENDPOINTS.PLAYBACK.COMPATIBILITY.path);
      const response = await fetchWithErrorHandling(url, {
        method: API_ENDPOINTS.PLAYBACK.COMPATIBILITY.method,
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          media_file_ids: mediaFileIds,
          device_profile: deviceProfile || {
            name: 'Web Browser',
            type: 'web',
            supported_video_codecs: ['h264'],
            supported_audio_codecs: ['aac', 'mp3', 'eac3', 'ac3'],
            supported_containers: ['mp4', 'webm', 'mov'], // Modern browsers support these
          },
        }),
      });

      const data = await response.json();
      const compatibility: Record<string, { method: 'direct' | 'remux' | 'transcode'; reason: string; can_direct_play: boolean }> = {};
      
      // Extract the compatibility object from the response
      const results = data.compatibility || data;
      
      // Transform the backend response to match our expected format
      for (const [fileId, result] of Object.entries(results)) {
        if (typeof result === 'object' && result !== null) {
          const res = result as any;
          if ('error' in res) {
            // Skip files with errors
            continue;
          }
          
          // Use method directly from backend response
          const method: 'direct' | 'remux' | 'transcode' = res.method || 'transcode';
          
          compatibility[fileId] = {
            method,
            reason: res.reason || '',
            can_direct_play: res.can_direct_play ?? (method === 'direct'),
          };
        }
      }
      
      console.log('✅ MediaPlaybackService: Playback compatibility received for', Object.keys(compatibility).length, 'files');
      return compatibility;
    } catch (error) {
      console.error('❌ MediaPlaybackService: Error getting playback compatibility:', error);
      logError(error as Error, { context: 'getPlaybackCompatibility', count: mediaFileIds.length });
      // Return empty object on error - frontend will use fallback logic
      return {};
    }
  }
}