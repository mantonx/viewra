import React, { useEffect, useRef } from 'react';

interface TestDashPlayerProps {
  src: string;
}

export const TestDashPlayer: React.FC<TestDashPlayerProps> = ({ src }) => {
  const videoRef = useRef<HTMLVideoElement>(null);
  const dashPlayerRef = useRef<any>(null);

  useEffect(() => {
    if (!videoRef.current || !src) return;

    const loadDashJs = async () => {
      try {
        console.log('🎬 Loading DASH.js for test player...');
        
        // Dynamically import dash.js
        const dashjs = await import('https://cdn.jsdelivr.net/npm/dashjs@latest/dist/dash.all.min.js');
        
        // Create player instance
        const player = dashjs.MediaPlayer().create();
        dashPlayerRef.current = player;
        
        // Initialize player
        player.initialize(videoRef.current, src, true);
        
        // Add event listeners for debugging
        player.on('error', (e: any) => {
          console.error('❌ DASH.js error:', e);
        });
        
        player.on('manifestLoaded', (e: any) => {
          console.log('✅ DASH.js manifest loaded:', e);
        });
        
        player.on('streamInitialized', (e: any) => {
          console.log('✅ DASH.js stream initialized:', e);
        });
        
        console.log('🎯 DASH.js player initialized with URL:', src);
      } catch (error) {
        console.error('❌ Failed to load DASH.js:', error);
      }
    };

    // Load script tag instead since dynamic import might not work
    const script = document.createElement('script');
    script.src = 'https://cdn.jsdelivr.net/npm/dashjs@latest/dist/dash.all.min.js';
    script.onload = () => {
      console.log('✅ DASH.js script loaded');
      
      // @ts-ignore
      if (window.dashjs) {
        // @ts-ignore
        const player = window.dashjs.MediaPlayer().create();
        dashPlayerRef.current = player;
        
        player.initialize(videoRef.current, src, true);
        
        player.on('error', (e: any) => {
          console.error('❌ DASH.js error:', e);
          console.error('❌ Error details:', {
            error: e.error,
            event: e.event,
            type: e.type
          });
        });
        
        player.on('manifestLoaded', (e: any) => {
          console.log('✅ DASH.js manifest loaded successfully');
        });
        
        player.on('streamInitialized', (e: any) => {
          console.log('✅ DASH.js stream initialized');
        });
        
        console.log('🎯 DASH.js player initialized with URL:', src);
      }
    };
    
    script.onerror = () => {
      console.error('❌ Failed to load DASH.js script');
    };
    
    document.head.appendChild(script);

    return () => {
      // Cleanup
      if (dashPlayerRef.current) {
        dashPlayerRef.current.destroy();
      }
      if (script.parentNode) {
        script.parentNode.removeChild(script);
      }
    };
  }, [src]);

  return (
    <div className="w-full h-full bg-black">
      <video
        ref={videoRef}
        className="w-full h-full"
        controls
      />
      <div className="text-white p-4">
        <p>Testing DASH playback with direct DASH.js</p>
        <p className="text-xs">URL: {src}</p>
      </div>
    </div>
  );
};