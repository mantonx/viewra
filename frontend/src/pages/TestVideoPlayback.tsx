import React, { useState } from 'react';
import { MediaPlayer, MediaProvider } from '@vidstack/react';
import { MediaService } from '../services/MediaService';

export default function TestVideoPlayback() {
  const [sessionId, setSessionId] = useState<string>('');
  const [manifestUrl, setManifestUrl] = useState<string>('');
  const [status, setStatus] = useState<string>('idle');
  const [error, setError] = useState<string>('');

  const startPlayback = async () => {
    try {
      setStatus('starting');
      setError('');
      
      // Use a known episode ID
      const mediaFileId = 'ffff2929-a038-46ba-a4ed-739dd08b88a2';
      
      const session = await MediaService.startTranscodingSession(
        mediaFileId,
        'dash',
        'h264',
        'aac',
        70,
        'balanced'
      );
      
      console.log('Session started:', session);
      setSessionId(session.id);
      const fullManifestUrl = `http://localhost:5175${session.manifest_url}`;
      setManifestUrl(fullManifestUrl);
      
      // Wait for manifest
      setStatus('waiting for manifest');
      const manifestReady = await MediaService.waitForManifest(fullManifestUrl);
      
      if (manifestReady) {
        setStatus('manifest ready - playing');
        
        // Verify the manifest is accessible
        const response = await fetch(fullManifestUrl);
        const manifestText = await response.text();
        console.log('Manifest content:', manifestText.substring(0, 500));
        
        // Vidstack will handle loading the manifest through the src prop
      }
    } catch (err) {
      console.error('Playback error:', err);
      setError(err.message || 'Unknown error');
      setStatus('error');
    }
  };

  const stopPlayback = async () => {
    if (sessionId) {
      try {
        await MediaService.stopTranscodingSession(sessionId);
        setSessionId('');
        setManifestUrl('');
        setStatus('stopped');
      } catch (err) {
        console.error('Stop error:', err);
      }
    }
  };

  return (
    <div className="p-8">
      <h1 className="text-2xl font-bold mb-4">Video Playback Test</h1>
      
      <div className="mb-4 space-y-2">
        <p>Status: <span className="font-mono">{status}</span></p>
        {sessionId && <p>Session ID: <span className="font-mono text-xs">{sessionId}</span></p>}
        {manifestUrl && <p>Manifest URL: <span className="font-mono text-xs">{manifestUrl}</span></p>}
        {error && <p className="text-red-500">Error: {error}</p>}
      </div>
      
      <div className="space-x-4 mb-4">
        <button 
          onClick={startPlayback}
          disabled={status === 'starting' || status === 'waiting for manifest'}
          className="px-4 py-2 bg-blue-500 text-white rounded disabled:opacity-50"
        >
          Start Playback
        </button>
        
        <button 
          onClick={stopPlayback}
          disabled={!sessionId}
          className="px-4 py-2 bg-red-500 text-white rounded disabled:opacity-50"
        >
          Stop Playback
        </button>
      </div>
      
      {manifestUrl && status === 'manifest ready - playing' && (
        <div className="mt-4">
          <p className="mb-2">Manifest is ready! Vidstack player will load this manifest.</p>
          <p className="text-sm text-gray-600 mb-4">Check browser console for manifest content.</p>
          
          {/* Vidstack Player for testing */}
          <MediaPlayer
            className="w-full max-w-4xl mx-auto"
            title="Test Video Playback"
            src={manifestUrl}
            autoPlay={false}
            crossOrigin="anonymous"
            playsInline
          >
            <MediaProvider />
          </MediaPlayer>
        </div>
      )}
    </div>
  );
}