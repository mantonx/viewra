import React, { useState, useEffect, useRef, useCallback } from 'react';
import { useParams, useNavigate, useSearchParams } from 'react-router-dom';
import { ArrowLeft, Play, Pause, Square, SkipBack, SkipForward, Volume2, VolumeX, Maximize, Minimize2 } from 'lucide-react';
import { Tooltip } from 'react-tooltip';
import shaka from 'shaka-player/dist/shaka-player.ui.js';


// Types
interface Episode {
  id: string;
  title: string;
  episode_number: number;
  air_date?: string;
  description?: string;
  duration?: number;
  still_image?: string;
  season: {
    id: string;
    season_number: number;
    tv_show: {
      id: string;
      title: string;
      description?: string;
      poster?: string;
      backdrop?: string;
      tmdb_id?: string;
    };
  };
}

interface MediaFile {
  id: string;
  path: string;
  container?: string;
  video_codec?: string;
  audio_codec?: string;
  resolution?: string;
  duration?: number;
  size_bytes: number;
}

interface PlaybackDecision {
  should_transcode: boolean;
  reason: string;
  direct_play_url?: string;
  stream_url: string;
  manifest_url?: string;
  media_info: {
    id: string;
    container: string;
    video_codec: string;
    audio_codec: string;
    resolution: string;
    duration: number;
    size_bytes: number;
  };
  transcode_params?: {
    target_codec: string;
    target_container: string;
    resolution: string;
    bitrate: number;
  };
  session_id?: string;
}

const VideoPlayer: React.FC = () => {
  const { episodeId } = useParams<{ episodeId: string }>();
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();

  // Debug flag - controlled by URL query param (?debug=true) or set manually
  const DEBUG = searchParams.get('debug') === 'true' || searchParams.get('debug') === '1';
  
  // Refs
  const videoRef = useRef<HTMLVideoElement>(null);
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const playerRef = useRef<any>(null);
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const uiRef = useRef<any>(null);
  const initializationRef = useRef(false); // Prevent multiple initializations

  // State
  const [, setEpisode] = useState<Episode | null>(null);
  const [mediaFile, setMediaFile] = useState<MediaFile | null>(null);
  const [playbackDecision, setPlaybackDecision] = useState<PlaybackDecision | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [isPlaying, setIsPlaying] = useState(false);
  const [duration, setDuration] = useState(0);
  const [seekableDuration, setSeekableDuration] = useState(0); // Available for seeking
  const [originalDuration, setOriginalDuration] = useState(0); // Full file duration
  const [currentTime, setCurrentTime] = useState(0);
  const [volume, setVolume] = useState(1);
  const [isMuted, setIsMuted] = useState(false);
  const [isFullscreen, setIsFullscreen] = useState(false);
  const [showControls, setShowControls] = useState(true);
  const [hoverTime, setHoverTime] = useState<number | null>(null);
  const [isSeekingAhead, setIsSeekingAhead] = useState(false); // Track seek-ahead state
  
  // Simple loading states
  const [isVideoLoading, setIsVideoLoading] = useState(true);
  const [isBuffering, setIsBuffering] = useState(false);

  // URL params
  const startTime = parseInt(searchParams.get('t') || '0', 10);
  const shouldAutoplay = searchParams.get('autoplay') !== 'false';
  
  // Debug URL params and check for saved position
  const savedPosition = localStorage.getItem(`video-position-${episodeId}`);
  if (DEBUG) {
    console.log('🔍 Video position debug:', { 
      episodeId,
      rawTParam: searchParams.get('t'), 
      startTime, 
      shouldAutoplay,
      savedPosition: savedPosition ? parseFloat(savedPosition) : null,
      currentDuration: duration,
      allParams: Object.fromEntries(searchParams.entries())
    });
  }

  // Session management state
  const [activeSessionIds, setActiveSessionIds] = useState<Set<string>>(new Set());
  const [isStoppingSession, setIsStoppingSession] = useState(false);

  // Helper function to validate session ID format (UUID-based)
  const isValidSessionId = useCallback((sessionId: string) => {
    if (!sessionId) return false;
    // Check for new UUID format: ffmpeg_xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
    const uuidPattern = /^ffmpeg_[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;
    return uuidPattern.test(sessionId);
  }, []);

  // Helper function to stop transcoding sessions with UUID validation
  const stopTranscodingSession = useCallback(async (sessionId: string) => {
    if (!sessionId || isStoppingSession) return;
    
    // Validate session ID format
    if (!isValidSessionId(sessionId)) {
      console.warn('⚠️ Skipping cleanup for invalid/old session ID format:', sessionId);
      // Remove from tracking set anyway since it's invalid
      setActiveSessionIds(prev => {
        const newSet = new Set(prev);
        newSet.delete(sessionId);
        return newSet;
      });
      return;
    }
    
    setIsStoppingSession(true);
    console.log('🛑 Stopping transcoding session (UUID-based):', sessionId);
    
    try {
      const response = await fetch(`/api/playback/session/${sessionId}`, {
        method: 'DELETE',
        headers: {
          'Content-Type': 'application/json',
        },
      });
      
      if (response.ok) {
        console.log('✅ Successfully stopped transcoding session:', sessionId);
        setActiveSessionIds(prev => {
          const newSet = new Set(prev);
          newSet.delete(sessionId);
          return newSet;
        });
      } else if (response.status === 404) {
        console.log('ℹ️ Session already cleaned up by backend:', sessionId);
        // Remove from tracking since it's already gone
        setActiveSessionIds(prev => {
          const newSet = new Set(prev);
          newSet.delete(sessionId);
          return newSet;
        });
      } else {
        console.warn('⚠️ Failed to stop transcoding session:', sessionId, response.status);
      }
    } catch (error) {
      console.error('❌ Error stopping transcoding session:', sessionId, error);
    } finally {
      setIsStoppingSession(false);
    }
  }, [isStoppingSession, isValidSessionId]);

  // Function to stop all active sessions
  const stopAllSessions = useCallback(async () => {
    const sessions = Array.from(activeSessionIds);
    if (sessions.length === 0) return;
    
    console.log('🛑 Stopping all active sessions:', sessions);
    await Promise.all(sessions.map(sessionId => stopTranscodingSession(sessionId)));
  }, [activeSessionIds, stopTranscodingSession]);

  // Enhanced video event handlers with session management
  const handlePause = useCallback(() => {
    setIsPlaying(false);
    // Optionally stop sessions when paused to save resources
    // This is aggressive but prevents resource waste
    if (playbackDecision?.session_id) {
      console.log('🔄 Video paused, stopping transcoding session to save resources');
      stopTranscodingSession(playbackDecision.session_id);
    }
  }, [playbackDecision?.session_id, stopTranscodingSession]);

  const handlePlay = useCallback(() => {
    setIsPlaying(true);
    // When resuming, we might need to restart transcoding
    // This will be handled by seeking or restarting the session
  }, []);

  // Enhanced navigation handler with cleanup
  const handleNavigation = useCallback(async () => {
    console.log('🚪 Navigating away, cleaning up sessions...');
    await stopAllSessions();
    navigate(-1);
  }, [navigate, stopAllSessions]);

  // Track session IDs when they're created
  useEffect(() => {
    if (playbackDecision?.session_id) {
      // Validate new session ID format
      if (isValidSessionId(playbackDecision.session_id)) {
        console.log('📝 Tracking new UUID-based session:', playbackDecision.session_id);
        setActiveSessionIds(prev => new Set(prev).add(playbackDecision.session_id!));
      } else {
        console.warn('⚠️ Received session with invalid/old format, not tracking:', playbackDecision.session_id);
      }
    }
  }, [playbackDecision?.session_id, isValidSessionId]);

  // Cleanup on unmount or episode change
  useEffect(() => {
    return () => {
      console.log('🧹 Component unmounting, cleaning up sessions...');
      // Use current value directly since this is cleanup
      const currentSessions = Array.from(activeSessionIds);
      currentSessions.forEach(sessionId => {
        // Only attempt cleanup for valid UUID-based session IDs
        if (isValidSessionId(sessionId)) {
          fetch(`/api/playback/session/${sessionId}`, {
            method: 'DELETE',
            headers: { 'Content-Type': 'application/json' },
          }).catch(error => console.error('Cleanup error for session:', sessionId, error));
        } else {
          console.log('🚫 Skipping cleanup for invalid session ID:', sessionId);
        }
      });
    };
  }, [episodeId, isValidSessionId]); // Clean up when episode changes

  // Enhanced beforeunload to clean up sessions when page is closed/refreshed
  useEffect(() => {
    const handleBeforeUnload = () => {
      // Send synchronous cleanup requests for valid UUID-based sessions only
      const currentSessions = Array.from(activeSessionIds);
      currentSessions.forEach(sessionId => {
        if (isValidSessionId(sessionId)) {
          navigator.sendBeacon(`/api/playback/session/${sessionId}`, 
            JSON.stringify({ method: 'DELETE' }));
        }
      });
    };

    window.addEventListener('beforeunload', handleBeforeUnload);
    return () => window.removeEventListener('beforeunload', handleBeforeUnload);
  }, [activeSessionIds, isValidSessionId]);

  // Clean up player
  const cleanupPlayer = useCallback(() => {
    if (uiRef.current && typeof uiRef.current.destroy === 'function') {
      try {
        uiRef.current.destroy();
      } catch (e) {
        console.warn('Error destroying UI:', e);
      }
      uiRef.current = null;
    }

    if (playerRef.current && typeof playerRef.current.destroy === 'function') {
      try {
        playerRef.current.destroy();
      } catch (e) {
        console.warn('Error destroying player:', e);
      }
      playerRef.current = null;
    }

    initializationRef.current = false;
  }, []);

  // Fast manifest availability check with aggressive polling
  const waitForManifest = useCallback(async (url: string, maxAttempts = 30, initialIntervalMs = 200) => {
    // Start checking immediately - no initial delay
    if (DEBUG) console.log('⚡ Fast checking manifest availability...');
    
    for (let attempt = 1; attempt <= maxAttempts; attempt++) {
      try {
        if (DEBUG) console.log(`🔄 Checking manifest (${attempt}/${maxAttempts}): ${url}`);
        
        const response = await fetch(url, { method: 'HEAD' });
        if (response.ok) {
          if (DEBUG) console.log('✅ Manifest is available!');
          return true;
        }
        
        // Progressive backoff: start fast, slow down gradually
        const delay = Math.min(initialIntervalMs * Math.pow(1.5, attempt - 1), 2000);
        if (DEBUG) console.log(`⏳ Manifest not ready (${response.status}), retrying in ${delay}ms...`);
        await new Promise(resolve => setTimeout(resolve, delay));
      } catch (error) {
        // Use shorter delays for network errors to retry quickly
        const delay = Math.min(initialIntervalMs * attempt, 1000);
        if (DEBUG) console.log(`⏳ Manifest check failed (${attempt}/${maxAttempts}), retrying in ${delay}ms:`, error);
        await new Promise(resolve => setTimeout(resolve, delay));
      }
    }
    
    throw new Error(`Manifest not available after ${maxAttempts} attempts`);
  }, [DEBUG]);

  // Initialize Shaka Player
  const initializePlayer = useCallback(async () => {
    if (!videoRef.current || !playbackDecision || initializationRef.current) {
      return;
    }

    // Prevent multiple initializations
    initializationRef.current = true;

    try {
      if (DEBUG) console.log('🎬 Initializing Shaka Player...');

      // Install polyfills
      if (shaka.polyfill) {
        shaka.polyfill.installAll();
      }

      // Check browser support
      if (!shaka.Player.isBrowserSupported()) {
        throw new Error('Browser not supported');
      }

      // Create player (modern approach without passing video element to constructor)
      const player = new shaka.Player();
      playerRef.current = player;
      
      // Attach player to video element
      await player.attach(videoRef.current);

      // Configure player for fast startup
      player.configure({
        streaming: {
          bufferingGoal: 10, // Reduced from 30 for faster startup
          rebufferingGoal: 3, // Reduced from 5 for faster startup
          bufferBehind: 30,
        },
        manifest: {
          retryParameters: {
            timeout: 5000, // Faster timeout
            maxAttempts: 3, // Fewer attempts for faster failure detection
            baseDelay: 200, // Faster initial retry
            backoffFactor: 1.5,
            fuzzFactor: 0.5,
          },
        },
      });

      // Player initialization starting

      // Get manifest URL (prefer manifest_url for DASH/HLS, fallback to stream_url)
      const manifestUrl = playbackDecision.manifest_url || playbackDecision.stream_url;
      if (DEBUG) console.log('🎬 Loading manifest/stream:', manifestUrl);

      // For DASH/HLS adaptive streaming, wait for manifest to be available
      if (playbackDecision.manifest_url && playbackDecision.should_transcode) {
        if (DEBUG) console.log('⏳ Waiting for DASH manifest to be generated...');
        await waitForManifest(manifestUrl);
      }

      // Load the manifest/stream
      await player.load(manifestUrl);
      if (DEBUG) console.log('✅ Player loaded successfully');

      // Set up video event listeners
      const video = videoRef.current;
      if (video) {
        const handleLoadedMetadata = () => {
          if (DEBUG) {
            console.log('🔍 Video metadata loaded:', { 
              duration: video.duration, 
              currentTime: video.currentTime,
              startTimeParam: startTime,
              seekableStart: video.seekable.length > 0 ? video.seekable.start(0) : 'N/A',
              seekableEnd: video.seekable.length > 0 ? video.seekable.end(0) : 'N/A',
              isFinite: isFinite(video.duration),
              isNaN: isNaN(video.duration)
            });
          }
          
          // Only set duration if it's a valid finite number
          if (isFinite(video.duration) && video.duration > 0) {
            setDuration(video.duration);
            if (DEBUG) console.log('✅ Valid duration set:', video.duration);
          } else {
            if (DEBUG) console.warn('❌ Invalid duration detected:', video.duration);
          }
          
          // Video metadata is ready
          setIsVideoLoading(false);
          setIsBuffering(false);
          
          // Determine start position: URL param > saved position > beginning
          let targetStartTime = 0;
          if (startTime > 0) {
            targetStartTime = startTime;
            console.log('📍 Using URL start time:', startTime);
          } else if (savedPosition && parseFloat(savedPosition) > 0) {
            targetStartTime = parseFloat(savedPosition);
            console.log('📍 Resuming from saved position:', targetStartTime);
          } else {
            console.log('📍 Starting from beginning');
          }
          
          video.currentTime = targetStartTime;
        };

        const handleTimeUpdate = () => {
          setCurrentTime(video.currentTime);
          
          // Update duration if it becomes available (important for DASH streams)
                      if (!duration || duration <= 0) {
             // Try multiple ways to get duration for DASH streams
             let detectedDuration = 0;
             
             if (isFinite(video.duration) && video.duration > 0) {
               detectedDuration = video.duration;
               if (DEBUG) console.log('🕒 Duration from video.duration:', detectedDuration);
             } else if (video.seekable && video.seekable.length > 0) {
               detectedDuration = video.seekable.end(video.seekable.length - 1);
               if (DEBUG) console.log('🕒 Duration from seekable range:', detectedDuration);
             } else if (video.buffered && video.buffered.length > 0) {
               detectedDuration = video.buffered.end(video.buffered.length - 1);
               if (DEBUG) console.log('🕒 Duration from buffered range:', detectedDuration);
             }
             
             if (detectedDuration > 0 && isFinite(detectedDuration)) {
               if (DEBUG) console.log('✅ Seekable duration detected during playback:', detectedDuration);
               setSeekableDuration(detectedDuration);
               
               // Only update display duration if we don't have original duration or if seekable >= original
               if (!originalDuration || detectedDuration >= originalDuration) {
                 setDuration(detectedDuration);
               }
             }
           }
          
          // Save position every 5 seconds
          if (Math.floor(video.currentTime) % 5 === 0) {
            localStorage.setItem(`video-position-${episodeId}`, video.currentTime.toString());
          }
        };
        // Use our enhanced handlers with session management
        const handlePlayEvent = handlePlay;
        const handlePauseEvent = handlePause;
        const handleVolumeChange = () => {
          setVolume(video.volume);
          setIsMuted(video.muted);
        };

        // Additional handler for duration detection
        const handleDurationChange = () => {
          console.log('🎬 Duration change event fired:', video.duration);
          if (isFinite(video.duration) && video.duration > 0) {
            setDuration(video.duration);
          }
        };

        const handleLoadedData = () => {
          console.log('📊 Loaded data event fired:', {
            duration: video.duration,
            seekable: video.seekable.length > 0 ? video.seekable.end(0) : 'none',
            buffered: video.buffered.length > 0 ? video.buffered.end(0) : 'none'
          });
          
          // Try to detect duration from multiple sources
          let detectedDuration = 0;
          if (isFinite(video.duration) && video.duration > 0) {
            detectedDuration = video.duration;
          } else if (video.seekable && video.seekable.length > 0) {
            detectedDuration = video.seekable.end(video.seekable.length - 1);
          }
          
          if (detectedDuration > 0) {
            if (DEBUG) console.log('✅ Seekable duration from loadeddata:', detectedDuration);
            setSeekableDuration(detectedDuration);
            
            // Only update display duration if we don't have original duration or if seekable >= original
            if (!originalDuration || detectedDuration >= originalDuration) {
              setDuration(detectedDuration);
            }
          }
          
          // Video data is loaded and ready to play
          setIsVideoLoading(false);
          setIsBuffering(false);
          
          // Attempt autoplay if enabled
          if (shouldAutoplay) {
            console.log('🎬 Attempting autoplay...');
            video.play().then(() => {
              console.log('✅ Autoplay successful');
            }).catch((error) => {
              console.warn('⚠️ Autoplay failed:', error.message);
            });
          }
        };

        // Additional handlers for better state tracking
        const handleCanPlay = () => {
          console.log('📺 Can play event fired');
          setIsVideoLoading(false);
          setIsBuffering(false);
        };

        const handleWaiting = () => {
          console.log('⏳ Video waiting/buffering...');
          setIsBuffering(true);
        };

        const handlePlaying = () => {
          console.log('▶️ Video playing');
          setIsVideoLoading(false);
          setIsBuffering(false);
        };

        const handleStalled = () => {
          console.log('🔄 Video stalled, buffering...');
          setIsBuffering(true);
        };

        video.addEventListener('loadedmetadata', handleLoadedMetadata);
        video.addEventListener('loadeddata', handleLoadedData);
        video.addEventListener('durationchange', handleDurationChange);
        video.addEventListener('timeupdate', handleTimeUpdate);
        video.addEventListener('play', handlePlayEvent);
        video.addEventListener('pause', handlePauseEvent);
        video.addEventListener('volumechange', handleVolumeChange);
        video.addEventListener('canplay', handleCanPlay);
        video.addEventListener('waiting', handleWaiting);
        video.addEventListener('playing', handlePlaying);
        video.addEventListener('stalled', handleStalled);

        // Store cleanup function
        (video as HTMLVideoElement & { cleanupVideoEvents?: () => void }).cleanupVideoEvents = () => {
          video.removeEventListener('loadedmetadata', handleLoadedMetadata);
          video.removeEventListener('loadeddata', handleLoadedData);
          video.removeEventListener('durationchange', handleDurationChange);
          video.removeEventListener('timeupdate', handleTimeUpdate);
          video.removeEventListener('play', handlePlay);
          video.removeEventListener('pause', handlePause);
          video.removeEventListener('volumechange', handleVolumeChange);
          video.removeEventListener('canplay', handleCanPlay);
          video.removeEventListener('waiting', handleWaiting);
          video.removeEventListener('playing', handlePlaying);
          video.removeEventListener('stalled', handleStalled);
        };
      }

      setLoading(false);
    } catch (err) {
      console.error('❌ Player initialization failed:', err);
      setError(`Player initialization failed: ${err instanceof Error ? err.message : 'Unknown error'}`);
      setLoading(false);
      initializationRef.current = false;
    }
  }, [playbackDecision, startTime, shouldAutoplay, savedPosition, DEBUG]);

  // Session request tracking to prevent duplicates
  const isRequestingSession = useRef(false);

  // Load episode data
  const loadEpisodeData = useCallback(async () => {
    if (!episodeId || isRequestingSession.current) {
      if (isRequestingSession.current) {
        console.log('🚦 Session request already in progress, skipping duplicate');
      }
      return;
    }

    try {
      isRequestingSession.current = true;
      setLoading(true);
      setError(null);

      // Get media files
      const filesResponse = await fetch(`/api/media/files?limit=1000`);
      const filesData = await filesResponse.json();

      const episodeFile = filesData.media_files?.find(
        (file: MediaFile & { media_id: string; media_type: string }) =>
          file.media_id === episodeId && file.media_type === 'episode'
      );

      if (!episodeFile) {
        throw new Error('No media file found for this episode');
      }

      setMediaFile(episodeFile);

      // Set original duration from file metadata
      if (episodeFile.duration && episodeFile.duration > 0) {
        setOriginalDuration(episodeFile.duration);
        setDuration(episodeFile.duration); // Start with original duration
        if (DEBUG) console.log('📏 Original file duration:', episodeFile.duration, 'seconds');
      }

      // Start both playback decision and metadata fetch in parallel for speed
      const deviceProfile = {
        user_agent: navigator.userAgent,
        supported_codecs: ["h264", "aac", "mp3"],
        max_resolution: "1080p",
        max_bitrate: 8000,
        supports_hevc: false,
        target_container: "dash"
      };

      // Parallel requests for speed
      const [decisionResponse, metadataResponse] = await Promise.all([
        fetch('/api/playback/decide', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            media_path: episodeFile.path,
            device_profile: deviceProfile
          })
        }),
        fetch(`/api/media/files/${episodeFile.id}/metadata`) // Start metadata fetch in parallel
      ]);

      if (!decisionResponse.ok) {
        throw new Error(`Playback decision failed: ${decisionResponse.statusText}`);
      }

      const decision = await decisionResponse.json();
      setPlaybackDecision(decision);

      // Process metadata while starting transcoding session
      if (metadataResponse.ok) {
        const metadata = await metadataResponse.json();
        setEpisode(metadata.episode);
      }

      // Start transcoding session if needed - ONLY ONCE
      if (decision.should_transcode) {
        console.log('🎬 Starting single transcoding session for:', episodeFile.path);
        
        const sessionResponse = await fetch('/api/playback/start', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            input_path: episodeFile.path,
            codec_opts: {
              video: decision.transcode_params?.target_codec || 'h264',
              audio: 'aac',
              container: decision.transcode_params?.target_container || 'dash',
              bitrate: decision.transcode_params?.bitrate ? `${decision.transcode_params.bitrate}k` : '6000k',
              quality: 23,
              preset: 'fast'
            },
            environment: {
              resolution: decision.transcode_params?.resolution || '1080p',
              priority: '5'
            }
          })
        });

        if (!sessionResponse.ok) {
          throw new Error(`Session start failed: ${sessionResponse.statusText}`);
        }

        const sessionData = await sessionResponse.json();
        console.log('✅ Transcoding session started:', sessionData.id);

        // Track this session to prevent duplicate cleanup
        setActiveSessionIds(prev => new Set(prev).add(sessionData.id));

        // Store session info for player initialization
        setPlaybackDecision(prev => ({
          ...prev!,
          session_id: sessionData.id,
          manifest_url: `/api/playback/stream/${sessionData.id}/manifest.mpd`
        }));
        
        // Reset seek-ahead state for new regular sessions
        setIsSeekingAhead(false);
      }

      // Episode metadata already loaded above in parallel

      // Data loading complete - now player can initialize
      setLoading(false);

    } catch (err) {
      console.error('❌ Failed to load episode data:', err);
      setError(err instanceof Error ? err.message : 'Failed to load episode');
      setLoading(false);
    } finally {
      isRequestingSession.current = false;
    }
  }, [episodeId]);

  // Control functions
  const togglePlayPause = useCallback(() => {
    if (videoRef.current) {
      if (isPlaying) {
        videoRef.current.pause();
      } else {
        videoRef.current.play();
      }
    }
  }, [isPlaying]);

  const stopVideo = useCallback(() => {
    if (videoRef.current) {
      videoRef.current.pause();
      videoRef.current.currentTime = 0;
      setCurrentTime(0);
      // Clear saved position when explicitly stopped
      localStorage.removeItem(`video-position-${episodeId}`);
      console.log('⏹️ Video stopped and position reset');
    }
  }, [episodeId]);

  const restartFromBeginning = useCallback(async () => {
    if (videoRef.current) {
      console.log('🔄 Restarting video from beginning, cleaning up sessions...');
      
      // Stop all current sessions since we're restarting
      await stopAllSessions();
      
      videoRef.current.currentTime = 0;
      setCurrentTime(0);
      // Clear saved position and start playing
      localStorage.removeItem(`video-position-${episodeId}`);
      videoRef.current.play();
      console.log('✅ Video restarted from beginning, sessions cleaned up');
    }
  }, [episodeId, stopAllSessions]);

  const toggleMute = useCallback(() => {
    if (videoRef.current) {
      videoRef.current.muted = !videoRef.current.muted;
    }
  }, []);

  const toggleFullscreen = useCallback(() => {
    if (!document.fullscreenElement) {
      videoRef.current?.parentElement?.requestFullscreen();
    } else {
      document.exitFullscreen();
    }
  }, []);

  // Enhanced seek-ahead with session cleanup
  const requestSeekAhead = useCallback(async (seekTime: number) => {
    if (!playbackDecision || !mediaFile) {
      console.warn('⚠️ Cannot request seek-ahead: missing playback decision or media file');
      return;
    }

    try {
      console.log('🚀 Requesting seek-ahead to time:', seekTime);
      
      // Extract session ID from manifest URL or use stored session ID
      let sessionId = playbackDecision.session_id;
      if (!sessionId && playbackDecision.manifest_url) {
        const urlMatch = playbackDecision.manifest_url.match(/\/stream\/([^/]+)\//);
        if (urlMatch) {
          sessionId = urlMatch[1];
        }
      }

      if (!sessionId) {
        console.warn('⚠️ No session ID available for seek-ahead');
        return;
      }

      // Stop any other sessions before starting seek-ahead transcoding
      const otherSessions = Array.from(activeSessionIds).filter(id => id !== sessionId);
      if (otherSessions.length > 0) {
        console.log('🧹 Cleaning up other sessions before seek-ahead:', otherSessions);
        await Promise.all(otherSessions.map(id => stopTranscodingSession(id)));
      }

      // Call seek-ahead API to start background transcoding
      const response = await fetch('/api/playback/seek-ahead', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          session_id: sessionId,
          seek_time: Math.floor(seekTime)
        })
      });

      if (response.ok) {
        const seekResponse = await response.json();
        console.log('✅ Seek-ahead transcoding started:', seekResponse);
        setIsSeekingAhead(true);
        
        // Add the new session to active sessions
        if (seekResponse.session_id) {
          setActiveSessionIds(prev => new Set([...prev, seekResponse.session_id]));
        }
        
        // If we have a new manifest URL, update the player
        if (seekResponse.manifest_url && playerRef.current) {
          console.log('🔄 Switching to new manifest URL:', seekResponse.manifest_url);
          
          // Update playback decision with new manifest URL
          setPlaybackDecision(prev => ({
            ...prev!,
            manifest_url: seekResponse.manifest_url,
            session_id: seekResponse.session_id
          }));
          
          // Load the new manifest in Shaka Player
          try {
            await playerRef.current.load(seekResponse.manifest_url);
            console.log('✅ New manifest loaded successfully');
            
            // The new stream starts from the seek position, so set current time to 0
            if (videoRef.current) {
              videoRef.current.currentTime = 0;
            }
          } catch (err) {
            console.error('❌ Failed to load new manifest:', err);
          }
        }
        
        // Reset seeking state after a short delay
        setTimeout(() => {
          setIsSeekingAhead(false);
        }, 5000);
      } else {
        console.warn('⚠️ Seek-ahead request failed:', response.status);
      }
      
    } catch (error) {
      console.error('❌ Failed to request seek-ahead:', error);
    }
  }, [playbackDecision, mediaFile, activeSessionIds, stopTranscodingSession]);

  // Simple, robust seek function
  const handleSeek = useCallback(async (progress: number) => {
    if (!videoRef.current || !playbackDecision) return;

    // Get duration from video element, fallback to original duration
    const rawDuration = videoRef.current.duration;
    const duration = (isFinite(rawDuration) && rawDuration > 0) ? rawDuration : originalDuration;
    
    // If we still don't have a valid duration, can't seek
    if (!duration || duration <= 0) {
      console.warn('❌ No valid duration available for seeking:', { rawDuration, duration, originalDuration });
      return;
    }

    const seekTime = progress * duration;

    console.log('🎯 Seeking to:', { 
      progress, 
      duration, 
      seekTime, 
      rawDuration,
      seekableDuration,
      buffered: videoRef.current.buffered.length > 0 ? videoRef.current.buffered.end(0) : 0
    });

    // Validate seek time before setting
    if (!isFinite(seekTime) || isNaN(seekTime) || seekTime < 0) {
      console.warn('❌ Invalid seek time:', { progress, duration, seekTime });
      return;
    }

    // For DASH/HLS, check actual buffered range instead of seekableDuration
    const actualBufferedEnd = videoRef.current.buffered.length > 0 
      ? videoRef.current.buffered.end(videoRef.current.buffered.length - 1)
      : 0;

    // For normal seeking within buffered content, just seek directly
    if (seekTime <= actualBufferedEnd) {
      videoRef.current.currentTime = seekTime;
      console.log('✅ Normal seek within buffered content:', seekTime);
      return;
    }

    // For DASH/HLS streams, check if we need seek-ahead beyond buffered content
    if (playbackDecision.transcode_params?.target_container === 'dash' || 
        playbackDecision.transcode_params?.target_container === 'hls') {
      
      // If seeking beyond buffered content by more than 30 seconds, use seek-ahead
      if (seekTime > actualBufferedEnd + 30) {
        console.log('🚀 Seeking beyond buffered content, starting seek-ahead transcoding', {
          seekTime,
          actualBufferedEnd,
          difference: seekTime - actualBufferedEnd
        });
        
        // Request seek-ahead transcoding
        await requestSeekAhead(seekTime);
        
        // Don't seek locally - let the new manifest load handle positioning
        console.log('⏸️ Waiting for new manifest to load...');
      } else {
        // Seeking just slightly beyond buffered - the player can handle this
        videoRef.current.currentTime = seekTime;
        console.log('✅ Seeking slightly beyond buffered content (within 30s):', seekTime);
      }
    } else {
      // Direct play - just seek normally
      videoRef.current.currentTime = seekTime;
      console.log('✅ Direct play seek:', seekTime);
    }
  }, [playbackDecision, originalDuration, requestSeekAhead]);

  // Skip functions
  const skipBackward = useCallback(() => {
    if (videoRef.current && duration > 0) {
      const newTime = Math.max(0, currentTime - 10);
      console.log('⏪ Skipping backward to:', newTime);
      handleSeek(newTime / duration);
    }
  }, [currentTime, duration, handleSeek]);

  const skipForward = useCallback(() => {
    if (videoRef.current && duration > 0) {
      const newTime = Math.min(duration, currentTime + 10);
      console.log('⏩ Skipping forward to:', newTime);
      handleSeek(newTime / duration);
    }
  }, [currentTime, duration, handleSeek]);

  // Keyboard shortcuts for seeking
  const handleKeyDown = useCallback((e: KeyboardEvent) => {
    if (videoRef.current && duration > 0) {
      switch (e.code) {
        case 'ArrowLeft':
          e.preventDefault();
          handleSeek((Math.max(0, currentTime - 10) / duration));
          break;
        case 'ArrowRight':
          e.preventDefault();
          handleSeek((Math.min(duration, currentTime + 10) / duration));
          break;
        case 'Home':
          e.preventDefault();
          handleSeek(0);
          break;
        case 'End':
          e.preventDefault();
          handleSeek(1);
          break;
      }
    }
  }, [currentTime, duration, handleSeek]);

  const handleVolumeChange = useCallback((newVolume: number) => {
    if (videoRef.current) {
      videoRef.current.volume = newVolume;
    }
  }, []);

  const formatTime = (time: number) => {
    // Handle invalid time values
    if (!isFinite(time) || isNaN(time) || time < 0) {
      return '0:00';
    }
    
    const hours = Math.floor(time / 3600);
    const minutes = Math.floor((time % 3600) / 60);
    const seconds = Math.floor(time % 60);
    
    if (hours > 0) {
      return `${hours}:${minutes.toString().padStart(2, '0')}:${seconds.toString().padStart(2, '0')}`;
    }
    return `${minutes}:${seconds.toString().padStart(2, '0')}`;
  };

  // Auto-hide controls
  useEffect(() => {
    let hideTimeout: ReturnType<typeof setTimeout>;

    const resetTimeout = () => {
      setShowControls(true);
      clearTimeout(hideTimeout);
      hideTimeout = setTimeout(() => setShowControls(false), 3000);
    };

    const handleMouseMove = () => resetTimeout();
    const handleMouseLeave = () => {
      clearTimeout(hideTimeout);
      setShowControls(false);
    };

    if (videoRef.current?.parentElement) {
      const container = videoRef.current.parentElement;
      container.addEventListener('mousemove', handleMouseMove);
      container.addEventListener('mouseleave', handleMouseLeave);
      resetTimeout();

      return () => {
        container.removeEventListener('mousemove', handleMouseMove);
        container.removeEventListener('mouseleave', handleMouseLeave);
        clearTimeout(hideTimeout);
      };
    }
  }, []);

  // Fullscreen change handler
  useEffect(() => {
    const handleFullscreenChange = () => {
      setIsFullscreen(!!document.fullscreenElement);
    };

    document.addEventListener('fullscreenchange', handleFullscreenChange);
    return () => document.removeEventListener('fullscreenchange', handleFullscreenChange);
  }, []);

  // Keyboard event handler
  useEffect(() => {
    document.addEventListener('keydown', handleKeyDown);
    return () => document.removeEventListener('keydown', handleKeyDown);
  }, [handleKeyDown]);

  // Load data on mount
  useEffect(() => {
    loadEpisodeData();
  }, [loadEpisodeData]);

  // Initialize player when ready
  useEffect(() => {
    console.log('🔍 Player init check:', { 
      hasPlaybackDecision: !!playbackDecision, 
      hasVideoRef: !!videoRef.current, 
      alreadyInitialized: initializationRef.current,
      isRequestingSession: isRequestingSession.current,
      manifestUrl: playbackDecision?.manifest_url 
    });
    
    // Only initialize if not already requesting session and not already initialized
    if (playbackDecision && videoRef.current && !initializationRef.current && !isRequestingSession.current) {
      console.log('✅ Triggering player initialization');
      initializePlayer();
    }
  }, [playbackDecision, initializePlayer]);

  // Callback ref to ensure we catch when video element is attached
  const videoCallbackRef = useCallback((element: HTMLVideoElement | null) => {
    videoRef.current = element;
    console.log('🔍 Video element attached:', !!element);
    
    // Try to initialize player when video element becomes available
    // Only if not already requesting a session and not already initialized
    if (element && playbackDecision && !initializationRef.current && !isRequestingSession.current) {
      console.log('✅ Video element ready - triggering player initialization');
      initializePlayer();
    }
  }, [playbackDecision, initializePlayer]);

  // Cleanup on unmount
  useEffect(() => {
    return () => {
      if (videoRef.current && (videoRef.current as HTMLVideoElement & { cleanupVideoEvents?: () => void }).cleanupVideoEvents) {
        (videoRef.current as HTMLVideoElement & { cleanupVideoEvents?: () => void }).cleanupVideoEvents?.();
      }
      if (cleanupPlayer) {
        cleanupPlayer();
      }
    };
  }, [cleanupPlayer]);

  // Clear any stale session state on component mount
  useEffect(() => {
    console.log('🚀 VideoPlayer mounted, clearing any stale session state...');
    
    // Clear localStorage entries related to old sessions (if any exist)
    const keysToRemove: string[] = [];
    for (let i = 0; i < localStorage.length; i++) {
      const key = localStorage.key(i);
      if (key && key.includes('session') && !key.includes('video-position')) {
        keysToRemove.push(key);
      }
    }
    keysToRemove.forEach(key => {
      console.log('🧹 Removing stale localStorage key:', key);
      localStorage.removeItem(key);
    });
    
    // Reset session tracking state
    setActiveSessionIds(new Set());
  }, [episodeId]); // Reset when episode changes

  if (loading) {
    return (
      <div className="flex items-center justify-center h-screen bg-black text-white">
        <div className="text-center">
          <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-white mx-auto mb-4"></div>
          <p>Loading video...</p>
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="flex items-center justify-center h-screen bg-black text-white">
        <div className="text-center max-w-md">
          <h2 className="text-xl font-bold mb-4">Playback Error</h2>
          <p className="text-red-400 mb-4">{error}</p>
          <button
            onClick={() => window.location.reload()}
            className="bg-blue-600 hover:bg-blue-700 px-4 py-2 rounded"
          >
            Reload Player
          </button>
        </div>
      </div>
    );
  }

  return (
    <div className="relative h-screen bg-black overflow-hidden">
      {/* Back button */}
      <button
        data-tooltip-id="back-button"
        onClick={handleNavigation}
        className="absolute top-4 left-4 z-50 bg-black/50 hover:bg-black/80 hover:scale-110 text-white p-2 rounded-full transition-all duration-200 shadow-lg"
      >
        <ArrowLeft className="w-6 h-6" />
      </button>
      <Tooltip id="back-button" content="Go back" place="bottom" />

      {/* Video container */}
      <div className="relative w-full h-full">
        <video
          ref={videoCallbackRef}
          className="w-full h-full object-contain"
          playsInline
          preload="auto"
          autoPlay={shouldAutoplay}
          muted={false}
          onDoubleClick={restartFromBeginning}
          title="Double-click to restart from beginning"
        />

        {/* Buffering indicator */}
        {isBuffering && (
          <div className="absolute inset-0 flex items-center justify-center bg-black/50 z-40">
            <div className="bg-black/80 rounded-lg p-4 flex items-center space-x-3">
              <div className="animate-spin rounded-full h-6 w-6 border-b-2 border-white"></div>
              <span className="text-white">Buffering...</span>
            </div>
          </div>
        )}

        {/* Loading spinner overlay - shows until video starts playing */}
        {isVideoLoading && !isPlaying && (
          <div className="absolute inset-0 flex items-center justify-center bg-black/20 z-40">
            <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-white"></div>
          </div>
        )}

        {/* Streaming info */}
        {playbackDecision && (
          <div className="absolute top-4 right-4 z-50 bg-black/70 text-white p-3 rounded-lg text-sm">
            <div className="font-semibold mb-1">
              {playbackDecision.should_transcode ? '🎬 DASH Streaming' : '📺 Direct Stream'}
            </div>
            <div className="text-xs text-gray-300">
              {playbackDecision.reason}
            </div>
            {isSeekingAhead && (
              <div className="text-xs text-blue-300 mt-1 animate-pulse">
                ⚡ Transcoding ahead...
              </div>
            )}
            {isStoppingSession && (
              <div className="text-xs text-orange-300 mt-1 animate-pulse">
                🛑 Cleaning up sessions...
              </div>
            )}
            {activeSessionIds.size > 1 && (
              <div className="text-xs text-yellow-300 mt-1">
                📊 {activeSessionIds.size} active sessions
              </div>
            )}
          </div>
        )}

        {/* Custom Controls */}
        <div
          className={`absolute bottom-0 left-0 right-0 bg-gradient-to-t from-black/80 to-transparent p-6 transition-opacity duration-300 ${
            showControls ? 'opacity-100' : 'opacity-0'
          }`}
        >
                  {/* Enhanced Progress Bar with Seek-Ahead Indication */}
        <div className="mb-4">
          <div className="relative group">
            {/* Hover time preview */}
            {hoverTime !== null && (
              <div
                className="absolute -top-8 transform -translate-x-1/2 bg-black bg-opacity-75 text-white px-2 py-1 rounded text-sm pointer-events-none z-10"
                style={{ left: `${(hoverTime / Math.max(duration, originalDuration)) * 100}%` }}
              >
                {formatTime(hoverTime)}
                {videoRef.current && videoRef.current.buffered.length > 0 && 
                 hoverTime > videoRef.current.buffered.end(videoRef.current.buffered.length - 1) && 
                 playbackDecision?.transcode_params?.target_container && (
                  <span className="ml-1 text-blue-300">⚡</span>
                )}
              </div>
            )}
            
            {/* Main progress track */}
            <div 
              className="w-full h-2 bg-gray-600 rounded-full cursor-pointer relative overflow-hidden group-hover:h-3 transition-all"
              onMouseMove={(e) => {
                const rect = e.currentTarget.getBoundingClientRect();
                const progress = (e.clientX - rect.left) / rect.width;
                const totalDuration = Math.max(duration, originalDuration);
                setHoverTime(progress * totalDuration);
              }}
              onMouseLeave={() => setHoverTime(null)}
              onClick={(e) => {
                const rect = e.currentTarget.getBoundingClientRect();
                const progress = (e.clientX - rect.left) / rect.width;
                
                // Use finite duration values only
                const validDuration = (isFinite(duration) && duration > 0) ? duration : 0;
                const validOriginalDuration = (isFinite(originalDuration) && originalDuration > 0) ? originalDuration : 0;
                const totalDuration = Math.max(validDuration, validOriginalDuration);
                
                // Validate values before seeking
                if (!isFinite(progress) || progress < 0 || progress > 1 || !totalDuration || totalDuration <= 0) {
                  console.warn('❌ Invalid progress bar click:', { 
                    progress, 
                    duration, 
                    originalDuration, 
                    validDuration, 
                    validOriginalDuration, 
                    totalDuration 
                  });
                  return;
                }
                
                console.log('🎯 Progress bar seek:', { progress, totalDuration });
                handleSeek(progress);
              }}
            >
              {/* Total duration background */}
              <div className="absolute inset-0 bg-gray-700 rounded-full"></div>
              
              {/* Buffered content (what's actually available for playback) */}
              {videoRef.current && videoRef.current.buffered.length > 0 && (
                <div
                  className="absolute top-0 left-0 h-full bg-gray-400 rounded-full"
                  style={{ 
                    width: `${Math.max(duration, originalDuration) > 0 
                      ? (videoRef.current.buffered.end(videoRef.current.buffered.length - 1) / Math.max(duration, originalDuration)) * 100 
                      : 0}%` 
                  }}
                  title="Buffered content"
                ></div>
              )}
              
              {/* Seek-ahead indicator for unbuffered content */}
              {videoRef.current && videoRef.current.buffered.length > 0 && 
               originalDuration > videoRef.current.buffered.end(videoRef.current.buffered.length - 1) && 
               playbackDecision?.transcode_params?.target_container && (
                <div
                  className="absolute top-0 bg-blue-400 bg-opacity-40 h-full rounded-full"
                  style={{ 
                    left: `${videoRef.current.buffered.end(videoRef.current.buffered.length - 1) / originalDuration * 100}%`,
                    width: `${((originalDuration - videoRef.current.buffered.end(videoRef.current.buffered.length - 1)) / originalDuration) * 100}%`
                  }}
                  title="Click to seek-ahead (will start transcoding from this point)"
                ></div>
              )}
              
              {/* Current playback position */}
              <div
                className="absolute top-0 left-0 h-full bg-red-500 rounded-full"
                style={{ width: `${Math.max(duration, originalDuration) > 0 ? (currentTime / Math.max(duration, originalDuration)) * 100 : 0}%` }}
              ></div>
              
              {/* Progress handle */}
              <div
                className="absolute top-1/2 w-4 h-4 bg-red-500 rounded-full transform -translate-y-1/2 -translate-x-1/2 opacity-0 group-hover:opacity-100 transition-opacity shadow-lg"
                style={{ left: `${Math.max(duration, originalDuration) > 0 ? (currentTime / Math.max(duration, originalDuration)) * 100 : 0}%` }}
              ></div>
            </div>
          </div>
          
          {/* Time display */}
          <div className="flex justify-between text-xs text-gray-300 mt-2">
            <span>{formatTime(currentTime)}</span>
            <span>{formatTime(Math.max(duration, originalDuration))}</span>
          </div>
        </div>

          {/* Control buttons */}
          <div className="flex items-center justify-between">
            <div className="flex items-center space-x-4">
              <button
                data-tooltip-id="skip-back"
                onClick={skipBackward}
                className="text-white hover:text-red-400 hover:scale-110 transition-all duration-200 p-2 rounded-full hover:bg-white/10 disabled:opacity-50 disabled:cursor-not-allowed disabled:hover:scale-100 disabled:hover:bg-transparent"
                disabled={!duration || duration <= 0}
              >
                <SkipBack className="w-6 h-6" />
              </button>
              <Tooltip id="skip-back" content="Skip backward 10 seconds" place="top" />

              <button
                data-tooltip-id="play-pause"
                onClick={togglePlayPause}
                className="text-white hover:text-red-400 hover:scale-110 transition-all duration-200 p-3 rounded-full hover:bg-white/10"
              >
                {isPlaying ? <Pause className="w-8 h-8" /> : <Play className="w-8 h-8" />}
              </button>
              <Tooltip id="play-pause" content={isPlaying ? "Pause" : "Play"} place="top" />

              <button
                data-tooltip-id="skip-forward"
                onClick={skipForward}
                className="text-white hover:text-red-400 hover:scale-110 transition-all duration-200 p-2 rounded-full hover:bg-white/10 disabled:opacity-50 disabled:cursor-not-allowed disabled:hover:scale-100 disabled:hover:bg-transparent"
                disabled={!duration || duration <= 0}
              >
                <SkipForward className="w-6 h-6" />
              </button>
              <Tooltip id="skip-forward" content="Skip forward 10 seconds" place="top" />

              <button
                data-tooltip-id="stop"
                onClick={stopVideo}
                className="text-white hover:text-red-400 hover:scale-110 transition-all duration-200 p-2 rounded-full hover:bg-white/10"
              >
                <Square className="w-6 h-6" />
              </button>
              <Tooltip id="stop" content="Stop and reset to beginning" place="top" />

              <div className="flex items-center space-x-2">
                <button
                  data-tooltip-id="volume"
                  onClick={toggleMute}
                  className="text-white hover:text-red-400 hover:scale-110 transition-all duration-200 p-2 rounded-full hover:bg-white/10"
                >
                  {isMuted ? <VolumeX className="w-6 h-6" /> : <Volume2 className="w-6 h-6" />}
                </button>
                <Tooltip id="volume" content={isMuted ? "Unmute" : "Mute"} place="top" />
                
                <input
                  data-tooltip-id="volume-slider"
                  type="range"
                  min="0"
                  max="1"
                  step="0.1"
                  value={isMuted ? 0 : volume}
                  onChange={(e) => handleVolumeChange(parseFloat(e.target.value))}
                  className="w-20 h-1 bg-gray-600 rounded-lg appearance-none cursor-pointer hover:bg-gray-500 transition-colors duration-200"
                />
                <Tooltip id="volume-slider" content={`Volume: ${Math.round((isMuted ? 0 : volume) * 100)}%`} place="top" />
              </div>
            </div>

            <div className="flex items-center space-x-4">
              <button
                data-tooltip-id="fullscreen"
                onClick={toggleFullscreen}
                className="text-white hover:text-red-400 hover:scale-110 transition-all duration-200 p-2 rounded-full hover:bg-white/10"
              >
                {isFullscreen ? <Minimize2 className="w-6 h-6" /> : <Maximize className="w-6 h-6" />}
              </button>
              <Tooltip id="fullscreen" content={isFullscreen ? "Exit fullscreen" : "Enter fullscreen"} place="top" />
            </div>
          </div>
        </div>
      </div>
    </div>
  );
};

export default VideoPlayer;
