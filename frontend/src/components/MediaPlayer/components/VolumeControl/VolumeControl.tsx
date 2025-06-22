import React, { useState, useCallback } from 'react';
import { Volume2, VolumeX } from 'lucide-react';
import { cn } from '@/utils/cn';
import { clampTime } from '@/utils/time';
import type { VolumeControlProps } from './types';

export const VolumeControl: React.FC<VolumeControlProps> = ({
  volume,
  isMuted,
  onVolumeChange,
  onToggleMute,
  className,
  showSlider = true,
  vertical = false,
}) => {
  const [isHovering, setIsHovering] = useState(false);

  const handleVolumeChange = useCallback((e: React.ChangeEvent<HTMLInputElement>) => {
    const newVolume = parseFloat(e.target.value);
    const clampedVolume = clampTime(newVolume, 1);
    onVolumeChange(clampedVolume);
  }, [onVolumeChange]);

  const displayVolume = isMuted ? 0 : volume;

  return (
    <div 
      className={cn(
        'flex items-center space-x-2',
        vertical && 'flex-col space-x-0 space-y-2',
        className
      )}
      onMouseEnter={() => setIsHovering(true)}
      onMouseLeave={() => setIsHovering(false)}
    >
      <button
        onClick={onToggleMute}
        className="text-player-text hover:text-primary hover:scale-110 transition-all duration-normal p-2 rounded-full hover:bg-player-text/10"
        title={isMuted ? 'Unmute' : 'Mute'}
      >
        {isMuted ? (
          <VolumeX className="w-6 h-6" />
        ) : (
          <Volume2 className="w-6 h-6" />
        )}
      </button>
      
      {showSlider && (
        <div className={cn(
          'overflow-hidden transition-all duration-200',
          vertical ? 'h-20' : 'w-0',
          isHovering && (vertical ? 'h-20' : 'w-20')
        )}>
          <input
            type="range"
            min="0"
            max="1"
            step="0.05"
            value={displayVolume}
            onChange={handleVolumeChange}
            className={cn(
              'h-1 rounded-lg appearance-none cursor-pointer transition-colors duration-normal',
              vertical && 'transform -rotate-90 origin-center w-20',
              !vertical && 'w-20',
              '[&::-webkit-slider-thumb]:appearance-none',
              '[&::-webkit-slider-thumb]:w-3',
              '[&::-webkit-slider-thumb]:h-3',
              '[&::-webkit-slider-thumb]:rounded-full',
              '[&::-webkit-slider-thumb]:bg-player-progress-played'
            )}
            style={{
              background: `linear-gradient(to right, rgb(var(--color-player-progress-played)) 0%, rgb(var(--color-player-progress-played)) ${displayVolume * 100}%, rgb(var(--color-player-progress-bg)) ${displayVolume * 100}%, rgb(var(--color-player-progress-bg)) 100%)`
            }}
            title={`Volume: ${Math.round(displayVolume * 100)}%`}
          />
        </div>
      )}
    </div>
  );
};