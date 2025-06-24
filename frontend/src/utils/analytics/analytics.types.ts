/**
 * Analytics and session tracking types
 */

export interface PlaybackSessionInfo {
  sessionId: string;
  mediaId: string;
  mediaType: 'movie' | 'episode';
  userId?: string;
  timestamp: number;
  
  // Playback details
  duration: number;
  currentTime: number;
  streamUrl: string;
  
  // Device info
  deviceProfile: {
    platform: string;
    browser: string;
    os: string;
    screenResolution: string;
    maxResolution: string;
    estimatedBandwidth: number;
    connectionType: string;
    
    // Location and network info
    ipAddress?: string;
    ipType?: 'ipv4' | 'ipv6';
    country?: string;
    countryCode?: string;
    region?: string;
    city?: string;
    timezone?: string;
    isp?: string;
    
    // Approximate location for CDN optimization
    latitude?: number;
    longitude?: number;
  };
  
  // Performance metrics
  buffering: {
    totalBufferingTime: number;
    bufferingEvents: number;
    lastBufferingTime?: number;
  };
  
  seeking: {
    totalSeeks: number;
    seekAheadRequests: number;
    averageSeekTime: number;
  };
  
  quality: {
    qualityChanges: number;
    currentBitrate?: number;
    averageBitrate?: number;
  };
  
  errors: Array<{
    timestamp: number;
    type: string;
    message: string;
    code?: number;
  }>;
}

export interface SessionEvent {
  sessionId: string;
  timestamp: number;
  eventType: 'play' | 'pause' | 'seek' | 'buffer_start' | 'buffer_end' | 'quality_change' | 'error' | 'ended';
  data?: Record<string, unknown>;
}

export interface SessionAnalytics {
  // Real-time metrics
  isPlaying: boolean;
  currentTime: number;
  duration: number;
  bufferingTime: number;
  
  // Quality metrics
  currentQuality: string;
  qualityChanges: number;
  averageBitrate: number;
  
  // User behavior
  totalSeeks: number;
  watchTime: number;
  engagementScore: number; // 0-100 based on watching behavior
  
  // Performance
  startupTime: number;
  rebufferingRatio: number; // % of time spent buffering
  errorCount: number;
}