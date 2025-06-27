import { describe, it, expect, vi, beforeEach } from 'vitest';
import { MediaService } from './MediaService';
import { apiCall } from '@/utils/api';

vi.mock('@/utils/api', () => ({
  apiCall: vi.fn(),
}));

describe('MediaService', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('startTranscodingSession', () => {
    it('should start a transcoding session with content hash', async () => {
      const mockResponse = {
        id: 'session-123',
        status: 'queued',
        manifest_url: '/api/v1/content/abc123def456/manifest.mpd',
        provider: 'ffmpeg-pipeline',
        content_hash: 'abc123def456',
        content_url: '/api/v1/content/abc123def456/',
      };

      vi.mocked(apiCall).mockResolvedValueOnce(mockResponse);

      const result = await MediaService.startTranscodingSession(
        'file-456',
        'dash',
        'h264'
      );

      expect(apiCall).toHaveBeenCalledWith('/api/playback/start', {
        method: 'POST',
        data: {
          media_file_id: 'file-456',
          container: 'dash',
          enable_abr: true,
        },
      });

      expect(result).toEqual(mockResponse);
      expect(result.content_hash).toBe('abc123def456');
      expect(result.content_url).toBe('/api/v1/content/abc123def456/');
    });

    it('should handle sessions that require content generation', async () => {
      const mockResponse = {
        id: 'session-789',
        status: 'queued',
        manifest_url: '/api/v1/content/newhash789/manifest.mpd',
        provider: 'ffmpeg-pipeline',
        content_hash: 'newhash789',
        content_url: '/api/v1/content/newhash789/',
      };

      vi.mocked(apiCall).mockResolvedValueOnce(mockResponse);

      const result = await MediaService.startTranscodingSession(
        'file-789',
        'dash',
        'h264'
      );

      expect(result).toEqual(mockResponse);
      expect(result.content_hash).toBe('newhash789');
      expect(result.content_url).toBe('/api/v1/content/newhash789/');
    });

    it('should handle HLS container with content hash', async () => {
      const mockResponse = {
        id: 'session-hls',
        status: 'queued',
        manifest_url: '/api/v1/content/def456ghi789/playlist.m3u8',
        provider: 'ffmpeg-pipeline',
        content_hash: 'def456ghi789',
        content_url: '/api/v1/content/def456ghi789/',
      };

      vi.mocked(apiCall).mockResolvedValueOnce(mockResponse);

      const result = await MediaService.startTranscodingSession(
        'file-hls',
        'hls',
        'h264'
      );

      expect(apiCall).toHaveBeenCalledWith('/api/playback/start', {
        method: 'POST',
        data: {
          media_file_id: 'file-hls',
          container: 'hls',
          enable_abr: true,
        },
      });

      expect(result.manifest_url).toContain('playlist.m3u8');
      expect(result.content_hash).toBe('def456ghi789');
    });
  });

  describe('getPlaybackDecision', () => {
    it('should return playback decision with content reuse info', async () => {
      const mockDecision = {
        should_transcode: true,
        reason: 'requires transcoding for compatibility',
        transcode_params: {
          target_container: 'dash',
          target_codec: 'h264',
          resolution: '1080p',
        },
        content_hash: 'existing123',
        content_url: '/api/v1/content/existing123/',
      };

      vi.mocked(apiCall).mockResolvedValueOnce(mockDecision);

      const result = await MediaService.getPlaybackDecision(
        '/path/to/video.mp4',
        'file-123',
        MediaService.getDefaultDeviceProfile()
      );

      expect(result).toEqual(mockDecision);
      expect(result.content_hash).toBe('existing123');
    });
  });

  describe('requestSeekAhead', () => {
    it('should request seek-ahead with new session', async () => {
      const mockResponse = {
        id: 'seek-session-123',
        status: 'queued',
        manifest_url: '/api/v1/content/seek123hash/manifest.mpd',
        provider: 'ffmpeg-pipeline',
        content_hash: 'seek123hash',
        content_url: '/api/v1/content/seek123hash/',
      };

      vi.mocked(apiCall).mockResolvedValueOnce(mockResponse);

      const result = await MediaService.requestSeekAhead({
        session_id: 'original-session',
        seek_position: 300,
      });

      expect(apiCall).toHaveBeenCalledWith('/api/playback/seek-ahead', {
        method: 'POST',
        data: {
          session_id: 'original-session',
          seek_position: 300,
        },
      });

      expect(result.content_hash).toBe('seek123hash');
    });
  });

  describe('getDefaultDeviceProfile', () => {
    it('should return a default device profile', () => {
      const profile = MediaService.getDefaultDeviceProfile();

      expect(profile).toHaveProperty('userAgent');
      expect(profile).toHaveProperty('supportedCodecs');
      expect(profile).toHaveProperty('maxResolution');
      expect(profile).toHaveProperty('maxBitrate');
      expect(profile.supportedCodecs).toContain('h264');
      expect(profile.supportedCodecs).toContain('aac');
    });
  });

  describe('Content-Addressable Storage Integration', () => {
    it('should handle content deduplication correctly', async () => {
      // First request - creates new content
      const firstResponse = {
        id: 'session-1',
        status: 'queued',
        manifest_url: '/api/v1/content/hash123/manifest.mpd',
        provider: 'ffmpeg-pipeline',
        content_hash: 'hash123',
        content_url: '/api/v1/content/hash123/',
      };

      vi.mocked(apiCall).mockResolvedValueOnce(firstResponse);

      const result1 = await MediaService.startTranscodingSession(
        'file-1',
        'dash',
        'h264'
      );

      expect(result1.content_hash).toBe('hash123');

      // Second request for same content - reuses existing
      const secondResponse = {
        id: 'session-2',
        status: 'completed',
        manifest_url: '/api/v1/content/hash123/manifest.mpd',
        provider: 'content-reuse',
        content_hash: 'hash123',
        content_url: '/api/v1/content/hash123/',
      };

      vi.mocked(apiCall).mockResolvedValueOnce(secondResponse);

      const result2 = await MediaService.startTranscodingSession(
        'file-1',
        'dash',
        'h264'
      );

      expect(result2.content_hash).toBe('hash123');
      expect(result2.provider).toBe('content-reuse');
    });

    it('should construct correct CDN URLs from content hash', async () => {
      const response = {
        id: 'session-cdn',
        status: 'completed',
        manifest_url: '/api/v1/content/cdnhash789/manifest.mpd',
        provider: 'ffmpeg-pipeline',
        content_hash: 'cdnhash789',
        content_url: '/api/v1/content/cdnhash789/',
      };

      vi.mocked(apiCall).mockResolvedValueOnce(response);

      const result = await MediaService.startTranscodingSession(
        'file-cdn',
        'dash',
        'h264'
      );

      // Verify CDN-friendly URLs
      expect(result.content_url).toBe('/api/v1/content/cdnhash789/');
      expect(result.manifest_url).toBe('/api/v1/content/cdnhash789/manifest.mpd');
      
      // Segments would be accessed as:
      // /api/v1/content/cdnhash789/video-0001.m4s
      // /api/v1/content/cdnhash789/audio-0001.m4s
      // etc.
    });
  });
});