/**
 * Tests for video player configuration utilities
 */

import { describe, it, expect, vi } from 'vitest';
import {
  getOptimalPlayerConfig,
  detectDevice,
  detectOptimalFormat,
  getOptimalSource,
  getMimeType,
  applyPlayerConfig,
  DASH_VOD_CONFIG,
  HLS_CONFIG,
} from './videoPlayerConfig';
import type { VidstackPlayerConfig } from './videoPlayerConfig.types';

// Mock navigator
const mockNavigator = (userAgent: string) => {
  Object.defineProperty(window, 'navigator', {
    value: { userAgent },
    writable: true,
  });
};

describe('videoPlayerConfig', () => {
  describe('detectDevice', () => {
    it('detects iOS devices correctly', () => {
      mockNavigator('Mozilla/5.0 (iPhone; CPU iPhone OS 14_0 like Mac OS X)');
      const device = detectDevice();
      
      expect(device.type).toBe('mobile');
      expect(device.isIOS).toBe(true);
      expect(device.isSafari).toBe(false);
    });

    it('detects Safari correctly', () => {
      mockNavigator('Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/14.0 Safari/605.1.15');
      const device = detectDevice();
      
      expect(device.type).toBe('desktop');
      expect(device.isIOS).toBe(false);
      expect(device.isSafari).toBe(true);
    });

    it('detects desktop Chrome correctly', () => {
      mockNavigator('Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36');
      const device = detectDevice();
      
      expect(device.type).toBe('desktop');
      expect(device.isIOS).toBe(false);
      expect(device.isSafari).toBe(false);
    });

    it('detects Android devices correctly', () => {
      mockNavigator('Mozilla/5.0 (Linux; Android 10; SM-G975F) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Mobile Safari/537.36');
      const device = detectDevice();
      
      expect(device.type).toBe('mobile');
      expect(device.isIOS).toBe(false);
      expect(device.isSafari).toBe(false);
    });
  });

  describe('detectOptimalFormat', () => {
    it('returns HLS for iOS devices', () => {
      mockNavigator('Mozilla/5.0 (iPhone; CPU iPhone OS 14_0 like Mac OS X)');
      const device = detectDevice();
      const format = detectOptimalFormat('http://example.com/video.m3u8', device);
      
      expect(format).toBe('hls');
    });

    it('returns HLS for Safari', () => {
      mockNavigator('Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/14.0 Safari/605.1.15');
      const device = detectDevice();
      const format = detectOptimalFormat('http://example.com/video.mpd', device);
      
      expect(format).toBe('hls');
    });

    it('returns DASH for desktop with MPD URL', () => {
      mockNavigator('Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36');
      const device = detectDevice();
      const format = detectOptimalFormat('http://example.com/video.mpd', device);
      
      expect(format).toBe('dash');
    });

    it('falls back to HLS for desktop without MPD URL', () => {
      mockNavigator('Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36');
      const device = detectDevice();
      const format = detectOptimalFormat('http://example.com/video.m3u8', device);
      
      expect(format).toBe('hls');
    });
  });

  describe('getOptimalPlayerConfig', () => {
    it('returns DASH config for DASH format on desktop', () => {
      const config = getOptimalPlayerConfig('dash', 'desktop');
      
      expect(config).toEqual(DASH_VOD_CONFIG);
      expect(config.buffer.forward).toBe(25);
      expect(config.abr.estimateStart).toBe(2000000);
      expect(config.seeking.enableSeekAhead).toBe(true);
    });

    it('returns HLS config for HLS format', () => {
      const config = getOptimalPlayerConfig('hls', 'desktop');
      
      expect(config).toEqual(HLS_CONFIG);
      expect(config.buffer.forward).toBe(20);
      expect(config.abr.estimateStart).toBe(1500000);
      expect(config.seeking.enableSeekAhead).toBe(false);
    });

    it('returns HLS config for mobile devices', () => {
      const config = getOptimalPlayerConfig('dash', 'mobile');
      
      expect(config).toEqual(HLS_CONFIG);
      expect(config.buffer.forward).toBe(20);
      expect(config.seeking.prefetchSegments).toBe(2);
    });
  });

  describe('getMimeType', () => {
    it('returns correct MIME type for DASH', () => {
      expect(getMimeType('dash')).toBe('application/dash+xml');
    });

    it('returns correct MIME type for HLS', () => {
      expect(getMimeType('hls')).toBe('application/vnd.apple.mpegurl');
    });
  });

  describe('getOptimalSource', () => {
    it('returns complete source configuration for DASH', () => {
      mockNavigator('Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36');
      const source = getOptimalSource('http://example.com/video.mpd');
      
      expect(source.src).toBe('http://example.com/video.mpd');
      expect(source.type).toBe('application/dash+xml');
      expect(source.config).toEqual(DASH_VOD_CONFIG);
    });

    it('returns complete source configuration for HLS', () => {
      mockNavigator('Mozilla/5.0 (iPhone; CPU iPhone OS 14_0 like Mac OS X)');
      const source = getOptimalSource('http://example.com/video.m3u8');
      
      expect(source.src).toBe('http://example.com/video.m3u8');
      expect(source.type).toBe('application/vnd.apple.mpegurl');
      expect(source.config).toEqual(HLS_CONFIG);
    });
  });

  describe('applyPlayerConfig', () => {
    it('handles null/undefined player safely', () => {
      const consoleSpy = vi.spyOn(console, 'log').mockImplementation(() => {});
      
      expect(() => {
        applyPlayerConfig(null, DASH_VOD_CONFIG);
        applyPlayerConfig(undefined, DASH_VOD_CONFIG);
        applyPlayerConfig('string', DASH_VOD_CONFIG);
      }).not.toThrow();
      
      consoleSpy.mockRestore();
    });

    it('applies buffering configuration when supported', () => {
      const consoleSpy = vi.spyOn(console, 'log').mockImplementation(() => {});
      const mockPlayer = {
        buffering: {},
        src: 'http://example.com/video.mpd',
      };
      
      applyPlayerConfig(mockPlayer, DASH_VOD_CONFIG);
      
      expect(mockPlayer.buffering).toEqual({
        forward: 25,
        backward: 15,
      });
      
      consoleSpy.mockRestore();
    });

    it('logs configuration for debugging', () => {
      // Set up desktop Chrome user agent for DASH format
      mockNavigator('Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36');
      
      const consoleSpy = vi.spyOn(console, 'log').mockImplementation(() => {});
      const mockPlayer = {
        src: 'http://example.com/video.mpd',
      };
      
      applyPlayerConfig(mockPlayer, DASH_VOD_CONFIG);
      
      expect(consoleSpy).toHaveBeenCalledWith(
        '🎛️ Player configuration applied:',
        expect.objectContaining({
          format: 'dash',
          buffer: DASH_VOD_CONFIG.buffer,
          abr: DASH_VOD_CONFIG.abr,
          seeking: DASH_VOD_CONFIG.seeking,
        })
      );
      
      consoleSpy.mockRestore();
    });

    it('logs ABR configuration when quality API is available', () => {
      // Set up desktop Chrome user agent for DASH format
      mockNavigator('Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36');
      
      const consoleSpy = vi.spyOn(console, 'log').mockImplementation(() => {});
      const mockPlayer = {
        qualities: {},
        src: 'http://example.com/video.mpd',
      };
      
      applyPlayerConfig(mockPlayer, DASH_VOD_CONFIG);
      
      expect(consoleSpy).toHaveBeenCalledWith(
        '📊 ABR configuration applied:',
        expect.objectContaining({
          estimateStart: DASH_VOD_CONFIG.abr.estimateStart,
          upgradeTarget: DASH_VOD_CONFIG.abr.upgradeTarget,
          downgradeTarget: DASH_VOD_CONFIG.abr.downgradeTarget,
        })
      );
      
      consoleSpy.mockRestore();
    });
  });

  describe('configuration constants', () => {
    it('has valid DASH VOD configuration', () => {
      expect(DASH_VOD_CONFIG).toEqual({
        buffer: {
          forward: 25,
          backward: 15,
        },
        abr: {
          enabled: true,
          estimateStart: 2000000,
          upgradeTarget: 0.85,
          downgradeTarget: 0.95,
          switchInterval: 8,
        },
        seeking: {
          enableSeekAhead: true,
          prefetchSegments: 4,
        },
        retry: {
          maxAttempts: 3,
          baseDelay: 1000,
          backoffFactor: 2,
        },
      });
    });

    it('has valid HLS configuration', () => {
      expect(HLS_CONFIG).toEqual({
        buffer: {
          forward: 20,
          backward: 10,
        },
        abr: {
          enabled: true,
          estimateStart: 1500000,
          upgradeTarget: 0.90,
          downgradeTarget: 0.95,
          switchInterval: 10,
        },
        seeking: {
          enableSeekAhead: false,
          prefetchSegments: 2,
        },
        retry: {
          maxAttempts: 2,
          baseDelay: 1500,
          backoffFactor: 1.5,
        },
      });
    });
  });
});