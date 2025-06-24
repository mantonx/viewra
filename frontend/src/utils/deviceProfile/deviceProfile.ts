/**
 * Device profile detection and capabilities for optimizing video transcoding
 */

import type {
  VideoCodecSupport,
  DeviceCapabilities,
  DeviceProfile,
  LocationInfo,
} from './deviceProfile.types';

/**
 * Detects device capabilities for optimal video transcoding
 */
export class DeviceProfileDetector {
  private static instance: DeviceProfileDetector;
  private cachedProfile: DeviceProfile | null = null;

  static getInstance(): DeviceProfileDetector {
    if (!this.instance) {
      this.instance = new DeviceProfileDetector();
    }
    return this.instance;
  }

  /**
   * Get device profile with full capability detection
   */
  async getDeviceProfile(): Promise<DeviceProfile> {
    if (this.cachedProfile) {
      return this.cachedProfile;
    }

    const capabilities = await this.detectCapabilities();
    
    this.cachedProfile = {
      userAgent: navigator.userAgent,
      supportedCodecs: this.getSupportedCodecs(capabilities.videoCodecs),
      maxResolution: capabilities.maxResolution,
      maxBitrate: this.estimateMaxBitrate(capabilities),
      supportsHEVC: capabilities.videoCodecs.hevc,
      targetContainer: 'hls', // Always HLS per our architecture
      capabilities,
    };

    return this.cachedProfile;
  }

  /**
   * Detect comprehensive device capabilities
   */
  private async detectCapabilities(): Promise<DeviceCapabilities> {
    const [
      videoCodecs,
      networkInfo,
      displayInfo,
      platformInfo,
      mediaFeatures,
      locationInfo
    ] = await Promise.all([
      this.detectVideoCodecSupport(),
      this.detectNetworkCapabilities(),
      this.detectDisplayCapabilities(),
      this.detectPlatformInfo(),
      this.detectMediaFeatures(),
      this.detectLocationInfo()
    ]);

    const performanceLevel = this.estimatePerformanceLevel(platformInfo, displayInfo);

    return {
      ...displayInfo,
      hardwareAcceleration: this.detectHardwareAcceleration(),
      estimatedPerformanceLevel: performanceLevel,
      ...networkInfo,
      videoCodecs,
      audioCodecs: this.detectAudioCodecs(),
      ...platformInfo,
      ...mediaFeatures,
      location: locationInfo,
    };
  }

  /**
   * Detect video codec support using MediaSource API
   */
  private async detectVideoCodecSupport(): Promise<VideoCodecSupport> {
    const support: VideoCodecSupport = {
      h264: false,
      hevc: false,
      vp8: false,
      vp9: false,
      av1: false,
    };

    if (!window.MediaSource) {
      // Fallback for browsers without MSE
      return {
        h264: true, // Assume basic H.264 support
        hevc: false,
        vp8: false,
        vp9: false,
        av1: false,
      };
    }

    // Test codec support
    const codecTests = [
      { codec: 'h264', mimeTypes: [
        'video/mp4; codecs="avc1.42E01E"',
        'video/mp4; codecs="avc1.640028"'
      ]},
      { codec: 'hevc', mimeTypes: [
        'video/mp4; codecs="hev1.1.6.L93.B0"',
        'video/mp4; codecs="hvc1.1.6.L93.B0"'
      ]},
      { codec: 'vp8', mimeTypes: [
        'video/webm; codecs="vp8"'
      ]},
      { codec: 'vp9', mimeTypes: [
        'video/webm; codecs="vp9"'
      ]},
      { codec: 'av1', mimeTypes: [
        'video/mp4; codecs="av01.0.05M.08"',
        'video/webm; codecs="av01.0.05M.08"'
      ]},
    ];

    for (const test of codecTests) {
      for (const mimeType of test.mimeTypes) {
        if (MediaSource.isTypeSupported(mimeType)) {
          support[test.codec as keyof VideoCodecSupport] = true;
          break;
        }
      }
    }

    return support;
  }

  /**
   * Detect network capabilities
   */
  private async detectNetworkCapabilities(): Promise<{
    estimatedBandwidth: number;
    connectionType: string;
  }> {
    // Use Network Information API if available
    const connection = (navigator as any).connection || 
                      (navigator as any).mozConnection || 
                      (navigator as any).webkitConnection;

    let estimatedBandwidth = 5000; // Default 5 Mbps
    let connectionType = 'unknown';

    if (connection) {
      connectionType = connection.effectiveType || connection.type || 'unknown';
      
      // Estimate bandwidth based on connection type
      switch (connection.effectiveType) {
        case 'slow-2g':
          estimatedBandwidth = 50;
          break;
        case '2g':
          estimatedBandwidth = 250;
          break;
        case '3g':
          estimatedBandwidth = 1500;
          break;
        case '4g':
          estimatedBandwidth = 10000;
          break;
        default:
          // Use downlink if available
          if (connection.downlink) {
            estimatedBandwidth = connection.downlink * 1000; // Convert Mbps to kbps
          }
      }
    }

    return {
      estimatedBandwidth,
      connectionType,
    };
  }

  /**
   * Detect display capabilities
   */
  private detectDisplayCapabilities(): {
    maxResolution: string;
    screenWidth: number;
    screenHeight: number;
    pixelRatio: number;
  } {
    const screenWidth = screen.width;
    const screenHeight = screen.height;
    const pixelRatio = window.devicePixelRatio || 1;

    // Calculate effective resolution
    const effectiveWidth = screenWidth * pixelRatio;
    const effectiveHeight = screenHeight * pixelRatio;
    const maxDimension = Math.max(effectiveWidth, effectiveHeight);

    let maxResolution: string;
    if (maxDimension >= 3840) {
      maxResolution = '2160p'; // 4K
    } else if (maxDimension >= 2560) {
      maxResolution = '1440p'; // 2K
    } else if (maxDimension >= 1920) {
      maxResolution = '1080p'; // Full HD
    } else if (maxDimension >= 1280) {
      maxResolution = '720p';  // HD
    } else if (maxDimension >= 854) {
      maxResolution = '480p';  // SD
    } else {
      maxResolution = '360p';  // Low res
    }

    return {
      maxResolution,
      screenWidth,
      screenHeight,
      pixelRatio,
    };
  }

  /**
   * Detect platform information
   */
  private detectPlatformInfo(): {
    platform: 'mobile' | 'tablet' | 'desktop' | 'tv' | 'unknown';
    os: string;
    browser: string;
  } {
    const userAgent = navigator.userAgent.toLowerCase();
    
    // Detect platform
    let platform: 'mobile' | 'tablet' | 'desktop' | 'tv' | 'unknown' = 'unknown';
    
    if (userAgent.includes('tv') || userAgent.includes('roku') || userAgent.includes('appletv')) {
      platform = 'tv';
    } else if (userAgent.includes('mobile') || userAgent.includes('android') && !userAgent.includes('tablet')) {
      platform = 'mobile';
    } else if (userAgent.includes('tablet') || userAgent.includes('ipad')) {
      platform = 'tablet';
    } else {
      platform = 'desktop';
    }

    // Detect OS
    let os = 'unknown';
    if (userAgent.includes('windows')) os = 'windows';
    else if (userAgent.includes('mac')) os = 'macos';
    else if (userAgent.includes('linux')) os = 'linux';
    else if (userAgent.includes('android')) os = 'android';
    else if (userAgent.includes('ios') || userAgent.includes('iphone') || userAgent.includes('ipad')) os = 'ios';

    // Detect browser
    let browser = 'unknown';
    if (userAgent.includes('chrome')) browser = 'chrome';
    else if (userAgent.includes('firefox')) browser = 'firefox';
    else if (userAgent.includes('safari') && !userAgent.includes('chrome')) browser = 'safari';
    else if (userAgent.includes('edge')) browser = 'edge';
    else if (userAgent.includes('opera')) browser = 'opera';

    return { platform, os, browser };
  }

  /**
   * Detect media-specific features
   */
  private detectMediaFeatures(): {
    supportsHDR: boolean;
    supportsMSE: boolean;
    supportsHLS: boolean;
  } {
    // HDR support detection
    const supportsHDR = 'screen' in window && 'colorGamut' in screen && 
                       (screen as any).colorGamut === 'p3' ||
                       window.matchMedia && window.matchMedia('(color-gamut: p3)').matches;

    // MSE support
    const supportsMSE = !!window.MediaSource;

    // Native HLS support (mainly Safari/iOS)
    const supportsHLS = !!(document.createElement('video').canPlayType('application/vnd.apple.mpegurl'));

    return {
      supportsHDR: !!supportsHDR,
      supportsMSE,
      supportsHLS,
    };
  }

  /**
   * Detect hardware acceleration capabilities
   */
  private detectHardwareAcceleration(): boolean {
    // Basic detection - more sophisticated methods could be added
    const canvas = document.createElement('canvas');
    const gl = canvas.getContext('webgl') || canvas.getContext('experimental-webgl');
    
    if (!gl) return false;

    const debugInfo = gl.getExtension('WEBGL_debug_renderer_info');
    if (debugInfo) {
      const renderer = gl.getParameter(debugInfo.UNMASKED_RENDERER_WEBGL);
      // Look for GPU indicators
      return !renderer.toLowerCase().includes('software');
    }

    return true; // Assume hardware acceleration if we can't detect otherwise
  }

  /**
   * Estimate performance level based on various factors
   */
  private estimatePerformanceLevel(
    platformInfo: { platform: string; os: string; browser: string },
    displayInfo: { screenWidth: number; screenHeight: number; pixelRatio: number }
  ): 'low' | 'medium' | 'high' | 'ultra' {
    let score = 0;

    // Platform scoring
    if (platformInfo.platform === 'desktop') score += 3;
    else if (platformInfo.platform === 'tablet') score += 2;
    else if (platformInfo.platform === 'mobile') score += 1;
    else if (platformInfo.platform === 'tv') score += 2;

    // Resolution scoring
    const totalPixels = displayInfo.screenWidth * displayInfo.screenHeight * displayInfo.pixelRatio;
    if (totalPixels >= 3840 * 2160) score += 4; // 4K+
    else if (totalPixels >= 1920 * 1080) score += 3; // 1080p+
    else if (totalPixels >= 1280 * 720) score += 2; // 720p+
    else score += 1;

    // Memory heuristic (if available)
    if ('memory' in performance && (performance as any).memory) {
      const memoryGB = (performance as any).memory.jsHeapSizeLimit / (1024 * 1024 * 1024);
      if (memoryGB >= 8) score += 2;
      else if (memoryGB >= 4) score += 1;
    }

    // Core count heuristic
    if ('hardwareConcurrency' in navigator) {
      const cores = navigator.hardwareConcurrency;
      if (cores >= 8) score += 2;
      else if (cores >= 4) score += 1;
    }

    // Score to performance level mapping
    if (score >= 10) return 'ultra';
    if (score >= 7) return 'high';
    if (score >= 4) return 'medium';
    return 'low';
  }

  /**
   * Get supported codec list for backend
   */
  private getSupportedCodecs(videoCodecs: VideoCodecSupport): string[] {
    const codecs: string[] = [];
    
    if (videoCodecs.h264) codecs.push('h264');
    if (videoCodecs.hevc) codecs.push('hevc');
    if (videoCodecs.vp8) codecs.push('vp8');
    if (videoCodecs.vp9) codecs.push('vp9');
    if (videoCodecs.av1) codecs.push('av1');
    
    // Always include AAC audio
    codecs.push('aac');
    
    return codecs;
  }

  /**
   * Detect supported audio codecs
   */
  private detectAudioCodecs(): string[] {
    const audio = document.createElement('audio');
    const codecs: string[] = [];

    // Test common audio formats
    if (audio.canPlayType('audio/mp4; codecs="mp4a.40.2"')) codecs.push('aac');
    if (audio.canPlayType('audio/mpeg')) codecs.push('mp3');
    if (audio.canPlayType('audio/ogg; codecs="vorbis"')) codecs.push('vorbis');
    if (audio.canPlayType('audio/webm; codecs="opus"')) codecs.push('opus');

    return codecs;
  }

  /**
   * Estimate maximum bitrate the device can handle
   */
  private estimateMaxBitrate(capabilities: DeviceCapabilities): number {
    let maxBitrate = capabilities.estimatedBandwidth * 0.8; // Use 80% of estimated bandwidth

    // Adjust based on performance level
    switch (capabilities.estimatedPerformanceLevel) {
      case 'low':
        maxBitrate = Math.min(maxBitrate, 1500); // 1.5 Mbps max
        break;
      case 'medium':
        maxBitrate = Math.min(maxBitrate, 5000); // 5 Mbps max
        break;
      case 'high':
        maxBitrate = Math.min(maxBitrate, 15000); // 15 Mbps max
        break;
      case 'ultra':
        maxBitrate = Math.min(maxBitrate, 50000); // 50 Mbps max
        break;
    }

    // Platform-specific adjustments
    if (capabilities.platform === 'mobile') {
      maxBitrate = Math.min(maxBitrate, 8000); // Mobile bandwidth conservation
    }

    return Math.max(maxBitrate, 500); // Minimum 500 kbps
  }

  /**
   * Detect location and network information
   */
  private async detectLocationInfo(): Promise<LocationInfo | undefined> {
    try {
      const locationInfo: LocationInfo = {};
      
      // Get IP-based location info from multiple sources
      const ipLocationData = await Promise.allSettled([
        this.getIPLocationFromService('https://ipapi.co/json/'),
        this.getIPLocationFromService('https://ip.nf/me.json'),
        this.getLocationFromCloudflare(),
      ]);
      
      // Use the first successful result
      for (const result of ipLocationData) {
        if (result.status === 'fulfilled' && result.value) {
          Object.assign(locationInfo, result.value);
          break;
        }
      }
      
      // Optionally get browser geolocation (with user permission)
      const browserLocation = await this.getBrowserLocation();
      if (browserLocation) {
        locationInfo.browserLocation = browserLocation;
      }
      
      return Object.keys(locationInfo).length > 0 ? locationInfo : undefined;
    } catch (error) {
      console.warn('Failed to detect location info:', error);
      return undefined;
    }
  }

  /**
   * Get IP location from various services
   */
  private async getIPLocationFromService(url: string): Promise<LocationInfo | null> {
    try {
      const response = await fetch(url, {
        method: 'GET',
        headers: {
          'Accept': 'application/json',
        },
        // Timeout after 5 seconds
        signal: AbortSignal.timeout(5000),
      });
      
      if (!response.ok) return null;
      
      const data = await response.json();
      
      // Normalize different API responses
      return this.normalizeLocationData(data, url);
    } catch (error) {
      console.debug(`Failed to get location from ${url}:`, error);
      return null;
    }
  }

  /**
   * Get location from Cloudflare headers (if available)
   */
  private async getLocationFromCloudflare(): Promise<LocationInfo | null> {
    try {
      // Cloudflare provides location headers in some cases
      const response = await fetch('/cdn-cgi/trace', {
        method: 'GET',
        signal: AbortSignal.timeout(3000),
      });
      
      if (!response.ok) return null;
      
      const text = await response.text();
      const data: Record<string, string> = {};
      
      text.split('\n').forEach(line => {
        const [key, value] = line.split('=');
        if (key && value) {
          data[key] = value;
        }
      });
      
      if (data.colo || data.country) {
        return {
          ipAddress: data.ip,
          country: data.country,
          countryCode: data.country,
          region: data.region,
          city: data.city,
        };
      }
      
      return null;
    } catch (error) {
      console.debug('Failed to get Cloudflare location:', error);
      return null;
    }
  }

  /**
   * Normalize location data from different APIs
   */
  private normalizeLocationData(data: any, source: string): LocationInfo {
    const location: LocationInfo = {};
    
    // Common fields across most APIs
    if (data.ip) location.ipAddress = data.ip;
    if (data.country || data.country_name) location.country = data.country || data.country_name;
    if (data.country_code || data.countryCode) location.countryCode = data.country_code || data.countryCode;
    if (data.region || data.region_name) location.region = data.region || data.region_name;
    if (data.city) location.city = data.city;
    if (data.timezone) location.timezone = data.timezone;
    if (data.isp || data.org) location.isp = data.isp || data.org;
    if (data.as || data.asn) location.asn = data.as || data.asn;
    if (data.latitude || data.lat) location.latitude = parseFloat(data.latitude || data.lat);
    if (data.longitude || data.lon) location.longitude = parseFloat(data.longitude || data.lon);
    
    // Detect IP type
    if (location.ipAddress) {
      location.ipType = location.ipAddress.includes(':') ? 'ipv6' : 'ipv4';
    }
    
    return location;
  }

  /**
   * Get browser geolocation (requires user permission)
   */
  private async getBrowserLocation(): Promise<LocationInfo['browserLocation'] | null> {
    try {
      if (!navigator.geolocation) return null;
      
      return new Promise((resolve) => {
        // Set a short timeout to avoid blocking
        const timeout = setTimeout(() => resolve(null), 2000);
        
        navigator.geolocation.getCurrentPosition(
          (position) => {
            clearTimeout(timeout);
            resolve({
              latitude: position.coords.latitude,
              longitude: position.coords.longitude,
              accuracy: position.coords.accuracy,
              timestamp: Date.now(),
            });
          },
          () => {
            clearTimeout(timeout);
            resolve(null); // User denied or error occurred
          },
          {
            timeout: 2000,
            enableHighAccuracy: false, // Faster, less battery drain
          }
        );
      });
    } catch (error) {
      console.debug('Browser geolocation not available:', error);
      return null;
    }
  }

  /**
   * Clear cached profile (useful for testing or when capabilities change)
   */
  clearCache(): void {
    this.cachedProfile = null;
  }
}

/**
 * Convenience function to get device profile
 */
export async function getDeviceProfile(): Promise<DeviceProfile> {
  return DeviceProfileDetector.getInstance().getDeviceProfile();
}

/**
 * Get a simplified device profile for quick decisions
 */
export function getBasicDeviceProfile(): DeviceProfile {
  const userAgent = navigator.userAgent;
  const isMobile = /mobile|android|ios|iphone|ipad/i.test(userAgent);
  const isApple = /safari|ios|iphone|ipad|mac/i.test(userAgent);
  
  return {
    userAgent,
    supportedCodecs: ['h264', 'aac'],
    maxResolution: isMobile ? '720p' : '1080p',
    maxBitrate: isMobile ? 3000 : 8000,
    supportsHEVC: isApple,
    targetContainer: 'hls',
    capabilities: {
      maxResolution: isMobile ? '720p' : '1080p',
      screenWidth: screen.width,
      screenHeight: screen.height,
      pixelRatio: window.devicePixelRatio || 1,
      hardwareAcceleration: true,
      estimatedPerformanceLevel: isMobile ? 'medium' : 'high',
      estimatedBandwidth: isMobile ? 3000 : 8000,
      connectionType: 'unknown',
      videoCodecs: {
        h264: true,
        hevc: isApple,
        vp8: false,
        vp9: false,
        av1: false,
      },
      audioCodecs: ['aac'],
      platform: isMobile ? 'mobile' : 'desktop',
      os: 'unknown',
      browser: 'unknown',
      supportsHDR: false,
      supportsHEVC: isApple,
      supportsMSE: !!window.MediaSource,
      supportsHLS: !!(document.createElement('video').canPlayType('application/vnd.apple.mpegurl')),
    },
  };
}