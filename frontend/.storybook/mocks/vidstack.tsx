import React from 'react';

// Mock MediaStore
export const createMockMediaStore = (initialState = {}) => {
  const defaultState = {
    playing: false,
    paused: true,
    duration: 0,
    currentTime: 0,
    volume: 1,
    muted: false,
    buffering: false,
    quality: null,
    ...initialState,
  };

  let state = defaultState;
  const subscribers: Array<() => void> = [];

  return {
    getState: () => state,
    setState: (newState: any) => {
      state = { ...state, ...newState };
      subscribers.forEach(callback => callback());
    },
    subscribe: (callback: () => void) => {
      subscribers.push(callback);
      return () => {
        const index = subscribers.indexOf(callback);
        if (index > -1) {
          subscribers.splice(index, 1);
        }
      };
    },
  };
};

// Mock hooks
export const useMediaStore = () => {
  // Use global mock store if available (for Storybook stories)
  if ((window as any).__mockMediaStore) {
    return (window as any).__mockMediaStore;
  }
  return createMockMediaStore();
};

export const useMediaRemote = () => ({
  play: () => {},
  pause: () => {},
  seek: () => {},
  setVolume: () => {},
  setMuted: () => {},
  requestFullscreen: () => {},
  exitFullscreen: () => {},
});

// Mock components
export const MediaPlayer = ({ children, ...props }: any) => (
  <div data-testid="vidstack-player" {...props}>
    {children}
  </div>
);

export const MediaProvider = ({ children }: any) => (
  <div data-testid="media-provider">{children}</div>
);

export const Poster = (props: any) => <img data-testid="poster" {...props} />;

export const Track = (props: any) => <track data-testid="track" {...props} />;

export const MediaGesture = ({ children, ...props }: any) => (
  <div data-testid="media-gesture" {...props}>{children}</div>
);