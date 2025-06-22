import React, { useState, useRef, useCallback, type MouseEvent } from 'react';
import { cn } from '@/utils/cn';
import { formatTime, formatProgress, isValidTime } from '@/utils/time';
import type { ProgressBarProps } from './types';

export const ProgressBar: React.FC<ProgressBarProps> = ({
  currentTime,
  duration,
  bufferedRanges,
  isSeekable,
  isSeekingAhead,
  onSeek,
  onSeekIntent,
  className,
  showTooltip = true,
  showBuffered = true,
  showSeekAheadIndicator = true,
}) => {
  const [hoverTime, setHoverTime] = useState<number | null>(null);
  const [isDragging, setIsDragging] = useState(false);
  const progressRef = useRef<HTMLDivElement>(null);

  const getProgressFromEvent = useCallback((e: MouseEvent<HTMLDivElement>): number => {
    if (!progressRef.current) return 0;
    
    const rect = progressRef.current.getBoundingClientRect();
    const x = e.clientX - rect.left;
    const progress = Math.max(0, Math.min(1, x / rect.width));
    
    return progress;
  }, []);

  const handleMouseMove = useCallback((e: MouseEvent<HTMLDivElement>) => {
    if (!isSeekable) return;
    
    const progress = getProgressFromEvent(e);
    const time = progress * duration;
    
    if (isValidTime(time)) {
      setHoverTime(time);
      
      if (onSeekIntent && !isDragging) {
        onSeekIntent(time);
      }
    }
  }, [duration, getProgressFromEvent, isSeekable, onSeekIntent, isDragging]);

  const handleMouseLeave = useCallback(() => {
    if (!isDragging) {
      setHoverTime(null);
    }
  }, [isDragging]);

  const handleClick = useCallback((e: MouseEvent<HTMLDivElement>) => {
    if (!isSeekable) return;
    
    const progress = getProgressFromEvent(e);
    onSeek(progress);
  }, [getProgressFromEvent, isSeekable, onSeek]);

  const handleMouseDown = useCallback(() => {
    if (!isSeekable) return;
    
    setIsDragging(true);
    
    const handleMouseMove = (e: globalThis.MouseEvent) => {
      if (!progressRef.current) return;
      
      const rect = progressRef.current.getBoundingClientRect();
      const x = e.clientX - rect.left;
      const progress = Math.max(0, Math.min(1, x / rect.width));
      
      onSeek(progress);
    };
    
    const handleMouseUp = () => {
      setIsDragging(false);
      document.removeEventListener('mousemove', handleMouseMove);
      document.removeEventListener('mouseup', handleMouseUp);
    };
    
    document.addEventListener('mousemove', handleMouseMove);
    document.addEventListener('mouseup', handleMouseUp);
  }, [isSeekable, onSeek]);

  const progress = duration > 0 ? formatProgress(currentTime, duration) : 0;
  
  // Debug log
  if (typeof window !== 'undefined' && (window as any).DEBUG_PROGRESS) {
    console.log('ProgressBar debug:', { currentTime, duration, progress, isSeekable });
  }



  const getMaxBufferedEnd = (): number => {
    if (bufferedRanges.length === 0) return 0;
    return Math.max(...bufferedRanges.map(range => range.end));
  };

  const showSeekAheadForHover = hoverTime !== null && 
    hoverTime > getMaxBufferedEnd() && 
    showSeekAheadIndicator;

  return (
    <div className={cn('relative group', className)}>
      {/* Hover tooltip */}
      {showTooltip && hoverTime !== null && (
        <div
          className="absolute -top-8 transform -translate-x-1/2 bg-player-controls-bg/90 text-player-text px-2 py-1 rounded text-sm pointer-events-none z-10 backdrop-blur-sm"
          style={{ left: `${(hoverTime / duration) * 100}%` }}
        >
          {formatTime(hoverTime)}
          {showSeekAheadForHover && (
            <span className="ml-1 text-player-progress-hover">⚡</span>
          )}
        </div>
      )}
      
      {/* Main progress bar */}
      <div 
        ref={progressRef}
        className={cn(
          "w-full h-3 bg-player-progress-bg rounded-full cursor-pointer relative transition-all duration-normal",
          "hover:h-4",
          !isSeekable && "cursor-not-allowed opacity-50",
          isDragging && "h-4"
        )}
        style={{ overflow: 'visible' }}
        onMouseMove={handleMouseMove}
        onMouseLeave={handleMouseLeave}
        onClick={handleClick}
        onMouseDown={handleMouseDown}
      >
        {/* Background track */}
        <div 
          className="absolute inset-0 rounded-full"
          style={{ backgroundColor: 'rgba(75, 85, 99, 0.5)' }}
        ></div>
        
        {/* Buffered ranges */}
        {showBuffered && bufferedRanges.map((range, index) => (
          <div
            key={index}
            className="absolute top-0 h-full rounded-full"
            style={{ 
              backgroundColor: 'rgba(156, 163, 175, 0.5)',
              left: `${(range.start / duration) * 100}%`,
              width: `${((range.end - range.start) / duration) * 100}%`
            }}
            title="Buffered content"
          />
        ))}
        
        {/* Seek-ahead indicator */}
        {showSeekAheadIndicator && duration > 0 && (
          <div
            className="absolute top-0 bg-player-progress-hover/40 h-full rounded-full"
            style={{ 
              left: `${(getMaxBufferedEnd() / duration) * 100}%`,
              width: `${((duration - getMaxBufferedEnd()) / duration) * 100}%`
            }}
            title="Seek-ahead available"
          />
        )}
        
        {/* Current progress */}
        <div
          className="absolute top-0 left-0 h-full rounded-full transition-all duration-fast"
          style={{ 
            width: `${progress}%`,
            backgroundColor: 'rgb(239, 68, 68)' // red-500
          }}
        />
        
        {/* Progress handle */}
        {isSeekable && (
          <div
            className={cn(
              "absolute top-1/2 w-4 h-4 rounded-full -translate-y-1/2 -translate-x-1/2 transition-all duration-fast z-20",
              hoverTime !== null || isDragging ? "scale-125" : "scale-100"
            )}
            style={{ 
              left: `${progress}%`,
              backgroundColor: '#ffffff',
              border: '2px solid rgb(239, 68, 68)',
              boxShadow: '0 2px 4px rgba(0,0,0,0.2)'
            }}
          />
        )}
      </div>
      
      {/* Loading indicator */}
      {isSeekingAhead && (
        <div className="absolute inset-0 flex items-center justify-center">
          <div className="bg-player-controls-bg/80 px-2 py-1 rounded text-xs text-player-text animate-pulse backdrop-blur-sm">
            Seeking...
          </div>
        </div>
      )}
    </div>
  );
};