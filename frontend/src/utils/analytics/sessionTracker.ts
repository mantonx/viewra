/**
 * Session tracking for playback analytics and debugging
 */

import type { DeviceProfile } from '../deviceProfile';
import type { PlaybackSessionInfo, SessionEvent, SessionAnalytics } from './analytics.types';
import { buildApiUrl } from '../../constants/api';

export class PlaybackSessionTracker {
  private sessionId: string;
  private mediaId: string;
  private mediaType: 'movie' | 'episode';
  private deviceProfile: DeviceProfile;
  private startTime: number;
  private events: SessionEvent[] = [];
  
  // Performance tracking
  private bufferingStartTime: number | null = null;
  private totalBufferingTime = 0;
  private bufferingEvents = 0;
  private seeks = 0;
  private seekTimes: number[] = [];
  private qualityChanges = 0;
  private errors: Array<{ timestamp: number; type: string; message: string; code?: number }> = [];
  
  // Real-time state
  private currentTime = 0;
  private duration = 0;
  private isPlaying = false;
  private currentQuality = '';
  private currentBitrate = 0;
  private bitrateHistory: number[] = [];
  
  // Sync interval
  private syncInterval: NodeJS.Timeout | null = null;
  private readonly SYNC_INTERVAL = 30000; // Send updates every 30 seconds
  
  constructor(
    sessionId: string,
    mediaId: string,
    mediaType: 'movie' | 'episode',
    deviceProfile: DeviceProfile
  ) {
    this.sessionId = sessionId;
    this.mediaId = mediaId;
    this.mediaType = mediaType;
    this.deviceProfile = deviceProfile;
    this.startTime = Date.now();
    
    // Start periodic sync
    this.startPeriodicSync();
    
    console.log('📊 Started session tracking:', {
      sessionId,
      mediaId,
      mediaType,
      platform: deviceProfile.capabilities.platform
    });
  }

  /**
   * Track playback events
   */
  trackEvent(eventType: SessionEvent['eventType'], data?: Record<string, unknown>): void {
    const event: SessionEvent = {
      sessionId: this.sessionId,
      timestamp: Date.now(),
      eventType,
      data,
    };
    
    this.events.push(event);
    
    // Handle specific event types
    switch (eventType) {
      case 'play':
        this.isPlaying = true;
        break;
      case 'pause':
        this.isPlaying = false;
        break;
      case 'seek':
        this.seeks++;
        if (data?.seekTime) {
          this.seekTimes.push(data.seekTime);
        }
        break;
      case 'buffer_start':
        this.bufferingStartTime = Date.now();
        this.bufferingEvents++;
        break;
      case 'buffer_end':
        if (this.bufferingStartTime) {
          this.totalBufferingTime += Date.now() - this.bufferingStartTime;
          this.bufferingStartTime = null;
        }
        break;
      case 'quality_change':
        this.qualityChanges++;
        if (data?.quality) this.currentQuality = data.quality;
        if (data?.bitrate) {
          this.currentBitrate = data.bitrate;
          this.bitrateHistory.push(data.bitrate);
        }
        break;
      case 'error':
        this.errors.push({
          timestamp: Date.now(),
          type: data?.type || 'unknown',
          message: data?.message || 'Unknown error',
          code: data?.code,
        });
        break;
    }
    
    console.log(`📊 Event tracked: ${eventType}`, data);
  }

  /**
   * Update current playback state
   */
  updatePlaybackState(currentTime: number, duration: number): void {
    this.currentTime = currentTime;
    this.duration = duration;
  }

  /**
   * Get current session analytics
   */
  getSessionAnalytics(): SessionAnalytics {
    const now = Date.now();
    const sessionDuration = now - this.startTime;
    const watchTime = this.isPlaying ? sessionDuration : sessionDuration - this.totalBufferingTime;
    
    // Calculate engagement score (0-100)
    const engagementScore = this.calculateEngagementScore(watchTime, sessionDuration);
    
    // Calculate rebuffering ratio
    const rebufferingRatio = sessionDuration > 0 ? (this.totalBufferingTime / sessionDuration) * 100 : 0;
    
    // Calculate average bitrate
    const averageBitrate = this.bitrateHistory.length > 0 
      ? this.bitrateHistory.reduce((sum, br) => sum + br, 0) / this.bitrateHistory.length
      : this.currentBitrate;
    
    // Calculate average seek time (computed but not used in return)
    // const averageSeekTime = this.seekTimes.length > 0
    //   ? this.seekTimes.reduce((sum, time) => sum + time, 0) / this.seekTimes.length
    //   : 0;

    return {
      isPlaying: this.isPlaying,
      currentTime: this.currentTime,
      duration: this.duration,
      bufferingTime: this.totalBufferingTime,
      currentQuality: this.currentQuality,
      qualityChanges: this.qualityChanges,
      averageBitrate,
      totalSeeks: this.seeks,
      watchTime: watchTime / 1000, // Convert to seconds
      engagementScore,
      startupTime: this.events.find(e => e.eventType === 'play')?.timestamp ? 
        (this.events.find(e => e.eventType === 'play')!.timestamp - this.startTime) / 1000 : 0,
      rebufferingRatio,
      errorCount: this.errors.length,
    };
  }

  /**
   * Get full session info for server reporting
   */
  getSessionInfo(): PlaybackSessionInfo {
    const analytics = this.getSessionAnalytics();
    
    return {
      sessionId: this.sessionId,
      mediaId: this.mediaId,
      mediaType: this.mediaType,
      timestamp: this.startTime,
      duration: this.duration,
      currentTime: this.currentTime,
      currentQuality: this.currentQuality,
      streamUrl: '', // This would be filled by the caller
      
      deviceProfile: {
        platform: this.deviceProfile.capabilities.platform,
        browser: this.deviceProfile.capabilities.browser,
        os: this.deviceProfile.capabilities.os,
        screenResolution: `${this.deviceProfile.capabilities.screenWidth}x${this.deviceProfile.capabilities.screenHeight}`,
        maxResolution: this.deviceProfile.maxResolution,
        estimatedBandwidth: this.deviceProfile.capabilities.estimatedBandwidth,
        connectionType: this.deviceProfile.capabilities.connectionType,
        
        // Include location information if available
        ...(this.deviceProfile.capabilities.location && {
          ipAddress: this.deviceProfile.capabilities.location.ipAddress,
          ipType: this.deviceProfile.capabilities.location.ipType,
          country: this.deviceProfile.capabilities.location.country,
          countryCode: this.deviceProfile.capabilities.location.countryCode,
          region: this.deviceProfile.capabilities.location.region,
          city: this.deviceProfile.capabilities.location.city,
          timezone: this.deviceProfile.capabilities.location.timezone,
          isp: this.deviceProfile.capabilities.location.isp,
          latitude: this.deviceProfile.capabilities.location.latitude,
          longitude: this.deviceProfile.capabilities.location.longitude,
        }),
      },
      
      buffering: {
        totalBufferingTime: this.totalBufferingTime,
        bufferingEvents: this.bufferingEvents,
        lastBufferingTime: this.bufferingStartTime,
      },
      
      seeking: {
        totalSeeks: this.seeks,
        seekAheadRequests: this.events.filter(e => e.eventType === 'seek' && e.data?.seekAhead).length,
        averageSeekTime: this.seekTimes.length > 0 ? 
          this.seekTimes.reduce((sum, time) => sum + time, 0) / this.seekTimes.length : 0,
      },
      
      quality: {
        qualityChanges: this.qualityChanges,
        currentBitrate: this.currentBitrate,
        averageBitrate: this.bitrateHistory.length > 0 ? 
          this.bitrateHistory.reduce((sum, br) => sum + br, 0) / this.bitrateHistory.length : undefined,
      },
      
      errors: this.errors,
    };
  }

  /**
   * Send session data to server
   */
  async sendSessionUpdate(): Promise<void> {
    try {
      const sessionInfo = this.getSessionInfo();
      const analytics = this.getSessionAnalytics();
      
      console.log('📊 Sending session update to server:', {
        sessionId: this.sessionId,
        watchTime: analytics.watchTime,
        bufferingEvents: this.bufferingEvents,
        errorCount: this.errors.length,
        engagementScore: analytics.engagementScore
      });
      
      const url = buildApiUrl('/analytics/session');
      const response = await fetch(url, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          session_info: sessionInfo,
          analytics: analytics,
          events: this.events.slice(-50), // Send last 50 events to avoid large payloads
        }),
      });
      
      if (!response.ok) {
        console.warn('Failed to send session update:', response.status, response.statusText);
      }
    } catch (error) {
      console.warn('Failed to send session update:', error);
    }
  }

  /**
   * Calculate user engagement score based on watching behavior
   */
  private calculateEngagementScore(watchTime: number, sessionDuration: number): number {
    if (sessionDuration === 0) return 0;
    
    // Base score from watch ratio
    const watchRatio = Math.min(watchTime / sessionDuration, 1);
    let score = watchRatio * 60; // Max 60 points for watch ratio
    
    // Penalty for excessive seeking (indicates user dissatisfaction)
    const seekPenalty = Math.min(this.seeks * 2, 20); // Max 20 point penalty
    score -= seekPenalty;
    
    // Penalty for excessive buffering (indicates poor experience)
    const bufferPenalty = Math.min(this.bufferingEvents * 3, 20); // Max 20 point penalty  
    score -= bufferPenalty;
    
    // Bonus for longer sessions (indicates engagement)
    const sessionBonus = Math.min(sessionDuration / (10 * 60 * 1000) * 10, 20); // Max 20 bonus for 10+ min sessions
    score += sessionBonus;
    
    // Penalty for errors
    const errorPenalty = Math.min(this.errors.length * 5, 20); // Max 20 point penalty
    score -= errorPenalty;
    
    return Math.max(0, Math.min(100, Math.round(score)));
  }

  /**
   * Start periodic sync with server
   */
  private startPeriodicSync(): void {
    this.syncInterval = setInterval(() => {
      this.sendSessionUpdate();
    }, this.SYNC_INTERVAL);
  }

  /**
   * Stop tracking and send final update
   */
  async stopTracking(): Promise<void> {
    if (this.syncInterval) {
      clearInterval(this.syncInterval);
      this.syncInterval = null;
    }
    
    // Send final session update
    await this.sendSessionUpdate();
    
    const finalAnalytics = this.getSessionAnalytics();
    console.log('📊 Session tracking stopped:', {
      sessionId: this.sessionId,
      finalAnalytics
    });
  }

  /**
   * Get session ID
   */
  getSessionId(): string {
    return this.sessionId;
  }
}