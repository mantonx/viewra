import type { MediaType, MediaFile, MediaItem, PlaybackDecision, DeviceProfile, TranscodingSession, SeekAheadRequest, SeekAheadResponse } from '../components/MediaPlayer/types';
import { isValidSessionId } from '../utils/mediaValidation';
import { API_ENDPOINTS, buildApiUrl, buildApiUrlWithParams } from '../constants/api';

export class MediaService {
  private static apiBaseUrl = '/api';

  static async getMediaFiles(mediaId: string, mediaType: MediaType): Promise<MediaFile | null> {
    try {
      const url = buildApiUrlWithParams(API_ENDPOINTS.MEDIA.FILES.path, { limit: 1000 });
      const response = await fetch(url, {
        method: API_ENDPOINTS.MEDIA.FILES.method,
      });
      if (!response.ok) {
        throw new Error(`Failed to fetch media files: ${response.statusText}`);
      }
      
      const data = await response.json();
      const mediaFile = data.media_files?.find(
        (file: MediaFile & { media_id: string; media_type: string }) =>
          file.media_id === mediaId && file.media_type === mediaType
      );
      
      return mediaFile || null;
    } catch (error) {
      console.error('Failed to get media files:', error);
      throw error;
    }
  }

  static async getMediaMetadata(mediaId: string, mediaFileId: string): Promise<MediaItem | null> {
    try {
      const url = buildApiUrl(API_ENDPOINTS.MEDIA.FILE_METADATA.path(mediaFileId));
      const response = await fetch(url, {
        method: API_ENDPOINTS.MEDIA.FILE_METADATA.method,
      });
      if (!response.ok) {
        if (response.status === 404) {
          return null;
        }
        throw new Error(`Failed to fetch media metadata: ${response.statusText}`);
      }
      
      const data = await response.json();
      return data.episode || data.movie || null;
    } catch (error) {
      console.error('Failed to get media metadata:', error);
      throw error;
    }
  }

  static async getPlaybackDecision(mediaPath: string, deviceProfile: DeviceProfile): Promise<PlaybackDecision> {
    try {
      const url = buildApiUrl(API_ENDPOINTS.PLAYBACK.DECIDE.path);
      const response = await fetch(url, {
        method: API_ENDPOINTS.PLAYBACK.DECIDE.method,
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          media_path: mediaPath,
          device_profile: deviceProfile,
        }),
      });

      if (!response.ok) {
        throw new Error(`Playback decision failed: ${response.statusText}`);
      }

      return await response.json();
    } catch (error) {
      console.error('Failed to get playback decision:', error);
      throw error;
    }
  }

  static async startTranscodingSession(
    inputPath: string,
    container: string = 'dash',
    videoCodec: string = 'h264',
    audioCodec: string = 'aac',
    quality: number = 23,
    speedPriority: string = 'balanced'
  ): Promise<TranscodingSession> {
    try {
      const url = buildApiUrl(API_ENDPOINTS.PLAYBACK.START.path);
      const response = await fetch(url, {
        method: API_ENDPOINTS.PLAYBACK.START.method,
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          input_path: inputPath,
          output_path: '',
          container,
          video_codec: videoCodec,
          audio_codec: audioCodec,
          quality,
          speed_priority: speedPriority,
        }),
      });

      if (!response.ok) {
        throw new Error(`Session start failed: ${response.statusText}`);
      }

      return await response.json();
    } catch (error) {
      console.error('Failed to start transcoding session:', error);
      throw error;
    }
  }

  static async stopTranscodingSession(sessionId: string): Promise<void> {
    try {
      const url = buildApiUrl(API_ENDPOINTS.PLAYBACK.SESSION.path(sessionId));
      const response = await fetch(url, {
        method: API_ENDPOINTS.PLAYBACK.SESSION.method,
        headers: {
          'Content-Type': 'application/json',
        },
      });

      if (!response.ok && response.status !== 404) {
        throw new Error(`Failed to stop session: ${response.statusText}`);
      }
    } catch (error) {
      console.error('Failed to stop transcoding session:', error);
      throw error;
    }
  }

  static async requestSeekAhead(request: SeekAheadRequest): Promise<SeekAheadResponse> {
    try {
      const url = buildApiUrl(API_ENDPOINTS.PLAYBACK.SEEK_AHEAD.path);
      const response = await fetch(url, {
        method: API_ENDPOINTS.PLAYBACK.SEEK_AHEAD.method,
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify(request),
      });

      if (!response.ok) {
        throw new Error(`Seek-ahead request failed: ${response.statusText}`);
      }

      return await response.json();
    } catch (error) {
      console.error('Failed to request seek-ahead:', error);
      throw error;
    }
  }

  static async waitForManifest(url: string, maxAttempts: number = 30, initialIntervalMs: number = 200): Promise<boolean> {
    for (let attempt = 1; attempt <= maxAttempts; attempt++) {
      try {
        const response = await fetch(url, { method: 'GET' });
        if (response.ok) {
          return true;
        }
        
        const delay = Math.min(initialIntervalMs * Math.pow(1.5, attempt - 1), 2000);
        await new Promise(resolve => setTimeout(resolve, delay));
      } catch (error) {
        const delay = Math.min(initialIntervalMs * attempt, 1000);
        await new Promise(resolve => setTimeout(resolve, delay));
      }
    }
    
    throw new Error(`Manifest not available after ${maxAttempts} attempts`);
  }

  static getDefaultDeviceProfile(): DeviceProfile {
    return {
      user_agent: navigator.userAgent,
      supported_codecs: ["h264", "aac", "mp3"],
      max_resolution: "1080p",
      max_bitrate: 8000,
      supports_hevc: false,
      target_container: "dash"
    };
  }

  static isValidSessionId(sessionId: string): boolean {
    return isValidSessionId(sessionId);
  }
}