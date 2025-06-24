/**
 * Local storage utilities for media player
 */

import type { PlayerSettings, StorageUsage } from './storage.types';

const STORAGE_KEYS = {
  VIDEO_POSITION: 'video-position',
  PLAYER_SETTINGS: 'player-settings',
  VOLUME: 'player-volume',
  MUTED: 'player-muted',
  SUBTITLES: 'player-subtitles',
  QUALITY: 'player-quality',
} as const;

/**
 * Safely gets an item from localStorage with error handling
 * @param key - Storage key
 * @param defaultValue - Default value if key doesn't exist
 * @returns Parsed value or default
 */
export const getStorageItem = <T>(key: string, defaultValue: T): T => {
  try {
    const item = localStorage.getItem(key);
    if (item === null) return defaultValue;
    return JSON.parse(item);
  } catch (error) {
    console.warn(`Failed to parse localStorage item "${key}":`, error);
    return defaultValue;
  }
};

/**
 * Safely sets an item in localStorage with error handling
 * @param key - Storage key
 * @param value - Value to store
 */
export const setStorageItem = <T>(key: string, value: T): void => {
  try {
    localStorage.setItem(key, JSON.stringify(value));
  } catch (error) {
    console.warn(`Failed to set localStorage item "${key}":`, error);
  }
};

/**
 * Removes an item from localStorage
 * @param key - Storage key
 */
export const removeStorageItem = (key: string): void => {
  try {
    localStorage.removeItem(key);
  } catch (error) {
    console.warn(`Failed to remove localStorage item "${key}":`, error);
  }
};

/**
 * Gets saved video position
 * @param mediaId - Media identifier
 * @returns Saved position in seconds
 */
export const getSavedPosition = (mediaId: string): number => {
  const key = `${STORAGE_KEYS.VIDEO_POSITION}-${mediaId}`;
  return getStorageItem(key, 0);
};

/**
 * Saves video position
 * @param mediaId - Media identifier
 * @param position - Position in seconds
 */
export const savePosition = (mediaId: string, position: number): void => {
  const key = `${STORAGE_KEYS.VIDEO_POSITION}-${mediaId}`;
  setStorageItem(key, position);
};

/**
 * Clears saved video position
 * @param mediaId - Media identifier
 */
export const clearSavedPosition = (mediaId: string): void => {
  const key = `${STORAGE_KEYS.VIDEO_POSITION}-${mediaId}`;
  removeStorageItem(key);
};

/**
 * Checks if there's a saved position for media
 * @param mediaId - Media identifier
 * @returns Whether a position is saved
 */
export const hasSavedPosition = (mediaId: string): boolean => {
  const position = getSavedPosition(mediaId);
  return position > 0;
};

/**
 * Gets saved player volume
 * @returns Volume (0-1)
 */
export const getSavedVolume = (): number => {
  return getStorageItem(STORAGE_KEYS.VOLUME, 1);
};

/**
 * Saves player volume
 * @param volume - Volume (0-1)
 */
export const saveVolume = (volume: number): void => {
  setStorageItem(STORAGE_KEYS.VOLUME, Math.max(0, Math.min(1, volume)));
};

/**
 * Gets saved mute state
 * @returns Whether player is muted
 */
export const getSavedMutedState = (): boolean => {
  return getStorageItem(STORAGE_KEYS.MUTED, false);
};

/**
 * Saves mute state
 * @param muted - Whether player is muted
 */
export const saveMutedState = (muted: boolean): void => {
  setStorageItem(STORAGE_KEYS.MUTED, muted);
};



/**
 * Gets saved player settings
 * @returns Player settings
 */
export const getPlayerSettings = (): PlayerSettings => {
  return getStorageItem(STORAGE_KEYS.PLAYER_SETTINGS, {
    autoplay: true,
    subtitlesEnabled: false,
    defaultQuality: 'auto',
    skipIntroSeconds: 10,
    skipOutroSeconds: 30,
    seekStepSeconds: 10,
  });
};

/**
 * Saves player settings
 * @param settings - Player settings to save
 */
export const savePlayerSettings = (settings: Partial<PlayerSettings>): void => {
  const currentSettings = getPlayerSettings();
  const newSettings = { ...currentSettings, ...settings };
  setStorageItem(STORAGE_KEYS.PLAYER_SETTINGS, newSettings);
};

/**
 * Clears all player-related storage
 */
export const clearPlayerStorage = (): void => {
  const keys = Object.values(STORAGE_KEYS);
  keys.forEach(key => {
    // Find all keys that start with this pattern
    for (let i = 0; i < localStorage.length; i++) {
      const storageKey = localStorage.key(i);
      if (storageKey?.startsWith(key)) {
        removeStorageItem(storageKey);
      }
    }
  });
};

/**
 * Gets storage usage information
 * @returns Storage usage stats
 */
export const getStorageUsage = (): StorageUsage => {
  try {
    // Estimate storage usage
    let used = 0;
    for (let i = 0; i < localStorage.length; i++) {
      const key = localStorage.key(i);
      if (key) {
        const value = localStorage.getItem(key);
        used += key.length + (value?.length || 0);
      }
    }
    
    // Browser localStorage is typically 5-10MB
    const available = 5 * 1024 * 1024; // 5MB estimate
    const percentage = (used / available) * 100;
    
    return { used, available, percentage };
  } catch (error) {
    console.warn('Failed to calculate storage usage:', error);
    return { used: 0, available: 0, percentage: 0 };
  }
};