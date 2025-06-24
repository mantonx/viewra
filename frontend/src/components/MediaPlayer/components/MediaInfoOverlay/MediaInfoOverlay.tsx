import React, { useState, useEffect } from 'react';
import { cn } from '@/utils/cn';
import { isEpisode } from '@/utils/mediaValidation';
import type { MediaInfoOverlayProps } from './MediaInfoOverlay.types';

export const MediaInfoOverlay: React.FC<MediaInfoOverlayProps> = ({
  media,
  className,
  position = 'top-left',
  showOnHover = false,
  autoHide = true,
  autoHideDelay = 5000,
}) => {
  const [isVisible, setIsVisible] = useState(!autoHide);

  useEffect(() => {
    if (autoHide && media) {
      setIsVisible(true);
      const timer = setTimeout(() => {
        setIsVisible(false);
      }, autoHideDelay);
      
      return () => clearTimeout(timer);
    }
  }, [media, autoHide, autoHideDelay]);

  if (!media) return null;

  const positionClasses = {
    'top-left': 'top-4 left-4',
    'top-right': 'top-4 right-4',
    'bottom-left': 'bottom-4 left-4',
    'bottom-right': 'bottom-4 right-4',
  };

  return (
    <div
      className={cn(
        'absolute z-20 bg-slate-800/70 text-white p-4 rounded-lg backdrop-blur-sm transition-opacity duration-500',
        positionClasses[position],
        isVisible ? 'opacity-100' : 'opacity-0',
        showOnHover && 'opacity-0 hover:opacity-100',
        className
      )}
    >
      {isEpisode(media) ? (
        <>
          <h3 className="text-lg font-semibold">{media.series.title}</h3>
          <p className="text-sm text-gray-300">
            Season {media.season_number}, Episode {media.episode_number}
          </p>
          <h4 className="text-md font-medium mt-1">{media.title}</h4>
          {media.description && (
            <p className="text-xs text-gray-400 mt-2 line-clamp-2">{media.description}</p>
          )}
        </>
      ) : (
        <>
          <h3 className="text-lg font-semibold">{media.title}</h3>
          {media.release_date && (
            <p className="text-sm text-gray-300">
              {new Date(media.release_date).getFullYear()}
            </p>
          )}
          {media.description && (
            <p className="text-xs text-gray-400 mt-2 line-clamp-2">{media.description}</p>
          )}
        </>
      )}
    </div>
  );
};