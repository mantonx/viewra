import React, { useState, useRef, useEffect } from 'react';
import { useMediaState, useMediaRemote } from '@vidstack/react';
import { Check, Settings } from 'lucide-react';
import { cn } from '@/utils/cn';

/**
 * Quality selector for adaptive bitrate streaming (HLS/DASH).
 * 
 * NOTE: This component is currently not used because we're using direct MP4 streaming
 * which doesn't support multiple quality levels. It's kept for future use when we
 * implement HLS/DASH streaming with transcoding.
 * 
 * When using this component, it must be rendered inside the <MediaPlayer> component
 * or passed a playerRef to avoid the "useMediaState requires RefObject" warning.
 */

interface QualitySelectorProps {
  className?: string;
}

export const QualitySelector: React.FC<QualitySelectorProps> = ({ className }) => {
  const remote = useMediaRemote();
  const qualities = useMediaState('qualities');
  const quality = useMediaState('quality');
  const autoQuality = useMediaState('autoQuality');
  
  const [isOpen, setIsOpen] = useState(false);
  const menuRef = useRef<HTMLDivElement>(null);

  // Close menu when clicking outside
  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      if (menuRef.current && !menuRef.current.contains(event.target as Node)) {
        setIsOpen(false);
      }
    };

    if (isOpen) {
      document.addEventListener('mousedown', handleClickOutside);
      return () => document.removeEventListener('mousedown', handleClickOutside);
    }
  }, [isOpen]);

  const handleQualityChange = (selectedQuality: any | null) => {
    if (!remote) return;
    
    if (selectedQuality === null) {
      // Enable auto quality
      remote.changeQuality(-1); // -1 typically means auto in Vidstack
    } else {
      // Select specific quality
      const qualityIndex = qualities.indexOf(selectedQuality);
      if (qualityIndex !== -1) {
        remote.changeQuality(qualityIndex);
      }
    }
    
    setIsOpen(false);
  };

  const getQualityLabel = (qualityItem: any): string => {
    if (!qualityItem) return 'Auto';
    const height = qualityItem.height || 0;
    
    if (height >= 2160) return '4K';
    if (height >= 1080) return '1080p HD';
    if (height >= 720) return '720p HD';
    if (height >= 480) return '480p';
    if (height >= 360) return '360p';
    if (height >= 240) return '240p';
    
    return `${height}p`;
  };

  const getCurrentQualityLabel = (): string => {
    if (autoQuality || !quality) return 'Auto';
    return getQualityLabel(quality);
  };

  // Don't render if no qualities available
  if (!qualities || qualities.length <= 1) {
    return null;
  }

  return (
    <div ref={menuRef} className={cn("relative", className)}>
      <button
        onClick={() => setIsOpen(!isOpen)}
        className={cn(
          "flex items-center gap-2 px-3 py-1.5 rounded",
          "bg-white/10 hover:bg-white/20 backdrop-blur-sm",
          "text-white text-sm font-medium",
          "transition-all duration-200",
          "border border-white/20"
        )}
        aria-label="Quality settings"
      >
        <Settings className="w-4 h-4" />
        <span>{getCurrentQualityLabel()}</span>
      </button>

      {isOpen && (
        <div className={cn(
          "absolute bottom-full right-0 mb-2",
          "bg-black/90 backdrop-blur-md rounded-lg",
          "border border-white/20 shadow-xl",
          "py-2 min-w-[160px]",
          "animate-fadeIn"
        )}>
          {/* Auto quality option */}
          <button
            onClick={() => handleQualityChange(null)}
            className={cn(
              "w-full px-4 py-2 text-left",
              "flex items-center justify-between",
              "hover:bg-white/10 transition-colors",
              "text-sm text-white"
            )}
          >
            <span className="flex items-center gap-2">
              <span className={cn(
                "font-medium",
                autoQuality && "text-green-400"
              )}>
                Auto
              </span>
              {autoQuality && quality && (
                <span className="text-xs text-gray-400">
                  ({getQualityLabel(quality)})
                </span>
              )}
            </span>
            {autoQuality && <Check className="w-4 h-4 text-green-400" />}
          </button>

          <div className="border-t border-white/10 my-1" />

          {/* Manual quality options */}
          {qualities
            .slice()
            .reverse() // Show highest quality first
            .map((q, index) => {
              const isSelected = !autoQuality && quality === q;
              const label = getQualityLabel(q);
              const bitrate = q.bitrate || q.bandwidth;
              
              return (
                <button
                  key={index}
                  onClick={() => handleQualityChange(q)}
                  className={cn(
                    "w-full px-4 py-2 text-left",
                    "flex items-center justify-between",
                    "hover:bg-white/10 transition-colors",
                    "text-sm text-white"
                  )}
                >
                  <span className="flex flex-col">
                    <span className={cn(
                      "font-medium",
                      isSelected && "text-green-400"
                    )}>
                      {label}
                    </span>
                    {bitrate && (
                      <span className="text-xs text-gray-400">
                        {(bitrate / 1000000).toFixed(1)} Mbps
                      </span>
                    )}
                  </span>
                  {isSelected && <Check className="w-4 h-4 text-green-400" />}
                </button>
              );
            })}
        </div>
      )}
    </div>
  );
};