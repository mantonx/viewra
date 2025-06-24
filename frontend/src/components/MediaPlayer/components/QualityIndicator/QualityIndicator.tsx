import React, { useEffect, useState } from 'react';
import { useMediaState } from '@vidstack/react';
import { cn } from '@/utils/cn';
import type { QualityInfo, QualityIndicatorProps } from './QualityIndicator.types';

export const QualityIndicator: React.FC<QualityIndicatorProps> = ({ className }) => {
  const quality = useMediaState('quality');
  const qualities = useMediaState('qualities');
  
  const [currentQuality, setCurrentQuality] = useState<QualityInfo | null>(null);
  const [isUpgrading, setIsUpgrading] = useState(false);
  const [showIndicator, setShowIndicator] = useState(false);

  useEffect(() => {
    if (!quality) return;

    let hideTimeout: NodeJS.Timeout;
    
    // Extract quality information from Vidstack quality object
    const newQuality: QualityInfo = {
      height: quality.height || 0,
      width: quality.width || 0,
      bandwidth: quality.bitrate || quality.bandwidth || 0,
      label: getQualityLabel(quality.height || 0),
    };

    // Check if quality changed
    if (!currentQuality || currentQuality.height !== newQuality.height) {
      setIsUpgrading(currentQuality ? newQuality.height > currentQuality.height : false);
      setCurrentQuality(newQuality);
      setShowIndicator(true);

      // Hide indicator after 3 seconds
      clearTimeout(hideTimeout);
      hideTimeout = setTimeout(() => {
        setShowIndicator(false);
      }, 3000);
    }

    return () => {
      clearTimeout(hideTimeout);
    };
  }, [quality, currentQuality]);

  const getQualityLabel = (height: number): string => {
    if (height >= 2160) return '4K';
    if (height >= 1080) return '1080p';
    if (height >= 720) return '720p';
    if (height >= 480) return '480p';
    if (height >= 360) return '360p';
    if (height >= 240) return '240p';
    return 'Auto';
  };

  const getQualityColor = (height: number): string => {
    if (height >= 1080) return '#4ade80'; // green-400
    if (height >= 720) return '#facc15';  // yellow-400
    if (height >= 480) return '#fb923c';  // orange-400
    return '#f87171';                      // red-400
  };

  if (!currentQuality || !showIndicator) {
    return null;
  }

  return (
    <div className={cn(
      "absolute top-5 right-5 flex flex-col items-end gap-2 pointer-events-none z-50",
      "animate-fadeIn",
      className
    )}>
      <div 
        className={cn(
          "flex items-center gap-1.5 px-3 py-1.5 bg-black/80 border-2 rounded-md backdrop-blur-sm",
          "transition-all duration-300",
          isUpgrading && "animate-pulse"
        )}
        style={{ borderColor: getQualityColor(currentQuality.height) }}
      >
        <span className="text-sm font-semibold text-white tracking-wider">
          {currentQuality.label}
        </span>
        {isUpgrading && (
          <svg className="w-4 h-4 text-green-400 animate-slideUp" viewBox="0 0 20 20" fill="currentColor">
            <path fillRule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-8.293l-3-3a1 1 0 00-1.414 0l-3 3a1 1 0 001.414 1.414L9 9.414V13a1 1 0 102 0V9.414l1.293 1.293a1 1 0 001.414-1.414z" clipRule="evenodd" />
          </svg>
        )}
      </div>
      <div className="text-xs text-white/70 bg-black/60 px-2 py-1 rounded backdrop-blur-sm">
        {(currentQuality.bandwidth / 1000000).toFixed(1)} Mbps
      </div>
    </div>
  );
};