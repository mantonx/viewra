import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, waitFor } from '@testing-library/react';
import { QualityIndicator } from './QualityIndicator';

// Mock useMediaStore
const mockStore = {
  subscribe: vi.fn(),
  getState: vi.fn(),
};

vi.mock('@vidstack/react', () => ({
  useMediaStore: () => mockStore,
}));

describe('QualityIndicator', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    // Reset mock implementations
    mockStore.subscribe.mockImplementation((callback) => {
      // Return unsubscribe function
      return vi.fn();
    });
    mockStore.getState.mockReturnValue({
      quality: null,
    });
  });

  it('renders nothing when there is no quality data', () => {
    const { container } = render(<QualityIndicator />);
    expect(container.firstChild).toBeNull();
  });

  it('renders quality indicator when quality data is available', async () => {
    mockStore.getState.mockReturnValue({
      quality: {
        height: 1080,
        width: 1920,
        bitrate: 5000000,
      },
    });

    // Trigger subscription callback
    mockStore.subscribe.mockImplementation((callback) => {
      setTimeout(() => callback(), 0);
      return vi.fn();
    });

    const { getByText } = render(<QualityIndicator />);

    await waitFor(() => {
      expect(getByText('1080p')).toBeInTheDocument();
      expect(getByText('5.0 Mbps')).toBeInTheDocument();
    });
  });

  it('displays correct quality labels for different resolutions', async () => {
    const testCases = [
      { height: 2160, label: '4K' },
      { height: 1080, label: '1080p' },
      { height: 720, label: '720p' },
      { height: 480, label: '480p' },
      { height: 360, label: '360p' },
      { height: 240, label: '240p' },
      { height: 144, label: 'Auto' },
    ];

    for (const testCase of testCases) {
      mockStore.getState.mockReturnValue({
        quality: {
          height: testCase.height,
          width: 1920,
          bitrate: 5000000,
        },
      });

      mockStore.subscribe.mockImplementation((callback) => {
        setTimeout(() => callback(), 0);
        return vi.fn();
      });

      const { getByText, unmount } = render(<QualityIndicator />);

      await waitFor(() => {
        expect(getByText(testCase.label)).toBeInTheDocument();
      });

      unmount();
    }
  });

  it('shows upgrade animation when quality improves', async () => {
    // Start with 480p
    mockStore.getState.mockReturnValue({
      quality: {
        height: 480,
        width: 854,
        bitrate: 1200000,
      },
    });

    let triggerUpdate: () => void;
    mockStore.subscribe.mockImplementation((callback) => {
      triggerUpdate = callback;
      setTimeout(() => callback(), 0);
      return vi.fn();
    });

    const { container, rerender } = render(<QualityIndicator />);

    await waitFor(() => {
      expect(container.querySelector('.animate-pulse')).not.toBeInTheDocument();
    });

    // Upgrade to 1080p
    mockStore.getState.mockReturnValue({
      quality: {
        height: 1080,
        width: 1920,
        bitrate: 5000000,
      },
    });

    // Trigger update
    triggerUpdate!();
    rerender(<QualityIndicator />);

    await waitFor(() => {
      const indicator = container.querySelector('.animate-pulse');
      expect(indicator).toBeInTheDocument();
    });
  });

  it('calculates bitrate display correctly', async () => {
    const testCases = [
      { bitrate: 500000, display: '0.5 Mbps' },
      { bitrate: 1500000, display: '1.5 Mbps' },
      { bitrate: 5500000, display: '5.5 Mbps' },
      { bitrate: 10000000, display: '10.0 Mbps' },
    ];

    for (const testCase of testCases) {
      mockStore.getState.mockReturnValue({
        quality: {
          height: 1080,
          width: 1920,
          bitrate: testCase.bitrate,
        },
      });

      mockStore.subscribe.mockImplementation((callback) => {
        setTimeout(() => callback(), 0);
        return vi.fn();
      });

      const { getByText, unmount } = render(<QualityIndicator />);

      await waitFor(() => {
        expect(getByText(testCase.display)).toBeInTheDocument();
      });

      unmount();
    }
  });

  it('applies correct quality colors based on resolution', async () => {
    const testCases = [
      { height: 1080, color: '#4ade80' }, // green-400
      { height: 720, color: '#facc15' },  // yellow-400
      { height: 480, color: '#fb923c' },  // orange-400
      { height: 360, color: '#f87171' },  // red-400
    ];

    for (const testCase of testCases) {
      mockStore.getState.mockReturnValue({
        quality: {
          height: testCase.height,
          width: 1920,
          bitrate: 5000000,
        },
      });

      mockStore.subscribe.mockImplementation((callback) => {
        setTimeout(() => callback(), 0);
        return vi.fn();
      });

      const { container, unmount } = render(<QualityIndicator />);

      await waitFor(() => {
        const indicator = container.querySelector('[style*="borderColor"]');
        expect(indicator).toHaveStyle({ borderColor: testCase.color });
      });

      unmount();
    }
  });

  it('hides indicator after 3 seconds', async () => {
    vi.useFakeTimers();

    mockStore.getState.mockReturnValue({
      quality: {
        height: 1080,
        width: 1920,
        bitrate: 5000000,
      },
    });

    mockStore.subscribe.mockImplementation((callback) => {
      setTimeout(() => callback(), 0);
      return vi.fn();
    });

    const { container } = render(<QualityIndicator />);

    // Should be visible initially
    await waitFor(() => {
      expect(container.firstChild).toBeInTheDocument();
    });

    // Fast forward 3 seconds
    vi.advanceTimersByTime(3000);

    await waitFor(() => {
      expect(container.firstChild).toBeNull();
    });

    vi.useRealTimers();
  });

  it('applies custom className prop', async () => {
    mockStore.getState.mockReturnValue({
      quality: {
        height: 1080,
        width: 1920,
        bitrate: 5000000,
      },
    });

    mockStore.subscribe.mockImplementation((callback) => {
      setTimeout(() => callback(), 0);
      return vi.fn();
    });

    const { container } = render(<QualityIndicator className="custom-class" />);

    await waitFor(() => {
      const indicator = container.firstChild;
      expect(indicator).toHaveClass('custom-class');
    });
  });

  it('cleans up subscription on unmount', () => {
    const unsubscribe = vi.fn();
    mockStore.subscribe.mockReturnValue(unsubscribe);

    const { unmount } = render(<QualityIndicator />);
    
    expect(mockStore.subscribe).toHaveBeenCalled();
    
    unmount();
    
    expect(unsubscribe).toHaveBeenCalled();
  });
});