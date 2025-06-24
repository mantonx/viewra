/**
 * Device Profile Detection Utilities
 * 
 * Provides comprehensive device capability detection for optimizing
 * video transcoding and streaming based on client capabilities.
 */

export type {
  VideoCodecSupport,
  DeviceCapabilities,
  DeviceProfile,
  LocationInfo,
} from './deviceProfile.types';

export {
  DeviceProfileDetector,
  getDeviceProfile,
  getBasicDeviceProfile,
} from './deviceProfile';