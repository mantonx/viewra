import React, { useEffect, useRef } from 'react';

interface DebugDashPlayerProps {
  manifestUrl: string;
}

export const DebugDashPlayer: React.FC<DebugDashPlayerProps> = ({ manifestUrl }) => {
  const videoRef = useRef<HTMLVideoElement>(null);
  const logRef = useRef<HTMLDivElement>(null);

  const log = (message: string, data?: any) => {
    const timestamp = new Date().toISOString().split('T')[1].split('.')[0];
    const logEntry = `[${timestamp}] ${message}`;
    console.log(logEntry, data);
    if (logRef.current) {
      logRef.current.innerHTML += `${logEntry}\n`;
      if (data) {
        logRef.current.innerHTML += `  ${JSON.stringify(data, null, 2)}\n`;
      }
      logRef.current.scrollTop = logRef.current.scrollHeight;
    }
  };

  useEffect(() => {
    if (!videoRef.current || !manifestUrl) return;

    log('Loading manifest:', manifestUrl);

    // Try native video element first
    const video = videoRef.current;
    
    // Check if browser supports DASH natively (unlikely but worth trying)
    if (video.canPlayType('application/dash+xml')) {
      log('Browser claims native DASH support');
      video.src = manifestUrl;
      video.play().catch(err => log('Native playback failed:', err.message));
    } else {
      log('No native DASH support, loading dash.js...');
      
      // Load dash.js dynamically
      const script = document.createElement('script');
      script.src = 'https://cdn.dashjs.org/latest/dash.all.min.js';
      script.onload = () => {
        log('dash.js loaded, initializing player...');
        
        // @ts-ignore
        const dashjs = window.dashjs;
        const player = dashjs.MediaPlayer().create();
        
        // Enable debug logging
        player.updateSettings({
          'debug': {
            'logLevel': dashjs.Debug.LOG_LEVEL_INFO
          }
        });
        
        // Add event listeners
        player.on(dashjs.MediaPlayer.events.ERROR, (e: any) => {
          log('DASH ERROR:', {
            error: e.error.message,
            code: e.error.code,
            data: e.error.data
          });
        });
        
        player.on(dashjs.MediaPlayer.events.MANIFEST_LOADED, (e: any) => {
          log('Manifest loaded successfully');
        });
        
        player.on(dashjs.MediaPlayer.events.STREAM_INITIALIZED, (e: any) => {
          log('Stream initialized');
        });
        
        player.on(dashjs.MediaPlayer.events.FRAGMENT_LOADING_STARTED, (e: any) => {
          log('Loading fragment:', e.request?.url);
        });
        
        try {
          log('Initializing dash.js player...');
          player.initialize(video, manifestUrl, true);
        } catch (err: any) {
          log('Failed to initialize player:', err.message);
        }
      };
      
      document.head.appendChild(script);
    }
  }, [manifestUrl]);

  return (
    <div className="debug-dash-player p-4">
      <h3 className="text-lg font-semibold mb-2">Debug DASH Player</h3>
      <p className="text-sm mb-2">Manifest URL: {manifestUrl}</p>
      
      <video
        ref={videoRef}
        controls
        className="w-full max-w-4xl mb-4"
      />
      
      <div className="bg-gray-100 p-2 rounded">
        <h4 className="font-semibold mb-1">Debug Log:</h4>
        <pre
          ref={logRef}
          className="text-xs font-mono h-48 overflow-y-auto bg-white p-2 rounded"
        />
      </div>
    </div>
  );
};