import type { MediaType, MediaFile, MediaItem, PlaybackDecision, TranscodingSession, SeekAheadRequest, SeekAheadResponse } from '../components/MediaPlayer/types';
import type { DeviceProfile } from '../utils/deviceProfile';
import { getDeviceProfile } from '../utils/deviceProfile';
import { isValidSessionId } from '../utils/mediaValidation';
import { API_ENDPOINTS, buildApiUrl, buildApiUrlWithParams } from '../constants/api';

export class MediaService {

  static async getMediaFiles(mediaId: string, mediaType: MediaType): Promise<MediaFile | null> {
    try {
      // First try to get the file directly by file ID
      const url = buildApiUrl(API_ENDPOINTS.MEDIA.FILE_BY_ID.path(mediaId));
      const response = await fetch(url, {
        method: 'GET',
      });
      
      if (response.ok) {
        const data = await response.json();
        const mediaFile = data.media_file;
        
        // Verify the media type matches what we expect
        if (mediaFile && mediaFile.media_type === mediaType) {
          return mediaFile;
        }
        return null;
      } else if (response.status === 404) {
        // If not found by file ID, search by media ID (episode metadata ID)
        console.log('🔍 MediaService: File ID not found, searching by media ID...');
        const searchResponse = await fetch(buildApiUrlWithParams('/media/', { limit: 50000 }));
        if (searchResponse.ok) {
          const searchData = await searchResponse.json();
          const foundFile = searchData.media?.find(
            (file: any) => file.media_id === mediaId && file.media_type === mediaType
          );
          if (foundFile) {
            console.log('✅ MediaService: Found episode by media ID, file ID:', foundFile.id);
            return foundFile;
          }
        }
        return null; // Not found by either method
      } else {
        throw new Error(`Failed to fetch media file: ${response.statusText}`);
      }
    } catch (error) {
      console.error('Failed to get media file:', error);
      throw error;
    }
  }

  static async getMediaMetadata(mediaId: string, mediaFileId: string): Promise<MediaItem | null> {
    try {
      const url = buildApiUrl(API_ENDPOINTS.MEDIA.FILE_METADATA.path(mediaFileId));
      const response = await fetch(url, {
        method: API_ENDPOINTS.MEDIA.FILE_METADATA.method,
      });
      if (!response.ok) {
        if (response.status === 404) {
          return null;
        }
        throw new Error(`Failed to fetch media metadata: ${response.statusText}`);
      }
      
      const data = await response.json();
      return data.episode || data.movie || null;
    } catch (error) {
      console.error('Failed to get media metadata:', error);
      throw error;
    }
  }

  static async getPlaybackDecision(mediaPath: string, fileId: string, deviceProfile?: DeviceProfile): Promise<PlaybackDecision> {
    try {
      // Get device profile if not provided
      const profile = deviceProfile || await getDeviceProfile();
      
      console.log('🎯 Getting playback decision with device profile:', {
        maxResolution: profile.maxResolution,
        maxBitrate: profile.maxBitrate,
        platform: profile.capabilities.platform,
        estimatedBandwidth: profile.capabilities.estimatedBandwidth,
        supportedCodecs: profile.supportedCodecs
      });

      const url = buildApiUrl(API_ENDPOINTS.PLAYBACK.DECIDE.path);
      const response = await fetch(url, {
        method: API_ENDPOINTS.PLAYBACK.DECIDE.method,
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          file_id: fileId,
          media_path: mediaPath,
          device_profile: {
            user_agent: profile.userAgent,
            supported_codecs: profile.supportedCodecs,
            max_resolution: profile.maxResolution,
            max_bitrate: profile.maxBitrate,
            supports_hevc: profile.supportsHEVC,
            target_container: profile.targetContainer,
            // Enhanced device info for better transcoding decisions
            platform: profile.capabilities.platform,
            estimated_bandwidth: profile.capabilities.estimatedBandwidth,
            performance_level: profile.capabilities.estimatedPerformanceLevel,
            connection_type: profile.capabilities.connectionType,
            supports_hdr: profile.capabilities.supportsHDR,
            
            // Location and network info for CDN optimization
            ...(profile.capabilities.location && {
              ip_address: profile.capabilities.location.ipAddress,
              ip_type: profile.capabilities.location.ipType,
              country: profile.capabilities.location.country,
              country_code: profile.capabilities.location.countryCode,
              region: profile.capabilities.location.region,
              city: profile.capabilities.location.city,
              timezone: profile.capabilities.location.timezone,
              isp: profile.capabilities.location.isp,
              latitude: profile.capabilities.location.latitude,
              longitude: profile.capabilities.location.longitude,
            }),
          },
        }),
      });

      if (!response.ok) {
        throw new Error(`Playback decision failed: ${response.statusText}`);
      }

      return await response.json();
    } catch (error) {
      console.error('Failed to get playback decision:', error);
      throw error;
    }
  }

  static async startTranscodingSession(
    mediaFileIdOrPath: string,
    container: string = 'dash',
    videoCodec: string = 'h264',
    audioCodec: string = 'aac',
    quality: number = 23,
    speedPriority: string = 'balanced',
    deviceProfile?: DeviceProfile
  ): Promise<TranscodingSession> {
    try {
      const url = buildApiUrl(API_ENDPOINTS.PLAYBACK.START.path);
      
      // Get device profile if not provided
      const profile = deviceProfile || await getDeviceProfile();
      
      // Check if this looks like a media file ID (UUID format)
      const isMediaFileId = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i.test(mediaFileIdOrPath);
      
      const body = isMediaFileId ? {
        media_file_id: mediaFileIdOrPath,
        container,
        seek_position: 0,
        enable_abr: true,
        device_profile: {
          user_agent: profile.userAgent,
          supported_codecs: profile.supportedCodecs,
          max_resolution: profile.maxResolution,
          max_bitrate: profile.maxBitrate,
          supports_hevc: profile.supportsHEVC,
          supports_av1: profile.capabilities.videoCodecs.av1,
          supports_hdr: profile.capabilities.supportsHDR,
          client_ip: profile.capabilities.location?.ipAddress || '',
        },
      } : {
        input_path: mediaFileIdOrPath,
        container,
        video_codec: videoCodec,
        audio_codec: audioCodec,
        quality,
        speed_priority: speedPriority,
        seek: 0,
        enable_abr: true,
      };
      
      const response = await fetch(url, {
        method: API_ENDPOINTS.PLAYBACK.START.method,
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify(body),
      });

      if (!response.ok) {
        throw new Error(`Session start failed: ${response.statusText}`);
      }

      return await response.json();
    } catch (error) {
      console.error('Failed to start transcoding session:', error);
      throw error;
    }
  }

  static async stopTranscodingSession(sessionId: string): Promise<void> {
    try {
      const url = buildApiUrl(API_ENDPOINTS.PLAYBACK.SESSION.path(sessionId));
      const response = await fetch(url, {
        method: API_ENDPOINTS.PLAYBACK.SESSION.method,
        headers: {
          'Content-Type': 'application/json',
        },
      });

      if (!response.ok && response.status !== 404) {
        throw new Error(`Failed to stop session: ${response.statusText}`);
      }
    } catch (error) {
      console.error('Failed to stop transcoding session:', error);
      throw error;
    }
  }

  static async requestSeekAhead(request: SeekAheadRequest): Promise<SeekAheadResponse> {
    try {
      const url = buildApiUrl(API_ENDPOINTS.PLAYBACK.SEEK_AHEAD.path);
      const response = await fetch(url, {
        method: API_ENDPOINTS.PLAYBACK.SEEK_AHEAD.method,
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify(request),
      });

      if (!response.ok) {
        throw new Error(`Seek-ahead request failed: ${response.statusText}`);
      }

      return await response.json();
    } catch (error) {
      console.error('Failed to request seek-ahead:', error);
      throw error;
    }
  }

  static async waitForManifest(
    url: string, 
    options: { 
      maxAttempts?: number; 
      checkInterval?: number;
      requireSegments?: boolean;
    } = {}
  ): Promise<boolean> {
    const maxAttempts = options.maxAttempts ?? 60;
    const checkInterval = options.checkInterval ?? 500;
    const requireSegments = options.requireSegments ?? true;
    
    console.log('⏳ Waiting for manifest to be ready...', { url, maxAttempts, requireSegments });
    
    for (let attempt = 1; attempt <= maxAttempts; attempt++) {
      try {
        console.log(`🔄 Attempt ${attempt}/${maxAttempts}: Checking manifest availability...`);
        
        // First check if manifest exists
        const response = await fetch(url, { 
          method: 'GET',
          headers: {
            'Cache-Control': 'no-cache, no-store, must-revalidate',
            'Pragma': 'no-cache',
            'Expires': '0'
          }
        });
        
        if (response.ok) {
          console.log(`✅ Manifest exists (${response.status}), checking content...`);
          
          const manifestText = await response.text();
          
          if (!manifestText || manifestText.trim().length === 0) {
            console.log(`⚠️ Attempt ${attempt}: Manifest is empty`);
          } else if (manifestText.includes('<MPD') && manifestText.includes('</MPD>')) {
            // Additional validation - check for required DASH elements
            const hasRequiredElements = 
              manifestText.includes('<Period') && 
              manifestText.includes('<AdaptationSet') &&
              manifestText.includes('xmlns="urn:mpeg:dash:schema:mpd:2011"');
              
            if (hasRequiredElements) {
              console.log(`✅ Attempt ${attempt}: Valid DASH manifest found, content length: ${manifestText.length}`);
              
              // Additional check - for dynamic manifests (live), check for segment info instead of duration
              if (manifestText.includes('type="dynamic"')) {
                // Dynamic manifest - check for segment availability
                if (manifestText.includes('<SegmentTemplate') || manifestText.includes('<SegmentList') || manifestText.includes('<SegmentTimeline')) {
                  console.log(`✅ Dynamic manifest contains segment information, ready for playback`);
                  return true;
                } else {
                  console.log(`⚠️ Attempt ${attempt}: Dynamic manifest but no segment info yet`);
                }
              } else if (manifestText.includes('mediaPresentationDuration') || manifestText.includes('duration=')) {
                console.log(`✅ Static manifest contains duration information, ready for playback`);
                return true;
              } else {
                console.log(`⚠️ Attempt ${attempt}: Manifest valid but may be incomplete (no duration info)`);
              }
            } else {
              console.log(`⚠️ Attempt ${attempt}: Manifest XML structure incomplete`);
            }
          } else {
            console.log(`⚠️ Attempt ${attempt}: Manifest exists but content appears invalid:`, manifestText.substring(0, 200));
          }
        } else {
          console.log(`⏳ Attempt ${attempt}: Manifest not available (${response.status})`);
        }
        
        if (attempt >= maxAttempts) {
          break;
        }
        
        // Use configured interval for fast checking
        await new Promise(resolve => setTimeout(resolve, checkInterval));
        
      } catch (error) {
        console.log(`❌ Attempt ${attempt}: Network error -`, error.message);
        
        if (attempt >= maxAttempts) {
          break;
        }
        
        await new Promise(resolve => setTimeout(resolve, checkInterval));
      }
    }
    
    const totalTime = Math.round((maxAttempts * checkInterval) / 1000);
    throw new Error(`Manifest not ready after ${maxAttempts} attempts (${totalTime}s). URL: ${url}`);
  }

  static async checkVideoSegmentExists(manifestUrl: string, manifestContent: string): Promise<boolean> {
    try {
      // Extract base URL from manifest
      const baseUrl = manifestUrl.substring(0, manifestUrl.lastIndexOf('/'));
      
      // Try common segment naming patterns
      const segmentUrls = [
        `${baseUrl}/init-0.m4s`,
        `${baseUrl}/chunk-0-1.m4s`,
        `${baseUrl}/segment-0.m4s`,
        `${baseUrl}/seg-1.m4s`
      ];
      
      // Check if any video segment exists
      for (const segmentUrl of segmentUrls) {
        try {
          const response = await fetch(segmentUrl, { method: 'HEAD' });
          if (response.ok && response.headers.get('content-length') !== '0') {
            return true;
          }
        } catch (e) {
          // Continue to next segment URL
        }
      }
      
      return false;
    } catch (error) {
      console.warn('Error checking video segment existence:', error);
      return false;
    }
  }

  static async validateManifest(
    url: string,
    options: { quick?: boolean } = {}
  ): Promise<boolean> {
    try {
      const response = await fetch(url, { method: 'GET' });
      if (!response.ok) {
        return false;
      }
      
      const manifestText = await response.text();
      
      // Basic validation checks
      if (!manifestText || manifestText.trim().length === 0) {
        console.warn('Manifest is empty');
        return false;
      }
      
      // Check if it's valid XML
      try {
        const parser = new DOMParser();
        const xmlDoc = parser.parseFromString(manifestText, 'text/xml');
        const parseError = xmlDoc.getElementsByTagName('parsererror');
        if (parseError.length > 0) {
          console.warn('Manifest XML parsing error:', parseError[0].textContent);
          return false;
        }
      } catch (xmlError) {
        console.warn('Failed to parse manifest XML:', xmlError);
        return false;
      }
      
      // Quick validation just checks basic structure
      if (options.quick) {
        const hasBasicStructure = manifestText.includes('<MPD') && manifestText.includes('<Period');
        console.log(`⚡ Quick manifest validation: ${hasBasicStructure ? 'valid' : 'invalid'}`);
        return hasBasicStructure;
      }
      
      // Full validation - check for DASH-specific elements
      if (!manifestText.includes('<MPD') || !manifestText.includes('xmlns="urn:mpeg:dash:schema:mpd:2011"')) {
        console.warn('Manifest does not appear to be a valid DASH MPD');
        return false;
      }
      
      // Check for required elements
      if (!manifestText.includes('<Period') || !manifestText.includes('<AdaptationSet')) {
        console.warn('Manifest missing required DASH elements (Period/AdaptationSet)');
        return false;
      }
      
      return true;
    } catch (error) {
      console.warn('Error validating manifest:', error);
      return false;
    }
  }

  static async waitForTranscodingProgress(
    sessionId: string,
    options: { maxAttempts?: number; checkInterval?: number } = {}
  ): Promise<boolean> {
    const maxAttempts = options.maxAttempts ?? 5;
    const checkInterval = options.checkInterval ?? 500;
    
    console.log('⏳ Waiting for transcoding to start...');
    
    // For DASH streaming, we only need to wait for the session to be running
    // The manifest wait will handle checking for actual content availability
    for (let attempt = 1; attempt <= maxAttempts; attempt++) {
      try {
        const url = buildApiUrl(`/playback/session/${sessionId}`);
        const response = await fetch(url, { method: 'GET' });
        
        if (response.ok) {
          const session = await response.json();
          const progressData = session?.Progress ? JSON.parse(session.Progress) : null;
          const progress = progressData?.percent_complete || 0;
          
          console.log(`📊 Transcoding progress: ${progress}%, Status: ${session?.Status}`);
          
          // For DASH, we can start as soon as the session is running
          if (session?.Status === 'running' || session?.Status === 'completed') {
            console.log('✅ Transcoding session is active, proceeding with manifest loading');
            return true;
          }
          
          if (session?.Status === 'failed' || session?.Status === 'cancelled') {
            throw new Error(`Transcoding failed with status: ${session.Status}`);
          }
        }
        
        // Use configured interval for faster startup
        await new Promise(resolve => setTimeout(resolve, checkInterval));
      } catch (error) {
        console.warn(`Attempt ${attempt}: Error checking transcoding status:`, error);
        await new Promise(resolve => setTimeout(resolve, checkInterval));
      }
    }
    
    console.warn('Transcoding status check timed out, proceeding anyway...');
    return true; // Proceed anyway to avoid blocking forever
  }

  static async stopAllSessions(): Promise<void> {
    try {
      const url = buildApiUrl('/playback/sessions/all');
      const response = await fetch(url, {
        method: 'DELETE',
        headers: {
          'Content-Type': 'application/json',
        },
      });

      if (!response.ok && response.status !== 404) {
        throw new Error(`Failed to stop all sessions: ${response.statusText}`);
      }
    } catch (error) {
      console.error('Failed to stop all sessions:', error);
      throw error;
    }
  }

  static getDefaultDeviceProfile(): DeviceProfile {
    return {
      userAgent: navigator.userAgent,
      supportedCodecs: ["h264", "aac"],
      maxResolution: "1080p",
      maxBitrate: 8000,
      supportsHEVC: false,
      targetContainer: "hls",
      capabilities: {
        maxResolution: "1080p",
        screenWidth: screen.width,
        screenHeight: screen.height,
        pixelRatio: window.devicePixelRatio || 1,
        hardwareAcceleration: true,
        estimatedPerformanceLevel: 'medium',
        estimatedBandwidth: 8000,
        connectionType: 'unknown',
        videoCodecs: {
          h264: true,
          hevc: false,
          vp8: false,
          vp9: false,
          av1: false,
        },
        audioCodecs: ['aac'],
        platform: 'desktop',
        os: 'unknown',
        browser: 'unknown',
        supportsHDR: false,
        supportsHEVC: false,
        supportsMSE: !!window.MediaSource,
        supportsHLS: !!(document.createElement('video').canPlayType('application/vnd.apple.mpegurl')),
      },
    };
  }

  static isValidSessionId(sessionId: string): boolean {
    return isValidSessionId(sessionId);
  }
}