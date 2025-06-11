import React, { useRef, useEffect, useState, useCallback } from 'react';
import { useParams, useNavigate, useSearchParams } from 'react-router-dom';
import { ArrowLeft } from 'lucide-react';

// Import Shaka Player
// @ts-expect-error - Shaka Player doesn't have proper TypeScript definitions
import shaka from 'shaka-player/dist/shaka-player.ui.js';
import 'shaka-player/dist/controls.css';

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

const VideoPlayer: React.FC = () => {
  const { episodeId } = useParams<{ episodeId: string }>();
  const navigate = useNavigate();
  const [searchParams, setSearchParams] = useSearchParams();

  const videoRef = useRef<HTMLVideoElement>(null);
  const playerRef = useRef<typeof shaka.Player | null>(null);
  const uiRef = useRef<typeof shaka.ui.Overlay | null>(null);

  const [episode, setEpisode] = useState<Episode | null>(null);
  const [mediaFile, setMediaFile] = useState<MediaFile | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [isPlaying, setIsPlaying] = useState(false);
  const [duration, setDuration] = useState(0);
  const [isFullscreen, setIsFullscreen] = useState(false);
  const [showControls, setShowControls] = useState(true);
  const [showPlayButton, setShowPlayButton] = useState(false);

  // Get start time from URL params
  const startTime = parseInt(searchParams.get('t') || '0', 10);

  // Load episode data
  const loadEpisodeData = useCallback(async () => {
    if (!episodeId) return;

    setLoading(true);
    setError(null);

    try {
      // First, find the media file for this episode
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

      // Get episode metadata
      const metadataResponse = await fetch(`/api/media/files/${episodeFile.id}/metadata`);
      const metadataData = await metadataResponse.json();

      if (metadataData.metadata?.type === 'episode') {
        setEpisode({
          id: metadataData.metadata.episode_id,
          title: metadataData.metadata.title,
          episode_number: metadataData.metadata.episode_number,
          air_date: metadataData.metadata.air_date,
          description: metadataData.metadata.description,
          duration: metadataData.metadata.duration,
          still_image: metadataData.metadata.still_image,
          season: metadataData.metadata.season,
        });
      }
    } catch (err) {
      console.error('Failed to load episode data:', err);
      setError(err instanceof Error ? err.message : 'Failed to load episode data');
    } finally {
      setLoading(false);
    }
  }, [episodeId]);

  // Initialize Shaka Player for adaptive streaming
  const initializePlayer = useCallback(async () => {
    if (!videoRef.current || !mediaFile) return;

    try {
      console.log('🎬 Initializing Shaka Player...');
      console.log('📁 Media file info:', {
        id: mediaFile.id,
        container: mediaFile.container,
        codec: mediaFile.video_codec,
        size: mediaFile.size_bytes,
        duration: mediaFile.duration,
      });

      // Install polyfills
      shaka.polyfill.installAll();

      // Check for browser support
      if (!shaka.Player.isBrowserSupported()) {
        throw new Error('Browser not supported for Shaka Player');
      }

      console.log('✅ Creating Shaka Player instance...');
      // Create a Player instance using the new pattern
      const player = new shaka.Player();
      await player.attach(videoRef.current);
      playerRef.current = player;

      console.log('⚙️ Configuring player...');
      // Configure player for adaptive streaming
      player.configure({
        streaming: {
          bufferingGoal: 30,
          rebufferingGoal: 5,
          bufferBehind: 30,
          ignoreTextStreamFailures: true,
          alwaysStreamText: false,
          startAtSegmentBoundary: false,
        },
        manifest: {
          retryParameters: {
            timeout: 30000,
            maxAttempts: 3,
            baseDelay: 1000,
            backoffFactor: 2,
            fuzzFactor: 0.5,
          },
        },
      });

      console.log('🛡️ Setting up error handling...');
      // Set up error handling
      player.addEventListener(
        'error',
        (event: { detail: { message?: string; category?: string } }) => {
          console.error('❌ Shaka Player error:', event.detail);
          const errorMessage = event.detail?.message || event.detail?.category || 'Unknown error';
          setError(`Playback error: ${errorMessage}`);
        }
      );

      console.log('🎮 Creating UI overlay...');
      // Create UI overlay
      let videoUrl: string;

      // Check if the file needs transcoding for Shaka Player compatibility
      const needsTranscoding =
        mediaFile.container && !['mp4', 'webm'].includes(mediaFile.container.toLowerCase());

      if (needsTranscoding) {
        // Use transcoding endpoint for incompatible formats (MKV, AVI, etc.)
        videoUrl = `/api/media/files/${mediaFile.id}/transcode.mp4?quality=720p`;
        console.log('🔄 Using transcoding endpoint for', mediaFile.container, 'file:', videoUrl);
        console.log('⚠️ TRANSCODING MODE - Keep this browser tab open to maintain stream!');
      } else {
        // Use direct streaming for compatible formats
        videoUrl = `/api/media/files/${mediaFile.id}/stream`;
        console.log('📺 Using direct streaming for', mediaFile.container, 'file:', videoUrl);
      }

      try {
        console.log('📡 Starting player.load() for video...');
        const loadStartTime = performance.now();

        // Add network monitoring for transcoded streams
        if (needsTranscoding) {
          console.log('🔍 Setting up connection monitoring for transcoded stream...');

          // Monitor for potential issues
          const connectionMonitor = setInterval(() => {
            const video = videoRef.current;
            if (video) {
              console.log('📊 Stream status:', {
                currentTime: video.currentTime.toFixed(2),
                buffered: video.buffered.length > 0 ? video.buffered.end(0).toFixed(2) : 'none',
                readyState: video.readyState,
                networkState: video.networkState,
                paused: video.paused,
                ended: video.ended,
              });

              // Check for stalled playback
              if (video.readyState < 3 && !video.paused && !video.ended) {
                console.warn('⚠️ Video seems to be stalling - ready state:', video.readyState);
              }
            }
          }, 10000); // Log every 10 seconds

          // Clear monitor when component unmounts or video ends
          const cleanup = () => {
            clearInterval(connectionMonitor);
          };

          // Store cleanup function
          (window as Window & { __videoStreamMonitor?: () => void }).__videoStreamMonitor = cleanup;
        }

        await player.load(videoUrl);
        const loadTime = performance.now() - loadStartTime;
        console.log(`✅ Video loaded successfully in ${loadTime.toFixed(2)}ms`);
      } catch (loadError) {
        console.error('❌ Failed to load video:', loadError);
        if (loadError instanceof Error && loadError.message.includes('NETWORK_ERROR')) {
          console.error('🌐 Network error detected - this could be due to:');
          console.error('   • Server-side transcoding process terminated');
          console.error('   • Network connection issues');
          console.error('   • Browser/tab was backgrounded causing connection timeout');
        }
        throw new Error(`Failed to load video: ${loadError}`);
      }

      // Wait for the video to have some basic metadata before creating UI
      const videoElement = videoRef.current;

      // Wait for loadedmetadata event to ensure video properties are available
      await new Promise<void>((resolve) => {
        const handleLoadedMetadata = () => {
          videoElement.removeEventListener('loadedmetadata', handleLoadedMetadata);
          resolve();
        };

        if (videoElement.readyState >= 1) {
          // Metadata already loaded
          resolve();
        } else {
          videoElement.addEventListener('loadedmetadata', handleLoadedMetadata);
        }
      });

      // For transcoded content, use native HTML5 controls instead of Shaka UI
      // Shaka UI has issues with progressive download MP4 streams
      if (needsTranscoding) {
        console.log('🎛️ Using native HTML5 controls for transcoded content');
        videoElement.controls = true;
        uiRef.current = null;
        setShowPlayButton(false);

        // Add additional monitoring for transcoded streams
        const handleError = (e: Event) => {
          console.error('❌ Video element error:', e);
          console.error('   Error details:', {
            error: videoElement.error,
            networkState: videoElement.networkState,
            readyState: videoElement.readyState,
          });
        };

        const handleStalled = () => {
          console.warn('⚠️ Video stalled - network might be slow or connection lost');
        };

        const handleWaiting = () => {
          console.log('⏳ Video is waiting for more data...');
        };

        const handleCanPlay = () => {
          console.log('▶️ Video can start playing');
        };

        videoElement.addEventListener('error', handleError);
        videoElement.addEventListener('stalled', handleStalled);
        videoElement.addEventListener('waiting', handleWaiting);
        videoElement.addEventListener('canplay', handleCanPlay);
      } else {
        // Only try Shaka UI for direct streaming content
        try {
          console.log('🎨 Creating Shaka UI overlay for direct stream...');
          const parentElement = videoElement.parentElement;

          if (!parentElement) {
            throw new Error('Video element must have a parent container');
          }

          // Configure UI options
          const uiConfig = {
            controlPanelElements: [
              'play_pause',
              'time_and_duration',
              'spacer',
              'mute',
              'volume',
              'fullscreen',
            ],
            addSeekBar: true,
            addBigPlayButton: true,
            enableKeyboardPlaybackControls: true,
            enableFullscreenOnRotation: true,
            forceLandscapeOnFullscreen: true,
          };

          const ui = new shaka.ui.Overlay(player, videoElement, parentElement);
          ui.configure(uiConfig);
          uiRef.current = ui;
          console.log('✅ UI configured successfully');

          // Hide our custom controls since Shaka UI is working
          setShowPlayButton(false);
        } catch (uiError) {
          console.warn('⚠️ Failed to create Shaka UI overlay:', uiError);
          console.warn('   Falling back to basic controls...');
          uiRef.current = null;

          // If UI overlay fails, add basic controls to the video element
          videoElement.controls = true;
          setShowPlayButton(true);
        }
      }

      // Try to start playback automatically
      try {
        console.log('🎵 Attempting auto-play...');
        await videoElement.play();
        console.log('✅ Auto-play successful');
        setShowPlayButton(false);
      } catch (playError) {
        console.warn('⚠️ Auto-play failed (expected in many browsers):', playError);
        setShowPlayButton(true);
      }

      // Set start time if provided
      if (startTime > 0 && videoElement) {
        console.log(`⏰ Setting start time to ${startTime} seconds`);
        videoElement.currentTime = startTime;
      }

      // Set up video event listeners with better monitoring
      let lastTimeUpdate = 0;
      const handleTimeUpdate = () => {
        const newTime = Math.floor(videoElement.currentTime);
        // Only update URL every 5 seconds to avoid excessive re-renders
        if (newTime > 0 && newTime !== lastTimeUpdate && newTime % 5 === 0) {
          lastTimeUpdate = newTime;
          // Use replace to avoid creating browser history entries
          setSearchParams(
            (prev) => {
              const newParams = new URLSearchParams(prev);
              newParams.set('t', newTime.toString());
              return newParams;
            },
            { replace: true }
          );
        }
      };

      const handleLoadedMetadata = () => {
        console.log('📊 Video metadata loaded:', {
          duration: videoElement.duration,
          videoWidth: videoElement.videoWidth,
          videoHeight: videoElement.videoHeight,
        });
        setDuration(videoElement.duration);
        if (startTime > 0) {
          videoElement.currentTime = startTime;
        }
      };

      const handlePlay = () => {
        console.log('▶️ Video started playing');
        setIsPlaying(true);
        setShowPlayButton(false);
      };
      const handlePause = () => {
        console.log('⏸️ Video paused');
        setIsPlaying(false);
      };

      videoElement.addEventListener('timeupdate', handleTimeUpdate);
      videoElement.addEventListener('loadedmetadata', handleLoadedMetadata);
      videoElement.addEventListener('play', handlePlay);
      videoElement.addEventListener('pause', handlePause);

      // Cleanup function
      return () => {
        console.log('🧹 Cleaning up video event listeners');
        videoElement.removeEventListener('timeupdate', handleTimeUpdate);
        videoElement.removeEventListener('loadedmetadata', handleLoadedMetadata);
        videoElement.removeEventListener('play', handlePlay);
        videoElement.removeEventListener('pause', handlePause);

        // Clean up connection monitor
        const windowWithMonitor = window as Window & { __videoStreamMonitor?: () => void };
        if (windowWithMonitor.__videoStreamMonitor) {
          windowWithMonitor.__videoStreamMonitor();
          delete windowWithMonitor.__videoStreamMonitor;
        }
      };
    } catch (err) {
      console.error('❌ Failed to initialize player:', err);
      setError(err instanceof Error ? err.message : 'Failed to initialize player');
    }
  }, [mediaFile]);

  // Cleanup player
  const cleanupPlayer = useCallback(() => {
    if (uiRef.current) {
      uiRef.current.destroy();
      uiRef.current = null;
    }
    if (playerRef.current) {
      playerRef.current.destroy();
      playerRef.current = null;
    }
  }, []);

  // Handle fullscreen
  const toggleFullscreen = useCallback(() => {
    if (!videoRef.current) return;

    if (!document.fullscreenElement) {
      videoRef.current.parentElement?.requestFullscreen();
      setIsFullscreen(true);
    } else {
      document.exitFullscreen();
      setIsFullscreen(false);
    }
  }, []);

  // Handle keyboard shortcuts
  const handleKeyDown = useCallback(
    (event: KeyboardEvent) => {
      if (!videoRef.current) return;

      switch (event.code) {
        case 'Space':
          event.preventDefault();
          if (isPlaying) {
            videoRef.current.pause();
          } else {
            videoRef.current.play();
          }
          break;
        case 'ArrowLeft':
          event.preventDefault();
          videoRef.current.currentTime = Math.max(0, videoRef.current.currentTime - 10);
          break;
        case 'ArrowRight':
          event.preventDefault();
          videoRef.current.currentTime = Math.min(duration, videoRef.current.currentTime + 10);
          break;
        case 'ArrowUp':
          event.preventDefault();
          videoRef.current.volume = Math.min(1, videoRef.current.volume + 0.1);
          break;
        case 'ArrowDown':
          event.preventDefault();
          videoRef.current.volume = Math.max(0, videoRef.current.volume - 0.1);
          break;
        case 'KeyF':
          event.preventDefault();
          toggleFullscreen();
          break;
        case 'KeyM':
          event.preventDefault();
          videoRef.current.muted = !videoRef.current.muted;
          break;
      }
    },
    [isPlaying, duration, toggleFullscreen]
  );

  // Auto-hide controls
  useEffect(() => {
    let timeout: number;

    const resetTimeout = () => {
      setShowControls(true);
      clearTimeout(timeout);
      timeout = window.setTimeout(() => {
        if (isPlaying) {
          setShowControls(false);
        }
      }, 3000);
    };

    const handleMouseMove = () => resetTimeout();
    const handleMouseLeave = () => {
      clearTimeout(timeout);
      if (isPlaying) {
        setShowControls(false);
      }
    };

    document.addEventListener('mousemove', handleMouseMove);
    document.addEventListener('mouseleave', handleMouseLeave);
    document.addEventListener('keydown', handleKeyDown);

    resetTimeout();

    return () => {
      clearTimeout(timeout);
      document.removeEventListener('mousemove', handleMouseMove);
      document.removeEventListener('mouseleave', handleMouseLeave);
      document.removeEventListener('keydown', handleKeyDown);
    };
  }, [isPlaying, handleKeyDown]);

  // Load episode data on mount
  useEffect(() => {
    loadEpisodeData();
  }, [loadEpisodeData]);

  // Initialize player when media file is loaded
  useEffect(() => {
    if (mediaFile) {
      initializePlayer();
    }
    return cleanupPlayer;
  }, [mediaFile]);

  // Handle fullscreen change
  useEffect(() => {
    const handleFullscreenChange = () => {
      setIsFullscreen(!!document.fullscreenElement);
    };

    document.addEventListener('fullscreenchange', handleFullscreenChange);
    return () => document.removeEventListener('fullscreenchange', handleFullscreenChange);
  }, []);

  if (loading) {
    return (
      <div className="min-h-screen bg-slate-900 flex items-center justify-center">
        <div className="text-center">
          <div className="animate-spin rounded-full h-12 w-12 border-t-2 border-b-2 border-purple-500 mx-auto mb-4"></div>
          <p className="text-slate-400">Loading episode...</p>
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="min-h-screen bg-slate-900 flex items-center justify-center">
        <div className="text-center max-w-md">
          <div className="text-red-400 text-6xl mb-4">⚠️</div>
          <h2 className="text-xl font-bold text-white mb-2">Playback Error</h2>
          <p className="text-slate-400 mb-4">{error}</p>
          <button
            onClick={() => navigate(-1)}
            className="px-4 py-2 bg-purple-600 hover:bg-purple-700 text-white rounded transition-colors"
          >
            Go Back
          </button>
        </div>
      </div>
    );
  }

  if (!episode || !mediaFile) {
    return (
      <div className="min-h-screen bg-slate-900 flex items-center justify-center">
        <div className="text-center">
          <p className="text-slate-400">Episode not found</p>
          <button
            onClick={() => navigate(-1)}
            className="mt-4 px-4 py-2 bg-purple-600 hover:bg-purple-700 text-white rounded transition-colors"
          >
            Go Back
          </button>
        </div>
      </div>
    );
  }

  return (
    <div
      className={`${isFullscreen ? 'fixed inset-0 z-50' : 'min-h-screen'} bg-black flex flex-col`}
    >
      {/* Header with show info - hidden in fullscreen */}
      {!isFullscreen && (
        <div className="bg-slate-900 p-4 flex items-center gap-4">
          <button
            onClick={() => navigate(-1)}
            className="p-2 hover:bg-slate-800 rounded-lg transition-colors"
          >
            <ArrowLeft className="w-5 h-5 text-white" />
          </button>

          <div className="flex items-center gap-4 flex-1">
            {episode.season.tv_show.poster && (
              <img
                src={episode.season.tv_show.poster}
                alt={episode.season.tv_show.title}
                className="w-12 h-16 object-cover rounded"
                onError={(e) => {
                  const target = e.target as HTMLImageElement;
                  if (episode.season.tv_show.id) {
                    target.src = `/api/v1/assets/entity/tv_show/${episode.season.tv_show.id}/preferred/poster/data`;
                  } else {
                    target.style.display = 'none';
                  }
                }}
              />
            )}

            <div>
              <h1 className="text-white font-bold text-lg">{episode.season.tv_show.title}</h1>
              <p className="text-slate-400">
                Season {episode.season.season_number}, Episode {episode.episode_number}:{' '}
                {episode.title}
              </p>
              {episode.description && (
                <p className="text-slate-500 text-sm mt-1 line-clamp-2">{episode.description}</p>
              )}
            </div>
          </div>

          <div className="text-slate-400 text-sm">
            {mediaFile.resolution && <span className="mr-4">{mediaFile.resolution}</span>}
            {mediaFile.video_codec && (
              <span className="mr-4">{mediaFile.video_codec.toUpperCase()}</span>
            )}
            {mediaFile.audio_codec && <span>{mediaFile.audio_codec.toUpperCase()}</span>}
          </div>
        </div>
      )}

      {/* Development monitoring overlay for transcoded streams */}
      {import.meta.env.DEV &&
        mediaFile?.container &&
        !['mp4', 'webm'].includes(mediaFile.container.toLowerCase()) && (
          <div className="fixed top-4 right-4 bg-black/80 text-white p-3 rounded text-xs font-mono z-50 max-w-xs">
            <div className="text-yellow-400 font-bold mb-1">🔄 TRANSCODING MODE</div>
            <div>Container: {mediaFile.container}</div>
            <div>Quality: 720p</div>
            <div className="text-yellow-300 mt-1 text-xs">⚠️ Keep tab open to maintain stream</div>
            <div className="text-gray-400 mt-1 text-xs">Check console for detailed logs</div>
          </div>
        )}

      {/* Video Player Container */}
      <div className={`relative flex-1 bg-black ${isFullscreen ? 'w-full h-full' : ''}`}>
        {/* Background artwork */}
        {episode.season.tv_show.backdrop && (
          <div
            className="absolute inset-0 opacity-10 bg-cover bg-center blur-sm"
            style={{
              backgroundImage: `url("${episode.season.tv_show.backdrop}")`,
            }}
          />
        )}

        {/* Video Element */}
        <video
          ref={videoRef}
          className="w-full h-full object-contain"
          poster={episode.still_image || episode.season.tv_show.backdrop}
          preload="metadata"
          crossOrigin="anonymous"
        />

        {/* Play button overlay for when auto-play is blocked */}
        {showPlayButton && (
          <div className="absolute inset-0 flex items-center justify-center bg-black/50 z-10">
            <button
              onClick={() => {
                if (videoRef.current) {
                  videoRef.current.play();
                  setShowPlayButton(false);
                }
              }}
              className="bg-purple-600 hover:bg-purple-700 text-white rounded-full p-6 transition-colors shadow-lg"
            >
              <svg className="w-12 h-12" fill="currentColor" viewBox="0 0 24 24">
                <path d="M8 5v14l11-7z" />
              </svg>
            </button>
          </div>
        )}

        {/* Custom overlay for additional info */}
        <div
          className={`absolute inset-0 pointer-events-none transition-opacity duration-300 ${showControls ? 'opacity-100' : 'opacity-0'}`}
        >
          {/* Episode info overlay */}
          <div className="absolute top-4 left-4 bg-black/70 rounded-lg p-3 max-w-md">
            <h3 className="text-white font-semibold text-sm">
              S{episode.season.season_number}E{episode.episode_number}: {episode.title}
            </h3>
            {episode.air_date && (
              <p className="text-slate-300 text-xs mt-1">
                Aired: {new Date(episode.air_date).toLocaleDateString()}
              </p>
            )}
          </div>

          {/* Technical info overlay */}
          <div className="absolute top-4 right-4 bg-black/70 rounded-lg p-2 text-xs text-slate-300">
            <div>Direct Play • No Transcoding</div>
            {mediaFile.container && <div>Container: {mediaFile.container.toUpperCase()}</div>}
            {mediaFile.video_codec && <div>Video: {mediaFile.video_codec.toUpperCase()}</div>}
            {mediaFile.audio_codec && <div>Audio: {mediaFile.audio_codec.toUpperCase()}</div>}
          </div>
        </div>
      </div>

      {/* Episode details - hidden in fullscreen */}
      {!isFullscreen && episode.description && (
        <div className="bg-slate-900 p-4">
          <h3 className="text-white font-semibold mb-2">Episode Description</h3>
          <p className="text-slate-400 text-sm leading-relaxed">{episode.description}</p>
        </div>
      )}
    </div>
  );
};

export default VideoPlayer;
