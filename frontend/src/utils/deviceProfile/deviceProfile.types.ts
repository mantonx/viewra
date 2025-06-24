/**
 * Type definitions for device profile detection
 */

export interface VideoCodecSupport {
  h264: boolean;
  hevc: boolean;
  vp8: boolean;
  vp9: boolean;
  av1: boolean;
}

export interface LocationInfo {
  // IP and network info
  ipAddress?: string;
  ipType?: 'ipv4' | 'ipv6';
  
  // Geographic location (from IP geolocation)
  country?: string;
  countryCode?: string;
  region?: string;
  city?: string;
  timezone?: string;
  
  // ISP and network info
  isp?: string;
  organization?: string;
  asn?: string;
  
  // Approximate coordinates (for CDN optimization)
  latitude?: number;
  longitude?: number;
  
  // Privacy-safe location (from browser geolocation API)
  browserLocation?: {
    latitude: number;
    longitude: number;
    accuracy: number;
    timestamp: number;
  };
}

export interface DeviceCapabilities {
  // Display capabilities
  maxResolution: string;
  screenWidth: number;
  screenHeight: number;
  pixelRatio: number;
  
  // Performance indicators
  hardwareAcceleration: boolean;
  estimatedPerformanceLevel: 'low' | 'medium' | 'high' | 'ultra';
  
  // Network capabilities
  estimatedBandwidth: number; // kbps
  connectionType: string;
  
  // Codec support
  videoCodecs: VideoCodecSupport;
  audioCodecs: string[];
  
  // Platform info
  platform: 'mobile' | 'tablet' | 'desktop' | 'tv' | 'unknown';
  os: string;
  browser: string;
  
  // Media features
  supportsHDR: boolean;
  supportsHEVC: boolean;
  supportsMSE: boolean;
  supportsHLS: boolean;
  
  // Location and network info
  location?: LocationInfo;
}

export interface DeviceProfile {
  userAgent: string;
  supportedCodecs: string[];
  maxResolution: string;
  maxBitrate: number;
  supportsHEVC: boolean;
  targetContainer: string;
  capabilities: DeviceCapabilities;
  
  // Session identifiers
  sessionId?: string;
  userId?: string;
}