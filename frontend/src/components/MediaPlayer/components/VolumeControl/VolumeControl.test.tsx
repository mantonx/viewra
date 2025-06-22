import { describe, it, expect, vi } from 'vitest';
import { render, fireEvent } from '@/test/utils';
import { VolumeControl } from './VolumeControl';

describe('VolumeControl', () => {
  const defaultProps = {
    volume: 0.8,
    isMuted: false,
    onVolumeChange: vi.fn(),
    onToggleMute: vi.fn(),
  };

  it('renders without crashing', () => {
    const { container } = render(<VolumeControl {...defaultProps} />);
    expect(container).toBeTruthy();
  });

  it('displays the correct volume icon when not muted', () => {
    const { container } = render(<VolumeControl {...defaultProps} />);
    const volumeIcon = container.querySelector('[title="Mute"]');
    expect(volumeIcon).toBeTruthy();
  });

  it('displays the muted icon when muted', () => {
    const { container } = render(
      <VolumeControl {...defaultProps} isMuted={true} />
    );
    const muteIcon = container.querySelector('[title="Unmute"]');
    expect(muteIcon).toBeTruthy();
  });

  it('calls onToggleMute when icon is clicked', () => {
    const onToggleMute = vi.fn();
    const { container } = render(
      <VolumeControl {...defaultProps} onToggleMute={onToggleMute} />
    );
    
    const button = container.querySelector('button');
    fireEvent.click(button!);
    
    expect(onToggleMute).toHaveBeenCalled();
  });

  it('shows volume slider on hover', () => {
    const { container } = render(<VolumeControl {...defaultProps} />);
    const controlContainer = container.firstChild as HTMLElement;
    
    // Initially slider should be hidden (width: 0)
    const slider = container.querySelector('input[type="range"]');
    expect(slider).toBeTruthy();
    
    // Hover should expand the slider
    fireEvent.mouseEnter(controlContainer);
    
    const sliderContainer = slider?.parentElement;
    expect(sliderContainer?.classList.contains('w-20')).toBe(true);
  });

  it('calls onVolumeChange with correct value when slider changes', () => {
    const onVolumeChange = vi.fn();
    const { container } = render(
      <VolumeControl {...defaultProps} onVolumeChange={onVolumeChange} />
    );
    
    const slider = container.querySelector('input[type="range"]');
    fireEvent.change(slider!, { target: { value: '0.5' } });
    
    expect(onVolumeChange).toHaveBeenCalledWith(0.5);
  });

  it('shows 0 volume when muted regardless of actual volume', () => {
    const { container } = render(
      <VolumeControl {...defaultProps} volume={0.8} isMuted={true} />
    );
    
    const slider = container.querySelector('input[type="range"]') as HTMLInputElement;
    expect(slider.value).toBe('0');
  });

  it('clamps volume values between 0 and 1', () => {
    const onVolumeChange = vi.fn();
    const { container } = render(
      <VolumeControl {...defaultProps} onVolumeChange={onVolumeChange} />
    );
    
    const slider = container.querySelector('input[type="range"]');
    
    // Try to set volume above 1
    fireEvent.change(slider!, { target: { value: '1.5' } });
    expect(onVolumeChange).toHaveBeenCalledWith(1);
    
    // Try to set volume below 0
    fireEvent.change(slider!, { target: { value: '-0.5' } });
    expect(onVolumeChange).toHaveBeenCalledWith(0);
  });

  it('hides slider when showSlider is false', () => {
    const { container } = render(
      <VolumeControl {...defaultProps} showSlider={false} />
    );
    
    const slider = container.querySelector('input[type="range"]');
    expect(slider).toBeFalsy();
  });

  it('renders in vertical mode when specified', () => {
    const { container } = render(
      <VolumeControl {...defaultProps} vertical={true} />
    );
    
    const controlContainer = container.firstChild as HTMLElement;
    expect(controlContainer.classList.contains('flex-col')).toBe(true);
    expect(controlContainer.classList.contains('space-y-2')).toBe(true);
  });
});