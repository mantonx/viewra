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
      onStoreUpdate(store);
    }
  }, [store, onStoreUpdate]);

  return null;
};