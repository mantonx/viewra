/**
 * Types for video player configuration utilities
 */

export interface VidstackPlayerConfig {
  // Buffering configuration
  buffer: {
    forward: number;     // Forward buffer duration in seconds
    backward: number;    // Backward buffer duration in seconds
  };
  
  // ABR configuration  
  abr: {
    enabled: boolean;
    estimateStart: number;    // Starting bandwidth estimate in bps
    upgradeTarget: number;    // Confidence threshold for quality upgrades (0-1)
    downgradeTarget: number;  // Confidence threshold for quality downgrades (0-1)
    switchInterval: number;   // Minimum interval between quality switches in seconds
  };
  
  // Seeking configuration
  seeking: {
    enableSeekAhead: boolean;
    prefetchSegments: number;
  };
  
  // Error handling
  retry: {
    maxAttempts: number;
    baseDelay: number;
    backoffFactor: number;
  };
}

export interface DeviceInfo {
  type: 'mobile' | 'desktop';
  isIOS: boolean;
  isSafari: boolean;
}

export type StreamFormat = 'dash' | 'hls';

export type DeviceType = 'mobile' | 'desktop';

export interface OptimalSource {
  src: string;
  type: string;
  config: VidstackPlayerConfig;
}