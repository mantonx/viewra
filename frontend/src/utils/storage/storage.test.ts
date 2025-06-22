// TODO: Add Jest tests once testing framework is set up
import {
  getStorageItem,
  setStorageItem,
  removeStorageItem,
  getSavedPosition,
  savePosition,
  clearSavedPosition,
  hasSavedPosition,
  getSavedVolume,
  saveVolume,
  getSavedMutedState,
  saveMutedState,
  getPlayerSettings,
  savePlayerSettings,
  clearPlayerStorage,
  getStorageUsage,
} from './storage';

describe('Storage Utils', () => {
  beforeEach(() => {
    localStorage.clear();
  });

  describe('basic storage operations', () => {
    it('should store and retrieve items', () => {
      setStorageItem('test', 'value');
      expect(getStorageItem('test', 'default')).toBe('value');
    });

    it('should return default for missing items', () => {
      expect(getStorageItem('missing', 'default')).toBe('default');
    });

    it('should remove items', () => {
      setStorageItem('test', 'value');
      removeStorageItem('test');
      expect(getStorageItem('test', 'default')).toBe('default');
    });
  });

  describe('position saving', () => {
    it('should save and retrieve positions', () => {
      savePosition('episode1', 123.5);
      expect(getSavedPosition('episode1')).toBe(123.5);
    });

    it('should check if position exists', () => {
      expect(hasSavedPosition('episode1')).toBe(false);
      savePosition('episode1', 123.5);
      expect(hasSavedPosition('episode1')).toBe(true);
    });

    it('should clear saved positions', () => {
      savePosition('episode1', 123.5);
      clearSavedPosition('episode1');
      expect(hasSavedPosition('episode1')).toBe(false);
    });
  });

  describe('player settings', () => {
    it('should save and retrieve volume', () => {
      saveVolume(0.8);
      expect(getSavedVolume()).toBe(0.8);
    });

    it('should save and retrieve muted state', () => {
      saveMutedState(true);
      expect(getSavedMutedState()).toBe(true);
    });

    it('should save and retrieve player settings', () => {
      savePlayerSettings({ autoplay: false, seekStepSeconds: 15 });
      const settings = getPlayerSettings();
      expect(settings.autoplay).toBe(false);
      expect(settings.seekStepSeconds).toBe(15);
    });
  });
});