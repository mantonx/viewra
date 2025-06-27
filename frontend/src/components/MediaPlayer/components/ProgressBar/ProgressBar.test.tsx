import { describe, it, expect, vi } from 'vitest';
import { render, fireEvent } from '@/test/utils';
import { ProgressBar } from './ProgressBar';

describe('ProgressBar', () => {
  const defaultProps = {
    currentTime: 30,
    duration: 120,
    bufferedRanges: [],
    isSeekable: true,
    isSeekingAhead: false,
    onSeek: vi.fn(),
    onSeekIntent: vi.fn(),
  };

  it('renders without crashing', () => {
    const { container } = render(<ProgressBar {...defaultProps} />);
    expect(container).toBeTruthy();
  });

  it('displays correct progress percentage', () => {
    const { container } = render(<ProgressBar {...defaultProps} />);
    const progressBar = container.querySelector('[style*="width: 25%"]');
    expect(progressBar).toBeTruthy();
  });

  it('shows tooltip on hover with correct time', () => {
    const { container, getByText } = render(<ProgressBar {...defaultProps} />);
    const progressContainer = container.querySelector('.w-full.h-3');
    
    // Mock getBoundingClientRect
    progressContainer!.getBoundingClientRect = vi.fn(() => ({
      left: 0,
      right: 400,
      top: 0,
      bottom: 20,
      width: 400,
      height: 20,
      x: 0,
      y: 0,
      toJSON: () => {},
    }));
    
    fireEvent.mouseMove(progressContainer!, {
      clientX: 100,
    });

    // Should show time at hover position (25% of 400px = 100px, 25% of 120s = 30s)
    expect(getByText('0:30')).toBeTruthy();
  });

  it('calls onSeek when clicked', () => {
    const onSeek = vi.fn();
    const { container } = render(
      <ProgressBar {...defaultProps} onSeek={onSeek} />
    );
    const progressContainer = container.querySelector('.w-full.h-3');
    
    // Mock getBoundingClientRect
    progressContainer!.getBoundingClientRect = vi.fn(() => ({
      left: 0,
      right: 400,
      top: 0,
      bottom: 20,
      width: 400,
      height: 20,
      x: 0,
      y: 0,
      toJSON: () => {},
    }));
    
    fireEvent.click(progressContainer!, {
      clientX: 200,
    });

    expect(onSeek).toHaveBeenCalledWith(0.5); // 200/400 = 0.5
  });

  it('shows buffered ranges', () => {
    const bufferedRanges = [
      { start: 0, end: 60 },
      { start: 80, end: 100 },
    ];
    const { container } = render(
      <ProgressBar {...defaultProps} bufferedRanges={bufferedRanges} />
    );
    
    const bufferedElements = container.querySelectorAll('[title="Buffered content"]');
    expect(bufferedElements).toHaveLength(2);
  });

  it('shows seeking indicator when isSeekingAhead is true', () => {
    const { getByText } = render(
      <ProgressBar {...defaultProps} isSeekingAhead={true} />
    );
    
    expect(getByText('Seeking...')).toBeTruthy();
  });

  it('disables interaction when not seekable', () => {
    const onSeek = vi.fn();
    const { container } = render(
      <ProgressBar {...defaultProps} isSeekable={false} onSeek={onSeek} />
    );
    const progressContainer = container.querySelector('.w-full.h-3');
    
    expect(progressContainer?.classList.contains('cursor-not-allowed')).toBe(true);
    expect(progressContainer?.classList.contains('opacity-50')).toBe(true);
    
    fireEvent.click(progressContainer!);
    expect(onSeek).not.toHaveBeenCalled();
  });

  it('handles edge cases for progress calculation', () => {
    // Test with 0 duration
    const { container } = render(
      <ProgressBar {...defaultProps} duration={0} />
    );
    const progressBar = container.querySelector('[style*="width: 0%"]');
    expect(progressBar).toBeTruthy();
  });

  it('calls onSeekIntent during hover when provided', () => {
    const onSeekIntent = vi.fn();
    const { container } = render(
      <ProgressBar {...defaultProps} onSeekIntent={onSeekIntent} />
    );
    const progressContainer = container.querySelector('.w-full.h-3');
    
    // Mock getBoundingClientRect
    progressContainer!.getBoundingClientRect = vi.fn(() => ({
      left: 0,
      right: 400,
      top: 0,
      bottom: 20,
      width: 400,
      height: 20,
      x: 0,
      y: 0,
      toJSON: () => {},
    }));
    
    fireEvent.mouseMove(progressContainer!, {
      clientX: 100,
    });

    expect(onSeekIntent).toHaveBeenCalledWith(30); // 25% of 120s
  });

  it('shows seek-ahead indicator for unbuffered areas', () => {
    const bufferedRanges = [
      { start: 0, end: 60 }, // First 60 seconds buffered
    ];
    const { container } = render(
      <ProgressBar
        {...defaultProps}
        bufferedRanges={bufferedRanges}
        showSeekAheadIndicator={true}
      />
    );
    
    // Check for seek-ahead indicator area
    const seekAheadArea = container.querySelector('[title="Seek-ahead available"]');
    expect(seekAheadArea).toBeTruthy();
    
    // It should cover the unbuffered portion (60s to 120s = 50% of width)
    expect(seekAheadArea?.getAttribute('style')).toContain('left: 50%');
    expect(seekAheadArea?.getAttribute('style')).toContain('width: 50%');
  });

  it('shows lightning bolt icon when hovering over unbuffered area', () => {
    const bufferedRanges = [
      { start: 0, end: 30 }, // First 30 seconds buffered
    ];
    const { container } = render(
      <ProgressBar
        {...defaultProps}
        bufferedRanges={bufferedRanges}
        showSeekAheadIndicator={true}
      />
    );
    const progressContainer = container.querySelector('.w-full.h-3');
    
    // Mock getBoundingClientRect
    progressContainer!.getBoundingClientRect = vi.fn(() => ({
      left: 0,
      right: 400,
      top: 0,
      bottom: 20,
      width: 400,
      height: 20,
      x: 0,
      y: 0,
      toJSON: () => {},
    }));
    
    // Hover over unbuffered area (75% = 90s, which is beyond buffered 30s)
    fireEvent.mouseMove(progressContainer!, {
      clientX: 300, // 75% of 400px
    });

    // Should show time tooltip with lightning bolt
    const tooltip = container.querySelector('.absolute.-top-8');
    expect(tooltip?.textContent).toContain('⚡');
  });

  it('handles dragging to seek', () => {
    const onSeek = vi.fn();
    const { container } = render(
      <ProgressBar {...defaultProps} onSeek={onSeek} />
    );
    const progressContainer = container.querySelector('.w-full.h-3');
    
    // Mock getBoundingClientRect
    progressContainer!.getBoundingClientRect = vi.fn(() => ({
      left: 0,
      right: 400,
      top: 0,
      bottom: 20,
      width: 400,
      height: 20,
      x: 0,
      y: 0,
      toJSON: () => {},
    }));
    
    // Start dragging
    fireEvent.mouseDown(progressContainer!);
    
    // Move mouse during drag
    fireEvent.mouseMove(document, {
      clientX: 200, // 50% position
    });
    
    // Since we're using document event listeners, we need to check if seek was called
    // during the drag (implementation may vary)
    
    // End dragging
    fireEvent.mouseUp(document);
    
    // onSeek should have been called at some point during or after the drag
    expect(onSeek).toHaveBeenCalled();
  });

  it('shows proper handle scaling on hover and drag', () => {
    const { container } = render(<ProgressBar {...defaultProps} />);
    const progressContainer = container.querySelector('.w-full.h-3');
    
    // Mock getBoundingClientRect
    progressContainer!.getBoundingClientRect = vi.fn(() => ({
      left: 0,
      right: 400,
      top: 0,
      bottom: 20,
      width: 400,
      height: 20,
      x: 0,
      y: 0,
      toJSON: () => {},
    }));
    
    // Initially, handle should be normal size
    let handle = container.querySelector('.w-4.h-4.rounded-full');
    expect(handle?.classList.contains('scale-100')).toBe(true);
    
    // On hover, handle should scale up
    fireEvent.mouseMove(progressContainer!, {
      clientX: 100,
    });
    
    handle = container.querySelector('.w-4.h-4.rounded-full');
    expect(handle?.classList.contains('scale-125')).toBe(true);
  });
});