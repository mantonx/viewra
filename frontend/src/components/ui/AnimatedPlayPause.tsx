import React from 'react';
import { cn } from '@/lib/utils';

interface AnimatedPlayPauseProps {
  isPlaying: boolean;
  size?: number;
  className?: string;
}

const AnimatedPlayPause: React.FC<AnimatedPlayPauseProps> = ({
  isPlaying,
  size = 18,
  className,
}) => {
  return (
    <div
      className={cn('relative flex items-center justify-center', className)}
      style={{ width: size, height: size }}
    >
      <svg
        width={size}
        height={size}
        viewBox="0 0 24 24"
        fill="currentColor"
        className="transition-all duration-300 ease-in-out"
      >
        {/* Play icon (triangle) */}
        <path
          d="M8 5v14l11-7z"
          className={cn(
            'transition-all duration-300 ease-in-out origin-center',
            isPlaying ? 'opacity-0 scale-75 translate-x-1' : 'opacity-100 scale-100 translate-x-0'
          )}
          style={{
            transformOrigin: 'center',
          }}
        />

        {/* Pause icon (two rectangles) */}
        <g
          className={cn(
            'transition-all duration-300 ease-in-out origin-center',
            isPlaying ? 'opacity-100 scale-100 translate-x-0' : 'opacity-0 scale-75 translate-x-1'
          )}
          style={{
            transformOrigin: 'center',
          }}
        >
          <rect x="6" y="4" width="4" height="16" />
          <rect x="14" y="4" width="4" height="16" />
        </g>
      </svg>
    </div>
  );
};

export default AnimatedPlayPause;
