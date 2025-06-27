import React, { useEffect } from 'react';
import { useMediaRemote, useMediaStore } from '@vidstack/react';

interface VidstackControlsProps {
  onRemoteReady: (remote: ReturnType<typeof useMediaRemote>) => void;
  onStoreUpdate: (store: ReturnType<typeof useMediaStore>) => void;
}

export const VidstackControls: React.FC<VidstackControlsProps> = ({ 
  onRemoteReady, 
  onStoreUpdate 
}) => {
  const remote = useMediaRemote();
  const store = useMediaStore();

  useEffect(() => {
    if (remote) {
      onRemoteReady(remote);
    }
  }, [remote, onRemoteReady]);

  useEffect(() => {
    if (store) {
      // Debug: Log store state when duration is suspiciously low
      if (store.duration < 1) {
        console.log('🎬 Vidstack store state (low duration):', {
          duration: store.duration,
          currentTime: store.currentTime,
          playing: store.playing,
          paused: store.paused,
          ended: store.ended,
          canPlay: store.canPlay,
          buffered: store.buffered,
          seekableStart: store.seekableStart,
          seekableEnd: store.seekableEnd,
        });
      }
      onStoreUpdate(store);
    }
  }, [store, onStoreUpdate]);

  return null;
};