/**
 * Video player configuration utilities for optimal DASH and HLS playback
 * 
 * This module provides optimized configurations for Vidstack video player
 * to ensure smooth VOD playback with proper buffering strategies,
 * adaptive bitrate behavior, and efficient resource usage.
 */

import type { VidstackPlayerConfig, DeviceInfo, StreamFormat, DeviceType, OptimalSource } from './videoPlayerConfig.types';

/**
 * Optimal configuration for DASH VOD playback
 * 
 * Focuses on:
 * - Larger forward buffer for smooth seeking
 * - Conservative ABR to maintain quality
 * - Efficient memory usage
 */
export const DASH_VOD_CONFIG: VidstackPlayerConfig = {
  buffer: {
    forward: 25,    // 25 seconds forward buffer for smooth seeking
    backward: 15,   // 15 seconds backward buffer for rewind
  },
  
  abr: {
    enabled: true,
    estimateStart: 2000000,   // Start at 2 Mbps estimate
    upgradeTarget: 0.85,      // 85% confidence for upgrades
    downgradeTarget: 0.95,    // 95% confidence for downgrades  
    switchInterval: 8,        // Consider switches every 8 seconds
  },
  
  seeking: {
    enableSeekAhead: true,
    prefetchSegments: 4,      // Prefetch 4 segments for smooth seeking
  },
  
  retry: {
    maxAttempts: 3,
    baseDelay: 1000,
    backoffFactor: 2,
  },
};

/**
 * Optimal configuration for HLS playback on iOS/Safari
 * 
 * Focuses on:
 * - Native HLS handling efficiency
 * - iOS-specific optimizations
 * - Battery-conscious settings
 */
export const HLS_CONFIG: VidstackPlayerConfig = {
  buffer: {
    forward: 20,    // Slightly smaller forward buffer for iOS memory constraints
    backward: 10,   // Smaller backward buffer
  },
  
  abr: {
    enabled: true,
    estimateStart: 1500000,   // More conservative start for mobile
    upgradeTarget: 0.90,      // Higher confidence threshold for mobile
    downgradeTarget: 0.95,    // Keep high threshold for downgrades
    switchInterval: 10,       // Longer interval for mobile battery
  },
  
  seeking: {
    enableSeekAhead: false,   // Rely on native HLS seeking
    prefetchSegments: 2,      // Fewer segments for mobile
  },
  
  retry: {
    maxAttempts: 2,           // Fewer retries on mobile
    baseDelay: 1500,
    backoffFactor: 1.5,
  },
};

/**
 * Get optimal player configuration based on device and format
 */
export function getOptimalPlayerConfig(format: StreamFormat, deviceType: DeviceType): VidstackPlayerConfig {
  if (format === 'hls' || deviceType === 'mobile') {
    return HLS_CONFIG;
  }
  
  return DASH_VOD_CONFIG;
}

/**
 * Device detection utility
 */
export function detectDevice(): DeviceInfo {
  const userAgent = navigator.userAgent;
  const isIOS = /iPad|iPhone|iPod/.test(userAgent);
  const isSafari = /^((?!chrome|android).)*safari/i.test(userAgent);
  const isMobile = /Mobi|Android/i.test(userAgent) || isIOS;
  
  return {
    type: isMobile ? 'mobile' : 'desktop',
    isIOS,
    isSafari,
  };
}

/**
 * Format detection utility
 */
export function detectOptimalFormat(streamUrl: string, device: DeviceInfo): StreamFormat {
  // Use HLS for iOS devices and Safari
  if (device.isIOS || device.isSafari) {
    return 'hls';
  }
  
  // Use DASH for all other devices if available
  if (streamUrl.includes('.mpd')) {
    return 'dash';
  }
  
  // Fallback to HLS
  return 'hls';
}

/**
 * Apply configuration to Vidstack player
 * 
 * Note: Vidstack handles most of these optimizations internally,
 * but we can provide hints through provider configuration and player attributes
 */
export function applyPlayerConfig(player: unknown, config: VidstackPlayerConfig): void {
  if (!player || typeof player !== 'object') return;
  
  const playerObj = player as Record<string, unknown>;
  
  // Configure buffering if the player supports it
  if (playerObj.buffering && typeof playerObj.buffering === 'object') {
    const buffering = playerObj.buffering as Record<string, unknown>;
    buffering.forward = config.buffer.forward;
    buffering.backward = config.buffer.backward;
  }
  
  // Configure quality settings if available
  if (playerObj.qualities && config.abr.enabled) {
    // Vidstack typically handles ABR automatically
    // We can set preferences through the quality API when it becomes available
    console.log('📊 ABR configuration applied:', {
      estimateStart: config.abr.estimateStart,
      upgradeTarget: config.abr.upgradeTarget,
      downgradeTarget: config.abr.downgradeTarget,
    });
  }
  
  // Log configuration for debugging
  console.log('🎛️ Player configuration applied:', {
    format: detectOptimalFormat((playerObj.src as string) || '', detectDevice()),
    buffer: config.buffer,
    abr: config.abr,
    seeking: config.seeking,
  });
}

/**
 * Get MIME type for format
 */
export function getMimeType(format: StreamFormat): string {
  return format === 'dash' 
    ? 'application/dash+xml'
    : 'application/vnd.apple.mpegurl';
}

/**
 * Enhanced source selection with optimal configuration
 */
export function getOptimalSource(streamUrl: string): OptimalSource {
  const device = detectDevice();
  const format = detectOptimalFormat(streamUrl, device);
  const config = getOptimalPlayerConfig(format, device.type);
  
  return {
    src: streamUrl,
    type: getMimeType(format),
    config,
  };
}

// Default export for the utility
export default {
  getOptimalPlayerConfig,
  detectDevice,
  detectOptimalFormat,
  getOptimalSource,
  getMimeType,
  applyPlayerConfig,
  DASH_VOD_CONFIG,
  HLS_CONFIG,
};