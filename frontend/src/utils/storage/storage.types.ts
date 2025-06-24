/**
 * Type definitions for storage utilities
 */

/**
 * Player settings interface
 */
export interface PlayerSettings {
  autoplay: boolean;
  subtitlesEnabled: boolean;
  defaultQuality: string;
  skipIntroSeconds: number;
  skipOutroSeconds: number;
  seekStepSeconds: number;
}

/**
 * Storage usage information
 */
export interface StorageUsage {
  used: number;
  available: number;
  percentage: number;
}