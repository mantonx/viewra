import React from 'react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, fireEvent } from '@/test/simple-utils';
import { VideoElement } from './VideoElement';

// Create a mock function that we can access in tests
let mockUseAtom = vi.fn();

// Mock jotai
vi.mock('jotai', () => ({
  atom: vi.fn((initialValue) => ({ init: initialValue })),
  useAtom: (...args: any[]) => mockUseAtom(...args),
}));

// Mock the atoms module
vi.mock('@/atoms/mediaPlayer', () => ({
  videoElementAtom: { init: null, toString: () => 'videoElementAtom' },
  shakaPlayerAtom: { init: null, toString: () => 'shakaPlayerAtom' },
  configAtom: { init: { debug: false }, toString: () => 'configAtom' },
}));

describe('VideoElement', () => {
  const defaultProps = {
    onLoadedMetadata: vi.fn(),
    onLoadedData: vi.fn(),
    onTimeUpdate: vi.fn(),
    onPlay: vi.fn(),
    onPause: vi.fn(),
    onVolumeChange: vi.fn(),
    onDurationChange: vi.fn(),
    onCanPlay: vi.fn(),
    onWaiting: vi.fn(),
    onPlaying: vi.fn(),
    onStalled: vi.fn(),
    onDoubleClick: vi.fn(),
  };

  beforeEach(() => {
    vi.clearAllMocks();
    
    // Reset mockUseAtom for each test
    mockUseAtom = vi.fn((atom: any) => {
      if (atom.toString().includes('videoElement')) {
        return [null, vi.fn()];
      }
      if (atom.toString().includes('shakaPlayer')) {
        return [null, vi.fn()];
      }
      if (atom.toString().includes('config')) {
        return [{ debug: false }, vi.fn()];
      }
      return [null, vi.fn()];
    });
  });

  it('renders video element', () => {
    const { container } = render(<VideoElement {...defaultProps} />);
    const video = container.querySelector('video');
    expect(video).toBeTruthy();
  });

  it('applies correct CSS classes', () => {
    const { container } = render(<VideoElement {...defaultProps} />);
    const video = container.querySelector('video');
    
    expect(video?.classList.contains('w-full')).toBe(true);
    expect(video?.classList.contains('h-full')).toBe(true);
    expect(video?.classList.contains('object-contain')).toBe(true);
  });

  it('sets video attributes correctly', () => {
    const { container } = render(
      <VideoElement 
        {...defaultProps} 
        autoPlay={true}
        muted={true}
        preload="metadata"
      />
    );
    const video = container.querySelector('video');
    
    expect(video?.getAttribute('autoplay')).toBe('');
    expect(video?.getAttribute('muted')).toBe('');
    expect(video?.getAttribute('preload')).toBe('metadata');
    expect(video?.hasAttribute('playsinline')).toBe(true);
  });

  it('attaches event listeners', () => {
    const { container } = render(<VideoElement {...defaultProps} />);
    const video = container.querySelector('video') as HTMLVideoElement;

    // Simulate events
    fireEvent.loadedMetadata(video);
    expect(defaultProps.onLoadedMetadata).toHaveBeenCalled();

    fireEvent.play(video);
    expect(defaultProps.onPlay).toHaveBeenCalled();

    fireEvent.pause(video);
    expect(defaultProps.onPause).toHaveBeenCalled();

    fireEvent.timeUpdate(video);
    expect(defaultProps.onTimeUpdate).toHaveBeenCalled();

    fireEvent.volumeChange(video);
    expect(defaultProps.onVolumeChange).toHaveBeenCalled();

    fireEvent.durationChange(video);
    expect(defaultProps.onDurationChange).toHaveBeenCalled();

    fireEvent.canPlay(video);
    expect(defaultProps.onCanPlay).toHaveBeenCalled();

    fireEvent.waiting(video);
    expect(defaultProps.onWaiting).toHaveBeenCalled();

    fireEvent.playing(video);
    expect(defaultProps.onPlaying).toHaveBeenCalled();

    fireEvent.stalled(video);
    expect(defaultProps.onStalled).toHaveBeenCalled();

    fireEvent.doubleClick(video);
    expect(defaultProps.onDoubleClick).toHaveBeenCalled();
  });

  it('cleans up event listeners on unmount', () => {
    const { container, unmount } = render(<VideoElement {...defaultProps} />);
    const video = container.querySelector('video') as HTMLVideoElement;

    // Mock removeEventListener
    const removeEventListenerSpy = vi.spyOn(video, 'removeEventListener');

    unmount();

    // Check that event listeners were removed
    expect(removeEventListenerSpy).toHaveBeenCalledWith('loadedmetadata', expect.any(Function));
    expect(removeEventListenerSpy).toHaveBeenCalledWith('play', expect.any(Function));
    expect(removeEventListenerSpy).toHaveBeenCalledWith('pause', expect.any(Function));
  });

  it('exposes ref methods correctly', () => {
    const ref = React.createRef<any>();
    render(
      <VideoElement {...defaultProps} ref={ref} />
    );

    expect(ref.current).toBeTruthy();
    expect(ref.current.videoElement).toBeTruthy();
    expect(ref.current.loadManifest).toBeInstanceOf(Function);
    expect(ref.current.destroy).toBeInstanceOf(Function);
  });

  it('handles custom className prop', () => {
    const { container } = render(
      <VideoElement {...defaultProps} className="custom-class" />
    );
    const video = container.querySelector('video');
    
    expect(video?.classList.contains('custom-class')).toBe(true);
    expect(video?.classList.contains('w-full')).toBe(true);
    expect(video?.classList.contains('h-full')).toBe(true);
  });

  it('only attaches provided event handlers', () => {
    const minimalProps = {
      onPlay: vi.fn(),
      onPause: vi.fn(),
    };
    
    const { container } = render(<VideoElement {...minimalProps} />);
    const video = container.querySelector('video') as HTMLVideoElement;
    
    // Check that only provided handlers were attached by triggering events
    fireEvent.play(video);
    fireEvent.pause(video);
    
    expect(minimalProps.onPlay).toHaveBeenCalled();
    expect(minimalProps.onPause).toHaveBeenCalled();
  });

  it('handles loadManifest method with Shaka player', async () => {
    const ref = React.createRef<any>();
    const mockShaka = {
      load: vi.fn().mockResolvedValue(undefined),
      destroy: vi.fn(),
    };
    
    // Mock useAtom to return our Shaka player
    mockUseAtom = vi.fn((atom: any) => {
      if (atom.toString().includes('shakaPlayer')) {
        return [mockShaka, vi.fn()];
      }
      if (atom.toString().includes('config')) {
        return [{ debug: false }, vi.fn()];
      }
      return [null, vi.fn()];
    });
    
    render(<VideoElement {...defaultProps} ref={ref} />);

    await ref.current?.loadManifest('test.mpd');
    expect(mockShaka.load).toHaveBeenCalledWith('test.mpd');
  });

  it('throws error when loadManifest called without Shaka player', async () => {
    const ref = React.createRef<any>();
    render(<VideoElement {...defaultProps} ref={ref} />);

    await expect(ref.current?.loadManifest('test.mpd')).rejects.toThrow('Shaka player not initialized');
  });

  it('handles destroy method', () => {
    const ref = React.createRef<any>();
    const mockShaka = {
      destroy: vi.fn(),
      load: vi.fn(),
    };
    const setShakaPlayer = vi.fn();
    
    mockUseAtom = vi.fn((atom: any) => {
      if (atom.toString().includes('shakaPlayer')) {
        return [mockShaka, setShakaPlayer];
      }
      if (atom.toString().includes('config')) {
        return [{ debug: false }, vi.fn()];
      }
      return [null, vi.fn()];
    });
    
    render(<VideoElement {...defaultProps} ref={ref} />);

    ref.current?.destroy();
    expect(mockShaka.destroy).toHaveBeenCalled();
    expect(setShakaPlayer).toHaveBeenCalledWith(null);
  });
});