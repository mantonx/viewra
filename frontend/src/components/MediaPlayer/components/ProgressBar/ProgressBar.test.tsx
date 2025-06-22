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
});