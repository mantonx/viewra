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
        className="text-white hover:text-blue-400 hover:scale-110 transition-all duration-200 p-2 rounded-full hover:bg-white/10"
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
              'h-1 rounded-lg appearance-none cursor-pointer transition-colors duration-200',
              vertical && 'transform -rotate-90 origin-center w-20',
              !vertical && 'w-20',
              '[&::-webkit-slider-thumb]:appearance-none',
              '[&::-webkit-slider-thumb]:w-3',
              '[&::-webkit-slider-thumb]:h-3',
              '[&::-webkit-slider-thumb]:rounded-full',
              '[&::-webkit-slider-thumb]:bg-blue-500'
            )}
            style={{
              background: `linear-gradient(to right, rgb(59, 130, 246) 0%, rgb(59, 130, 246) ${displayVolume * 100}%, rgb(107, 114, 128) ${displayVolume * 100}%, rgb(107, 114, 128) 100%)`
            }}
            title={`Volume: ${Math.round(displayVolume * 100)}%`}
          />
        </div>
      )}
    </div>
  );
};